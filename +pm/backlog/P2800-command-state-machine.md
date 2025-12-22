# P2800 - Command State Machine

**Created**: 2025-12-21
**Priority**: P2800 (High - Architecture)
**Status**: Backlog
**Effort**: Medium (3-5 days)
**Depends on**: P2000 (Unified Host State - Done)

---

## Problem

Commands (Pull, Switch, Test) are executed without formal validation of preconditions or post-conditions. Users don't know:

1. **Before**: Is this action appropriate right now?
2. **During**: What's the current progress?
3. **After**: Did the action achieve its goal?

This leads to confusion when actions "succeed" (exit 0) but don't produce the expected result.

---

## Solution

Implement a **command state machine** with atomic, idempotent validation functions for each phase:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    COMMAND LIFECYCLE STATE MACHINE                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌───────────────┐    ┌───────────────┐    ┌───────────────┐               │
│   │  IDLE         │───▶│  VALIDATING   │───▶│  QUEUED       │               │
│   │               │    │  (pre)        │    │               │               │
│   └───────────────┘    └───────┬───────┘    └───────┬───────┘               │
│          ▲                     │ fail               │                       │
│          │              ┌──────▼──────┐             │                       │
│          │              │  BLOCKED    │             │                       │
│          │              │  (show why) │             ▼                       │
│          │              └─────────────┘    ┌───────────────┐                │
│          │                                 │  RUNNING      │                │
│          │                                 │  + progress   │                │
│          │                                 └───────┬───────┘                │
│          │                                         │                        │
│          │                                         ▼                        │
│          │                                 ┌───────────────┐                │
│          │                                 │  VALIDATING   │                │
│          │                                 │  (post)       │                │
│          │                                 └───────┬───────┘                │
│          │                                         │                        │
│          │         ┌───────────────────────────────┼───────────────┐        │
│          │         ▼                               ▼               ▼        │
│   ┌──────┴────────────┐                ┌───────────────┐  ┌─────────────┐   │
│   │  SUCCESS          │                │  PARTIAL      │  │  FAILED     │   │
│   │  (goal achieved)  │                │  (exit 0 but  │  │  (exit ≠ 0) │   │
│   └───────────────────┘                │  goal not met)│  └─────────────┘   │
│                                        └───────────────┘                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Core Principle: Atomic & Idempotent Validators

Every validation function MUST be:

| Property       | Meaning                                     |
| -------------- | ------------------------------------------- |
| **Atomic**     | Checks exactly ONE condition                |
| **Idempotent** | Same state → same result (no side effects)  |
| **Pure**       | Only reads state, never modifies            |
| **Typed**      | Returns structured result, not just boolean |
| **Loggable**   | Result can be logged/displayed to user      |

---

## Validation Functions Specification

### Type Definitions

```go
// ValidationResult is returned by all validators
type ValidationResult struct {
    Valid   bool   // Can proceed?
    Code    string // Machine-readable code for UI logic
    Message string // Human-readable explanation
}

// CommandState tracks the full lifecycle
type CommandState struct {
    HostID      string
    Command     string        // "pull", "switch", "test", "pull-switch"
    State       string        // "idle", "validating", "queued", "running", "validating-post", "success", "partial", "failed"
    StartedAt   *time.Time
    CompletedAt *time.Time
    ExitCode    *int
    PreCheck    *ValidationResult
    PostCheck   *ValidationResult
    Progress    *CommandProgress
}

type CommandProgress struct {
    Phase       string  // "fetching", "building", "activating", etc.
    Current     int     // e.g., 12
    Total       int     // e.g., 47
    Description string  // "Building derivation foo..."
}
```

### Pre-Condition Validators (Before Command)

```go
// ═══════════════════════════════════════════════════════════════════════════
// PRE-CONDITION VALIDATORS
// Each function checks ONE thing. Combine with AND logic for full validation.
// ═══════════════════════════════════════════════════════════════════════════

// CanExecuteCommand checks if ANY command can run on this host
func CanExecuteCommand(host *Host) ValidationResult {
    if !host.Online {
        return ValidationResult{false, "host_offline", "Host is offline"}
    }
    if host.PendingCommand != "" {
        return ValidationResult{false, "command_pending",
            fmt.Sprintf("Command '%s' already running", host.PendingCommand)}
    }
    return ValidationResult{true, "ok", "Host ready for commands"}
}

// CanPull checks if Pull is meaningful for this host
func CanPull(host *Host) ValidationResult {
    base := CanExecuteCommand(host)
    if !base.Valid {
        return base
    }

    git := host.UpdateStatus.Git
    if git.Status == "ok" {
        return ValidationResult{false, "already_current",
            "Git already up to date"}
    }
    if git.Status == "unknown" {
        return ValidationResult{true, "unknown_state",
            "Git status unknown - pull may help"}
    }
    // git.Status == "outdated"
    return ValidationResult{true, "outdated",
        fmt.Sprintf("Git outdated: %s", git.Message)}
}

// CanSwitch checks if Switch is meaningful for this host
func CanSwitch(host *Host) ValidationResult {
    base := CanExecuteCommand(host)
    if !base.Valid {
        return base
    }

    // Check if git is current (prerequisite for meaningful switch)
    git := host.UpdateStatus.Git
    if git.Status == "outdated" {
        return ValidationResult{false, "git_outdated",
            "Pull required before switch (git outdated)"}
    }

    system := host.UpdateStatus.System
    if system.Status == "ok" {
        return ValidationResult{false, "already_current",
            "System already up to date"}
    }
    if system.Status == "unknown" {
        return ValidationResult{true, "unknown_state",
            "System status unknown - switch may help"}
    }
    // system.Status == "outdated"
    return ValidationResult{true, "outdated",
        fmt.Sprintf("System outdated: %s", system.Message)}
}

// CanTest checks if Test is meaningful for this host
func CanTest(host *Host) ValidationResult {
    base := CanExecuteCommand(host)
    if !base.Valid {
        return base
    }
    // Test can always run if host is online and not busy
    return ValidationResult{true, "ok", "Ready to test"}
}

// CanPullSwitch checks if Pull+Switch sequence is meaningful
func CanPullSwitch(host *Host) ValidationResult {
    base := CanExecuteCommand(host)
    if !base.Valid {
        return base
    }

    // At least one of git or system should need update
    git := host.UpdateStatus.Git
    system := host.UpdateStatus.System

    if git.Status == "ok" && system.Status == "ok" {
        return ValidationResult{false, "already_current",
            "Both git and system already up to date"}
    }

    return ValidationResult{true, "ok", "Ready for pull + switch"}
}
```

### Post-Condition Validators (After Command)

```go
// ═══════════════════════════════════════════════════════════════════════════
// POST-CONDITION VALIDATORS
// Each function checks if the command achieved its GOAL, not just exit code.
// ═══════════════════════════════════════════════════════════════════════════

// ValidatePullResult checks if Pull achieved its goal
func ValidatePullResult(hostBefore, hostAfter *Host, exitCode int) ValidationResult {
    if exitCode != 0 {
        return ValidationResult{false, "exit_nonzero",
            fmt.Sprintf("Pull failed with exit code %d", exitCode)}
    }

    // Goal: Git compartment should now be green (or at least different)
    if hostAfter.UpdateStatus.Git.Status == "ok" {
        return ValidationResult{true, "goal_achieved",
            "Pull successful - git now up to date"}
    }

    // Check if generation changed (partial success)
    if hostBefore.Generation != hostAfter.Generation {
        return ValidationResult{true, "partial",
            fmt.Sprintf("Pull completed - generation changed (%s → %s) but git still shows outdated",
                hostBefore.Generation[:7], hostAfter.Generation[:7])}
    }

    return ValidationResult{false, "goal_not_achieved",
        "Pull completed (exit 0) but git compartment still outdated"}
}

// ValidateSwitchResult checks if Switch achieved its goal
func ValidateSwitchResult(hostBefore, hostAfter *Host, exitCode int) ValidationResult {
    if exitCode != 0 {
        return ValidationResult{false, "exit_nonzero",
            fmt.Sprintf("Switch failed with exit code %d", exitCode)}
    }

    // Goal: System compartment should now be green
    if hostAfter.UpdateStatus.System.Status == "ok" {
        // Bonus check: agent version should match if agent was updated
        if hostBefore.AgentOutdated && !hostAfter.AgentOutdated {
            return ValidationResult{true, "goal_achieved_with_agent",
                "Switch successful - system current, agent updated"}
        }
        return ValidationResult{true, "goal_achieved",
            "Switch successful - system now up to date"}
    }

    // Check if we're waiting for agent restart
    if hostAfter.AgentOutdated && hostBefore.AgentVersion != hostAfter.AgentVersion {
        return ValidationResult{true, "pending_restart",
            "Switch completed - waiting for agent restart"}
    }

    return ValidationResult{false, "goal_not_achieved",
        "Switch completed (exit 0) but system compartment still outdated"}
}

// ValidateTestResult checks if Test passed
func ValidateTestResult(host *Host, exitCode int, output string) ValidationResult {
    if exitCode != 0 {
        return ValidationResult{false, "test_failed",
            fmt.Sprintf("Test failed with exit code %d", exitCode)}
    }
    return ValidationResult{true, "test_passed", "All tests passed"}
}

// ValidatePullSwitchResult checks if Pull+Switch achieved combined goal
func ValidatePullSwitchResult(hostBefore, hostAfter *Host, exitCode int) ValidationResult {
    if exitCode != 0 {
        return ValidationResult{false, "exit_nonzero",
            fmt.Sprintf("Pull+Switch failed with exit code %d", exitCode)}
    }

    gitOK := hostAfter.UpdateStatus.Git.Status == "ok"
    systemOK := hostAfter.UpdateStatus.System.Status == "ok"

    if gitOK && systemOK {
        return ValidationResult{true, "goal_achieved",
            "Pull+Switch successful - fully up to date"}
    }

    if gitOK && !systemOK {
        return ValidationResult{false, "partial_git_only",
            "Pull succeeded but switch did not update system"}
    }

    if !gitOK && systemOK {
        return ValidationResult{true, "partial_system_only",
            "System updated but git still shows outdated (may be stale)"}
    }

    return ValidationResult{false, "goal_not_achieved",
        "Pull+Switch completed but neither git nor system updated"}
}
```

---

## UI Integration

### Pre-Validation UI Flow

```
User clicks "Switch" on host
         │
         ▼
┌─────────────────────────────────────┐
│ CanSwitch(host) called              │
└─────────────────┬───────────────────┘
                  │
     ┌────────────┴────────────┐
     │                         │
     ▼                         ▼
 Valid=true               Valid=false
     │                         │
     ▼                         ▼
 Execute command         Show dialog:
                         ┌─────────────────────────────┐
                         │ ⚠ Cannot Switch             │
                         │                             │
                         │ Git is outdated.            │
                         │ Pull first to get latest.   │
                         │                             │
                         │ [Cancel] [Pull First] [Force]│
                         └─────────────────────────────┘
```

### Post-Validation UI Flow

```
Command completes (exit code received)
         │
         ▼
┌─────────────────────────────────────┐
│ ValidateSwitchResult() called       │
│ with hostBefore, hostAfter, exit    │
└─────────────────┬───────────────────┘
                  │
     ┌────────────┼────────────┬────────────┐
     │            │            │            │
     ▼            ▼            ▼            ▼
 SUCCESS       PARTIAL       FAILED     PENDING
     │            │            │            │
     ▼            ▼            ▼            ▼
 Toast:        Toast:        Toast:     Toast:
 "✓ Switch     "⚠ Switch     "✗ Switch   "⧖ Switch done,
  complete"     done but      failed"     restarting..."
                outdated"
```

### System Log Integration

**Every state transition is logged verbosely.** The System Log (P4020) becomes the authoritative record of what happened and why.

---

## Verbose Logging Specification

### Principle: No Silent Transitions

Every state machine transition MUST log:

1. **What** happened (state change, validation result)
2. **Why** it happened (the condition that triggered it)
3. **What's next** (expected next step or required action)

### Log Message Format

```
[TIMESTAMP] [ICON] [HOST] [STATE] → [MESSAGE]
```

| Component | Description                   |
| --------- | ----------------------------- |
| TIMESTAMP | HH:MM:SS format               |
| ICON      | ✓ ⚠ ✗ ℹ ⧖ based on severity |
| HOST      | Hostname (e.g., hsb1)         |
| STATE     | Current state in brackets     |
| MESSAGE   | Human-readable explanation    |

### Complete Log Sequence Examples

#### Example 1: Successful Switch

```
14:23:05  ℹ  hsb1 [IDLE→VALIDATING]     User clicked Switch
14:23:05  ℹ  hsb1 [PRE-CHECK]           Checking CanExecuteCommand...
14:23:05  ✓  hsb1 [PRE-CHECK]           CanExecuteCommand: PASS (host online, no pending command)
14:23:05  ℹ  hsb1 [PRE-CHECK]           Checking CanSwitch...
14:23:05  ✓  hsb1 [PRE-CHECK]           CanSwitch: PASS (git=ok, system=outdated)
14:23:05  ℹ  hsb1 [PRE-CHECK]           Capturing pre-state snapshot (generation=abc1234, agentVersion=2.0.0)
14:23:05  ℹ  hsb1 [VALIDATING→QUEUED]   Pre-checks passed, queueing command
14:23:05  ℹ  hsb1 [QUEUED→RUNNING]      Command sent to agent: nixos-rebuild switch --flake .#hsb1
14:23:06  ℹ  hsb1 [RUNNING]             Agent acknowledged command start
14:23:08  ℹ  hsb1 [RUNNING]             Progress: evaluating flake...
14:23:15  ℹ  hsb1 [RUNNING]             Progress: building derivation 1/12
14:23:45  ℹ  hsb1 [RUNNING]             Progress: building derivation 12/12
14:24:02  ℹ  hsb1 [RUNNING]             Progress: activating new configuration
14:24:30  ℹ  hsb1 [RUNNING→VALIDATING]  Command completed (exit code: 0)
14:24:30  ℹ  hsb1 [POST-CHECK]          Running ValidateSwitchResult...
14:24:30  ℹ  hsb1 [POST-CHECK]          Comparing: system.status before=outdated, after=ok
14:24:30  ✓  hsb1 [POST-CHECK]          ValidateSwitchResult: PASS (goal_achieved)
14:24:30  ✓  hsb1 [VALIDATING→SUCCESS]  Switch complete - system now up to date
14:24:31  ℹ  hsb1 [SUCCESS]             Agent version: 2.0.0 → 2.1.0 (restart expected)
14:24:33  ✓  hsb1 [SUCCESS]             Agent reconnected with version 2.1.0
```

#### Example 2: Switch Blocked by Precondition

```
14:25:00  ℹ  gpc0 [IDLE→VALIDATING]     User clicked Switch
14:25:00  ℹ  gpc0 [PRE-CHECK]           Checking CanExecuteCommand...
14:25:00  ✓  gpc0 [PRE-CHECK]           CanExecuteCommand: PASS (host online, no pending command)
14:25:00  ℹ  gpc0 [PRE-CHECK]           Checking CanSwitch...
14:25:00  ✗  gpc0 [PRE-CHECK]           CanSwitch: FAIL (git_outdated)
14:25:00  ⚠  gpc0 [VALIDATING→BLOCKED]  Cannot switch: Git is outdated, pull required first
14:25:00  ℹ  gpc0 [BLOCKED]             Showing option dialog to user...
```

#### Example 3: Switch with Partial Success

```
14:30:00  ℹ  imac0 [IDLE→VALIDATING]    User clicked Switch
14:30:00  ✓  imac0 [PRE-CHECK]          All pre-checks passed
14:30:00  ℹ  imac0 [QUEUED→RUNNING]     Command sent: home-manager switch --flake .#imac0
14:31:45  ℹ  imac0 [RUNNING→VALIDATING] Command completed (exit code: 0)
14:31:45  ℹ  imac0 [POST-CHECK]         Running ValidateSwitchResult...
14:31:45  ℹ  imac0 [POST-CHECK]         Comparing: system.status before=outdated, after=outdated
14:31:45  ⚠  imac0 [POST-CHECK]         ValidateSwitchResult: PARTIAL (goal_not_achieved)
14:31:45  ⚠  imac0 [VALIDATING→PARTIAL] Switch exited 0 but system still outdated
14:31:45  ℹ  imac0 [PARTIAL]            Possible causes: nix store not updated, agent cache stale
14:31:45  ℹ  imac0 [PARTIAL]            Suggestion: Try "Refresh Status" or re-run switch
```

#### Example 4: Command Failed

```
14:35:00  ℹ  csb0 [IDLE→VALIDATING]     User clicked Pull
14:35:00  ✓  csb0 [PRE-CHECK]           All pre-checks passed
14:35:00  ℹ  csb0 [QUEUED→RUNNING]      Command sent: git fetch && git reset --hard origin/main
14:35:02  ✗  csb0 [RUNNING→VALIDATING]  Command completed (exit code: 128)
14:35:02  ℹ  csb0 [POST-CHECK]          Running ValidatePullResult...
14:35:02  ✗  csb0 [POST-CHECK]          ValidatePullResult: FAIL (exit_nonzero)
14:35:02  ✗  csb0 [VALIDATING→FAILED]   Pull failed with exit code 128
14:35:02  ℹ  csb0 [FAILED]              Check output log for error details
```

#### Example 5: Bulk Pull All

```
14:40:00  ℹ  [BULK]                     User clicked "Pull All" (5 hosts selected)
14:40:00  ℹ  hsb0 [PRE-CHECK]           CanPull: PASS (git=outdated)
14:40:00  ℹ  hsb1 [PRE-CHECK]           CanPull: SKIP (git=ok, already current)
14:40:00  ℹ  gpc0 [PRE-CHECK]           CanPull: PASS (git=outdated)
14:40:00  ℹ  imac0 [PRE-CHECK]          CanPull: PASS (git=outdated)
14:40:00  ✗  csb0 [PRE-CHECK]           CanPull: FAIL (host offline)
14:40:00  ℹ  [BULK]                     Executing on 3 hosts (1 skipped, 1 blocked)
14:40:00  ℹ  hsb0 [QUEUED→RUNNING]      Pull started
14:40:00  ℹ  gpc0 [QUEUED→RUNNING]      Pull started
14:40:00  ℹ  imac0 [QUEUED→RUNNING]     Pull started
14:40:05  ✓  hsb0 [SUCCESS]             Pull complete - git now up to date
14:40:06  ✓  imac0 [SUCCESS]            Pull complete - git now up to date
14:40:08  ✓  gpc0 [SUCCESS]             Pull complete - git now up to date
14:40:08  ✓  [BULK]                     Pull All complete: 3 success, 1 skipped, 1 offline
```

### Log Level Configuration

| Level   | Icon | When Used                               |
| ------- | ---- | --------------------------------------- |
| DEBUG   | ·    | Internal state details (off by default) |
| INFO    | ℹ   | State transitions, progress updates     |
| SUCCESS | ✓    | Validation passed, goal achieved        |
| WARNING | ⚠   | Partial success, non-blocking issues    |
| ERROR   | ✗    | Failures, blocked actions               |

### Implementation: LogEntry Structure

```go
type LogEntry struct {
    Timestamp time.Time
    Level     string    // "debug", "info", "success", "warning", "error"
    HostID    string    // Empty for bulk/system messages
    State     string    // Current state machine state
    Message   string    // Human-readable message
    Code      string    // Machine-readable code for filtering
    Details   map[string]any // Optional structured data
}

// Example usage in validation
func (sm *CommandStateMachine) runPreChecks(host *Host, command string) {
    sm.log(LogEntry{
        Level:   "info",
        HostID:  host.ID,
        State:   "PRE-CHECK",
        Message: "Checking CanExecuteCommand...",
    })

    result := CanExecuteCommand(host)

    sm.log(LogEntry{
        Level:   levelFromResult(result),
        HostID:  host.ID,
        State:   "PRE-CHECK",
        Message: fmt.Sprintf("CanExecuteCommand: %s (%s)",
            passOrFail(result.Valid), result.Message),
        Code:    result.Code,
        Details: map[string]any{
            "valid":   result.Valid,
            "online":  host.Online,
            "pending": host.PendingCommand,
        },
    })
}
```

### WebSocket Broadcast

All log entries are broadcast to connected browsers:

```json
{
  "type": "state_machine_log",
  "payload": {
    "timestamp": "2025-12-21T14:23:05Z",
    "level": "info",
    "host_id": "hsb1",
    "state": "PRE-CHECK",
    "message": "CanSwitch: PASS (git=ok, system=outdated)",
    "code": "outdated"
  }
}
```

---

## State Storage

### Per-Host Command State (in hostStore)

```javascript
hostStore = {
  hsb1: {
    // ... existing fields ...

    // NEW: Current/last command state
    commandState: {
      command: "switch",
      state: "running", // idle|validating|queued|running|success|partial|failed
      startedAt: "2025-12-21T14:23:05Z",
      completedAt: null,
      exitCode: null,
      preCheck: { valid: true, code: "outdated", message: "System outdated" },
      postCheck: null, // Populated after completion
      progress: { phase: "building", current: 12, total: 47 },
    },
  },
};
```

### Command History (optional, for debugging)

```javascript
commandHistory = {
  hsb1: [
    { command: "switch", state: "success", startedAt: "...", completedAt: "...", ... },
    { command: "pull", state: "success", startedAt: "...", completedAt: "...", ... },
    // Last N commands
  ]
}
```

---

## Implementation Order

1. **Phase 1: Validation Functions (Go)**
   - Implement all `Can*()` pre-validators
   - Implement all `Validate*Result()` post-validators
   - Add unit tests for each validator

2. **Phase 2: State Machine (Go)**
   - Add `CommandState` to host tracking
   - Capture `hostBefore` snapshot when command starts
   - Run post-validation when command completes
   - Broadcast validation results via WebSocket

3. **Phase 3: UI Integration (JS)**
   - Show pre-validation dialogs when `Valid=false`
   - Update System Log with validation messages
   - Show appropriate toast based on post-validation result
   - Display progress during command execution

4. **Phase 4: Progress Reporting (Agent)**
   - Parse nix build output for derivation counts
   - Send progress updates via WebSocket
   - Display progress bar in UI

---

## Acceptance Criteria

### Pre-Validation

- [ ] Clicking action when precondition fails shows explanatory dialog
- [ ] Dialog offers sensible alternatives (Pull First, Force, Cancel)
- [ ] Force option bypasses pre-validation (for edge cases)

### Post-Validation

- [ ] Commands ending with exit 0 but goal not met show warning toast
- [ ] Commands achieving goal show success toast
- [ ] All validation results appear in System Log
- [ ] Partial success states clearly communicated

### Idempotency

- [ ] Running same validator twice with same state returns same result
- [ ] Validators have no side effects
- [ ] Validators are unit tested

### Verbose Logging

- [ ] Every state transition logged with WHAT, WHY, WHAT'S NEXT
- [ ] Pre-check logs show each validator name and result
- [ ] Post-check logs show before/after comparison values
- [ ] Bulk actions log per-host breakdown (pass/skip/blocked)
- [ ] Progress updates logged during command execution
- [ ] All logs appear in System Log (P4020) in real-time
- [ ] Log entries include machine-readable `code` for filtering
- [ ] WebSocket broadcasts all log entries to browsers

---

## Related

- **P2000** - Unified Host State (provides hostStore foundation) - Done
- **P4020** - Tabbed Output Panel (displays progress, logs validations)
- **P6600** - Status Papertrail (merged into P4020 - status history in host tabs)
