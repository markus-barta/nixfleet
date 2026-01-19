# P1920: Switch Completion Not Detected (macOS)

**Priority**: P1 (Critical - Core Functionality Broken)
**Status**: Backlog
**Created**: 2026-01-19
**Type**: Bug
**References**: P1910 (UI Stuck), P2800 (Command State Machine)

---

## Summary

On macOS, switch completion is never detected by the dashboard. The agent is killed by `launchctl bootout` BEFORE it can send the completion status. This breaks the entire switch verification flow.

**Impact**:

- UI stuck in "Building" state indefinitely
- System compartment keeps blinking blue
- No completion message shown
- Hard refresh (Cmd+Shift+R) doesn't help
- Dashboard never knows switch completed

---

## Root Cause Analysis

### The macOS Switch Flow (Broken)

```
1. User clicks "Switch"
2. Dashboard sends command to agent
3. Agent starts `home-manager switch` in new session (Setsid)
4. home-manager runs `launchctl bootout` to stop old agent
5. ❌ AGENT KILLED HERE - before sendStatus() at line 334
6. Switch continues in background (new session survives)
7. Switch completes, launchd restarts agent
8. New agent reconnects
9. Dashboard never received TypeStatus → doesn't enter AWAITING_RECONNECT
10. Dashboard doesn't call HandleAgentReconnect()
11. UI stuck forever
```

### Evidence from Logs

```
1:43PM INF Executing switch host=mba-imac-work
1:43PM DBG command sent command=switch host=mba-imac-work
1:43PM DBG agent reports pending command agent_pending=switch
... (many heartbeats with pending_command=switch) ...
1:45PM DBG client unregistered id=mba-imac-work  ← Agent killed
1:45PM INF agent registered hostname=mba-imac-work  ← New agent
```

**Missing**: `command status command=switch exit_code=0` (never sent!)

### Code Location

`src/internal/agent/commands.go:334-355`:

```go
a.sendStatus(status, command, exitCode, message)  // Line 334 - NEVER REACHED

// ...

if exitCode == 0 && (command == "switch" || command == "pull-switch") {
    a.log.Info().Msg("switch completed successfully, restarting...")
    time.Sleep(500 * time.Millisecond)  // Line 348
    a.Shutdown()
    os.Exit(0)  // Line 351
}
```

The agent is killed by `launchctl bootout` during `runWithStreaming()` (line 359+), so it never reaches line 334.

---

## Why NixOS Works (Probably)

On NixOS:

1. `nixos-rebuild switch` doesn't kill the agent mid-command
2. Agent completes `runWithStreaming()`, returns exit code
3. Agent sends `TypeStatus` at line 334
4. Agent restarts via `os.Exit(101)` at line 353
5. Dashboard receives status → enters `AWAITING_RECONNECT`
6. New agent reconnects → `HandleAgentReconnect()` → `completeWithSuccess()`

---

## Fix Options

### Option A: Pre-emptive Status (Before Switch)

Send a "switch starting, expect disconnect" message before running the command:

```go
case "switch":
    a.sendOperationProgress("system", "in_progress", 0, 3)
    a.statusChecker.SetSystemWorking()

    // NEW: Pre-emptive notification for macOS
    if runtime.GOOS == "darwin" {
        a.sendPreemptiveSwitch()  // Tell dashboard to expect disconnect
    }

    cmd, err = a.buildSwitchCommand()
```

Dashboard enters `AWAITING_RECONNECT` immediately on receiving this message.

**Pros**: Simple, works with existing reconnect flow
**Cons**: Dashboard assumes success before knowing exit code

### Option B: Infer Completion from Reconnect

Dashboard infers switch completion when:

1. Agent was executing "switch" command
2. Agent disconnected
3. Agent reconnected with fresh binary

```go
func (h *Hub) handleAgentRegister(msg *agentMessage) {
    // ... existing code ...

    // Check if this host had a pending switch command
    if h.lifecycleManager != nil {
        key := h.hostKey(payload.Hostname)
        if h.lifecycleManager.HasPendingSwitch(key) {
            // Infer completion from reconnect
            h.lifecycleManager.InferSwitchCompletion(key, freshness)
        }
    }
}
```

**Pros**: No agent changes needed, works retroactively
**Cons**: Can't distinguish success from crash

### Option C: Wrapper Script (macOS Only)

Run switch via wrapper that sends status after completion:

```bash
#!/bin/bash
home-manager switch --flake "$1"
EXIT_CODE=$?
# Send status via HTTP or file
curl -X POST "http://localhost:8000/agent/status" -d "exit_code=$EXIT_CODE"
```

**Pros**: Guaranteed status delivery
**Cons**: Complex, requires HTTP endpoint, security concerns

### Option D: File-Based Status (Recommended)

Agent writes status to file before switch, dashboard reads on reconnect:

```go
// Before switch
statusFile := filepath.Join(os.TempDir(), "nixfleet-switch-status.json")
os.WriteFile(statusFile, []byte(`{"command":"switch","started_at":"..."}`), 0644)

// After switch (in wrapper or new agent)
// Read file, send status, delete file
```

**Pros**: Survives agent death, simple, no network
**Cons**: Requires file coordination

### Option E: Track by Lifecycle State (Simplest)

Dashboard tracks that switch was EXECUTING when agent disconnected:

```go
func (h *Hub) handleClientUnregister(client *Client) {
    // ... existing code ...

    // Check if agent was executing switch
    if h.lifecycleManager != nil {
        key := h.hostKey(hostID)
        cmd := h.lifecycleManager.GetActiveCommand(key)
        if cmd != nil && cmd.OpID == "switch" && cmd.Status == StatusExecuting {
            // Agent died during switch - enter awaiting reconnect
            h.lifecycleManager.EnterAwaitingReconnectOnDisconnect(key)
        }
    }
}
```

**Pros**: No agent changes, works with existing flow
**Cons**: Assumes disconnect during switch = success (risky)

---

## Recommended Fix: Option E + Timeout

1. When agent disconnects during switch, enter `AWAITING_RECONNECT`
2. Start reconnect timeout (30s)
3. On reconnect, verify freshness as normal
4. If timeout, mark as `TIMEOUT` (user must verify manually)

This is the safest approach because:

- No agent changes needed (works with existing agents)
- Uses existing reconnect verification flow
- Handles both success and crash cases
- Timeout prevents stuck state

---

## Additional Issues Found

### 1. `IsTerminal()` Missing States

`StatusSuspicious`, `StatusStaleBinary`, `StatusKilled`, `StatusPartial` are NOT in `IsTerminal()`:

```go
func (s OpStatus) IsTerminal() bool {
    switch s {
    case StatusSuccess, StatusError, StatusTimeout, StatusSkipped, StatusBlocked:
        return true
    }
    return false
}
```

This means `command_finished` is never emitted for these states!

**Fix**: Add missing states to `IsTerminal()`.

### 2. Agent `source_commit=unknown`

Logs show `source_commit=unknown` on reconnect:

```
agent registered hostname=mba-imac-work source_commit=unknown
```

This breaks freshness verification. Agent should report actual source commit.

---

## Acceptance Criteria

- ✅ Switch completion detected on macOS (home-manager)
- ✅ UI updates to show completion (green system compartment)
- ✅ Works without hard refresh
- ✅ Timeout if agent doesn't reconnect
- ✅ Freshness verification works (source_commit not "unknown")
- ✅ All terminal states emit `command_finished`

---

## Testing

### Manual Test

1. Open dashboard
2. Click "Switch" on macOS host
3. Wait for switch to complete (~30-60s)
4. Verify:
   - System compartment turns green
   - Log shows completion message
   - No stuck animations

### Edge Cases

- Agent crashes during switch (not killed by launchctl)
- Switch fails (exit code != 0)
- Agent doesn't reconnect (timeout)
- Multiple switches in quick succession

---

## Related Issues

- P1910: Switch Completion UI Stuck (symptom)
- P2800: Command State Machine
- P1900: Stale Detection Bugs
- CORE-004: State Sync

---

## Notes

**Why Setsid?**

The agent uses `Setsid: true` so the switch process survives agent death:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setsid: true, // Create new session, become session leader
}
```

Without this, `launchctl bootout` would kill both the agent AND the switch process.

**Why launchctl bootout?**

`home-manager switch` runs `launchctl bootout` to stop the old agent service before activating the new configuration. This is correct behavior - the old agent must stop so the new one can start.

**The Real Problem**

The agent assumes it will survive long enough to send status (line 334), but on macOS it's killed mid-command. The fix must account for this race condition.
