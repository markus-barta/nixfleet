# T-P4395: Stop Command Test

**Feature**: P4395 - Stop Command Implementation
**Priority**: High (Safety-Critical)
**Last Verified**: 2025-12-15

---

## Preconditions

- [ ] NixFleet dashboard is running at https://fleet.barta.cm/
- [ ] User is logged in to the dashboard
- [ ] At least one host is online (preferably a test host, not production)
- [ ] Browser console open to see logs

---

## Test Steps

### Step 1: Start a Long-Running Command

1. Find an online host
2. Click the "Test" button to start a test command (tests typically take a few seconds)
3. Alternatively, you could trigger a "Switch" on a dev host

**Expected**:

- Command starts running
- Test button transforms into a red Stop button
- Other buttons (Pull, Switch) become disabled
- Status dot changes to yellow/running state

### Step 2: Verify Stop Button Appearance

1. While the command is running, examine the Stop button

**Expected**:

- Button is RED (danger styling)
- Button shows a STOP icon (square)
- Button text says "Stop" (in card view) or is icon-only (in table view)
- Button is ENABLED (not grayed out)

### Step 3: Click Stop Button

1. Click the Stop button while the command is still running
2. Watch the dashboard and browser console

**Expected**:

- Stop command is sent to the agent
- Command terminates (may show "stopped" or "terminated by user" status)
- Buttons return to normal state (Test button reappears)
- Host returns to idle state (green status dot)

### Step 4: Verify Agent Received Stop

1. Check agent logs (if accessible) or dashboard logs
2. Look for SIGTERM/SIGKILL messages

**Expected**:

- Agent log shows "stopping command" or similar
- Agent log shows SIGTERM sent to process group
- Command exit code is 130 (SIGTERM) or similar

### Step 5: Verify Stop With No Command (Edge Case)

1. Ensure no command is running on a host
2. Use browser console to manually send a stop command:
   ```javascript
   sendCommand("hostname", "stop");
   ```

**Expected**:

- Agent handles gracefully (no crash)
- Returns "no command running" error or similar
- Dashboard does not break

---

## Pass/Fail Criteria

| Criterion                        | Pass | Fail |
| -------------------------------- | ---- | ---- |
| Test→Stop button swap on command | [ ]  | [ ]  |
| Stop button is red and enabled   | [ ]  | [ ]  |
| Stop terminates running command  | [ ]  | [ ]  |
| Buttons return to normal after   | [ ]  | [ ]  |
| Stop with no command is graceful | [ ]  | [ ]  |

**Overall Result**: [ ] PASS / [ ] FAIL

---

## Notes

_Record any observations or issues here_

---

## Safety Notes

- This feature is PRD FR-1.11: **MUST** priority
- Essential for canceling runaway commands
- Uses process group kill (kills child processes too)
- SIGKILL fallback after 3 seconds if SIGTERM fails
