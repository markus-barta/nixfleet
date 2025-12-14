# T08 - End-to-End Test Flow

**Backlog**: P4000, P4200 (Agent + Dashboard)
**Priority**: Must Have

---

## Purpose

Verify the complete test execution flow from user click to final results with progress tracking.

---

## Prerequisites

- Dashboard running
- Agent connected on a host with test scripts
- Test scripts in `hosts/{hostname}/tests/T*.sh`
- User authenticated in browser

---

## Flow Diagram

```text
┌─────────┐       ┌───────────┐       ┌─────────┐
│ Browser │       │ Dashboard │       │  Agent  │
└────┬────┘       └─────┬─────┘       └────┬────┘
     │                  │                  │
     │ Click "Test"     │                  │
     │─────────────────▶│                  │
     │                  │                  │
     │                  │ WS: command=test │
     │                  │─────────────────▶│
     │                  │                  │
     │ WS: command_queued                  │ Count tests
     │◀─────────────────│                  │─────────────
     │                  │                  │
     │                  │ WS: test_progress│ Run T01.sh
     │                  │◀─────────────────│─────────────
     │                  │ (1/5, passed: 1) │
     │ WS: test_progress│                  │
     │◀─────────────────│                  │
     │                  │                  │
     │ Show: ✦ 1/5      │                  │ Run T02.sh
     │                  │ WS: test_progress│─────────────
     │                  │◀─────────────────│
     │                  │ (2/5, passed: 2) │
     │ WS: test_progress│                  │
     │◀─────────────────│                  │
     │                  │                  │
     │ Show: ✦ 2/5      │ ... repeat ...   │
     │                  │                  │
     │                  │ WS: test_progress│
     │                  │◀─────────────────│
     │                  │ (5/5, passed: 4, │
     │                  │  running: false) │
     │ WS: test_progress│                  │
     │◀─────────────────│                  │
     │                  │                  │
     │ Show: 4/5 passed │                  │
     │ (color: warning) │                  │
```

---

## Scenarios

### Scenario 1: All Tests Pass

**Given** a host has 5 test scripts
**And** all scripts exit with code 0
**When** the user clicks "Test"
**Then** the UI shows "Testing..." with progress indicator
**And** progress updates as each test completes: 1/5, 2/5, ...
**And** final result shows "5/5 passed" with green indicator
**And** buttons are re-enabled

### Scenario 2: Some Tests Fail

**Given** a host has 5 test scripts
**And** 2 scripts fail (exit code != 0)
**When** the user clicks "Test"
**Then** progress updates show pass/fail counts
**And** final result shows "3/5 passed" with yellow/red indicator
**And** failed test names are shown in the result

### Scenario 3: No Tests Defined

**Given** a host has no test scripts
**When** the user clicks "Test"
**Then** the agent reports "no tests"
**And** the UI shows "No tests defined"
**And** this is not treated as an error

### Scenario 4: Test Timeout

**Given** a test script runs for > 5 minutes
**When** the user clicks "Stop"
**Then** the running test is killed
**And** the result shows "Stopped by user"
**And** partial results are preserved

### Scenario 5: Heartbeats During Tests

**Given** tests are running (may take several minutes)
**When** tests are in progress
**Then** heartbeats continue
**And** the host never appears as "stale"

---

## Test Script Contract

Tests must follow this contract:

```bash
#!/usr/bin/env bash
# T01-example-test.sh

# Exit 0 for pass, non-zero for fail
# Output is captured but not required

if some_condition; then
    exit 0  # Pass
else
    echo "Expected X but got Y" >&2
    exit 1  # Fail
fi
```

---

## Verification Steps

### Manual Test

```bash
# 1. Create test scripts on target host
ssh hsb0 "mkdir -p ~/Code/nixcfg/hosts/hsb0/tests"
ssh hsb0 "echo '#!/bin/bash\nexit 0' > ~/Code/nixcfg/hosts/hsb0/tests/T01-pass.sh"
ssh hsb0 "echo '#!/bin/bash\nexit 1' > ~/Code/nixcfg/hosts/hsb0/tests/T02-fail.sh"
ssh hsb0 "chmod +x ~/Code/nixcfg/hosts/hsb0/tests/*.sh"

# 2. Trigger tests from dashboard
# Click "Test" on hsb0

# 3. Observe progress updates
# Should see "1/2", then "2/2"

# 4. Verify final result
# Should show "1/2 passed"
```

### Automated Test

```go
func TestE2E_TestFlow(t *testing.T) {
    // Setup
    dashboard := startDashboard(t)
    agent := startAgent(t, "test-host")
    browser := connectBrowserWS(t, dashboard)

    // Configure agent with mock tests
    agent.setTestScripts([]TestScript{
        {Name: "T01-pass", ExitCode: 0},
        {Name: "T02-pass", ExitCode: 0},
        {Name: "T03-fail", ExitCode: 1},
    })

    // Send test command
    sendCommand(t, dashboard, "test-host", "test")

    // Verify progress updates
    progress1 := browser.waitForMessage(t, "test_progress")
    assert.Equal(t, 1, progress1.Current)
    assert.Equal(t, 3, progress1.Total)
    assert.True(t, progress1.Running)

    progress2 := browser.waitForMessage(t, "test_progress")
    assert.Equal(t, 2, progress2.Current)

    // Verify final result
    final := browser.waitForMessage(t, "test_progress")
    assert.False(t, final.Running)
    assert.Equal(t, 2, final.Passed)
    assert.Equal(t, 3, final.Total)
}
```

---

## UI States

| State       | Display         | Color         |
| ----------- | --------------- | ------------- |
| No tests    | "No tests"      | Gray          |
| Running     | "✦ 3/5"         | Blue          |
| All passed  | "5/5 passed"    | Green         |
| Some failed | "3/5 passed"    | Yellow/Orange |
| All failed  | "0/5 passed"    | Red           |
| Stopped     | "Stopped (2/5)" | Gray          |

---

## Related

- T03: Agent Commands (test command execution)
- T06: Dashboard Commands (test progress handling)
- T07: E2E Deploy Flow (similar pattern)
