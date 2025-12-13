# Command Timeout & Stop Button

**Created**: 2025-12-13
**Priority**: Medium
**Status**: Backlog

---

## Problem

When a command hangs or takes longer than expected, the user has no way to:
1. Know it's stuck (buttons just stay disabled)
2. Cancel it and try something else
3. Recover without manual SSH intervention

Currently, "Unlock actions" in the ellipsis menu only clears local UI state — it doesn't actually stop the running process.

---

## Requirements

### 1. Per-Command Timeouts (Dashboard)

Each command type should have a configurable expected timeout:

| Command | Expected Timeout | Notes |
|---------|------------------|-------|
| `pull` | 2 minutes | Network-bound |
| `pull-reset` | 2 minutes | Same as pull |
| `switch` | 15 minutes | Can be long for rebuilds |
| `test` | 30 minutes | Test suites vary |
| `update` | 10 minutes | Flake update + switch |
| `restart` | 30 seconds | Quick |

Dashboard tracks when command was sent and compares to timeout.

### 2. Visual Warning When Timeout Exceeded

When a command exceeds its timeout:
- Show warning indicator (e.g., ⚠️ or yellow/orange styling)
- Update status entry: "Running for 10+ min..."

### 3. Stop Button Replaces Action Buttons

When timeout exceeded:
- Replace the 3 main buttons (Pull, Switch, Test) with single button:
  - **"Stop: [command]"** (e.g., "Stop: switch")
- Button should be prominent (red/warning color)
- Clicking sends `stop` command to agent

For commands triggered from ellipsis menu:
- Same behavior — replace main buttons with Stop button

### 4. Agent: Track Running Process

Agent must track the PID of the currently running command:
- Store PID when spawning command subprocess
- Only the tracked PID can be stopped (security)
- Use process group kill (`kill -TERM -$PID`) to handle child processes

### 5. Agent: Handle Stop Command (Enhanced)

Current `stop` command only works for tests. Extend to all commands:

```bash
case "$command" in
  stop)
    if [[ -n "$CURRENT_PID" ]] && kill -0 "$CURRENT_PID" 2>/dev/null; then
      kill -TERM -"$CURRENT_PID" 2>/dev/null  # Kill process group
      sleep 2
      kill -KILL -"$CURRENT_PID" 2>/dev/null || true  # Force kill if needed
      report_status "ok" "$(get_generation)" "Stopped by user: $CURRENT_COMMAND"
      CURRENT_PID=""
      CURRENT_COMMAND=""
    else
      report_status "ok" "$(get_generation)" "No running command to stop"
    fi
    ;;
```

### 6. Confirm Stop Success → Unlock Buttons

When agent reports successful stop:
- Dashboard receives status update via SSE
- pending_command is cleared
- Buttons are re-enabled
- Status shows "Stopped by user"

If stop fails:
- Dashboard shows error
- Buttons remain locked
- User can retry or use manual "Unlock actions"

---

## Edge Cases

| Scenario | Handling |
|----------|----------|
| Command finishes right as stop is sent | Agent reports "No running command to stop", dashboard unlocks |
| Stop fails (permission error) | Agent reports error, dashboard keeps locked, suggest SSH |
| Network disconnect during stop | Dashboard times out, shows warning, suggests manual check |
| User clicks Stop multiple times | Debounce on dashboard, agent handles idempotently |

---

## Security Considerations

- Agent only kills processes it spawned (tracked PID)
- Never kill arbitrary processes by name or pattern
- Process group kill ensures children are cleaned up
- Command must have been initiated by authenticated dashboard session

---

## Acceptance Criteria

- [ ] Dashboard tracks command send time
- [ ] Dashboard has per-command timeout configuration
- [ ] Warning shown when timeout exceeded
- [ ] Stop button replaces action buttons when stuck
- [ ] Agent tracks PID of running command
- [ ] Agent kills process group on stop (handles children)
- [ ] Agent reports stop success/failure
- [ ] Dashboard unlocks buttons on successful stop
- [ ] Works for all command types (pull, switch, test, update, etc.)

---

## Notes

- This builds on existing `stop` command for tests
- Consider making timeouts configurable per-host (some hosts are slower)
- Future: Show real-time progress from long-running commands (streaming output)

