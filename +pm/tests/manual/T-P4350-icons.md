# T-P4350: SVG Icon System Test

**Feature**: P4350 - UI: SVG Icon System
**Priority**: High
**Last Verified**: 2025-12-15

---

## Preconditions

- [ ] NixFleet dashboard is running at https://fleet.barta.cm/
- [ ] User is logged in to the dashboard
- [ ] At least one host is online

---

## Test Steps

### Step 1: Verify Icon Definitions Exist

1. Open browser developer tools (F12)
2. Go to Elements tab
3. Search for `<svg class="icon-defs"`
4. Expand the `<defs>` element inside

**Expected**: Should find 24+ `<symbol>` elements with ids like `icon-download`, `icon-refresh`, `icon-flask`, etc.

### Step 2: Verify Command Buttons Have Icons

1. Look at any online host row in the table
2. Examine the action buttons (Pull, Switch, Test)

**Expected**:

- Pull button: Download icon (arrow pointing down)
- Switch button: Refresh icon (circular arrow)
- Test button: Flask icon (laboratory flask)

### Step 3: Verify Icons Inherit Color

1. Compare the icon color to the button text color
2. The icons should match the text color (using `currentColor`)

**Expected**: Icons are the same color as button text (not a separate fixed color)

### Step 4: Verify Metrics Icons

1. Look at the metrics column for any online host
2. Should show CPU and RAM with icons

**Expected**:

- CPU icon (chip/processor shape) next to CPU percentage
- RAM icon (memory stick shape) next to RAM percentage

### Step 5: Verify No Emojis or Unicode

1. Scan the entire dashboard for emoji characters
2. Look for any broken Unicode (like "Te t" instead of "Test")

**Expected**: No emojis visible, all text renders correctly

---

## Pass/Fail Criteria

| Criterion                  | Pass | Fail |
| -------------------------- | ---- | ---- |
| 24+ icon symbols in defs   | [ ]  | [ ]  |
| Command buttons have icons | [ ]  | [ ]  |
| Icons inherit color        | [ ]  | [ ]  |
| Metrics show CPU/RAM icons | [ ]  | [ ]  |
| No emojis or broken chars  | [ ]  | [ ]  |

**Overall Result**: [ ] PASS / [ ] FAIL

---

## Notes

_Record any observations or issues here_
