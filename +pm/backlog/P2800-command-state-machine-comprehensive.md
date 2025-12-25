# P2800 - Command State Machine (Comprehensive)

**Created**: 2025-12-21
**Updated**: 2025-12-25 (Aligned with Implementation Reality)
**Priority**: P2800 (High - Architecture)
**Status**: In Progress (~70% Backend Complete)
**Effort**: 16-18 days (expanded from 10-14 due to lifecycle integration)
**Depends on**: P2000 (Unified Host State - Done)

---

## 📊 Implementation Status (Dec 25)

| Component                 | Status     | Reality vs. Spec                                                            |
| :------------------------ | :--------- | :-------------------------------------------------------------------------- |
| **State Machine Engine**  | ✅ Done    | Core transitions and snapshotting implemented in `command_state.go`.        |
| **3-Layer Freshness**     | ✅ Done    | Agent reports Commit/Path/Hash; Dashboard verifies on reconnect.            |
| **Pre-Check Validators**  | ✅ Done    | All pre-validators implemented and wired into `/api/command`.               |
| **Reboot Integration**    | ✅ Done    | `ABORTED_BY_REBOOT` and `POST_REBOOT` recovery logic active.                |
| **Post-Check Validators** | 🟡 Partial | Validators exist but aren't yet called in the Hub's `handleStatus`.         |
| **Timeout UI**            | ❌ Pending | Frontend modals for Wait/Kill/Ignore actions are missing.                   |
| **Protocol Upgrade**      | ❌ Pending | Still using legacy `status` message; `command_complete` defined but unused. |

---

## Table of Contents

1. [Overview](#overview)
2. [Design Decisions](#design-decisions)
3. [Agent Lifecycle Integration](#agent-lifecycle-integration)
4. [Command Lifecycle State Machine](#command-lifecycle-state-machine)
5. [Validators Specification](#validators-specification)
6. [Agent Binary Freshness Detection](#agent-binary-freshness-detection-p2810)
7. [Command Timeout & Abort](#command-timeout--abort)
8. [Reboot Integration (P6900)](#reboot-integration-p6900)
9. [Protocol Changes](#protocol-changes)
10. [UI Integration](#ui-integration)
11. [Full Success Criteria](#full-success-criteria)
12. [Test Strategy](#test-strategy-170-tests)
13. [Implementation Plan](#implementation-plan)
14. [Acceptance Criteria](#acceptance-criteria)
15. [Risk Mitigation](#risk-mitigation)

---

## 1. Overview [✅ Backend Done]

### Scope: Command Lifecycle & Agent Stability

**This is NOT just a UX feature.** P2800 is a critical step toward achieving the PRD's #1 requirement:

> "After any operation — switch, pull, reboot, crash, network outage, power failure — the most recent working agent MUST be running and connected to the dashboard within 60 seconds of the host being reachable."

P2800 covers:

1. **Command Validation** - Pre/post-condition checking
2. **Command Lifecycle** - Full state machine from click to verified completion
3. **Agent Restart Integration** - Ties into exit 101, reconnection, binary verification
4. **Timeout & Abort** - Graceful handling of stuck commands
5. **Binary Freshness** - 3-layer detection of stale agent binaries

### Problem

Commands (Pull, Switch, Test) have multiple failure modes that create confusion and reduce fleet reliability:

**User Experience Problems:**

1. **Before**: Is this action appropriate right now?
2. **During**: What's the current progress?
3. **After**: Did the action achieve its goal?

**Agent Stability Problems (from PRD Known Failure Modes):**

| Failure Mode                         | Frequency  | Impact                  |
| ------------------------------------ | ---------- | ----------------------- |
| Agent runs old binary after switch   | >10 times  | Silent feature breakage |
| Agent dead after home-manager switch | Common     | Lost visibility/control |
| Pull doesn't update repo             | Occasional | Config drift            |
| Agent shows old generation           | Common     | Confusing status        |
| Isolated repo on wrong branch        | Occasional | Build failures          |

### Solution

Implement a **command state machine** with **full agent lifecycle integration**:

1. **Atomic, idempotent validation functions** for each phase
2. **Reconnection-based completion signal** for switch commands
3. **3-layer binary freshness detection** to catch stale agent binaries
4. **Graceful timeout handling** with user control
5. **Comprehensive test coverage** (165+ tests) including known failure modes

### Goal

**Measurable stability improvement:**

- 100% of switches result in verified fresh agent binary
- 0 silent failures (all failures visible with actionable guidance)
- Agent reconnects within 60s of switch completion
- Known failure modes have E2E test coverage

---

## Design Decisions

### Decision 1: Concurrent Command Handling

**Choice**: Block with "command pending" error

When user clicks a command while another is running on the same host:

- Return HTTP 409 with `code: "command_pending"`
- UI shows "Command already running" message
- No queueing, no cancellation of first command

### Decision 2: Switch Completion Signal

**Choice**: Reconnection-based completion (not message-based)

The agent **exits with code 101** after successful switch on NixOS (or gets killed by launchd on macOS). It cannot send a `command_complete` message because it's dead.

**Design**: Use agent reconnection as the definitive completion signal for switch commands.

```
SWITCH LIFECYCLE (NixOS):
  1. User clicks Switch
  2. Pre-validation passes
  3. Agent executes nixos-rebuild switch
  4. Switch exits 0
  5. Agent sends final status message
  6. Agent waits 500ms for message delivery
  7. Agent exits with code 101                    ← AGENT DIES HERE
  8. systemd restarts agent with new binary
  9. New agent connects to dashboard
  10. Dashboard receives registration with:
      - source_commit (new)
      - store_path (new)
      - binary_hash (new)
  11. Dashboard compares with pre-switch snapshot
  12. If binary changed: SUCCESS
  13. If binary unchanged: STALE_BINARY_DETECTED

SWITCH LIFECYCLE (macOS):
  1. User clicks Switch
  2. Pre-validation passes
  3. Agent executes home-manager switch in NEW SESSION (Setsid)
  4. home-manager calls launchctl bootout
  5. launchd kills agent                          ← AGENT DIES HERE
  6. Switch continues in detached session
  7. Switch completes
  8. launchd restarts agent with new binary
  9-13. Same as NixOS
```

**Key Insight**: For switch, "command complete" = "agent reconnected with new binary"

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

## 3. Agent Lifecycle Integration [✅ Backend Done]

> **Reality Check (Dec 25)**: The `Exit 101` mechanism is implemented in `v2/internal/agent/commands.go`. The dashboard correctly detects reconnection and performs 3-layer verification in `v2/internal/dashboard/hub.go`.

### The Exit 101 Mechanism

NixFleet agents self-restart after successful switch to pick up the new binary:

```go
// From v2/internal/agent/commands.go
if exitCode == 0 && (command == "switch" || command == "pull-switch") && runtime.GOOS != "darwin" {
    a.log.Info().Msg("switch completed successfully, restarting to pick up new binary")
    time.Sleep(500 * time.Millisecond)
    a.Shutdown()
    os.Exit(101) // Triggers RestartForceExitStatus in systemd
}
```

**NixOS systemd configuration:**

```nix
systemd.services.nixfleet-agent = {
  restartIfChanged = false;  # Don't restart DURING switch
  stopIfChanged = false;
  serviceConfig = {
    Restart = "always";
    RestartSec = 3;
    RestartForceExitStatus = "101";  # Exit 101 = force restart
  };
};
```

**macOS launchd configuration:**

```nix
launchd.agents.nixfleet-agent = {
  config = {
    KeepAlive = true;   # Restart on any exit
    RunAtLoad = true;   # Start on login
  };
};
```

### Agent Reconnection Flow

When agent reconnects after restart, dashboard must:

1. **Detect it's a reconnection** (same host_id, different connection)
2. **Check for pending switch verification** (was switch running before disconnect?)
3. **Compare binary freshness** (3-layer verification)
4. **Update command state** (AWAITING_RECONNECT → SUCCESS or STALE_BINARY)

```go
// Dashboard: On agent registration
func (s *Server) handleAgentRegistration(conn *WebSocket, reg RegistrationPayload) {
    host := s.getOrCreateHost(reg.HostID)

    // Check if we were waiting for reconnection after switch
    if host.CommandState == "AWAITING_RECONNECT" {
        preSnapshot := s.cmdStateMachine.GetSnapshot(host.ID)

        // 3-layer binary freshness check
        commitChanged := preSnapshot.SourceCommit != reg.SourceCommit
        pathChanged := preSnapshot.StorePath != reg.StorePath
        hashChanged := preSnapshot.BinaryHash != reg.BinaryHash

        if pathChanged || hashChanged {
            // Binary definitely changed - SUCCESS
            s.cmdStateMachine.TransitionTo(host.ID, "SUCCESS",
                "Agent restarted with new binary")
        } else if commitChanged {
            // Commit changed but binary didn't - SUSPICIOUS
            s.cmdStateMachine.TransitionTo(host.ID, "SUSPICIOUS",
                "Source commit changed but binary unchanged (cache issue?)")
        } else {
            // Nothing changed - STALE BINARY
            s.cmdStateMachine.TransitionTo(host.ID, "STALE_BINARY",
                "Agent running old binary - run nix-collect-garbage -d and switch again")
        }
    }
}
```

### State Transitions for Switch

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SWITCH COMMAND STATE MACHINE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌───────┐   ┌───────────┐   ┌─────────┐   ┌─────────┐                     │
│   │ IDLE  │──▶│ VALIDATING│──▶│ QUEUED  │──▶│ RUNNING │                     │
│   └───────┘   └─────┬─────┘   └─────────┘   └────┬────┘                     │
│       ▲             │                            │                          │
│       │        ┌────▼────┐                       │                          │
│       │        │ BLOCKED │                       │ exit 0                   │
│       │        └─────────┘                       ▼                          │
│       │                              ┌───────────────────┐                  │
│       │                              │ AWAITING_RECONNECT│◀─── exit 101     │
│       │                              │ (agent died)      │     (NixOS)      │
│       │                              └─────────┬─────────┘     or killed    │
│       │                                        │               (macOS)      │
│       │                                        │                            │
│       │         ┌──────────────────────────────┼──────────────────┐         │
│       │         │                              │                  │         │
│       │         ▼                              ▼                  ▼         │
│   ┌───┴─────────────┐              ┌───────────────┐    ┌─────────────┐     │
│   │     SUCCESS     │              │  STALE_BINARY │    │   TIMEOUT   │     │
│   │ (binary fresh)  │              │  (binary old) │    │ (no reconn) │     │
│   └─────────────────┘              └───────────────┘    └─────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Pull/Test Completion (Non-Switch Commands)

For commands that don't restart the agent:

```
PULL LIFECYCLE:
  1. User clicks Pull
  2. Pre-validation passes
  3. Agent executes git fetch + reset --hard
  4. Agent forces status refresh
  5. Agent sends command_complete { exit_code, fresh_status }
  6. Dashboard runs post-validation immediately
  7. SUCCESS or PARTIAL

TEST LIFECYCLE:
  1. User clicks Test
  2. Pre-validation passes
  3. Agent executes test scripts
  4. Agent sends command_complete { exit_code, output }
  5. Dashboard evaluates test results
  6. TEST_PASSED or TEST_FAILED
```

**Key Difference**: Pull and Test use `command_complete` message. Switch uses reconnection.

---

## 4. Command Lifecycle State Machine [✅ Backend Done]

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

## 5. Validators Specification [🟡 Partial]

> **Reality Check (Dec 25)**: All `CanExecute`, `CanPull`, etc. functions are implemented and active. `ValidateResult` functions exist but need to be wired into the Hub's message handling for Pull/Test commands.

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

## 6. Agent Binary Freshness Detection (P2810) [✅ Done]

> **Reality Check (Dec 25)**: 3-layer verification is fully active. Agents report Commit, Path, and Hash in registration, and the Dashboard compares them against the snapshot.

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

## 7. Command Timeout & Abort [🟡 Partial]

> **Reality Check (Dec 25)**: The timeout loop in `server.go` and `CheckTimeouts` in `command_state.go` are functional. The missing link is the UI modal for user intervention (Wait/Kill/Ignore).

### Timeout Configuration

| Command                  | Warning | Hard Timeout | Justification                        |
| ------------------------ | ------- | ------------ | ------------------------------------ |
| Pull                     | 2 min   | 5 min        | Network fetch + reset should be fast |
| Switch                   | 10 min  | 30 min       | Large rebuilds can take time         |
| Pull-Switch              | 12 min  | 35 min       | Combined operation                   |
| Test                     | 5 min   | 10 min       | Test suites vary                     |
| Reconnect (after switch) | 30s     | 90s          | Agent restart should be quick        |

### Timeout States

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         TIMEOUT HANDLING                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────┐                                                               │
│   │ RUNNING │                                                               │
│   └────┬────┘                                                               │
│        │                                                                    │
│        ├──── warning timeout ────▶ ┌──────────────────┐                     │
│        │                           │ RUNNING_WARNING  │                     │
│        │                           │ "Taking longer   │                     │
│        │                           │  than expected"  │                     │
│        │                           └────────┬─────────┘                     │
│        │                                    │                               │
│        └──── hard timeout ─────────────────▶│                               │
│                                             ▼                               │
│                                    ┌──────────────────┐                     │
│                                    │ TIMEOUT_PENDING  │                     │
│                                    │ User chooses:    │                     │
│                                    │ [Wait] [Kill]    │                     │
│                                    │ [Ignore]         │                     │
│                                    └────────┬─────────┘                     │
│                    ┌────────────────────────┼────────────────┐              │
│                    │                        │                │              │
│                    ▼                        ▼                ▼              │
│          ┌─────────────────┐    ┌───────────────┐    ┌─────────────┐        │
│          │ Still RUNNING   │    │ KILLED        │    │ IGNORED     │        │
│          │ (extend timeout)│    │ (SIGTERM sent)│    │ (no action) │        │
│          └─────────────────┘    └───────────────┘    └─────────────┘        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### User Actions on Timeout

| Action               | Behavior                                       | When to Use                      |
| -------------------- | ---------------------------------------------- | -------------------------------- |
| **Wait +5min**       | Extend timeout, continue monitoring            | Large rebuild in progress        |
| **Wait +30min**      | Extend significantly                           | Known slow operation             |
| **Kill Process**     | Send SIGTERM via agent, then SIGKILL if needed | Stuck process                    |
| **Ignore**           | Mark as IGNORED, stop monitoring               | False alarm, manual intervention |
| **Abort & Rollback** | Kill + suggest rollback command                | Failed switch                    |

### Reconnect Timeout (Switch-Specific)

After switch command exits:

1. **Agent disconnects** (exit 101 or killed)
2. **Dashboard starts reconnect timer** (90s default)
3. **If agent reconnects within timeout**: Verify binary freshness
4. **If timeout expires**:

```
┌─────────────────────────────────────────────────────────────────┐
│              ⚠ Switch Timeout                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Agent has not reconnected after switch.                        │
│                                                                 │
│  Possible causes:                                               │
│  • Switch is still running (building derivations)               │
│  • Agent failed to start (check systemd/launchd logs)           │
│  • Network issue                                                │
│  • Host rebooted during switch                                  │
│                                                                 │
│  Suggested actions:                                             │
│  1. SSH to host and check: journalctl -u nixfleet-agent         │
│  2. Verify switch status: systemctl status nixfleet-agent       │
│  3. Manual restart: sudo systemctl restart nixfleet-agent       │
│                                                                 │
│  [Wait +5min] [Mark Offline] [SSH Guide]                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Kill Process Flow

When user clicks "Kill Process":

```go
// Dashboard sends kill command
type KillCommandMessage struct {
    Type    string `json:"type"`  // "kill_command"
    Payload struct {
        HostID  string `json:"host_id"`
        Signal  string `json:"signal"`  // "SIGTERM" or "SIGKILL"
        PID     int    `json:"pid,omitempty"`  // Optional: specific PID
    } `json:"payload"`
}

// Agent handles kill
func (a *Agent) handleKillCommand(signal string) {
    if a.commandPID == nil {
        a.log.Warn().Msg("no command PID to kill")
        return
    }

    pid := *a.commandPID

    switch signal {
    case "SIGTERM":
        syscall.Kill(pid, syscall.SIGTERM)
        // Wait 5s, then SIGKILL if still running
        go func() {
            time.Sleep(5 * time.Second)
            if a.isProcessRunning(pid) {
                syscall.Kill(pid, syscall.SIGKILL)
            }
        }()
    case "SIGKILL":
        syscall.Kill(pid, syscall.SIGKILL)
    }

    a.sendOutput("⚠️ Command terminated by user", "stderr")
}
```

### Timeout Logging

Every timeout event is logged verbosely:

```
14:35:00  ⧖  hsb1 [RUNNING]           Switch running for 10m (warning threshold)
14:35:00  ⚠  hsb1 [RUNNING_WARNING]   Taking longer than expected - notify user
14:45:00  ⧖  hsb1 [RUNNING_WARNING]   Switch running for 20m (still below hard timeout)
14:55:00  ✗  hsb1 [TIMEOUT_PENDING]   Hard timeout (30m) reached - awaiting user action
14:55:05  ℹ  hsb1 [TIMEOUT_PENDING]   User selected: Wait +5min
14:55:05  ⧖  hsb1 [RUNNING]           Timeout extended to 35m
```

---

## 8. Reboot Integration (P6900) [✅ Backend Done]

P6900 (Forced Reboot with TOTP) is the **nuclear option** for completely stuck hosts. It bypasses the command state machine but P2800 must be aware of it.

> **Reality Check (Dec 25)**: `HandleRebootTriggered` and `HandlePostRebootReconnect` are implemented in `v2/internal/dashboard/command_state.go`. Reboots during commands correctly clear snapshots and transition to recovery states.

### Escalation Path

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    TIMEOUT ESCALATION PATH                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Command Running                                                           │
│        │                                                                    │
│        ▼                                                                    │
│   ┌────────────────┐                                                        │
│   │ TIMEOUT_PENDING│                                                        │
│   └───────┬────────┘                                                        │
│           │                                                                 │
│           ├──── Wait ────▶ Continue monitoring                              │
│           │                                                                 │
│           ├──── Kill ────▶ Send SIGTERM/SIGKILL                             │
│           │                     │                                           │
│           │                     ├── Success ──▶ Command terminated          │
│           │                     │                                           │
│           │                     └── Failed ───▶  ┌──────────────────────┐   │
│           │                          (no resp)   │ KILL_FAILED          │   │
│           │                                      │ "Agent unresponsive" │   │
│           │                                      │                      │   │
│           │                                      │ [Reboot Host] ◀──────│   │
│           │                                      │ (requires TOTP)      │   │
│           │                                      └──────────┬───────────┘   │
│           │                                                 │               │
│           │                                                 ▼               │
│           │                                      ┌──────────────────────┐   │
│           │                                      │ ABORTED_BY_REBOOT    │   │
│           │                                      │ (host rebooting)     │   │
│           │                                      └──────────────────────┘   │
│           │                                                                 │
│           └──── Ignore ──▶ Stop monitoring, mark IGNORED                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### UI: Timeout Dialog with Reboot Option

```
┌─────────────────────────────────────────────────────────────────┐
│              ⚠ Command Timeout                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Switch on hsb1 has exceeded the 30-minute timeout.            │
│                                                                 │
│  [Wait +5min] [Wait +30min] [Kill Process] [Ignore]            │
│                                                                 │
│  ─────────────────────────────────────────────────────────────  │
│                                                                 │
│  If Kill Process fails (agent unresponsive):                   │
│                                                                 │
│  [🔒 Reboot Host]  ← Requires TOTP, last resort                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

After Kill fails:

```
┌─────────────────────────────────────────────────────────────────┐
│              ⚠ Kill Failed - Agent Unresponsive                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  The agent on hsb1 is not responding to kill command.           │
│  The process may be stuck in an uninterruptible state.          │
│                                                                 │
│  Options:                                                       │
│  • Wait for the process to complete naturally                   │
│  • SSH to host and manually investigate                         │
│  • Reboot the host (nuclear option)                             │
│                                                                 │
│  [Wait] [SSH Guide] [🔒 Reboot Host]                            │
│                                                                 │
│  ⚠ Reboot requires TOTP and will immediately restart the host   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### State: ABORTED_BY_REBOOT

When reboot is triggered via P6900, P2800 must:

```go
// Called when reboot command is sent to a host with pending command
func (sm *CommandStateMachine) HandleRebootTriggered(hostID string) {
    state := sm.GetState(hostID)

    if state == nil || state.State == "idle" {
        // No pending command, nothing to clean up
        return
    }

    // Log the abort
    sm.Log(LogEntry{
        Level:   LogLevelWarning,
        HostID:  hostID,
        State:   fmt.Sprintf("%s→ABORTED_BY_REBOOT", state.State),
        Message: fmt.Sprintf("Command '%s' aborted due to host reboot", state.Command),
        Code:    "aborted_by_reboot",
    })

    // Clear snapshot (no longer valid after reboot)
    sm.ClearSnapshot(hostID)

    // Mark state
    sm.TransitionTo(hostID, "ABORTED_BY_REBOOT", "Host reboot initiated")

    // Store the aborted command for post-reboot logging
    sm.SetPendingRebootRecovery(hostID, state.Command)
}
```

### Post-Reboot Detection

When agent reconnects after a reboot:

```go
func (s *Server) handleAgentRegistration(conn *WebSocket, reg RegistrationPayload) {
    host := s.getOrCreateHost(reg.HostID)

    // Check for pending reboot recovery
    if abortedCommand := s.cmdStateMachine.GetPendingRebootRecovery(host.ID); abortedCommand != "" {
        // Log the recovery
        s.cmdStateMachine.Log(LogEntry{
            Level:   LogLevelWarning,
            HostID:  host.ID,
            State:   "POST_REBOOT",
            Message: fmt.Sprintf("Host rebooted during '%s'. Manual verification may be needed.", abortedCommand),
            Code:    "post_reboot_recovery",
        })

        // Clear the pending recovery marker
        s.cmdStateMachine.ClearPendingRebootRecovery(host.ID)

        // Transition to IDLE (don't auto-retry!)
        s.cmdStateMachine.TransitionTo(host.ID, "IDLE", "Recovered after reboot")

        // Notify UI
        s.broadcastToast(host.ID, "warning",
            fmt.Sprintf("%s rebooted during %s. Verify system state manually.",
                host.Hostname, abortedCommand))
    }

    // Continue with normal registration...
}
```

### Logging: Reboot Events

```
15:30:00  ⧖  hsb1 [RUNNING]           Switch running for 30m (hard timeout)
15:30:00  ✗  hsb1 [TIMEOUT_PENDING]   Awaiting user action
15:30:15  ℹ  hsb1 [TIMEOUT_PENDING]   User selected: Kill Process
15:30:15  ℹ  hsb1 [KILLING]           Sending SIGTERM to PID 12345
15:30:20  ⧖  hsb1 [KILLING]           Waiting for process to terminate...
15:30:35  ✗  hsb1 [KILL_FAILED]       Process not responding after 20s
15:31:00  ⚠  hsb1 [KILL_FAILED]       User selected: Reboot Host
15:31:00  ⚠  hsb1 [RUNNING→ABORTED]   Command 'switch' aborted due to host reboot
15:31:00  ℹ  hsb1 [ABORTED_BY_REBOOT] Snapshot cleared, awaiting host reconnection
... host reboots ...
15:32:30  ℹ  hsb1 [POST_REBOOT]       Host reconnected after reboot
15:32:30  ⚠  hsb1 [POST_REBOOT]       Host rebooted during 'switch'. Manual verification may be needed.
15:32:30  ✓  hsb1 [POST_REBOOT→IDLE]  Recovered after reboot
```

### Important: No Auto-Retry

After reboot recovery, we explicitly **do NOT auto-retry** the command because:

1. **Unknown State**: We don't know if the command partially completed
2. **User Intent**: User chose to reboot, not retry
3. **Safety**: Auto-retry could cause loops (reboot → retry → stuck → reboot)
4. **Manual Check**: User should verify system state before re-running

If the user wants to retry, they click the command button again (goes through normal pre-validation).

### P6900 Integration Point

P6900 must call `HandleRebootTriggered()` before sending reboot command:

```go
// In P6900's handleReboot handler, before sending command to agent
if cmdState := s.cmdStateMachine.GetState(hostID); cmdState != nil && cmdState.State != "idle" {
    s.cmdStateMachine.HandleRebootTriggered(hostID)
}

// Then send reboot command to agent
```

---

## 9. Protocol Changes [🟡 Partial]

> **Reality Check (Dec 25)**: Heartbeat additions for freshness are DONE. The `command_complete` message is defined but the Agent and Dashboard still use the legacy `status` message for Pull/Test commands.

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

## 10. UI Integration [🟡 Partial]

> **Reality Check (Dec 25)**: Pre-check dialogs and progress dots are implemented. Post-validation toasts and the timeout action modal are still missing.

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
                         ┌───────────────────────────────┐
                         │ ⚠ Cannot Switch               │
                         │                               │
                         │ Git is outdated.              │
                         │ Pull first to get latest.     │
                         │                               │
                         │ [Cancel] [Pull First] [Force] │
                         └───────────────────────────────┘
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

## Full Success Criteria

### Pull Command

```
┌─────────────────────────────────────────────────────────────────┐
│  PULL SUCCESS requires ALL of:                                  │
├─────────────────────────────────────────────────────────────────┤
│  □ Pre-validation passed                                        │
│  □ git fetch exit code 0                                        │
│  □ git reset --hard exit code 0                                 │
│  □ Agent sent command_complete                                  │
│  □ Git compartment status = "ok"                                │
│  ─────────────────────────────────────────────────────────────  │
│  PARTIAL if:                                                    │
│  • Exit code 0 but Git compartment still shows outdated         │
│    (GitHub Pages cache delay)                                   │
│  ─────────────────────────────────────────────────────────────  │
│  FAILED if:                                                     │
│  • Pre-validation failed                                        │
│  • Exit code ≠ 0                                                │
│  • Timeout exceeded                                             │
└─────────────────────────────────────────────────────────────────┘
```

### Switch Command

```
┌─────────────────────────────────────────────────────────────────┐
│  SWITCH SUCCESS requires ALL of:                                │
├─────────────────────────────────────────────────────────────────┤
│  □ Pre-validation passed                                        │
│  □ nixos-rebuild/home-manager switch exit code 0                │
│  □ Agent disconnected (exit 101 or killed)                      │
│  □ Agent reconnected within 90s                                 │
│  □ Binary hash CHANGED (3-layer verification)                   │
│  □ System compartment status = "ok"                             │
│  ─────────────────────────────────────────────────────────────  │
│  STALE_BINARY if:                                               │
│  • Exit code 0, agent reconnected, but binary unchanged         │
│  • Guidance: "Run nix-collect-garbage -d and switch again"      │
│  ─────────────────────────────────────────────────────────────  │
│  TIMEOUT if:                                                    │
│  • Agent did not reconnect within timeout                       │
│  • Guidance: SSH to host, check journalctl                      │
│  ─────────────────────────────────────────────────────────────  │
│  FAILED if:                                                     │
│  • Pre-validation failed                                        │
│  • Exit code ≠ 0                                                │
└─────────────────────────────────────────────────────────────────┘
```

### Pull-Switch Command

```
┌─────────────────────────────────────────────────────────────────┐
│  PULL-SWITCH SUCCESS requires ALL of:                           │
├─────────────────────────────────────────────────────────────────┤
│  □ Pull phase: git fetch + reset exit code 0                    │
│  □ Switch phase: nixos-rebuild exit code 0                      │
│  □ Agent disconnected and reconnected                           │
│  □ Binary hash CHANGED                                          │
│  □ Git compartment status = "ok"                                │
│  □ System compartment status = "ok"                             │
│  ─────────────────────────────────────────────────────────────  │
│  PARTIAL_GIT_ONLY if:                                           │
│  • Pull succeeded but switch failed                             │
│  ─────────────────────────────────────────────────────────────  │
│  PARTIAL_SYSTEM_ONLY if:                                        │
│  • System updated but Git still shows outdated (cache)          │
└─────────────────────────────────────────────────────────────────┘
```

### Test Command

```
┌─────────────────────────────────────────────────────────────────┐
│  TEST SUCCESS requires ALL of:                                  │
├─────────────────────────────────────────────────────────────────┤
│  □ Pre-validation passed                                        │
│  □ All test scripts exit code 0                                 │
│  □ Agent sent command_complete with results                     │
│  ─────────────────────────────────────────────────────────────  │
│  TEST_FAILED if:                                                │
│  • Any test script exit code ≠ 0                                │
│  • Output shows which tests failed                              │
└─────────────────────────────────────────────────────────────────┘
```

### Success Verification Timing

| Command     | When Verified                        | How Verified                             |
| ----------- | ------------------------------------ | ---------------------------------------- |
| Pull        | Immediately after `command_complete` | Compare Git status before/after          |
| Switch      | After agent reconnection             | 3-layer binary freshness + System status |
| Pull-Switch | After agent reconnection             | Git + System + Binary freshness          |
| Test        | Immediately after `command_complete` | Parse exit codes                         |

---

## Test Strategy (170+ Tests)

### Test Summary

| Category                     | P2800   | P2810  | New    | Total   |
| ---------------------------- | ------- | ------ | ------ | ------- |
| Unit Tests                   | 50      | 18     | -      | **68**  |
| Race Condition Tests         | 11      | -      | -      | **11**  |
| Self-Healing Tests           | 6       | -      | -      | **6**   |
| Post-Validation Timing       | 7       | -      | -      | **7**   |
| Timeout & Abort Tests        | -       | -      | 8      | **8**   |
| Reconnection Tests           | -       | -      | 7      | **7**   |
| Reboot Integration Tests     | -       | -      | 5      | **5**   |
| Integration Tests            | 17      | 11     | -      | **28**  |
| E2E Mock Tests               | 6       | 3      | -      | **9**   |
| E2E Fleet Tests              | 6       | 3      | -      | **9**   |
| E2E Known Failure Mode Tests | -       | -      | 12     | **12**  |
| **TOTAL**                    | **103** | **35** | **32** | **170** |

### File Structure

```
v2/tests/integration/
├── validators_test.go                    # P2800 unit tests (validators)
├── race_conditions_test.go               # Concurrency tests
├── t13_command_state_machine_test.go     # P2800 integration tests
├── t13_self_healing_test.go              # Orphaned state detection
├── t13_post_validation_timing_test.go    # Timing tests
├── t13_timeout_abort_test.go             # NEW: Timeout & abort handling
├── t13_reconnection_test.go              # NEW: Agent reconnection verification
├── t13_reboot_integration_test.go        # NEW: P6900 reboot integration
├── t13_e2e_mock_test.go                  # Mock E2E tests
├── t13_e2e_fleet_test.go                 # Real fleet tests
├── t13_known_failures_test.go            # NEW: PRD failure mode tests
├── t14_freshness_test.go                 # P2810 comparison logic
├── t14_agent_reporting_test.go           # P2810 agent reporting
└── t14_e2e_test.go                       # P2810 E2E tests

tests/specs/
├── T13-command-state-machine.md          # P2800 spec (human-readable)
├── T13-known-failure-modes.md            # NEW: Failure mode test spec
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

### Timeout & Abort Tests (8 tests)

| Scenario        | Tests | Description                                      |
| --------------- | ----- | ------------------------------------------------ |
| Warning Timeout | 2     | Pull and switch trigger warning at threshold     |
| Hard Timeout    | 2     | Transition to TIMEOUT_PENDING after hard timeout |
| User Actions    | 3     | Wait, Kill, Ignore behaviors work correctly      |
| Kill Signal     | 1     | SIGTERM followed by SIGKILL if needed            |

### Reconnection Tests (7 tests)

| Scenario          | Tests | Description                                      |
| ----------------- | ----- | ------------------------------------------------ |
| Normal Reconnect  | 2     | Agent reconnects after exit 101, verified fresh  |
| Stale Binary      | 2     | Agent reconnects but binary unchanged → detected |
| Reconnect Timeout | 2     | Agent fails to reconnect → TIMEOUT state         |
| macOS Launchd     | 1     | Agent survives home-manager switch via Setsid    |

### Reboot Integration Tests (5 tests)

| Scenario                   | Tests | Description                                     |
| -------------------------- | ----- | ----------------------------------------------- |
| Reboot During Command      | 2     | Triggers ABORTED_BY_REBOOT, clears snapshot     |
| Post-Reboot Recovery       | 2     | Agent reconnects, logs warning, returns to IDLE |
| No Auto-Retry After Reboot | 1     | Verify command is NOT auto-retried after reboot |

### E2E Known Failure Mode Tests (12 tests)

These tests specifically target the failure modes documented in the PRD:

| PRD Failure Mode                     | Tests | Test Description                                          |
| ------------------------------------ | ----- | --------------------------------------------------------- |
| Agent runs old binary after switch   | 3     | Simulate Nix cache returning old binary, verify detection |
| Agent dead after home-manager switch | 2     | Verify macOS agent survives via Setsid, launchd restarts  |
| Pull doesn't update repo             | 2     | Force git conflict scenario, verify reset --hard fixes    |
| Agent shows old generation           | 2     | Verify generation updates after successful switch         |
| Isolated repo on wrong branch        | 2     | Force diverged repo, verify reset --hard origin/main      |
| Network partition during switch      | 1     | Disconnect during switch, verify recovery on reconnect    |

**Test Implementation Notes:**

```go
// t13_known_failures_test.go

func TestStaleBinaryAfterSwitch(t *testing.T) {
    // Setup: Mock agent with same binary hash before/after
    // Action: Trigger switch, wait for reconnect
    // Assert: State transitions to STALE_BINARY
    // Assert: Log contains guidance message
}

func TestMacOSAgentSurvivesSwitch(t *testing.T) {
    // Requires: imac0 (real macOS host)
    // Setup: Record agent PID
    // Action: Trigger home-manager switch
    // Assert: Agent reconnects within 90s
    // Assert: Binary hash changed
    // Assert: Old PID no longer running
}

func TestIsolatedRepoRecovery(t *testing.T) {
    // Setup: Manually create merge conflict in isolated repo
    // Action: Trigger pull
    // Assert: Pull succeeds (reset --hard)
    // Assert: Repo matches origin/main exactly
}

func TestNetworkPartitionRecovery(t *testing.T) {
    // Requires: gpc0 (NixOS host)
    // Setup: Start switch
    // Action: Disconnect network for 30s mid-switch
    // Assert: Switch continues (was already in progress)
    // Action: Reconnect network
    // Assert: Agent reconnects with new binary
}
```

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

**Total Effort**: 16-18 days (increased from 10-14 due to expanded scope)

### Phase 1: Unit Tests & Fixtures (Days 1-3) [✅ COMPLETED]

**Goal**: 100% validator and comparison logic coverage

1. Create test fixtures and helpers
2. Implement P2800 validators (50 tests)
3. Implement P2810 comparison logic (18 tests)

### Phase 2: Race Condition & Self-Healing Tests (Days 4-5) [✅ COMPLETED]

**Goal**: Concurrent access and edge cases covered

1. Implement race condition tests (11 tests)
2. Implement self-healing detection tests (6 tests)
3. Implement post-validation timing tests (7 tests)

### Phase 3: Timeout & Abort Backend (Day 6) [✅ COMPLETED]

**Goal**: Graceful timeout handling (Backend)

1. Implement timeout state machine tests (8 tests)
2. Implement kill command handling (Backend)
3. Implement state machine loop in `server.go`

### Phase 4: Reconnection Backend (Days 7-8) [✅ COMPLETED]

**Goal**: Agent lifecycle integration (Backend)

1. Implement reconnection detection tests (7 tests)
2. Implement `EnterAwaitingReconnect` and verification logic
3. Test stale binary detection on reconnect

### Phase 5: Integration Tests (Days 9-10) [✅ COMPLETED]

**Goal**: State machine and agent reporting flows (Backend)

1. Implement P2800 state machine tests (17 tests)
2. Implement P2810 agent reporting tests (11 tests)
3. Integrate timeout and reconnection logic

### Phase 6: Agent & Dashboard Changes (Days 11-13) [🟡 IN PROGRESS]

**Goal**: Protocol and lifecycle changes implemented

**Agent Changes:**

- ✅ Add `StorePath` to heartbeat (computed on startup)
- ✅ Add `BinaryHash` to heartbeat (SHA256 of binary)
- 🟡 Add `command_complete` message for non-switch commands
- ✅ Keep exit 101 mechanism for switch commands
- ✅ Implement kill command handler (SIGTERM/SIGKILL)

**Dashboard Changes:**

- ✅ Implement AWAITING_RECONNECT state
- ✅ Handle agent reconnection with binary verification
- 🟡 Implement timeout action API (`/api/hosts/{id}/timeout-action`)
- ✅ 3-layer binary freshness comparison on reconnect
- ✅ Orphaned state detection (RUNNING without agent)

### Phase 10: The Last Mile (Remaining Wiring) [🆕 NEXT]

**Goal**: Close the loop between Backend, Protocol, and UI

1. **Post-Validation**: Wire `sm.RunPostChecks` into `Hub.handleStatus` for Pull/Test.
2. **Protocol**: Switch Agent/Dashboard to `TypeCommandComplete` messages.
3. **UI**: Implement the `timeoutActionDialog` modal in `dashboard.templ`.
4. **Polish**: Finalize state machine log broadcasts for real-time UI feedback.

### Phase 7: Known Failure Mode E2E Tests (Days 14-15)

**Goal**: PRD failure modes have test coverage

1. Set up failure simulation infrastructure
2. Implement known failure mode tests (12 tests)
3. Test on real fleet (gpc0, imac0)
4. Verify recovery paths work

**Deliverable**: All PRD failure modes covered by E2E tests

### Phase 8: E2E Fleet Tests & Polish (Days 16-17)

**Goal**: Real-world behavior verified

1. Implement mock E2E tests (9 tests)
2. Implement standard fleet tests on gpc0 and imac0 (9 tests)
3. Run full test suite 3x to catch flaky tests
4. Performance testing (latency, memory)

**Deliverable**: All E2E tests passing, no flaky tests

### Phase 9: CI & Documentation (Day 18)

**Goal**: Automation and documentation complete

1. Update CI pipeline with new test categories
2. Add fleet test stage (scheduled, not on every PR)
3. Update PRD with resolved failure modes
4. Documentation complete

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

### Agent Lifecycle Integration

- [ ] Switch enters AWAITING_RECONNECT after agent disconnects
- [ ] Agent reconnection triggers binary freshness verification
- [ ] SUCCESS only after agent reconnects with fresh binary
- [ ] Reconnect timeout (90s) triggers TIMEOUT state
- [ ] Exit 101 mechanism verified on NixOS hosts
- [ ] Setsid mechanism verified on macOS hosts

### Timeout & Abort

- [ ] Warning shown at warning timeout threshold
- [ ] Hard timeout triggers TIMEOUT_PENDING with user options
- [ ] User can extend timeout (Wait +5min, +30min)
- [ ] User can kill process (SIGTERM → SIGKILL)
- [ ] User can ignore timeout (mark IGNORED)
- [ ] Kill command delivered to agent and executed

### Full Success Criteria

- [ ] Pull SUCCESS requires exit 0 + Git compartment green
- [ ] Switch SUCCESS requires exit 0 + reconnect + fresh binary + System green
- [ ] Pull-Switch SUCCESS requires all of the above combined
- [ ] Test SUCCESS requires exit 0 for all test scripts

### Reboot Integration (P6900)

- [ ] Reboot Host option shown in KILL_FAILED state
- [ ] Reboot triggers ABORTED_BY_REBOOT state transition
- [ ] Pending snapshot is cleared on reboot
- [ ] Aborted command is logged with clear message
- [ ] Post-reboot: agent reconnection detected
- [ ] Post-reboot: warning logged about manual verification
- [ ] Post-reboot: state transitions to IDLE (not auto-retry)
- [ ] Post-reboot: UI toast notification shown

### Idempotency

- [ ] Running same validator twice with same state returns same result
- [ ] Validators have no side effects
- [ ] Validators are unit tested (100% coverage)

### Verbose Logging

- [ ] Every state transition logged with WHAT, WHY, WHAT'S NEXT
- [ ] Pre-check logs show each validator name and result
- [ ] Post-check logs show before/after comparison values
- [ ] All logs appear in System Log in real-time
- [ ] Timeout events logged with timestamps

### Test Coverage

- [ ] All 170 tests passing
- [ ] No race detector warnings
- [ ] No flaky tests (run 3x, all pass)
- [ ] Fleet tests pass on gpc0 and imac0
- [ ] All PRD Known Failure Modes have E2E test coverage
- [ ] Reboot integration tests pass

---

## Risk Mitigation

### Risk 1: Flaky Tests

**Mitigation**:

- Use deterministic test data with fixtures
- Mock time-dependent operations
- Race detector enabled (`go test -race`)
- Run tests 3x in CI before merge

### Risk 2: Switch Completion Detection

**Mitigation**:

- Reconnection-based design eliminates timing races
- Agent disconnection is the trigger, not a message
- Clear state machine: RUNNING → AWAITING_RECONNECT → SUCCESS/STALE/TIMEOUT
- Fallback: reconnect timeout with user guidance

### Risk 3: Nix Cache Returns Stale Binary

**Mitigation**:

- 3-layer verification catches this
- Detection triggers even when we can't prevent it
- Log provides user guidance: "Run nix-collect-garbage -d"
- E2E test specifically targets this failure mode

### Risk 4: Fleet Tests Damage Real Hosts

**Mitigation**:

- Use only gpc0 and imac0 (dev machines)
- Timeout protection (10 minute max)
- No auto-destructive operations
- Rollback commands documented

### Risk 5: Agent Dies During Timeout Handling

**Mitigation**:

- If agent disconnects during TIMEOUT_PENDING, state machine handles gracefully
- Reconnection resets timeout state
- User can always "Mark Offline" to clear stuck state

### Risk 6: macOS Launchd Race Condition

**Mitigation**:

- Switch runs in new session (Setsid: true)
- Switch process survives agent death
- launchd restarts agent after switch completes
- E2E test verifies this flow on imac0

### Risk 7: Network Partition During Switch

**Mitigation**:

- Switch continues on host (detached from agent death)
- Dashboard enters AWAITING_RECONNECT
- On network restore, agent reconnects with new binary
- Timeout provides graceful degradation with user guidance

---

## Related Documents

- [T13 Spec](../../tests/specs/T13-command-state-machine.md) - P2800 test specification
- [T14 Spec](../../tests/specs/T14-agent-binary-freshness.md) - P2810 test specification
- [PRD Agent Resilience](../PRD.md#critical-requirement-agent-resilience) - Why this matters
- [P4020 Tabbed Output Panel](./P4020-tabbed-output-panel.md) - Displays state machine logs
- [P6900 Forced Reboot](./P6900-forced-reboot-with-totp.md) - Nuclear option for stuck hosts (integrated)

---

## Revision History

| Date       | Changes                                                            |
| ---------- | ------------------------------------------------------------------ |
| 2025-12-21 | Initial P2800, P2810, test strategy created as separate files      |
| 2025-12-22 | Refined test strategy with design decisions, 138 tests             |
| 2025-12-22 | Consolidated into single comprehensive document                    |
| 2025-12-22 | Major expansion: Agent Lifecycle Integration, Timeout/Abort,       |
|            | Full Success Criteria, Known Failure Mode E2E tests (165 tests)    |
|            | Changed to reconnection-based completion for switch commands       |
| 2025-12-22 | Added P6900 Reboot Integration: escalation path, ABORTED_BY_REBOOT |
|            | state, post-reboot detection, no auto-retry policy (170 tests)     |
