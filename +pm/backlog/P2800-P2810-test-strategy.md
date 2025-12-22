# P2800 & P2810 - Comprehensive Test Strategy

**Created**: 2025-12-21
**Updated**: 2025-12-22
**Priority**: Critical (Blocks P2800 & P2810)
**Status**: Planning → Refined
**Effort**: 7-10 days

---

## Overview

This document defines a **professional, comprehensive test strategy** to ensure P2800 (Command State Machine) and P2810 (Agent Binary Freshness Detection) work correctly **at all costs**.

**Goal**: 100% guaranteed E2E functionality with:

- Senior developer-level rigor
- Race condition and concurrency coverage
- Self-healing detection (conservative)
- Paranoid binary freshness verification

---

## Design Decisions

Before diving into tests, these design decisions drive the implementation:

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

## Test Pyramid

```
        ┌─────────────┐
        │   E2E Tests │  ← Full system, real scenarios
        │   (10-15)   │
        ├─────────────┤
        │ Integration │  ← State machine, validation flows
        │   (20-30)   │
        ├─────────────┤
        │  Unit Tests │  ← Pure validators, edge cases
        │   (40-50)   │
        └─────────────┘
```

### Distribution

| Layer           | Tests | Purpose                           | Speed          | Coverage Target       |
| --------------- | ----- | --------------------------------- | -------------- | --------------------- |
| **Unit**        | 40-50 | Validator functions, pure logic   | Fast (<1s)     | 100% validator code   |
| **Integration** | 20-30 | State machine, snapshots, logging | Medium (5-10s) | All state transitions |
| **E2E**         | 10-15 | Full flows, UI integration        | Slow (30-60s)  | Critical user paths   |

**Total**: ~80-100 tests covering both features.

---

## Test Organization

### File Structure

```
v2/tests/integration/
├── t13_command_state_machine_test.go    # P2800 integration tests
├── t14_agent_freshness_test.go          # P2810 integration tests
└── validators_test.go                   # P2800 unit tests (validators)

tests/specs/
├── T13-command-state-machine.md         # P2800 spec (human-readable)
└── T14-agent-binary-freshness.md         # P2810 spec (human-readable)
```

### Test Naming Convention

```go
// Unit tests (validators)
func TestCanPull_HostOffline(t *testing.T)
func TestCanPull_GitOutdated(t *testing.T)
func TestValidateSwitchResult_GoalAchieved(t *testing.T)

// Integration tests (state machine)
func TestT13_StateMachine_PreCheckFlow(t *testing.T)
func TestT13_StateMachine_PostCheckFlow(t *testing.T)
func TestT13_StateMachine_Logging(t *testing.T)

// E2E tests (full flows)
func TestT13_E2E_SuccessfulSwitchFlow(t *testing.T)
func TestT13_E2E_BlockedSwitchFlow(t *testing.T)
func TestT14_E2E_StaleBinaryDetection(t *testing.T)
```

---

## P2800 Test Coverage

### Unit Tests (Validators)

**File**: `v2/tests/integration/validators_test.go`

#### Pre-Validators (5 functions × ~5 cases = 25 tests)

1. **CanExecuteCommand** (3 tests)
   - Host online, no pending → PASS
   - Host offline → FAIL
   - Command pending → FAIL

2. **CanPull** (5 tests)
   - Git outdated → PASS
   - Git current → FAIL
   - Git unknown → PASS
   - Host offline → FAIL (inherited)
   - Command pending → FAIL (inherited)

3. **CanSwitch** (6 tests)
   - System outdated, git current → PASS
   - Git outdated → FAIL
   - System current → FAIL
   - Agent outdated → PASS
   - Git unknown → PASS
   - Host offline → FAIL (inherited)

4. **CanTest** (2 tests)
   - Host online → PASS
   - Host offline → FAIL

5. **CanPullSwitch** (4 tests)
   - Git OR system outdated → PASS
   - Both current → FAIL
   - Host offline → FAIL (inherited)
   - Status unknown → PASS

#### Post-Validators (4 functions × ~5 cases = 20 tests)

1. **ValidatePullResult** (5 tests)
   - Exit 0, git "ok" → SUCCESS
   - Exit 0, generation changed → PARTIAL
   - Exit 0, nothing changed → FAIL
   - Exit != 0 → FAIL
   - Missing snapshot → FALLBACK

2. **ValidateSwitchResult** (6 tests)
   - Exit 0, system "ok" → SUCCESS
   - Exit 0, agent updated → SUCCESS (with agent)
   - Exit 0, agent version changed but outdated → PENDING_RESTART
   - Exit 0, system outdated → FAIL
   - Exit != 0 → FAIL
   - Missing snapshot → FALLBACK

3. **ValidateTestResult** (2 tests)
   - Exit 0 → PASS
   - Exit != 0 → FAIL

4. **ValidatePullSwitchResult** (5 tests)
   - Exit 0, both "ok" → SUCCESS
   - Exit 0, git ok, system outdated → FAIL (partial_git_only)
   - Exit 0, system ok, git outdated → PARTIAL (partial_system_only)
   - Exit 0, neither updated → FAIL
   - Exit != 0 → FAIL

#### Idempotency Tests (5 tests)

- Each validator called twice with same input → same output
- No side effects (no state mutation)

**Total Unit Tests**: ~50 tests

### Race Condition Tests

**File**: `v2/tests/integration/race_conditions_test.go`

These tests verify correct behavior under concurrent access:

#### RC-1: Rapid Button Clicks (2 tests)

```go
func TestRC1_RapidClicks_SameHost(t *testing.T) {
    // Click Switch 5 times in 100ms on same host
    // Expect: First succeeds, next 4 return 409 "command_pending"
}

func TestRC1_RapidClicks_DifferentHosts(t *testing.T) {
    // Click Switch on 5 different hosts simultaneously
    // Expect: All 5 succeed (no interference)
}
```

#### RC-2: Command During Heartbeat (2 tests)

```go
func TestRC2_CommandDuringHeartbeat(t *testing.T) {
    // Send heartbeat and command simultaneously
    // Expect: No deadlock, both processed correctly
}

func TestRC2_HeartbeatDuringPostValidation(t *testing.T) {
    // Heartbeat arrives while post-validation running
    // Expect: Post-validation uses snapshot, not heartbeat data
}
```

#### RC-3: Two Browsers Same Host (2 tests)

```go
func TestRC3_TwoBrowsers_CommandBlocking(t *testing.T) {
    // Browser A clicks Switch
    // Browser B clicks Switch 50ms later
    // Expect: B sees "command_pending" immediately
}

func TestRC3_TwoBrowsers_StateSync(t *testing.T) {
    // Both browsers receive same state updates
    // Expect: No split-brain, consistent view
}
```

#### RC-4: Snapshot Integrity (3 tests)

```go
func TestRC4_SnapshotCaptureRace(t *testing.T) {
    // Heartbeat updates state while snapshot being captured
    // Expect: Snapshot is consistent (deep copy)
}

func TestRC4_SnapshotCleanupRace(t *testing.T) {
    // Post-validation runs while new command starting
    // Expect: Old snapshot cleaned, new snapshot created
}

func TestRC4_ConcurrentSnapshotsMultiHost(t *testing.T) {
    // Snapshots captured for multiple hosts simultaneously
    // Expect: No cross-contamination between hosts
}
```

#### RC-5: Agent Disconnect During Command (2 tests)

```go
func TestRC5_AgentDisconnectMidCommand(t *testing.T) {
    // Agent disconnects while command RUNNING
    // Expect: State transitions to orphaned, log warning
}

func TestRC5_AgentReconnectAfterDisconnect(t *testing.T) {
    // Agent reconnects after mid-command disconnect
    // Expect: Graceful recovery, no duplicate execution
}
```

**Total Race Condition Tests**: ~11 tests

### Integration Tests (State Machine)

**File**: `v2/tests/integration/t13_command_state_machine_test.go`

1. **Pre-Check Flow** (3 tests)
   - RunPreChecks logs each step
   - Returns ValidationResult
   - Handles unknown command

2. **Snapshot Capture** (2 tests)
   - CaptureSnapshot stores state
   - Deep copy of UpdateStatus

3. **Post-Check Flow** (3 tests)
   - RunPostChecks validates result
   - Logs before/after comparison
   - Cleans up snapshot

4. **Logging** (4 tests)
   - Log stores entries (bounded to 1000)
   - Log broadcasts to browsers
   - Log includes all required fields
   - Log store overflow handling

5. **State Transitions** (5 tests)
   - IDLE → VALIDATING → QUEUED → RUNNING → SUCCESS
   - IDLE → VALIDATING → BLOCKED
   - RUNNING → VALIDATING → PARTIAL
   - RUNNING → VALIDATING → FAILED
   - Missing snapshot fallback

**Total Integration Tests**: ~17 tests

### Self-Healing Detection Tests

**File**: `v2/tests/integration/t13_self_healing_test.go`

These tests verify orphaned state detection (conservative: log only, no auto-cleanup):

#### Orphaned Snapshot Detection (3 tests)

```go
func TestSelfHealing_OrphanedSnapshot_Detected(t *testing.T) {
    // Create snapshot, don't complete command
    // Wait for detection interval (60s in tests, shorter)
    // Expect: Log entry "orphaned_snapshot" for host
}

func TestSelfHealing_OrphanedSnapshot_Threshold(t *testing.T) {
    // Threshold: snapshot_age > (command_timeout + 5min)
    // Test with different command types (switch=10min, pull=5min)
    // Expect: Correct threshold per command type
}

func TestSelfHealing_OrphanedSnapshot_NotFalsePositive(t *testing.T) {
    // Long-running switch (9 minutes)
    // Expect: NOT flagged as orphaned while command running
}
```

#### Stuck RUNNING State Detection (3 tests)

```go
func TestSelfHealing_StuckRunning_Detected(t *testing.T) {
    // Host in RUNNING state for > 15 minutes
    // Expect: Log entry "stuck_running" for host
}

func TestSelfHealing_StuckRunning_ClearedOnComplete(t *testing.T) {
    // Host was stuck, then command completes
    // Expect: Stuck warning cleared
}

func TestSelfHealing_StuckRunning_AgentOffline(t *testing.T) {
    // Host in RUNNING state, agent goes offline
    // Expect: Log entry includes "agent_disconnected" context
}
```

#### Detection Thresholds

| State             | Threshold                                      | Alert                    |
| ----------------- | ---------------------------------------------- | ------------------------ |
| Orphaned snapshot | `snapshot.created_at + command_timeout + 5min` | Log: "orphaned_snapshot" |
| Stuck RUNNING     | `command_started_at + 15min`                   | Log: "stuck_running"     |
| Stuck QUEUED      | `queued_at + 1min`                             | Log: "stuck_queued"      |

**Total Self-Healing Tests**: ~6 tests

### Post-Validation Timing Tests

**File**: `v2/tests/integration/t13_post_validation_timing_test.go`

These tests verify the explicit + fallback timing design:

#### Primary Path: command_complete Message (3 tests)

```go
func TestPostValidation_CommandComplete_Immediate(t *testing.T) {
    // Agent sends command_complete with fresh_status
    // Expect: Post-validation runs immediately
    // Expect: Uses fresh_status, not stale heartbeat data
}

func TestPostValidation_CommandComplete_StatusIncluded(t *testing.T) {
    // Verify command_complete message structure
    // Expect: { exit_code, fresh_status: { git, system, generation, ... } }
}

func TestPostValidation_CommandComplete_AgentRestart(t *testing.T) {
    // Switch completes, agent exits 101, restarts
    // New agent sends first heartbeat with updated state
    // Expect: Post-validation waits for reconnect, uses fresh state
}
```

#### Fallback Path: Timeout (4 tests)

```go
func TestPostValidation_Fallback_Timeout(t *testing.T) {
    // command_complete not received within timeout
    // Expect: Falls back to heartbeat-based detection
}

func TestPostValidation_Fallback_HeartbeatChanged(t *testing.T) {
    // Timeout triggers, next heartbeat has changed state
    // Expect: Post-validation uses heartbeat state
}

func TestPostValidation_Fallback_HeartbeatUnchanged(t *testing.T) {
    // Timeout triggers, next heartbeat has same state
    // Expect: Log warning, fallback to exit code only
}

func TestPostValidation_Fallback_TimeoutValues(t *testing.T) {
    // Verify correct timeouts per command type
    // switch: 30s, pull: 10s, test: 5s
}
```

**Total Post-Validation Timing Tests**: ~7 tests

### E2E Tests (Full Flows)

E2E tests run against **actual fleet** for maximum realism.

#### Test Hosts

| Host    | Platform | SSH                    | Purpose             |
| ------- | -------- | ---------------------- | ------------------- |
| `gpc0`  | NixOS    | `ssh mba@gpc0.lan`     | nixos-rebuild tests |
| `imac0` | macOS    | `ssh markus@imac0.lan` | home-manager tests  |

#### Mock E2E Tests (In-Memory)

**File**: `v2/tests/integration/t13_e2e_mock_test.go`

1. **Successful Switch Flow** (1 test)
   - Pre-check → snapshot → command → post-check → SUCCESS
   - Verify all logs present
   - Verify state transitions

2. **Blocked Switch Flow** (1 test)
   - Pre-check fails → BLOCKED
   - Verify HTTP 409 response
   - Verify UI dialog would show

3. **Partial Success Flow** (1 test)
   - Switch completes but goal not achieved
   - Verify PARTIAL state
   - Verify warning in log

4. **Bulk Actions** (1 test)
   - Pull All with multiple hosts
   - Verify per-host validation
   - Verify summary log

5. **Command Timeout** (1 test)
   - Command runs longer than timeout
   - Verify fallback behavior
   - Verify stuck state detection

6. **Agent Restart Flow** (1 test)
   - Switch triggers agent restart (exit 101)
   - Verify dashboard waits for reconnect
   - Verify fresh state after reconnect

**Total Mock E2E Tests**: ~6 tests

#### Real Fleet E2E Tests

**File**: `v2/tests/integration/t13_e2e_fleet_test.go`

**IMPORTANT**: These tests require actual SSH access and run against real hosts.
Run with: `go test -v -tags=fleet ./tests/integration/... -run "Fleet"`

##### NixOS Tests (gpc0)

```go
//go:build fleet

func TestFleet_NixOS_PullSuccess(t *testing.T) {
    // SSH to gpc0, ensure git is behind
    // Trigger Pull via dashboard API
    // Verify: command completes, git status updates
}

func TestFleet_NixOS_SwitchSuccess(t *testing.T) {
    // Ensure gpc0 needs switch (after pull)
    // Trigger Switch via dashboard API
    // Verify: command completes, system status "ok"
    // Verify: Agent restarts with new binary (if changed)
}

func TestFleet_NixOS_SwitchLongRunning(t *testing.T) {
    // Trigger switch that takes 5+ minutes
    // Verify: Heartbeats continue during build
    // Verify: Progress updates received
    // Verify: No timeout false positives
}

func TestFleet_NixOS_AgentRestart(t *testing.T) {
    // Trigger switch that updates agent
    // Verify: Agent exits 101
    // Verify: systemd restarts agent
    // Verify: New agent connects within 30s
    // Verify: Dashboard detects fresh binary
}
```

##### macOS Tests (imac0)

```go
//go:build fleet

func TestFleet_macOS_PullSuccess(t *testing.T) {
    // SSH to imac0, ensure git is behind
    // Trigger Pull via dashboard API
    // Verify: command completes, git status updates
}

func TestFleet_macOS_SwitchSuccess(t *testing.T) {
    // Ensure imac0 needs switch
    // Trigger Switch via dashboard API
    // Verify: home-manager switch completes
    // Verify: Agent restarts via launchd
}

func TestFleet_macOS_AgentSurvivesSwitch(t *testing.T) {
    // Critical: Agent must survive home-manager switch
    // Verify: Agent stays connected during switch
    // Verify: Setsid prevents launchd kill
    // Verify: Agent restarts with new binary after
}
```

##### Cross-Platform Tests

```go
//go:build fleet

func TestFleet_BulkPullAll(t *testing.T) {
    // Trigger Pull All on [gpc0, imac0]
    // Verify: Both complete (or appropriate skip/block)
    // Verify: Summary log accurate
}

func TestFleet_NetworkPartition(t *testing.T) {
    // Disconnect gpc0 from network for 60s
    // Verify: Dashboard shows offline
    // Verify: Agent reconnects when network returns
    // Verify: Exponential backoff observed
}
```

**Total Real Fleet E2E Tests**: ~9 tests

#### E2E Test Safety

Real fleet tests must be **safe**:

1. **Dry-run option**: Tests can use `nixos-rebuild build` instead of `switch`
2. **Known-good state**: Before each test, verify host is in known state
3. **Rollback ready**: Always have rollback command ready
4. **Timeout protection**: Tests timeout after 10 minutes

```go
func setupFleetTest(t *testing.T, host string) {
    // Verify host is online
    // Verify no pending commands
    // Capture initial state for comparison
    t.Cleanup(func() {
        // Log final state
        // Don't auto-rollback (conservative)
    })
}
```

**Total E2E Tests**: ~15 tests (6 mock + 9 fleet)

---

## P2810 Test Coverage

P2810 uses **3-layer paranoid verification** based on change detection.

### Binary Freshness Data Model

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

### Unit Tests (Comparison Logic)

**File**: `v2/tests/integration/t14_freshness_test.go`

#### Layer 1: Source Commit (5 tests)

```go
func TestFreshness_Commit_Match(t *testing.T)        // same → fresh
func TestFreshness_Commit_Differ(t *testing.T)       // different → outdated
func TestFreshness_Commit_AgentUnknown(t *testing.T) // graceful
func TestFreshness_Commit_ExpectedUnknown(t *testing.T)
func TestFreshness_Commit_BothUnknown(t *testing.T)
```

#### Layer 2: Store Path (4 tests)

```go
func TestFreshness_StorePath_Changed(t *testing.T)   // different path → fresh
func TestFreshness_StorePath_Unchanged(t *testing.T) // same path → stale
func TestFreshness_StorePath_Empty(t *testing.T)     // graceful
func TestFreshness_StorePath_Format(t *testing.T)    // valid Nix store format
```

#### Layer 3: Binary Hash (4 tests)

```go
func TestFreshness_Hash_Changed(t *testing.T)   // different hash → fresh
func TestFreshness_Hash_Unchanged(t *testing.T) // same hash → stale
func TestFreshness_Hash_Empty(t *testing.T)     // graceful
func TestFreshness_Hash_SHA256Format(t *testing.T)
```

#### Combined Detection (5 tests)

```go
func TestFreshness_AllChanged(t *testing.T)     // all 3 changed → FRESH
func TestFreshness_NoneChanged(t *testing.T)    // none changed → STALE
func TestFreshness_OnlyCommitChanged(t *testing.T)  // suspicious (cache hit?)
func TestFreshness_OnlyPathChanged(t *testing.T)    // FRESH
func TestFreshness_DecisionMatrix(t *testing.T) // all 8 combinations
```

**Decision Matrix**:

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

**Total Unit Tests**: ~18 tests

### Integration Tests (Agent Reporting)

**File**: `v2/tests/integration/t14_agent_reporting_test.go`

#### Agent Reports All 3 Layers (6 tests)

```go
func TestAgentReports_SourceCommit_Register(t *testing.T)
func TestAgentReports_SourceCommit_Heartbeat(t *testing.T)
func TestAgentReports_StorePath_Linux(t *testing.T)   // /proc/self/exe
func TestAgentReports_StorePath_Darwin(t *testing.T)  // _NSGetExecutablePath
func TestAgentReports_BinaryHash_Computed(t *testing.T)
func TestAgentReports_AllFields_Heartbeat(t *testing.T)
```

#### Dashboard Storage & Comparison (5 tests)

```go
func TestDashboard_StoresBeforeState(t *testing.T)
func TestDashboard_ComparesAfterRestart(t *testing.T)
func TestDashboard_SetsAgentOutdatedFlag(t *testing.T)
func TestDashboard_ClearsAgentOutdatedFlag(t *testing.T)
func TestDashboard_LogsDetectionResult(t *testing.T)
```

**Total Integration Tests**: ~11 tests

### E2E Tests (Full Detection Flow)

**File**: `v2/tests/integration/t14_e2e_test.go`

#### Mock E2E Tests

```go
func TestT14_E2E_StaleBinaryDetected(t *testing.T) {
    // Switch completes
    // Agent doesn't restart (simulated)
    // Post-validation detects all 3 layers unchanged
    // Expect: Log warning "stale_binary_detected"
}

func TestT14_E2E_FreshBinaryAfterSwitch(t *testing.T) {
    // Switch completes
    // Agent restarts with new binary
    // All 3 layers changed
    // Expect: AgentOutdated = false
}

func TestT14_E2E_SuspiciousCache(t *testing.T) {
    // Switch completes
    // Commit changed but path/hash unchanged
    // Expect: Log warning "possible_cache_hit"
    // Expect: Guidance shown to user
}
```

#### Real Fleet E2E Tests

```go
//go:build fleet

func TestFleet_T14_NixOS_BinaryUpdated(t *testing.T) {
    // On gpc0: Trigger switch that changes agent
    // Verify: Agent restarts
    // Verify: New commit/path/hash reported
    // Verify: AgentOutdated = false
}

func TestFleet_T14_macOS_BinaryUpdated(t *testing.T) {
    // On imac0: Same test for macOS
    // Verify: launchd restarts agent correctly
}

func TestFleet_T14_CacheScenario(t *testing.T) {
    // Simulate Nix cache returning old binary
    // (Hard to reproduce reliably)
    // At minimum: verify detection logic triggers
}
```

**Total E2E Tests**: ~6 tests

### P2810 Implementation Requirements

For tests to pass, agent must:

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
   // Use _NSGetExecutablePath or similar
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

4. **Include in heartbeat**:
   ```go
   type HeartbeatPayload struct {
       // ... existing ...
       SourceCommit string `json:"source_commit"`
       StorePath    string `json:"store_path"`
       BinaryHash   string `json:"binary_hash"`
   }
   ```

---

## Test Summary

### Test Count by Category

| Category               | P2800   | P2810  | Shared | Total   |
| ---------------------- | ------- | ------ | ------ | ------- |
| Unit Tests             | 50      | 18     | -      | **68**  |
| Race Condition Tests   | 11      | -      | -      | **11**  |
| Self-Healing Tests     | 6       | -      | -      | **6**   |
| Post-Validation Timing | 7       | -      | -      | **7**   |
| Integration Tests      | 17      | 11     | -      | **28**  |
| E2E Mock Tests         | 6       | 3      | -      | **9**   |
| E2E Fleet Tests        | 6       | 3      | -      | **9**   |
| **TOTAL**              | **103** | **35** | -      | **138** |

---

## Test Implementation Plan

### Phase 1: Unit Tests & Fixtures (Days 1-3)

**Goal**: 100% validator and comparison logic coverage

1. Create test fixtures and helpers
   - `test_fixtures.go` - Host, Snapshot, Freshness factories
   - `test_helpers.go` - Common assertions, mock setup

2. Implement P2800 validators (50 tests)
   - Pre-validators: CanExecuteCommand, CanPull, CanSwitch, CanTest, CanPullSwitch
   - Post-validators: ValidatePullResult, ValidateSwitchResult, ValidateTestResult, ValidatePullSwitchResult
   - Idempotency tests

3. Implement P2810 comparison logic (18 tests)
   - Layer 1: Source commit
   - Layer 2: Store path
   - Layer 3: Binary hash
   - Combined decision matrix

**Deliverable**: All unit tests passing, >95% coverage

### Phase 2: Race Condition & Self-Healing Tests (Days 4-5)

**Goal**: Concurrent access and edge cases covered

1. Implement race condition tests (11 tests)
   - Rapid clicks, concurrent commands
   - Heartbeat/command interleaving
   - Snapshot integrity under concurrency

2. Implement self-healing detection tests (6 tests)
   - Orphaned snapshot detection
   - Stuck RUNNING state detection
   - Threshold verification

3. Implement post-validation timing tests (7 tests)
   - Primary path (command_complete with fresh_status)
   - Fallback path (timeout + heartbeat)

**Deliverable**: All concurrency tests passing, no race detector warnings

### Phase 3: Integration Tests (Days 6-7)

**Goal**: State machine and agent reporting flows tested

1. Implement P2800 state machine tests (17 tests)
   - Pre-check flow
   - Snapshot capture
   - Post-check flow
   - Logging and broadcast

2. Implement P2810 agent reporting tests (11 tests)
   - Agent reports all 3 layers
   - Dashboard stores and compares
   - Detection result logging

**Deliverable**: All integration tests passing

### Phase 4: E2E Mock Tests (Day 8)

**Goal**: Full flows tested in isolation

1. Implement P2800 mock E2E tests (6 tests)
   - Success, blocked, partial, bulk, timeout, restart flows

2. Implement P2810 mock E2E tests (3 tests)
   - Stale detected, fresh detected, suspicious cache

**Deliverable**: All mock E2E tests passing

### Phase 5: E2E Fleet Tests (Days 9-10)

**Goal**: Real-world behavior verified

1. Set up fleet test infrastructure
   - SSH access to gpc0 and imac0
   - Safety guards (timeout, rollback ready)
   - Test tagging (`//go:build fleet`)

2. Implement NixOS fleet tests (gpc0) - 4 tests
   - Pull, switch, long-running, agent restart

3. Implement macOS fleet tests (imac0) - 3 tests
   - Pull, switch, agent survives switch

4. Implement cross-platform tests - 2 tests
   - Bulk pull, network partition

**Deliverable**: All fleet tests passing on real hosts

### Phase 6: CI & Documentation (Day 10)

**Goal**: Automation and documentation complete

1. Update CI pipeline
   - Unit + Integration + Mock E2E on every PR
   - Fleet tests on merge to main (optional/manual trigger)
   - Coverage reporting

2. Documentation
   - Update test README
   - Add fleet test execution guide
   - Document known limitations

**Deliverable**: CI green, docs complete

---

## Test Execution Strategy

### Local Development

```bash
# Run all P2800/P2810 tests
cd v2 && go test -v ./tests/integration/... -run "T13|T14|validators"

# Run with coverage
cd v2 && go test -coverprofile=coverage.out ./tests/integration/... -run "T13|T14|validators"
cd v2 && go tool cover -html=coverage.out

# Run specific test
cd v2 && go test -v ./tests/integration/... -run "TestT13_E2E_SuccessfulSwitchFlow"

# Run unit tests only (fast)
cd v2 && go test -v ./tests/integration/... -run "validators"

# Run integration tests only
cd v2 && go test -v ./tests/integration/... -run "T13_|T14_"

# Run E2E tests only (slow)
cd v2 && go test -v ./tests/integration/... -run "T13_E2E|T14_E2E"
```

### CI Pipeline

```yaml
# .github/workflows/ci.yml
- name: Run P2800/P2810 Tests
  run: |
    cd v2
    go test -v -coverprofile=coverage.out ./tests/integration/... -run "T13|T14|validators"
    go tool cover -func=coverage.out | grep total

- name: Upload Coverage
  uses: codecov/codecov-action@v3
  with:
    file: ./v2/coverage.out
```

### Pre-Commit Hook (Optional)

```bash
#!/bin/bash
# .git/hooks/pre-commit
cd v2 && go test ./tests/integration/... -run "T13|T14|validators" || exit 1
```

---

## Coverage Requirements

### Minimum Coverage Targets

| Component                 | Target | Critical Paths          |
| ------------------------- | ------ | ----------------------- |
| Validators (P2800)        | 100%   | All validator functions |
| State Machine (P2800)     | 95%    | All state transitions   |
| Logging (P2800)           | 90%    | Log storage, broadcast  |
| Commit Comparison (P2810) | 100%   | All comparison logic    |
| Agent Reporting (P2810)   | 95%    | Register, heartbeat     |

### Critical Paths (Must Have 100% Coverage)

1. **CanSwitch** - Most complex validator, critical for UX
2. **ValidateSwitchResult** - Detects partial success
3. **Commit Comparison** - Core of P2810
4. **State Transitions** - Core of P2800

---

## Test Data & Fixtures

### Host Fixtures

```go
// test_fixtures.go
func createTestHost(id string, online bool, gitStatus, systemStatus string) *templates.Host {
    return &templates.Host{
        ID:            id,
        Online:        online,
        PendingCommand: "",
        UpdateStatus: &templates.UpdateStatus{
            Git: templates.StatusCheck{
                Status: gitStatus,
                Message: fmt.Sprintf("Git status: %s", gitStatus),
            },
            System: templates.StatusCheck{
                Status: systemStatus,
                Message: fmt.Sprintf("System status: %s", systemStatus),
            },
        },
    }
}
```

### Snapshot Fixtures

```go
func createTestSnapshot(generation, agentVersion string, agentOutdated bool) HostSnapshot {
    return HostSnapshot{
        Generation:    generation,
        AgentVersion:  agentVersion,
        AgentOutdated: agentOutdated,
        UpdateStatus: &templates.UpdateStatus{
            Git: templates.StatusCheck{Status: "ok"},
            System: templates.StatusCheck{Status: "ok"},
        },
    }
}
```

---

## Success Criteria

### Before Marking P2800/P2810 as Done

#### Test Counts

- [ ] **All unit tests passing** (68+ tests)
- [ ] **All race condition tests passing** (11 tests)
- [ ] **All self-healing tests passing** (6 tests)
- [ ] **All post-validation timing tests passing** (7 tests)
- [ ] **All integration tests passing** (28+ tests)
- [ ] **All mock E2E tests passing** (9 tests)
- [ ] **All fleet E2E tests passing** (9 tests on gpc0 + imac0)

#### Coverage Targets

| Component        | Target | Must Include                     |
| ---------------- | ------ | -------------------------------- |
| Validators       | 100%   | All CanX and ValidateX functions |
| State Machine    | 95%    | All state transitions            |
| Race Conditions  | 100%   | All identified race scenarios    |
| Binary Freshness | 100%   | All 3 layers, decision matrix    |
| Self-Healing     | 90%    | Detection thresholds             |

#### Quality Gates

- [ ] **No race detector warnings** (`go test -race`)
- [ ] **No flaky tests** (run 3x in CI, all pass)
- [ ] **CI pipeline green** on every PR
- [ ] **Fleet tests pass** on both gpc0 and imac0

### Verification Checklist

#### Local Verification

- [ ] `go test -race ./tests/integration/...` → all pass, no warnings
- [ ] `go test -coverprofile=cov.out ./tests/integration/...` → >95%
- [ ] Run each E2E test 3 times → consistent results

#### Fleet Verification (Manual)

- [ ] SSH to gpc0, trigger switch via dashboard → works
- [ ] SSH to imac0, trigger switch via dashboard → works
- [ ] Disconnect gpc0 network 60s → agent reconnects
- [ ] Kill agent on imac0 → launchd restarts it
- [ ] Force stale binary scenario → detection works

#### Critical Path Verification

These scenarios MUST work perfectly:

1. **Happy Path Switch**
   - User clicks Switch
   - Pre-check passes
   - Command executes
   - Post-check confirms goal achieved
   - Log shows full trace

2. **Blocked Command**
   - User clicks Switch when git outdated
   - Pre-check fails
   - HTTP 409 returned
   - Log shows "git_outdated" code

3. **Partial Success Detection**
   - Switch exits 0 but system still outdated
   - Post-check detects goal not achieved
   - Log shows warning

4. **Stale Binary Detection**
   - Switch completes
   - Agent binary unchanged
   - Detection triggers
   - Log shows "stale_binary_detected"

5. **Agent Restart Flow**
   - Switch updates agent
   - Agent exits 101
   - systemd/launchd restarts
   - New agent connects with fresh binary
   - AgentOutdated = false

---

## Risk Mitigation

### Risk 1: Flaky Tests

**Mitigation**:

- Use deterministic test data with fixtures
- Mock time-dependent operations (`time.Now()` → injectable clock)
- Use test fixtures instead of real system calls
- Run tests 3x in CI before merge
- Race detector enabled (`go test -race`)

### Risk 2: Incomplete Coverage

**Mitigation**:

- Set coverage targets (95%+ validators, 100% binary freshness)
- Review coverage reports before merging
- Add tests for any bug fixes (regression tests)
- Cover all 8 combinations of binary freshness matrix

### Risk 3: Slow Test Execution

**Mitigation**:

| Test Type   | When to Run      | Timeout |
| ----------- | ---------------- | ------- |
| Unit tests  | Every save (IDE) | 10s     |
| Race tests  | On commit        | 30s     |
| Integration | On commit        | 60s     |
| Mock E2E    | On PR            | 120s    |
| Fleet E2E   | On merge/manual  | 10min   |

- Fleet tests tagged `//go:build fleet`, excluded by default
- Parallel test execution where safe

### Risk 4: Tests Don't Catch Real Bugs

**Mitigation**:

- Fleet tests against actual gpc0 and imac0
- Test the exact scenarios that have broken before (10+ times per PRD)
- Test error paths, not just happy paths
- Manual verification of critical flows before release

### Risk 5: Race Conditions in Production

**Mitigation**:

- All concurrent tests run with `-race` flag
- Explicit tests for identified race scenarios (11 tests)
- Snapshot deep copy verified
- Lock contention tested

### Risk 6: Fleet Tests Damage Real Hosts

**Mitigation**:

- Use only gpc0 and imac0 (dev machines)
- Tests use known-good state as baseline
- Timeout protection (10 minute max)
- No auto-destructive operations
- Rollback commands documented (not auto-executed)

### Risk 7: Post-Validation Timing Race

**Mitigation**:

- Primary path: Agent sends `command_complete` with fresh_status included
- No waiting for heartbeat in primary path
- Fallback path with explicit timeouts
- Tests verify both paths work correctly

### Risk 8: Nix Cache Returns Stale Binary

**Mitigation**:

- 3-layer verification catches this:
  - Commit changed but path unchanged → suspicious
  - Hash unchanged → definite stale
- Detection triggers even when we can't prevent it
- Log provides user guidance: "Run nix-collect-garbage -d"

---

## Implementation Dependencies

Before tests can pass, these implementation changes are required:

### Agent Changes

| Change                                         | Why                      | Effort |
| ---------------------------------------------- | ------------------------ | ------ |
| Add `StorePath` to heartbeat                   | Layer 2 verification     | Small  |
| Add `BinaryHash` to heartbeat                  | Layer 3 verification     | Small  |
| Add `fresh_status` to `command_complete`       | Timing fix               | Medium |
| Force status refresh before `command_complete` | Accurate post-validation | Small  |

### Dashboard Changes

| Change                                        | Why                 | Effort |
| --------------------------------------------- | ------------------- | ------ |
| Handle `command_complete` with `fresh_status` | Primary timing path | Medium |
| Fallback timeout logic                        | Backup timing path  | Medium |
| 3-layer comparison logic                      | Binary freshness    | Medium |
| Orphaned state detection                      | Self-healing        | Small  |
| Log "stale_binary_detected" entries           | User visibility     | Small  |

### Protocol Changes

```go
// command_complete message (agent → dashboard)
type CommandCompleteMessage struct {
    Type    string `json:"type"`  // "command_complete"
    Payload struct {
        Command     string        `json:"command"`
        ExitCode    int           `json:"exit_code"`
        FreshStatus *UpdateStatus `json:"fresh_status"`  // NEW
    } `json:"payload"`
}

// heartbeat additions (agent → dashboard)
type HeartbeatPayload struct {
    // ... existing ...
    SourceCommit string `json:"source_commit"`  // Layer 1
    StorePath    string `json:"store_path"`     // Layer 2 (NEW)
    BinaryHash   string `json:"binary_hash"`    // Layer 3 (NEW)
}
```

---

## Related Documents

- [T13 Spec](../../tests/specs/T13-command-state-machine.md) - P2800 test specification
- [T14 Spec](../../tests/specs/T14-agent-binary-freshness.md) - P2810 test specification
- [P2800 Task](./P2800-command-state-machine.md) - P2800 implementation task
- [P2810 Task](./P2810-agent-binary-freshness-detection.md) - P2810 implementation task
- [Test README](../../tests/README.md) - General test documentation
- [PRD Agent Resilience](../PRD.md#critical-requirement-agent-resilience) - Why this matters

---

## Next Steps

1. ✅ **Review this strategy** - Refined 2025-12-22
2. **Implement agent changes** (StorePath, BinaryHash, fresh_status)
3. **Implement dashboard changes** (timing, 3-layer verification)
4. **Create test files** and fixtures
5. **Implement Phase 1-3** (unit → race → integration)
6. **Implement Phase 4** (mock E2E)
7. **Implement Phase 5** (fleet E2E on gpc0, imac0)
8. **Integrate with CI** (Phase 6)

---

## Notes

- **Test-first approach**: Write tests before/alongside implementation
- **Incremental**: Implement tests incrementally, don't wait until end
- **Review coverage**: Check coverage after each phase
- **Fix bugs immediately**: If tests reveal bugs, fix before continuing
- **Document gaps**: If something can't be tested, document why
- **Fleet tests are optional but critical**: Can skip in CI, must run before release

---

## Revision History

| Date       | Changes                                                                                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2025-12-21 | Initial test strategy created                                                                                                                                             |
| 2025-12-22 | Refined with: concurrent command handling, post-validation timing, self-healing detection, 3-layer binary verification, fleet E2E tests, expanded test counts (138 total) |
