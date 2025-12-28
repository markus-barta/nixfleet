# P3900 - Tests Compartment: Fifth Compartment

**Created**: 2025-12-28  
**Priority**: P3900 (🔴 Critical Path - Sprint 1)  
**Status**: Backlog  
**Effort**: 4-5 hours  
**Depends on**: P3700, P3800

---

## User Story

**As a** fleet administrator  
**I want** a dedicated Tests compartment  
**So that** I can see if my system is actually working, separate from whether the deployment succeeded

---

## Problem

Current System compartment conflates two concepts:

1. **Deployment success** - Did the switch command succeed?
2. **System health** - Is the system actually working?

### Example of Current Problem

```
After switch:
┌─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │
│   🟢    │   🟢    │   🟢    │   🟢    │  <- System shows GREEN
└─────────┴─────────┴─────────┴─────────┘

But... X11 won't start, networking is down!
User thinks system is fine (green), but tests would catch this.
```

---

## Solution

**Fifth compartment for test results:**

The five compartments form a complete deployment pipeline:

1. **Agent** - Is my tooling current?
2. **Git** - Is my config current?
3. **Lock** - Are my dependencies current?
4. **System** - Is my deployment successful?
5. **Tests** - Is my system actually working?

### After Fix

```
After switch with test failure:
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │ Tests   │
│   🟢    │   🟢    │   🟢    │   🟢    │   🔴    │  <- Tests show RED
└─────────┴─────────┴─────────┴─────────┴─────────┘

Now it's clear: deployment succeeded, but system is broken!
```

---

## Acceptance Criteria

### Test Execution

- [ ] Tests auto-run after switch (configurable per-host)
- [ ] Tests can be manually triggered via button
- [ ] Test results are persisted in State Store
- [ ] Test output streams to log panel in real-time

### Compartment States

| Color              | Meaning           | When                         |
| ------------------ | ----------------- | ---------------------------- |
| 🟢 Green           | All tests passed  | After successful test run    |
| 🟡 Yellow          | Tests not run yet | After switch, before test    |
| 🔴 Red             | Tests failed      | After failed test run        |
| 🔵 Blue (animated) | Tests running     | During test execution        |
| ⚪ Gray            | Tests disabled    | Host has no tests configured |

### UI/UX

- [ ] Fifth compartment displays in host table
- [ ] Tooltip shows: "Last test: 2 min ago (8/8 passed)"
- [ ] Click opens test results popup
- [ ] Failed tests show which test failed + output
- [ ] Tests 🟡 after switch prompts: "Run tests to verify system"

### Configuration

- [ ] Per-host setting: "Auto-run tests after switch" (default: enabled)
- [ ] Global setting: "Test timeout" (default: 60s)
- [ ] Tests can be disabled per-host (shows gray)

---

## Technical Design

### Agent Changes

```go
// internal/agent/tests.go
type TestRunner struct {
    a *Agent
}

func (tr *TestRunner) RunTests(ctx context.Context) TestResults {
    // Run existing test command
    // Parse results (pass/fail counts)
    // Return structured results
}

// Report test status in heartbeat
type HeartbeatPayload struct {
    // ... existing fields ...
    TestStatus *protocol.StatusCheck  // Test compartment status
}
```

### Dashboard Changes

```go
// internal/ops/registry.go
// Existing "test" op, enhanced with compartment tracking

func opTest() *Op {
    return &Op{
        ID:          "test",
        Description: "Run system tests",
        Validator: func(ctx context.Context, h Host) error {
            // Tests can always run
            return nil
        },
        PostCheck: func(ctx context.Context, h Host, exitCode int) error {
            // Update test compartment based on exit code
            if exitCode != 0 {
                // Mark tests as failed (red)
                return fmt.Errorf("tests failed")
            }
            // Mark tests as passed (green)
            return nil
        },
        CanRunOnDashboard: false,
        CanRunOnAgent:     true,
    }
}
```

### Database Schema

```sql
-- Add test_status_json to hosts table
ALTER TABLE hosts ADD COLUMN test_status_json TEXT;

-- Track test history
CREATE TABLE IF NOT EXISTS test_runs (
    id          TEXT PRIMARY KEY,
    host_id     TEXT NOT NULL,
    started_at  DATETIME NOT NULL,
    finished_at DATETIME,
    exit_code   INTEGER,
    passed      INTEGER,
    failed      INTEGER,
    output      TEXT,
    FOREIGN KEY (host_id) REFERENCES hosts(id)
);
CREATE INDEX IF NOT EXISTS idx_test_runs_host ON test_runs(host_id, started_at DESC);
```

---

## Test Auto-Run Workflow

```
User clicks "Switch" on host:
         ↓
1. Switch command sent
         ↓
2. Switch completes (exit 0)
         ↓
3. System compartment: 🟢 Green
   Tests compartment:  🟡 Yellow (not run yet)
         ↓
4. [If auto-run enabled] Test command sent automatically
         ↓
5. Tests running
   Tests compartment:  🔵 Blue (animated)
         ↓
6a. Tests pass (exit 0)
    Tests compartment: 🟢 Green

6b. Tests fail (exit != 0)
    Tests compartment: 🔴 Red
    Show notification: "Tests failed on gpc0"
    Offer rollback option
```

---

## UI Components

### Compartment Button

```html
<!-- Fifth compartment in host table -->
<button
  class="compartment-btn"
  data-compartment="tests"
  @click="showTestResults()"
>
  <svg class="icon icon-tests"><use href="#icon-check-circle"></use></svg>
  <span
    class="compartment-indicator compartment-indicator--{{ getTestsStatus() }}"
  ></span>
</button>
```

### Test Results Popup

```
┌─────────────────────────────────────────────────┐
│ gpc0 - Test Results                             │
├─────────────────────────────────────────────────┤
│ ✓ Network connectivity                          │
│ ✓ SSH service                                   │
│ ✓ X11 display                                   │
│ ✗ GPU acceleration           <- FAILED          │
│   Error: nvidia-smi not found                   │
│ ✓ Docker daemon                                 │
│ ✓ Home directory permissions                    │
│ ✓ Nix store integrity                           │
│ ✓ System services                               │
│                                                 │
│ 7/8 tests passed                                │
│ Run time: 12s                                   │
│                                                 │
│     [View Full Output]  [Rollback]  [Close]     │
└─────────────────────────────────────────────────┘
```

---

## Rollback Integration

When tests fail (🔴):

1. **Automatic prompt**: "Tests failed. Rollback to previous generation?"
2. **User chooses**:
   - Rollback → Runs `nixos-rebuild --rollback switch`
   - Fix manually → Dismisses prompt, keeps current system
3. **After rollback**: Tests show yellow (need to re-run on old generation)

---

## Testing Strategy

### Unit Tests

```go
func TestTestCompartmentStates(t *testing.T) {
    // After switch, before test → yellow
    // During test → blue/working
    // Test pass → green
    // Test fail → red
}
```

### Integration Tests

- [ ] Auto-run tests after switch
- [ ] Manual test trigger works
- [ ] Test results persist across dashboard restart
- [ ] Test compartment updates in real-time via WebSocket

---

## Configuration Options

```yaml
# Per-host in nixcfg
nixfleet.hosts.gpc0 = {
  tests = {
    enabled = true;
    autoRunAfterSwitch = true;
    timeout = 60;  # seconds
  };
};
```

Or in dashboard settings UI (future: P6400).

---

## Out of Scope

- Custom test definitions per-host (future: P5401)
- Test history chart/trends (future: P5402)
- Test notifications via email/webhook (future: P5403)
- Parallel test execution (future: P5404)

---

## Related

- **P5200**: Lock Compartment - Version-Based Tracking
- **P5300**: System Compartment - Inference-Based Status
- **P5500**: Generation Tracking and Visibility
- **P5600**: Rollback Operations
