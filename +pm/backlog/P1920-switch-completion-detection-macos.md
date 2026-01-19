# P1920: Switch Completion Not Detected on macOS

**Priority**: P1 (Critical - Core Functionality Broken)
**Status**: Implemented (Ready for Testing)
**Created**: 2026-01-19
**Implemented**: 2026-01-19
**Type**: Bug
**Supersedes**: P1910 (UI stuck - symptom), P8820 (stale pending - symptom)
**References**: P2800 (Command State Machine), CORE-004 (State Sync)

---

## Summary

On macOS, switch completion is never detected. Agent is killed by `launchctl bootout` BEFORE sending completion status. Dashboard never enters `AWAITING_RECONNECT`, UI stuck forever.

**Symptoms**:

- System compartment keeps blinking blue indefinitely
- Log panel shows yellow dot blinking
- "Building" badge persists
- Host shows `pending_command=switch` forever
- Tests blocked (can't run while "busy")
- Hard refresh (Cmd+Shift+R) doesn't help

---

## Root Cause

### The macOS Kill Race

```
1. Agent starts `home-manager switch` in new session (Setsid)
2. home-manager runs `launchctl bootout` → kills agent
3. ❌ Agent dies BEFORE sendStatus() at line 334
4. Switch continues in background (new session survives)
5. Switch completes, launchd restarts agent
6. New agent reconnects
7. Dashboard never received TypeStatus → doesn't enter AWAITING_RECONNECT
8. HandleAgentReconnect() never called
9. pending_command stuck, UI stuck, tests blocked
```

### Evidence from Logs

```
1:43PM INF Executing switch host=mba-imac-work
1:43PM DBG command sent command=switch host=mba-imac-work
1:43PM DBG agent reports pending command agent_pending=switch
... (many heartbeats with pending_command=switch) ...
1:45PM DBG client unregistered id=mba-imac-work  ← Agent killed
1:45PM INF agent registered hostname=mba-imac-work  ← New agent
... (heartbeats continue, still showing pending_command=switch) ...
```

**Missing**: `command status command=switch exit_code=0` (never sent!)

### Code Location

`src/internal/agent/commands.go:334-355`:

```go
a.sendStatus(status, command, exitCode, message)  // Line 334 - NEVER REACHED

// P7100: Send immediate heartbeat
a.sendHeartbeat()  // Line 338 - NEVER REACHED

// Auto-restart after successful switch
if exitCode == 0 && (command == "switch" || command == "pull-switch") {
    time.Sleep(500 * time.Millisecond)  // Line 348 - NEVER REACHED
    a.Shutdown()
    os.Exit(0)  // Line 351 - NEVER REACHED (already dead)
}
```

Agent is killed during `runWithStreaming()` (line 359+), never reaches completion code.

---

## Why NixOS Works

On NixOS:

1. `nixos-rebuild switch` doesn't kill the agent mid-command
2. Agent completes `runWithStreaming()`, returns exit code
3. Agent sends `TypeStatus` at line 334 ✅
4. Agent restarts via `os.Exit(101)` at line 353
5. Dashboard receives status → enters `AWAITING_RECONNECT` ✅
6. New agent reconnects → `HandleAgentReconnect()` → `completeWithSuccess()` ✅

---

## Fix Strategy (Recommended)

### Option E: Detect Disconnect During Switch

When agent disconnects while executing switch, infer completion and enter `AWAITING_RECONNECT`:

**Location**: `src/internal/dashboard/hub.go` in `handleClientUnregister()`

```go
func (h *Hub) handleClientUnregister(client *Client, shouldNotify bool) {
    // ... existing code ...

    // P1920: Detect disconnect during switch execution (macOS kill race)
    if h.lifecycleManager != nil && hostID != "" {
        key := h.hostKey(hostID)
        cmd := h.lifecycleManager.GetActiveCommand(key)

        // If agent was executing switch and disconnected, assume completion
        if cmd != nil &&
           (cmd.OpID == "switch" || cmd.OpID == "pull-switch") &&
           cmd.Status == ops.StatusExecuting {
            h.log.Info().
                Str("host", hostID).
                Str("op", cmd.OpID).
                Msg("P1920: agent disconnected during switch - entering AWAITING_RECONNECT")

            // Infer exit 0 (will be verified on reconnect via freshness check)
            h.lifecycleManager.EnterAwaitingReconnectOnDisconnect(key)
        }
    }
}
```

**New method in lifecycle.go**:

```go
// EnterAwaitingReconnectOnDisconnect handles agent disconnect during switch.
// Called when agent dies mid-switch (e.g., macOS launchctl bootout).
func (lm *LifecycleManager) EnterAwaitingReconnectOnDisconnect(hostID string) {
    lm.activeMu.Lock()
    cmd := lm.active[hostID]
    lm.activeMu.Unlock()

    if cmd == nil || cmd.Status != StatusExecuting {
        return
    }

    // Stop timeout watcher
    close(cmd.cancelTimeout)

    // Enter awaiting reconnect (same as HandleCommandComplete with exit 0)
    _, _ = lm.enterAwaitingReconnect(cmd)
}
```

**Flow (Fixed)**:

```
1. Agent starts switch
2. launchctl bootout kills agent
3. Dashboard detects disconnect → EnterAwaitingReconnectOnDisconnect()
4. Status = AWAITING_RECONNECT
5. New agent reconnects → HandleAgentReconnect()
6. Verify freshness → completeWithSuccess()
7. Emit command_finished
8. UI updates ✅
```

---

## Additional Fixes Required

### 1. IsTerminal() Missing States

`StatusSuspicious`, `StatusStaleBinary`, `StatusKilled`, `StatusPartial` not in terminal list:

**Location**: `src/internal/ops/op.go:121-127`

```go
func (s OpStatus) IsTerminal() bool {
    switch s {
    case StatusSuccess, StatusError, StatusTimeout, StatusSkipped, StatusBlocked,
         StatusSuspicious, StatusStaleBinary, StatusKilled, StatusPartial:  // ADD THESE
        return true
    }
    return false
}
```

**Impact**: `command_finished` never emitted for these states → UI stuck.

### 2. Agent source_commit=unknown

Logs show `source_commit=unknown` on reconnect, breaking freshness verification.

**Investigation needed**:

- Check how agent detects source commit
- Verify it's included in registration payload
- Ensure it's not "unknown" for built agents

---

## Acceptance Criteria

- ✅ Switch completion detected on macOS (home-manager)
- ✅ UI updates to show completion (green system compartment)
- ✅ Works without hard refresh
- ✅ Timeout if agent doesn't reconnect (30s)
- ✅ Freshness verification works (source_commit not "unknown")
- ✅ All terminal states emit `command_finished`
- ✅ Tests unblocked after switch completes
- ✅ No stuck `pending_command` for idle hosts

---

## Testing

### Manual Test (macOS)

1. Open dashboard
2. Click "Switch" on macOS host (mba-imac-work)
3. Wait for switch to complete (~30-60s)
4. Verify:
   - System compartment turns green (not blinking)
   - Log shows "Switch completed successfully"
   - "Building" badge cleared
   - Can run tests (not blocked)

### Manual Test (NixOS)

Same test on NixOS host to ensure no regression.

### Edge Cases

- Agent crashes during switch (not killed by launchctl)
- Switch fails (exit code != 0)
- Agent doesn't reconnect (timeout)
- Multiple switches in quick succession
- Dashboard restart during switch

---

## Implementation Plan

### Phase 1: Detect Disconnect During Switch

**Files**: `src/internal/dashboard/hub.go`, `src/internal/ops/lifecycle.go`

1. Add `GetActiveCommand(hostID)` to lifecycle manager
2. Add `EnterAwaitingReconnectOnDisconnect(hostID)` to lifecycle manager
3. Call from `handleClientUnregister()` when agent disconnects during switch

### Phase 2: Fix IsTerminal()

**File**: `src/internal/ops/op.go`

Add missing states to `IsTerminal()` switch.

### Phase 3: Fix source_commit Detection

**File**: `src/internal/agent/agent.go` or build system

Investigate why `source_commit=unknown` and fix.

### Phase 4: Clear operationProgress on command_finished

**File**: `src/internal/templates/dashboard.templ`

Already fixed in P1910 commit (4bed56a).

---

## Related

- P1110: ✅ COMPLETE (compartment correctness)
- P1120: ✅ COMPLETE (state sync wiring)
- P1900: ✅ FIXED (stale detection state bugs)
- P1910: Superseded (UI stuck - symptom of this issue)
- P8820: Superseded (stale pending - symptom of this issue)
- P2800: Command State Machine
- CORE-004: State Sync

---

## Implementation Summary (2026-01-19)

### Changes Made

**1. Added disconnect detection in `hub.go:handleUnregister()`**:

- Detects when agent disconnects during switch execution
- Calls `EnterAwaitingReconnectOnDisconnect()` to transition to AWAITING_RECONNECT
- Works for both `switch` and `pull-switch` operations

**2. New method in `lifecycle.go`**:

- `EnterAwaitingReconnectOnDisconnect(hostID)` - handles agent kill during switch
- Stops timeout watcher, enters AWAITING_RECONNECT state
- Reuses existing `enterAwaitingReconnect()` logic

**3. Fixed `IsTerminal()` in `op.go`**:

- Added missing terminal states: `StatusSuspicious`, `StatusStaleBinary`, `StatusKilled`, `StatusPartial`
- Ensures `command_finished` events are emitted for all terminal states

**4. Interface updates**:

- Added `GetActiveCommand()` and `EnterAwaitingReconnectOnDisconnect()` to `lifecycleManagerInterface`
- Added wrapper methods in `lifecycle_adapter.go`

### Files Modified

- `src/internal/dashboard/hub.go` - disconnect detection
- `src/internal/ops/lifecycle.go` - reconnect on disconnect
- `src/internal/ops/op.go` - terminal states fix
- `src/internal/dashboard/lifecycle_adapter.go` - interface wrappers

### Testing Required

Manual test on macOS (mba-imac-work):

1. Open dashboard
2. Click "Switch" on macOS host
3. Verify system compartment turns green (not stuck blinking)
4. Verify log shows completion
5. Verify can run tests (not blocked)

---

## Notes

**Why Setsid?**

Agent uses `Setsid: true` so switch process survives agent death:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setsid: true, // Create new session, become session leader
}
```

Without this, `launchctl bootout` would kill both agent AND switch process.

**Why launchctl bootout?**

`home-manager switch` runs `launchctl bootout` to stop old agent service before activating new configuration. This is correct - old agent must stop so new one can start.

**The Race**

Agent assumes it survives long enough to send status (500ms sleep at line 348), but `launchctl bootout` kills it immediately. Fix must handle this race.
