# T-P4385: Button States & Locking Test

**Feature**: P4385 - UI: Button States & Locking
**Priority**: High
**Last Verified**: 2025-12-15

---

## Preconditions

- [ ] NixFleet dashboard is running at https://fleet.barta.cm/
- [ ] User is logged in to the dashboard
- [ ] At least one host is online with all buttons enabled

---

## Test Steps

### Step 1: Verify Offline Host Buttons Disabled

1. Find a host that is offline (red/gray status dot)
2. Look at the action buttons for that host

**Expected**: All buttons (Pull, Switch, Test) are disabled (grayed out, not clickable)

### Step 2: Verify Buttons Disable on Command

1. Find an online host with enabled buttons
2. Click the "Pull" button
3. Immediately look at the other buttons (Switch, Test)

**Expected**:

- All buttons become disabled immediately after clicking
- A progress indicator or badge appears showing the pending command

### Step 3: Verify Buttons Re-enable After Command Complete

1. Wait for the Pull command to complete (watch the logs panel or status)
2. Look at the buttons again

**Expected**: All buttons become enabled again after command completes

### Step 4: Verify Double-Click Prevention

1. Find an online host
2. Try to double-click the Switch button very quickly

**Expected**: Only one command is sent (check browser console for duplicate API calls)

### Step 5: Verify Stop Button Stays Enabled (During Command)

1. Click any command button (Pull, Switch, or Test)
2. While the command is running, look at the Test/Stop button

**Expected**:

- Test button transforms into a Stop button (red, with stop icon)
- Stop button remains ENABLED while other buttons are disabled

---

## Pass/Fail Criteria

| Criterion                          | Pass | Fail |
| ---------------------------------- | ---- | ---- |
| Offline host buttons disabled      | [ ]  | [ ]  |
| Buttons disable on command click   | [ ]  | [ ]  |
| Buttons re-enable after completion | [ ]  | [ ]  |
| Double-click prevented             | [ ]  | [ ]  |
| Stop button enabled during busy    | [ ]  | [ ]  |

**Overall Result**: [ ] PASS / [ ] FAIL

---

## Notes

_Record any observations or issues here_
