# P2800 & P2810 - Comprehensive Test Strategy

**Created**: 2025-12-21
**Priority**: Critical (Blocks P2800 & P2810)
**Status**: Planning
**Effort**: 5-7 days

---

## Overview

This document defines a **professional, comprehensive test strategy** to ensure P2800 (Command State Machine) and P2810 (Agent Binary Freshness Detection) work correctly **at all costs**.

**Goal**: 100% confidence that these features work correctly before marking tasks as done.

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

### E2E Tests (Full Flows)

**File**: `v2/tests/integration/t13_command_state_machine_test.go`

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
   - Verify warning toast

4. **Bulk Actions** (1 test)
   - Pull All with multiple hosts
   - Verify per-host validation
   - Verify summary log

**Total E2E Tests**: ~4 tests

---

## P2810 Test Coverage

### Unit Tests (Commit Comparison)

**File**: `v2/tests/integration/validators_test.go` (or separate file)

1. **Commit Comparison Logic** (5 tests)
   - Commits match → AgentOutdated = false
   - Commits differ → AgentOutdated = true
   - Agent commit "unknown" → AgentOutdated = false
   - Expected commit "unknown" → AgentOutdated = false
   - Both unknown → AgentOutdated = false

2. **Edge Cases** (3 tests)
   - Empty strings → treated as unknown
   - Partial commits (7 chars) → compared correctly
   - Case sensitivity → case-sensitive

**Total Unit Tests**: ~8 tests

### Integration Tests (Agent Reporting)

**File**: `v2/tests/integration/t14_agent_freshness_test.go`

1. **Agent Reports Source Commit** (3 tests)
   - SourceCommit in RegisterPayload
   - SourceCommit in HeartbeatPayload
   - SourceCommit "unknown" if not set

2. **Dashboard Detection** (4 tests)
   - Dashboard stores expected commit
   - Dashboard compares commits
   - Dashboard sets AgentOutdated flag
   - Dashboard clears flag when updated

**Total Integration Tests**: ~7 tests

### E2E Tests (Full Detection Flow)

**File**: `v2/tests/integration/t14_agent_freshness_test.go`

1. **Stale Binary Detection** (1 test)
   - Switch completes
   - Agent binary unchanged
   - Post-validation detects and warns

2. **Fresh Binary After Switch** (1 test)
   - Switch completes
   - Agent restarts
   - Next heartbeat shows updated commit
   - AgentOutdated cleared

**Total E2E Tests**: ~2 tests

---

## Test Implementation Plan

### Phase 1: Unit Tests (Days 1-2)

**Goal**: 100% validator code coverage

1. Create `validators_test.go` with test helpers
2. Implement all pre-validator tests (25 tests)
3. Implement all post-validator tests (20 tests)
4. Implement idempotency tests (5 tests)
5. Implement P2810 commit comparison tests (8 tests)

**Deliverable**: All unit tests passing, >95% coverage

### Phase 2: Integration Tests (Days 3-4)

**Goal**: All state machine flows tested

1. Create `t13_command_state_machine_test.go`
2. Implement state machine integration tests (17 tests)
3. Create `t14_agent_freshness_test.go`
4. Implement agent freshness integration tests (7 tests)

**Deliverable**: All integration tests passing

### Phase 3: E2E Tests (Day 5)

**Goal**: Critical user paths verified

1. Implement P2800 E2E tests (4 tests)
2. Implement P2810 E2E tests (2 tests)
3. Verify all logs present
4. Verify state transitions correct

**Deliverable**: All E2E tests passing

### Phase 4: CI Integration (Day 6)

**Goal**: Tests run automatically

1. Update `.github/workflows/ci.yml`
2. Add test coverage reporting
3. Add test failure notifications
4. Verify tests run on every PR

**Deliverable**: CI runs all tests automatically

### Phase 5: Documentation & Review (Day 7)

**Goal**: Tests documented and reviewed

1. Update test README with new tests
2. Review test coverage report
3. Document any gaps
4. Create test execution guide

**Deliverable**: Complete test documentation

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

- [ ] **All unit tests passing** (50+ tests)
- [ ] **All integration tests passing** (24+ tests)
- [ ] **All E2E tests passing** (6+ tests)
- [ ] **Coverage >95%** for validators and state machine
- [ ] **Coverage >90%** for logging and commit comparison
- [ ] **CI runs all tests** on every PR
- [ ] **Test documentation** complete
- [ ] **No flaky tests** (all tests deterministic)

### Verification Checklist

- [ ] Run full test suite locally → all pass
- [ ] Run test suite in CI → all pass
- [ ] Review coverage report → targets met
- [ ] Manual E2E verification → matches test expectations
- [ ] Test edge cases manually → handled correctly

---

## Risk Mitigation

### Risk 1: Flaky Tests

**Mitigation**:

- Use deterministic test data
- Mock time-dependent operations
- Use test fixtures instead of real system calls
- Run tests multiple times in CI

### Risk 2: Incomplete Coverage

**Mitigation**:

- Set coverage targets (95%+)
- Review coverage reports before merging
- Add tests for any bug fixes
- Cover edge cases explicitly

### Risk 3: Slow Test Execution

**Mitigation**:

- Separate unit/integration/E2E tests
- Run unit tests on every save (IDE)
- Run integration tests on commit
- Run E2E tests on PR only

### Risk 4: Tests Don't Catch Real Bugs

**Mitigation**:

- Test against real scenarios (E2E)
- Test edge cases explicitly
- Test error paths
- Manual verification of critical flows

---

## Related Documents

- [T13 Spec](../tests/specs/T13-command-state-machine.md) - P2800 test specification
- [T14 Spec](../tests/specs/T14-agent-binary-freshness.md) - P2810 test specification
- [P2800 Task](../backlog/P2800-command-state-machine.md) - P2800 implementation task
- [P2810 Task](../backlog/P2810-agent-binary-freshness-detection.md) - P2810 implementation task
- [Test README](../../tests/README.md) - General test documentation

---

## Next Steps

1. **Review this strategy** with team
2. **Create test files** (validators*test.go, t13*\_.go, t14\_\_.go)
3. **Implement Phase 1** (unit tests)
4. **Implement Phase 2** (integration tests)
5. **Implement Phase 3** (E2E tests)
6. **Integrate with CI** (Phase 4)
7. **Document and review** (Phase 5)

---

## Notes

- **Test-first approach**: Write tests before/alongside implementation
- **Incremental**: Implement tests incrementally, don't wait until end
- **Review coverage**: Check coverage after each phase
- **Fix bugs immediately**: If tests reveal bugs, fix before continuing
- **Document gaps**: If something can't be tested, document why
