# P8910: Switch Completion UI Stuck in "Building" State

**Priority**: P0 (Critical - UX Broken)
**Status**: Backlog
**Created**: 2026-01-19
**Type**: Bug
**References**: P2800 (Command State Machine), CORE-004 (State Sync)

---

## Summary

After successful switch command (even on macOS with home-manager), the UI remains stuck:

- System compartment keeps blinking blue (working state)
- Log panel shows yellow dot blinking
- "Building" badge persists below log
- Last log entries: "Activating onFilesChange", "Activating setupLaunchAgents"

**Impact**: User cannot tell if switch completed successfully. Must manually refresh or check logs.

---

## Observed Behavior

**Test Case**: `mba-imac-work` (macOS, home-manager switch)

1. User clicks "Switch"
2. Command executes successfully (exit 0)
3. Agent sends completion status
4. Agent restarts (as designed)
5. Agent reconnects
6. **BUG**: UI never updates to show completion

**UI State (Stuck)**:

- System compartment: 🔵 blinking (should be 🟢 solid)
- Log panel: 🟡 blinking (should show completion)
- Badge: "Building" (should be cleared)
- Last output: activation messages (no completion message)

---

## Expected Behavior

After successful switch:

1. System compartment: 🟢 solid green
2. Log panel: ✅ "Switch completed successfully"
3. Badge: cleared or "Complete"
4. Status: "Current (inferred from exit 0)"

---

## Technical Analysis

### Agent Flow (Correct ✅)

```
1. executeCommand("switch")
2. SetSystemWorking() → status = "working"
3. sendOperationProgress("system", "in_progress", 0, 3)
4. Run home-manager switch
5. Exit 0 → sendOperationProgress("system", "complete", 3, 3)
6. SetSystemOk("Switch successful (exit 0)")
7. sendStatus("ok", "switch", 0, "")
8. sendHeartbeat() → push fresh status
9. Agent restarts (line 345-355)
```

### Dashboard Flow (Should Work ✅)

```
1. handleStatus() receives TypeStatus
2. HandleCommandComplete() → enterAwaitingReconnect()
3. Status = StatusAwaitingReconnect
4. updateAndBroadcast() → emit command_progress
5. Agent reconnects → handleAgentRegister()
6. HandleAgentReconnect() → verify freshness
7. completeWithSuccess() → Status = StatusSuccess
8. updateAndBroadcast() → emit command_finished
9. clearActive() → clear pending_command
```

### Hypothesis: Missing Broadcast or UI Not Consuming

**Possible Causes**:

1. **`command_finished` not emitted** (lifecycle.go:646)
   - `updateAndBroadcast()` might not be calling state sync
   - Check if `BroadcastCommandState()` is wired correctly

2. **UI not consuming `command_finished`** (dashboard.templ)
   - `applyDelta()` handles `command_finished` (line 1574)
   - But might not be clearing operation progress
   - Might not be resetting system compartment to non-working

3. **Operation progress not cleared**
   - Agent sends `operation_progress` with "complete"
   - But UI might keep showing "in_progress" from earlier message
   - No explicit "clear progress" message

4. **System status not updated in UI**
   - Agent sets `SetSystemOk()` before restart
   - Heartbeat after reconnect should include status
   - But UI might not be applying it

---

## Investigation Steps

### 1. Check Broadcast Emission

```bash
# On dashboard, grep logs for:
grep "command_finished" /path/to/dashboard.log
grep "StatusSuccess" /path/to/dashboard.log
```

**Expected**: Should see `command_finished` event after reconnect.

### 2. Check UI Console

Open browser console during switch:

- Look for `command_finished` message
- Check if `applyDelta()` is called
- Verify `hostStore.update()` is clearing progress

### 3. Check Operation Progress

Agent sends:

```json
{
  "type": "operation_progress",
  "payload": {
    "host_id": "mba-imac-work",
    "progress": {
      "system": {
        "status": "complete",
        "current": 3,
        "total": 3
      }
    }
  }
}
```

UI should:

- Clear blinking animation
- Show completion state
- Remove "Building" badge

### 4. Check System Compartment

After reconnect, heartbeat should include:

```json
{
  "update_status": {
    "system": {
      "status": "ok",
      "message": "Switch successful (exit 0)",
      "checked_at": "..."
    }
  }
}
```

UI should:

- Stop blue blinking
- Show green solid
- Update tooltip

---

## Likely Root Cause

**Missing `operation_progress` clear on completion.**

The agent sends `operation_progress` with `status: "complete"`, but the UI might be:

1. Not recognizing "complete" as a terminal state
2. Not clearing the blinking animation
3. Not removing the "Building" badge

**Code Location**: `dashboard.templ` line 1487-1492

```javascript
case 'operation_progress':
    if (!hostId) return;
    hostStore.update(hostId, {
        operationProgress: payload.progress
    });
    break;
```

This just stores the progress, but doesn't:

- Clear blinking when status = "complete"
- Remove badge
- Update log panel indicator

---

## Fix Strategy

### Option A: Clear Progress on `command_finished`

When `command_finished` delta arrives, explicitly clear `operationProgress`:

```javascript
} else if (change.type === 'command_finished') {
    const cmd = change.fields || {};
    const hostId = cmd.host_id;
    // ... existing code ...

    // Clear operation progress
    hostStore.update(hostId, {
        operationProgress: null  // or {}
    });
}
```

### Option B: Recognize "complete" Status

In `operation_progress` handler, check if all phases are complete:

```javascript
case 'operation_progress':
    if (!hostId) return;
    const progress = payload.progress;

    // Check if all phases are complete
    const allComplete = Object.values(progress).every(
        phase => phase && phase.status === 'complete'
    );

    hostStore.update(hostId, {
        operationProgress: allComplete ? null : progress
    });
    break;
```

### Option C: Agent Sends Explicit Clear

Agent sends empty progress after completion:

```go
// After sendStatus()
a.sendOperationProgress("", "clear", 0, 0)
```

Dashboard interprets this as "clear all progress".

---

## Acceptance Criteria

- ✅ After successful switch, system compartment shows 🟢 solid green
- ✅ Log panel shows ✅ "Switch completed successfully"
- ✅ "Building" badge is cleared
- ✅ No blinking animations persist
- ✅ Works for both NixOS and macOS (home-manager)
- ✅ Works after agent restart/reconnect

---

## Testing

### Manual Test

1. Open dashboard
2. Click "Switch" on any host
3. Wait for completion
4. Verify:
   - System compartment turns green (not blinking)
   - Log shows completion message
   - Badge cleared
   - No stuck animations

### Automated Test

```javascript
// In dashboard.templ or test file
test("operation progress clears on completion", () => {
  const hostId = "test-host";

  // Send in_progress
  handleMessage({
    type: "operation_progress",
    payload: {
      host_id: hostId,
      progress: { system: { status: "in_progress", current: 1, total: 3 } },
    },
  });

  // Send complete
  handleMessage({
    type: "operation_progress",
    payload: {
      host_id: hostId,
      progress: { system: { status: "complete", current: 3, total: 3 } },
    },
  });

  // Verify progress cleared
  const host = hostStore.get(hostId);
  expect(host.operationProgress).toBeNull();
});
```

---

## Related Issues

- P2800: Command State Machine
- P8900: Stale Detection Bugs (similar symptom - stuck "working")
- CORE-004: State Sync (delta emission)

---

## Notes

**Why agent restarts after switch?**

- NixOS: systemd service has `RestartForceExitStatus=101`
- macOS: launchd `KeepAlive=true` restarts on exit
- Purpose: Pick up new binary from switch

**Why await reconnect?**

- Verify agent binary was actually updated
- Detect stale binary (switch didn't update agent)
- Provide user feedback on binary freshness

**Reconnect timeout**: 30 seconds (configurable in `TimeoutConfig`)
