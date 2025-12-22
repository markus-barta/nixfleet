# T13 - Command State Machine (P2800)

**Backlog**: P2800 (Command State Machine)
**Priority**: Must Have
**Status**: 🟡 Pending

---

## Purpose

Verify that the command state machine correctly validates pre-conditions, tracks state transitions, validates post-conditions, and logs all operations verbosely.

---

## Prerequisites

- Dashboard running with CommandStateMachine initialized
- Mock or real agent connection
- Host state tracking (P2000 - Unified Host State)

---

## Test Structure

### Unit Tests (Pure Validators)

Test each validator function in isolation with various host states.

### Integration Tests (State Machine)

Test state transitions, snapshot capture, and post-validation.

### E2E Tests (Full Flow)

Test complete command lifecycle from user action to final state.

---

## Scenarios

### Scenario 1: Pre-Validation - CanExecuteCommand

**Given** a host with various states
**When** CanExecuteCommand is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Host online, no pending command → `{valid: true, code: "ok"}`
- ❌ Host offline → `{valid: false, code: "host_offline"}`
- ❌ Host online but command pending → `{valid: false, code: "command_pending"}`

### Scenario 2: Pre-Validation - CanPull

**Given** a host with git status
**When** CanPull is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Git outdated → `{valid: true, code: "outdated"}`
- ❌ Git already current → `{valid: false, code: "already_current"}`
- ✅ Git status unknown → `{valid: true, code: "unknown_state"}`
- ❌ Host offline (inherited from CanExecuteCommand) → `{valid: false, code: "host_offline"}`

### Scenario 3: Pre-Validation - CanSwitch

**Given** a host with git and system status
**When** CanSwitch is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ System outdated, git current → `{valid: true, code: "outdated"}`
- ❌ Git outdated → `{valid: false, code: "git_outdated"}`
- ❌ System already current → `{valid: false, code: "already_current"}`
- ✅ Agent outdated → `{valid: true, code: "outdated"}` (even if system ok)

### Scenario 4: Pre-Validation - CanTest

**Given** a host
**When** CanTest is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Host online, no pending command → `{valid: true, code: "ok"}`
- ❌ Host offline → `{valid: false, code: "host_offline"}`

### Scenario 5: Pre-Validation - CanPullSwitch

**Given** a host with git and system status
**When** CanPullSwitch is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Git outdated OR system outdated → `{valid: true, code: "ok"}`
- ❌ Both git and system current → `{valid: false, code: "already_current"}`
- ❌ Host offline → `{valid: false, code: "host_offline"}`

### Scenario 6: Post-Validation - ValidatePullResult

**Given** before/after snapshots and exit code
**When** ValidatePullResult is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Exit 0, git status changed to "ok" → `{valid: true, code: "goal_achieved"}`
- ✅ Exit 0, generation changed but git still outdated → `{valid: true, code: "partial"}`
- ❌ Exit 0, nothing changed → `{valid: false, code: "goal_not_achieved"}`
- ❌ Exit != 0 → `{valid: false, code: "exit_nonzero"}`

### Scenario 7: Post-Validation - ValidateSwitchResult

**Given** before/after snapshots and exit code
**When** ValidateSwitchResult is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Exit 0, system status changed to "ok" → `{valid: true, code: "goal_achieved"}`
- ✅ Exit 0, agent outdated before, updated after → `{valid: true, code: "goal_achieved_with_agent"}`
- ✅ Exit 0, agent version changed but still outdated → `{valid: true, code: "pending_restart"}`
- ❌ Exit 0, system still outdated → `{valid: false, code: "goal_not_achieved"}`
- ❌ Exit != 0 → `{valid: false, code: "exit_nonzero"}`

### Scenario 8: Post-Validation - ValidateTestResult

**Given** exit code
**When** ValidateTestResult is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Exit 0 → `{valid: true, code: "test_passed"}`
- ❌ Exit != 0 → `{valid: false, code: "test_failed"}`

### Scenario 9: Post-Validation - ValidatePullSwitchResult

**Given** before/after snapshots and exit code
**When** ValidatePullSwitchResult is called
**Then** it returns appropriate ValidationResult

**Test Cases**:

- ✅ Exit 0, both git and system "ok" → `{valid: true, code: "goal_achieved"}`
- ❌ Exit 0, git ok but system outdated → `{valid: false, code: "partial_git_only"}`
- ✅ Exit 0, system ok but git outdated → `{valid: true, code: "partial_system_only"}`
- ❌ Exit 0, neither updated → `{valid: false, code: "goal_not_achieved"}`
- ❌ Exit != 0 → `{valid: false, code: "exit_nonzero"}`

### Scenario 10: State Machine - Pre-Check Flow

**Given** a host and command
**When** RunPreChecks is called
**Then** it logs each step and returns ValidationResult

**Verification**:

- Log entry for "Checking CanExecuteCommand..."
- Log entry for CanExecuteCommand result
- Log entry for command-specific validator (if base passed)
- Log entry for final result
- Returns ValidationResult

### Scenario 11: State Machine - Snapshot Capture

**Given** a host
**When** CaptureSnapshot is called
**Then** it stores host state for post-validation

**Verification**:

- Snapshot stored with host ID as key
- Snapshot contains: Generation, AgentVersion, AgentOutdated, UpdateStatus
- UpdateStatus is deep copied (not reference)

### Scenario 12: State Machine - Post-Check Flow

**Given** a host with captured snapshot and command result
**When** RunPostChecks is called
**Then** it logs validation and returns ValidationResult

**Verification**:

- Log entry for "Running ValidateXResult..."
- Log entry for validation result with before/after details
- Snapshot cleaned up after validation
- Returns ValidationResult

### Scenario 13: State Machine - Logging

**Given** various log entries
**When** Log is called
**Then** entries are stored and broadcast

**Verification**:

- Entry stored in logStore (bounded to 1000)
- Entry logged to zerolog with appropriate level
- Entry broadcast to browsers via WebSocket
- Entry includes timestamp, level, host_id, state, message, code

### Scenario 14: E2E - Successful Switch Flow

**Given** host with outdated system
**When** user triggers switch
**Then** complete flow executes with logging

**Verification**:

1. Pre-check logs: "User clicked switch" → "Checking CanExecuteCommand..." → "CanSwitch: PASS"
2. Snapshot captured
3. Command sent to agent
4. State transitions: IDLE → VALIDATING → QUEUED → RUNNING
5. Post-check logs: "Running ValidateSwitchResult..." → "goal_achieved"
6. State transitions: RUNNING → VALIDATING → SUCCESS
7. Final state: SUCCESS with appropriate toast

### Scenario 15: E2E - Blocked Switch Flow

**Given** host with outdated git
**When** user triggers switch
**Then** command is blocked with explanation

**Verification**:

1. Pre-check logs: "User clicked switch" → "CanSwitch: FAIL (git_outdated)"
2. State transitions: IDLE → VALIDATING → BLOCKED
3. HTTP 409 response with validation result
4. UI shows dialog: "Cannot switch: Git is outdated, pull required first"

### Scenario 16: E2E - Partial Success Flow

**Given** host that switches but system still outdated
**When** switch completes with exit 0
**Then** post-validation detects partial success

**Verification**:

1. Post-check logs: "ValidateSwitchResult: PARTIAL (goal_not_achieved)"
2. State transitions: RUNNING → VALIDATING → PARTIAL
3. Warning toast shown: "Switch done but system still outdated"

### Scenario 17: E2E - Bulk Actions

**Given** multiple hosts selected
**When** user triggers "Pull All"
**Then** each host validated independently

**Verification**:

- Log entry for bulk action start
- Per-host pre-check logs (pass/skip/blocked)
- Summary log: "Pull All complete: X success, Y skipped, Z blocked"

---

## Idempotency Tests

### Test: Validator Idempotency

**Given** same host state
**When** validator called twice
**Then** same result returned

**Test Cases**:

- CanPull called twice with same host → identical ValidationResult
- CanSwitch called twice with same host → identical ValidationResult
- Validators have no side effects (no state mutation)

---

## Edge Cases

### Edge Case 1: Missing Snapshot

**Given** post-check called without snapshot
**When** RunPostChecks is called
**Then** graceful fallback to exit code check

**Verification**:

- Warning log: "No pre-command snapshot found"
- Returns ValidationResult based on exit code only

### Edge Case 2: Unknown Command

**Given** command not in switch statement
**When** RunPreChecks/RunPostChecks called
**Then** graceful handling

**Verification**:

- Pre-check: returns `{valid: true, code: "unknown_command"}`
- Post-check: falls back to exit code validation

### Edge Case 3: Log Store Overflow

**Given** 1000+ log entries
**When** new entry added
**Then** oldest entries removed

**Verification**:

- Log store maintains max 1000 entries
- Oldest 100 entries removed when limit reached

---

## Verification Commands

```bash
# Run all P2800 tests
cd v2 && go test -v ./tests/integration/... -run "T13"

# Run specific test
cd v2 && go test -v ./tests/integration/... -run "TestT13_PreValidation_CanPull"

# Run with coverage
cd v2 && go test -coverprofile=coverage.out ./tests/integration/... -run "T13"
```

---

## Acceptance Criteria

- [ ] All pre-validators unit tested (100% coverage)
- [ ] All post-validators unit tested (100% coverage)
- [ ] State machine integration tests pass
- [ ] E2E flow tests pass
- [ ] All state transitions logged
- [ ] Log entries broadcast to browsers
- [ ] Idempotency verified for all validators
- [ ] Edge cases handled gracefully
