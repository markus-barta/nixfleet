# P5300 - System Compartment: Inference-Based Status

**Created**: 2025-12-28  
**Priority**: P5300 (High - Core Feature)  
**Status**: Backlog  
**Effort**: 2-3 hours  
**Depends on**: P5200

---

## User Story

**As a** fleet administrator  
**I want** the System compartment to show accurate status without expensive checks  
**So that** I know when my running system differs from the config, without waiting 60 seconds

---

## Problem

Current System compartment runs expensive checks:

- Runs `nix build --dry-run` (30-60 seconds)
- Consumes significant CPU/RAM
- Makes hosts unresponsive
- **Result: Always shows gray (check too expensive to run automatically)**

### Current Broken Behavior

```
After pulling new config:

┌─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │
│   🟢    │   🟢    │   🟢    │   ⚪    │  <- System shows GRAY
└─────────┴─────────┴─────────┴─────────┘

User thinks: "Is my system outdated or not?"
Reality: System IS outdated, but check is too expensive to run
```

---

## Solution

**Inference-based status** instead of expensive checks:

Infer System status from:

1. **Last command result** (pull succeeded → system outdated)
2. **Lock status** (lock outdated → system must be outdated)
3. **Command exit code** (switch failed → system error)

### Logic

```go
func inferSystemStatus(host Host) string {
    // If Lock is outdated, System MUST be outdated
    if host.Lock.Status == "outdated" {
        return "outdated"
    }

    // If last command was "pull" with exit 0
    if host.LastCommand == "pull" && host.LastExitCode == 0 {
        return "outdated"  // Yellow: pulled new config, need switch
    }

    // If last command was "switch" with exit 0
    if host.LastCommand == "switch" && host.LastExitCode == 0 {
        return "ok"  // Green: successfully applied
    }

    // If last command was "switch" with non-zero exit
    if host.LastCommand == "switch" && host.LastExitCode != 0 {
        return "error"  // Red: switch failed
    }

    // If Lock is current and no recent operations
    if host.Lock.Status == "ok" {
        return "ok"  // Assume system is current
    }

    return "unknown"  // Only on first heartbeat
}
```

---

## Acceptance Criteria

### Status Inference

- [ ] System shows yellow after successful pull
- [ ] System shows green after successful switch
- [ ] System shows red after failed switch
- [ ] System shows yellow when Lock is outdated
- [ ] System shows gray only on first connection (before any commands)

### Performance

- [ ] No expensive checks on heartbeat (<1ms per host)
- [ ] Status updates immediately after command completion
- [ ] No `nix build --dry-run` calls during normal operation

### UI/UX

- [ ] System tooltip explains: "Inferred from last command result"
- [ ] System tooltip shows: "Last switch: 2 min ago (exit 0)"
- [ ] When red: tooltip shows "Switch failed: <error message>"
- [ ] Manual refresh option available (runs expensive check on-demand)

---

## Technical Design

### Dashboard Changes

```go
// internal/dashboard/compartments.go
func (s *Server) computeSystemStatus(host *Host) StatusCheck {
    // Inference logic (see above)
    status := inferSystemStatus(host)

    var message string
    switch status {
    case "ok":
        message = "System is current"
    case "outdated":
        message = "System needs rebuild (run switch)"
    case "error":
        message = fmt.Sprintf("Switch failed: %s", host.LastCommandError)
    case "unknown":
        message = "Status unknown (no commands run yet)"
    }

    return StatusCheck{
        Status:    status,
        Message:   message,
        CheckedAt: time.Now().Format(time.RFC3339),
    }
}
```

### Agent Changes

Agent no longer runs `nix build --dry-run` automatically:

- Remove automatic system status checks
- Keep manual `refresh-system` command for on-demand checks
- System status is now computed dashboard-side

### Database Schema

Track last command result:

```sql
-- Add to hosts table (may already exist)
ALTER TABLE hosts ADD COLUMN last_command TEXT;
ALTER TABLE hosts ADD COLUMN last_exit_code INTEGER;
ALTER TABLE hosts ADD COLUMN last_command_at DATETIME;
ALTER TABLE hosts ADD COLUMN last_command_error TEXT;
```

---

## State Transitions

### Normal Update Flow

```
Initial state:
┌─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟡    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┘

After Pull (exit 0):
┌─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟡    │  <- System inferred from Pull
└─────────┴─────────┴─────────┴─────────┘

After Switch (exit 0):
┌─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟢    │  <- System inferred from Switch
└─────────┴─────────┴─────────┴─────────┘
```

### Failed Update Flow

```
After Switch (exit 1):
┌─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🔴    │  <- System error from failed Switch
└─────────┴─────────┴─────────┴─────────┘

Tooltip: "Switch failed: evaluation error in configuration"
```

---

## Testing Strategy

### Unit Tests

```go
func TestInferSystemStatus(t *testing.T) {
    tests := []struct {
        lastCmd      string
        exitCode     int
        lockStatus   string
        wantStatus   string
    }{
        {"pull", 0, "ok", "outdated"},
        {"switch", 0, "ok", "ok"},
        {"switch", 1, "ok", "error"},
        {"", 0, "outdated", "outdated"},
    }
    // ... test cases
}
```

### Integration Tests

- [ ] Pull → System yellow
- [ ] Switch after pull → System green
- [ ] Switch fails → System red
- [ ] Lock outdated → System yellow (regardless of last command)

---

## Migration Notes

- Existing hosts with unknown last command show gray until first command
- After first pull/switch, status becomes accurate
- No protocol changes required

---

## Manual Refresh Option

For paranoid users who want the expensive check:

```
Host → ⋮ → Refresh System (expensive)
```

This runs `nix build --dry-run` on-demand and updates the status.

**Future enhancement** (P5301): Run expensive check in background every 24 hours.

---

## Out of Scope

- Detecting manual system changes outside NixFleet (future: P5301)
- Showing what would change in a switch (future: P5302)
- Dry-run preview before switch (future: P5303)

---

## Related

- **P5200**: Lock Compartment - Version-Based Tracking
- **P5400**: Tests Compartment - Fifth Compartment
- **P5500**: Generation Tracking and Visibility
