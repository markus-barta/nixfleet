# P4395 - Stop Command Implementation

**Priority**: High  
**Status**: Pending  
**Effort**: Medium  
**References**: PRD FR-1.4, PRD FR-1.11, `+pm/legacy/v1.0/dashboard.html`

## Problem

Cannot cancel a running command. This is dangerous for:

- Runaway tests that never complete
- Stuck switch operations
- Accidental commands

PRD FR-1.11: "Track command PID for stop capability" - **Must** priority

## Current State

- Agent already tracks `commandPID` in `a.busyMu`
- Agent has `handleStop()` stub but doesn't actually kill
- Dashboard has "stop" command in protocol
- UI has Stop button styling but no wiring

## Solution

### Agent: Kill Process

```go
func (a *Agent) handleStop() {
    a.busyMu.Lock()
    pid := a.commandPID
    a.busyMu.Unlock()

    if pid == 0 {
        a.sendStatus("error", "No command running")
        return
    }

    process, err := os.FindProcess(pid)
    if err != nil {
        a.sendStatus("error", fmt.Sprintf("Process %d not found", pid))
        return
    }

    // Send SIGTERM first (graceful)
    if err := process.Signal(syscall.SIGTERM); err != nil {
        // If SIGTERM fails, try SIGKILL
        process.Kill()
    }

    // Process cleanup happens in command goroutine
    a.sendStatus("stopped", fmt.Sprintf("Killed process %d", pid))
}
```

### Agent: Handle Kill in Command Goroutine

```go
func (a *Agent) executeCommand(command string) {
    // ... existing setup ...

    // Wait for command with context for cancellation
    done := make(chan error, 1)
    go func() {
        done <- cmd.Wait()
    }()

    select {
    case err := <-done:
        // Normal completion
        if err != nil {
            a.sendStatus("error", err.Error())
        } else {
            a.sendStatus("complete", "")
        }
    case <-a.stopChan:
        // Killed by stop command
        cmd.Process.Kill()
        a.sendStatus("stopped", "Command terminated")
    }
}
```

### Dashboard: Send Stop Command

Already have `POST /api/hosts/{id}/command` with `{"command": "stop"}`.

### UI: Stop Button

Already have button styling. Need to:

1. Show Stop instead of Test when `test_running` or `pending_command`
2. Keep Stop enabled even when busy (that's the point)
3. Send "stop" command on click

### Requirements

- [ ] Agent: Implement actual process kill in `handleStop()`
- [ ] Agent: Handle SIGTERM/SIGKILL gracefully
- [ ] Agent: Clear busy state after kill
- [ ] Dashboard: Forward stop command to agent
- [ ] UI: Dynamic Stop/Test button swap
- [ ] UI: Keep Stop enabled during busy
- [ ] Test: Verify kill works on long-running test

## Related

- P4380 (Dropdown) - Stop shown during test
- P4385 (Button States) - Stop special case
- T03 (Commands) - Stop command specification
