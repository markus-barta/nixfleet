# T-P4370: Table Columns Test

**Feature**: P4370 - UI: Complete Table Columns
**Priority**: High
**Last Verified**: 2025-12-15

---

## Preconditions

- [ ] NixFleet dashboard is running at https://fleet.barta.cm/
- [ ] User is logged in to the dashboard
- [ ] At least one NixOS host and one macOS host are registered
- [ ] Browser window is wide enough to show table view (>1024px)

---

## Test Steps

### Step 1: Verify Column Layout

1. Look at the host table columns
2. Verify the following columns are present (left to right):
   - Status (with ripple/dot indicator)
   - Host (hostname with theme color)
   - Type (OS icon)
   - Metrics (CPU/RAM)
   - Version (OS version)
   - Generation (NixOS generation)
   - Last Seen (relative time)
   - Actions (buttons)

**Expected**: All 8 columns visible with proper alignment

### Step 2: Verify Column Widths

1. Look at the column widths
2. Status column should be narrow (~80px)
3. Type column should be narrow (~50px, icon only)
4. Actions column should be wide enough for all buttons

**Expected**: Columns have appropriate widths, not too wide or too narrow

### Step 3: Verify OS Type Icons

1. Find a NixOS host in the table
2. Look at the Type column

**Expected**: NixOS snowflake icon displayed (not text "nixos")

3. Find a macOS host in the table
4. Look at the Type column

**Expected**: Apple icon displayed (not text "macos")

### Step 4: Verify Heartbeat Ripple Animation

1. Find an online host (green status)
2. Look at the Status column

**Expected**: Animated ripple effect emanating from status dot

3. Find an offline host (red status)
4. Look at the Status column

**Expected**: Static red dot (no animation)

### Step 5: Verify Metrics Display

1. Find an online host with metrics
2. Look at the Metrics column

**Expected**:

- CPU icon followed by percentage
- RAM icon followed by percentage
- Values highlighted in warning color if ≥80%

3. Hover over the RAM metric

**Expected**: Tooltip shows "RAM: X%, Swap: Y%, Load: Z"

### Step 6: Verify Offline Host Dimming

1. Find an offline host row
2. Compare its appearance to an online host row

**Expected**: Offline row is dimmed (~50% opacity)

3. Hover over the offline row

**Expected**: Opacity increases slightly on hover

---

## Pass/Fail Criteria

| Criterion                | Pass | Fail |
| ------------------------ | ---- | ---- |
| All 8 columns present    | [ ]  | [ ]  |
| Column widths reasonable | [ ]  | [ ]  |
| NixOS icon shown         | [ ]  | [ ]  |
| macOS icon shown         | [ ]  | [ ]  |
| Heartbeat ripple works   | [ ]  | [ ]  |
| Metrics icons displayed  | [ ]  | [ ]  |
| Metrics hover titles     | [ ]  | [ ]  |
| Offline hosts dimmed     | [ ]  | [ ]  |

**Overall Result**: [ ] PASS / [ ] FAIL

---

## Known Limitations (Blocked on Agent Changes)

The following features are NOT implemented (require agent changes):

- Location column (cloud/home/work icons)
- Device Type column (server/desktop/laptop/gaming icons)
- Tests column (progress/results)
- Config hash column

These are documented but deferred until agent metadata is extended.

---

## Notes

_Record any observations or issues here_
