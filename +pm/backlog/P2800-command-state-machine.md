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

All validation results logged to System Log (P4020):

```
14:23:05  ℹ  Switch started on hsb1
14:23:05  ⚠  Pre-check: Git outdated - pull recommended
14:24:32  ✓  Switch completed (exit 0)
14:24:33  ✓  Post-check: System now up to date
14:24:33  ⚠  Post-check: Agent version mismatch, awaiting restart
14:24:35  ✓  Agent restarted with new version
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

---

## Related

- **P2000** - Unified Host State (provides hostStore foundation) - Done
- **P4020** - Tabbed Output Panel (displays progress, logs validations)
- **P6600** - Status Papertrail (may be merged into P4020)
