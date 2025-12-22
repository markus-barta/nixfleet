# P2800 - Command State Machine (Comprehensive)

**Created**: 2025-12-21
**Updated**: 2025-12-22
**Priority**: P2800 (High - Architecture)
**Status**: Backlog → Planning
**Effort**: 10-14 days
**Depends on**: P2000 (Unified Host State - Done)

---

## Table of Contents

1. [Overview](#overview)
2. [Design Decisions](#design-decisions)
3. [Command Lifecycle State Machine](#command-lifecycle-state-machine)
4. [Validators Specification](#validators-specification)
5. [Agent Binary Freshness Detection](#agent-binary-freshness-detection-p2810)
6. [Protocol Changes](#protocol-changes)
7. [UI Integration](#ui-integration)
8. [Test Strategy](#test-strategy-138-tests)
9. [Implementation Plan](#implementation-plan)
10. [Acceptance Criteria](#acceptance-criteria)
11. [Risk Mitigation](#risk-mitigation)

---

## Overview

### Problem

Commands (Pull, Switch, Test) are executed without formal validation of preconditions or post-conditions. Users don't know:

1. **Before**: Is this action appropriate right now?
2. **During**: What's the current progress?
3. **After**: Did the action achieve its goal?

This leads to confusion when actions "succeed" (exit 0) but don't produce the expected result.

Additionally, the Lock/System compartments don't detect when the **agent binary itself** is outdated. This leads to:

1. User triggers `switch` via dashboard
2. Switch completes successfully (exit 0)
3. Lock compartment shows GREEN (repo is current)
4. But the **running agent** is still the OLD binary!
5. New features don't work
6. User is confused - "I switched but nothing changed"

This has happened **>10 times** during development!

### Solution

Implement a **command state machine** with:

1. **Atomic, idempotent validation functions** for each phase
2. **3-layer binary freshness detection** to catch stale agent binaries
3. **Comprehensive test coverage** (138 tests) to guarantee correctness

### Goal

100% guaranteed E2E functionality with:

- Senior developer-level rigor
- Race condition and concurrency coverage
- Self-healing detection (conservative)
- Paranoid binary freshness verification

---

## Design Decisions

### Decision 1: Concurrent Command Handling

**Choice**: Block with "command pending" error

When user clicks a command while another is running on the same host:

- Return HTTP 409 with `code: "command_pending"`
- UI shows "Command already running" message
- No queueing, no cancellation of first command

### Decision 2: Post-Validation Timing

**Choice**: Explicit `command_complete` message with fallback

```
PRIMARY PATH:
  Agent completes command
    → Agent forces status refresh (git, system, generation)
    → Agent sends command_complete { exit_code, fresh_status }
    → Dashboard receives fresh_status
    → Dashboard runs post-validation IMMEDIATELY
    → No timing issues!

FALLBACK PATH (if command_complete not received):
  Timeout after:
    - switch: 30s (agent may restart)
    - pull: 10s
    - test: 5s
  → Wait for next heartbeat
  → If state changed: run post-validation
  → If state unchanged: log warning, fallback to exit code only
```

### Decision 3: Self-Healing Scope

**Choice**: Conservative (detect and alert, no auto-cleanup)

- Detect orphaned snapshots (command started, never completed)
- Detect stuck RUNNING state
- Log warnings (visible in per-host UI log)
- Do NOT auto-cleanup or auto-retry

### Decision 4: E2E Test Environment

**Choice**: Against actual fleet

| Host    | Platform | Role                       |
| ------- | -------- | -------------------------- |
| `gpc0`  | NixOS    | nixos-rebuild switch tests |
| `imac0` | macOS    | home-manager switch tests  |

### Decision 5: Binary Freshness Verification

**Choice**: Paranoid (3-layer verification)

| Layer            | What                             | How                                      |
| ---------------- | -------------------------------- | ---------------------------------------- |
| 1. Source Commit | Git commit agent was built from  | ldflags at build time                    |
| 2. Store Path    | Nix store path of running binary | Agent reads /proc/self/exe or equivalent |
| 3. Binary Hash   | SHA256 of actual binary          | Agent computes on startup                |

Detection is based on **change**, not expected values:

- Before switch: capture commit, path, hash
- After switch + restart: compare new values
- If unchanged → stale binary detected

---

## Command Lifecycle State Machine

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

### Core Principle: Atomic & Idempotent Validators

Every validation function MUST be:

| Property       | Meaning                                     |
| -------------- | ------------------------------------------- |
| **Atomic**     | Checks exactly ONE condition                |
| **Idempotent** | Same state → same result (no side effects)  |
| **Pure**       | Only reads state, never modifies            |
| **Typed**      | Returns structured result, not just boolean |
| **Loggable**   | Result can be logged/displayed to user      |

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

---

## Validators Specification

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
    if system.Status == "ok" && !host.AgentOutdated {
        return ValidationResult{false, "already_current",
            "System already up to date"}
    }
    if system.Status == "unknown" {
        return ValidationResult{true, "unknown_state",
            "System status unknown - switch may help"}
    }
    // system.Status == "outdated" OR host.AgentOutdated
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

    if git.Status == "ok" && system.Status == "ok" && !host.AgentOutdated {
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

## Agent Binary Freshness Detection (P2810)

### The Problem

The agent binary is a Nix derivation built from the `nixfleet` flake input. When:

- `nixcfg/flake.lock` is updated to point to new nixfleet commit
- `home-manager switch` or `nixos-rebuild switch` runs
- Nix may **substitute** (download) the binary from cache instead of building
- If cache has OLD binary, user gets old code even though switch "succeeded"

### 3-Layer Paranoid Verification

```go
type AgentFreshness struct {
    // Layer 1: Source commit (from ldflags at build)
    SourceCommit       string  // e.g., "abc1234"

    // Layer 2: Nix store path
    StorePath          string  // e.g., "/nix/store/xxx-nixfleet-agent-2.0.0"

    // Layer 3: Binary hash
    BinaryHash         string  // SHA256 of /proc/self/exe

    // Comparison results (computed by dashboard)
    CommitChanged      bool
    StorePathChanged   bool
    HashChanged        bool
}

// Detection logic: compare before/after switch
func detectStaleBinary(before, after AgentFreshness) StaleBinaryResult {
    if !before.CommitChanged && !before.StorePathChanged && !before.HashChanged {
        return STALE  // Nothing changed = stale
    }
    if before.StorePathChanged || before.HashChanged {
        return FRESH  // Binary definitely changed
    }
    // Commit changed but path/hash didn't = suspicious
    return SUSPICIOUS
}
```

### Decision Matrix

| Commit Changed | Path Changed | Hash Changed | Verdict              |
| :------------: | :----------: | :----------: | -------------------- |
|       ✓        |      ✓       |      ✓       | FRESH                |
|       ✓        |      ✓       |      ✗       | FRESH (path changed) |
|       ✓        |      ✗       |      ✓       | FRESH (hash changed) |
|       ✓        |      ✗       |      ✗       | SUSPICIOUS (cache?)  |
|       ✗        |      ✓       |      ✓       | FRESH (rebuild)      |
|       ✗        |      ✓       |      ✗       | FRESH (path changed) |
|       ✗        |      ✗       |      ✓       | FRESH (hash changed) |
|       ✗        |      ✗       |      ✗       | STALE                |

### Agent Implementation Requirements

1. **Build with ldflags**:

   ```nix
   ldflags = [
     "-X main.SourceCommit=${src.rev or "unknown"}"
   ];
   ```

2. **Report StorePath**:

   ```go
   // Linux
   storePath, _ := os.Readlink("/proc/self/exe")

   // macOS
   exe, _ := os.Executable()
   storePath = exe // Already resolved
   ```

3. **Report BinaryHash**:

   ```go
   func computeBinaryHash() string {
       exe, _ := os.Executable()
       f, _ := os.Open(exe)
       h := sha256.New()
       io.Copy(h, f)
       return hex.EncodeToString(h.Sum(nil))
   }
   ```

---

## Protocol Changes

### command_complete Message (Agent → Dashboard)

```go
type CommandCompleteMessage struct {
    Type    string `json:"type"`  // "command_complete"
    Payload struct {
        Command     string        `json:"command"`
        ExitCode    int           `json:"exit_code"`
        FreshStatus *UpdateStatus `json:"fresh_status"`  // NEW: Fresh status after command
    } `json:"payload"`
}
```

### Heartbeat Additions (Agent → Dashboard)

```go
type HeartbeatPayload struct {
    // ... existing fields ...
    SourceCommit string `json:"source_commit"`  // Layer 1: Git commit
    StorePath    string `json:"store_path"`     // Layer 2: Nix store path
    BinaryHash   string `json:"binary_hash"`    // Layer 3: SHA256 hash
}
```

### WebSocket Log Broadcast (Dashboard → Browsers)

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

### Example Log Sequences

#### Successful Switch

```
14:23:05  ℹ  hsb1 [IDLE→VALIDATING]     User clicked Switch
14:23:05  ℹ  hsb1 [PRE-CHECK]           Checking CanExecuteCommand...
14:23:05  ✓  hsb1 [PRE-CHECK]           CanExecuteCommand: PASS (host online, no pending command)
14:23:05  ℹ  hsb1 [PRE-CHECK]           Checking CanSwitch...
14:23:05  ✓  hsb1 [PRE-CHECK]           CanSwitch: PASS (git=ok, system=outdated)
14:23:05  ℹ  hsb1 [PRE-CHECK]           Capturing pre-state snapshot (generation=abc1234, agentVersion=2.0.0)
14:23:05  ℹ  hsb1 [VALIDATING→QUEUED]   Pre-checks passed, queueing command
14:23:05  ℹ  hsb1 [QUEUED→RUNNING]      Command sent to agent: nixos-rebuild switch --flake .#hsb1
14:24:30  ℹ  hsb1 [RUNNING→VALIDATING]  Command completed (exit code: 0)
14:24:30  ℹ  hsb1 [POST-CHECK]          Running ValidateSwitchResult...
14:24:30  ℹ  hsb1 [POST-CHECK]          Comparing: system.status before=outdated, after=ok
14:24:30  ✓  hsb1 [POST-CHECK]          ValidateSwitchResult: PASS (goal_achieved)
14:24:30  ✓  hsb1 [VALIDATING→SUCCESS]  Switch complete - system now up to date
```

#### Blocked Switch

```
14:25:00  ℹ  gpc0 [IDLE→VALIDATING]     User clicked Switch
14:25:00  ℹ  gpc0 [PRE-CHECK]           Checking CanExecuteCommand...
14:25:00  ✓  gpc0 [PRE-CHECK]           CanExecuteCommand: PASS (host online, no pending command)
14:25:00  ℹ  gpc0 [PRE-CHECK]           Checking CanSwitch...
14:25:00  ✗  gpc0 [PRE-CHECK]           CanSwitch: FAIL (git_outdated)
14:25:00  ⚠  gpc0 [VALIDATING→BLOCKED]  Cannot switch: Git is outdated, pull required first
```

#### Stale Binary Detected

```
14:30:00  ℹ  imac0 [POST-CHECK]         Checking agent binary freshness...
14:30:00  ℹ  imac0 [POST-CHECK]         Before: commit=abc1234, path=/nix/store/xxx, hash=sha256:...
14:30:00  ℹ  imac0 [POST-CHECK]         After:  commit=abc1234, path=/nix/store/xxx, hash=sha256:...
14:30:00  ⚠  imac0 [POST-CHECK]         STALE BINARY DETECTED: No change in commit, path, or hash
14:30:00  ℹ  imac0 [POST-CHECK]         Suggestion: Run nix-collect-garbage -d and switch again
```

### Log Level Configuration

| Level   | Icon | When Used                               |
| ------- | ---- | --------------------------------------- |
| DEBUG   | ·    | Internal state details (off by default) |
| INFO    | ℹ   | State transitions, progress updates     |
| SUCCESS | ✓    | Validation passed, goal achieved        |
| WARNING | ⚠   | Partial success, non-blocking issues    |
| ERROR   | ✗    | Failures, blocked actions               |

---

## Test Strategy (138 Tests)

### Test Summary

| Category               | P2800   | P2810  | Total   |
| ---------------------- | ------- | ------ | ------- |
| Unit Tests             | 50      | 18     | **68**  |
| Race Condition Tests   | 11      | -      | **11**  |
| Self-Healing Tests     | 6       | -      | **6**   |
| Post-Validation Timing | 7       | -      | **7**   |
| Integration Tests      | 17      | 11     | **28**  |
| E2E Mock Tests         | 6       | 3      | **9**   |
| E2E Fleet Tests        | 6       | 3      | **9**   |
| **TOTAL**              | **103** | **35** | **138** |

### File Structure

```
v2/tests/integration/
├── validators_test.go                    # P2800 unit tests (validators)
├── race_conditions_test.go               # Concurrency tests
├── t13_command_state_machine_test.go     # P2800 integration tests
├── t13_self_healing_test.go              # Orphaned state detection
├── t13_post_validation_timing_test.go    # Timing tests
├── t13_e2e_mock_test.go                  # Mock E2E tests
├── t13_e2e_fleet_test.go                 # Real fleet tests
├── t14_freshness_test.go                 # P2810 comparison logic
├── t14_agent_reporting_test.go           # P2810 agent reporting
└── t14_e2e_test.go                       # P2810 E2E tests

tests/specs/
├── T13-command-state-machine.md          # P2800 spec (human-readable)
└── T14-agent-binary-freshness.md         # P2810 spec (human-readable)
```

### Unit Tests: Validators (50 tests)

#### Pre-Validators

| Validator         | Tests | Cases                                                  |
| ----------------- | ----- | ------------------------------------------------------ |
| CanExecuteCommand | 3     | Online/no pending, offline, command pending            |
| CanPull           | 5     | Outdated, current, unknown, offline, pending           |
| CanSwitch         | 6     | System outdated, git outdated, current, agent outdated |
| CanTest           | 2     | Online, offline                                        |
| CanPullSwitch     | 4     | Git OR system outdated, both current, offline, unknown |

#### Post-Validators

| Validator                | Tests | Cases                                              |
| ------------------------ | ----- | -------------------------------------------------- |
| ValidatePullResult       | 5     | Success, partial, no change, exit ≠ 0, no snapshot |
| ValidateSwitchResult     | 6     | Success, agent updated, pending restart, failed    |
| ValidateTestResult       | 2     | Pass, fail                                         |
| ValidatePullSwitchResult | 5     | Both ok, git only, system only, neither, failed    |

#### Idempotency (5 tests)

- Each validator called twice → same result
- No side effects

### Unit Tests: Binary Freshness (18 tests)

| Layer       | Tests | Cases                                           |
| ----------- | ----- | ----------------------------------------------- |
| Commit      | 5     | Match, differ, agent unknown, expected unknown  |
| Store Path  | 4     | Changed, unchanged, empty, format validation    |
| Binary Hash | 4     | Changed, unchanged, empty, SHA256 format        |
| Combined    | 5     | All changed, none changed, suspicious scenarios |

### Race Condition Tests (11 tests)

| Category           | Tests | Scenarios                                      |
| ------------------ | ----- | ---------------------------------------------- |
| Rapid Clicks       | 2     | Same host (blocked), different hosts (allowed) |
| Heartbeat Timing   | 2     | During command, during post-validation         |
| Two Browsers       | 2     | Command blocking, state sync                   |
| Snapshot Integrity | 3     | Capture race, cleanup race, multi-host         |
| Agent Disconnect   | 2     | Mid-command, reconnect after                   |

### Self-Healing Tests (6 tests)

| Detection Type    | Tests | Thresholds                             |
| ----------------- | ----- | -------------------------------------- |
| Orphaned Snapshot | 3     | Detected, threshold, no false positive |
| Stuck RUNNING     | 3     | Detected, cleared, agent offline       |

### E2E Fleet Tests (9 tests)

| Platform | Host  | Tests | Scenarios                           |
| -------- | ----- | ----- | ----------------------------------- |
| NixOS    | gpc0  | 4     | Pull, switch, long-running, restart |
| macOS    | imac0 | 3     | Pull, switch, survives switch       |
| Cross    | both  | 2     | Bulk pull, network partition        |

### Test Execution

```bash
# Run all tests
cd v2 && go test -v -race ./tests/integration/...

# Run with coverage
cd v2 && go test -coverprofile=coverage.out ./tests/integration/...

# Run fleet tests (requires SSH access)
cd v2 && go test -v -tags=fleet ./tests/integration/... -run "Fleet"
```

---

## Implementation Plan

### Phase 1: Unit Tests & Fixtures (Days 1-3)

**Goal**: 100% validator and comparison logic coverage

1. Create test fixtures and helpers
2. Implement P2800 validators (50 tests)
3. Implement P2810 comparison logic (18 tests)

**Deliverable**: All unit tests passing, >95% coverage

### Phase 2: Race Condition & Self-Healing Tests (Days 4-5)

**Goal**: Concurrent access and edge cases covered

1. Implement race condition tests (11 tests)
2. Implement self-healing detection tests (6 tests)
3. Implement post-validation timing tests (7 tests)

**Deliverable**: All concurrency tests passing, no race detector warnings

### Phase 3: Integration Tests (Days 6-7)

**Goal**: State machine and agent reporting flows tested

1. Implement P2800 state machine tests (17 tests)
2. Implement P2810 agent reporting tests (11 tests)

**Deliverable**: All integration tests passing

### Phase 4: Agent & Dashboard Changes (Days 8-9)

**Goal**: Protocol changes implemented

Agent:

- Add `StorePath` to heartbeat
- Add `BinaryHash` to heartbeat
- Add `fresh_status` to `command_complete`
- Force status refresh before `command_complete`

Dashboard:

- Handle `command_complete` with `fresh_status`
- Fallback timeout logic
- 3-layer comparison logic
- Orphaned state detection

### Phase 5: E2E Tests (Days 10-12)

**Goal**: Real-world behavior verified

1. Implement mock E2E tests (9 tests)
2. Set up fleet test infrastructure
3. Implement fleet tests on gpc0 and imac0 (9 tests)

**Deliverable**: All E2E tests passing

### Phase 6: CI & Documentation (Days 13-14)

**Goal**: Automation and documentation complete

1. Update CI pipeline
2. Documentation complete

---

## Acceptance Criteria

### Pre-Validation

- [ ] Clicking action when precondition fails shows explanatory dialog
- [ ] Dialog offers sensible alternatives (Pull First, Force, Cancel)
- [ ] Force option bypasses pre-validation (for edge cases)

### Post-Validation

- [ ] Commands ending with exit 0 but goal not met show warning
- [ ] Commands achieving goal show success
- [ ] All validation results appear in System Log
- [ ] Partial success states clearly communicated

### Binary Freshness (P2810)

- [ ] Agent reports source commit in heartbeat
- [ ] Agent reports store path in heartbeat
- [ ] Agent reports binary hash in heartbeat
- [ ] Dashboard detects stale binary (3-layer verification)
- [ ] Log shows "stale_binary_detected" with guidance

### Idempotency

- [ ] Running same validator twice with same state returns same result
- [ ] Validators have no side effects
- [ ] Validators are unit tested (100% coverage)

### Verbose Logging

- [ ] Every state transition logged with WHAT, WHY, WHAT'S NEXT
- [ ] Pre-check logs show each validator name and result
- [ ] Post-check logs show before/after comparison values
- [ ] All logs appear in System Log in real-time

### Test Coverage

- [ ] All 138 tests passing
- [ ] No race detector warnings
- [ ] No flaky tests (run 3x, all pass)
- [ ] Fleet tests pass on gpc0 and imac0

---

## Risk Mitigation

### Risk 1: Flaky Tests

**Mitigation**:

- Use deterministic test data with fixtures
- Mock time-dependent operations
- Race detector enabled (`go test -race`)
- Run tests 3x in CI before merge

### Risk 2: Post-Validation Timing Race

**Mitigation**:

- Primary path: Agent sends `command_complete` with fresh_status included
- No waiting for heartbeat in primary path
- Fallback path with explicit timeouts

### Risk 3: Nix Cache Returns Stale Binary

**Mitigation**:

- 3-layer verification catches this
- Detection triggers even when we can't prevent it
- Log provides user guidance: "Run nix-collect-garbage -d"

### Risk 4: Fleet Tests Damage Real Hosts

**Mitigation**:

- Use only gpc0 and imac0 (dev machines)
- Timeout protection (10 minute max)
- No auto-destructive operations
- Rollback commands documented

---

## Related Documents

- [T13 Spec](../../tests/specs/T13-command-state-machine.md) - P2800 test specification
- [T14 Spec](../../tests/specs/T14-agent-binary-freshness.md) - P2810 test specification
- [PRD Agent Resilience](../PRD.md#critical-requirement-agent-resilience) - Why this matters
- [P4020 Tabbed Output Panel](./P4020-tabbed-output-panel.md) - Displays state machine logs

---

## Revision History

| Date       | Changes                                                       |
| ---------- | ------------------------------------------------------------- |
| 2025-12-21 | Initial P2800, P2810, test strategy created as separate files |
| 2025-12-22 | Refined test strategy with design decisions, 138 tests        |
| 2025-12-22 | Consolidated into single comprehensive document               |
