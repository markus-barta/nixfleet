# P8800: Scrollbar Wiggle Fix (3rd Iteration)

**Priority**: P8800 (High - UX Polish)  
**Type**: Bug  
**Status**: Backlog  
**Epic**: UI/UX Polish

## Summary

Page wiggles left/right by a few pixels every time a host sends a heartbeat (or no hosts sends one), caused by vertical scrollbar appearing/disappearing. This is the **THIRD TIME** we're fixing this issue.

## Symptoms

1. **Horizontal wiggle**: Entire page shifts left/right by ~10-15px
2. **Heartbeat fade clipping**: Ripple animation edges are cut off, doesn't smoothly fade with background on all sides - looks like image is larger than div
3. **Scrollbar flickering**: Vertical scrollbar appears/disappears on each heartbeat (or no hosts send one)

## Root Causes

### Cause 0: JS Clobbers the Last-Seen Cell Classes (CSS Fixes Don’t Stick)

**File**: `src/internal/templates/dashboard.templ` (around `renderHost()` and the 1s last-seen interval)

- The table renders `Last Seen` cells as `class="col-right col-last-seen"` (stable layout hook).
- But the live updater does `cell.className = result.className` (e.g. `last-seen-ok`), which **removes** `col-last-seen` and `col-right`.
- Result: Any CSS width rule added for `.col-last-seen` silently stops applying after the first tick/heartbeat, and the table can keep reflowing.

### Cause 1: Table Column Width Not Fixed

**File**: `src/internal/templates/base.templ` (lines ~418-478)

- `.host-table` has no `table-layout: fixed`
- Columns can dynamically resize based on content
- Every text change causes table reflow and potential height change

### Cause 2: Last Seen Column Has No Fixed Width

**File**: `src/internal/templates/base.templ` (no width defined for `.col-last-seen`)

- Last Seen text updates **every second** (dashboard.templ:4018)
- Text changes width: `"5s"` (2ch) → `"59s"` (3ch) → `"1m"` (2ch) → `"10m"` (3ch) → `"1h"` (2ch) → `"10h"` (3ch)
- Variable-width text in unfixed column causes horizontal reflow
- Horizontal reflow changes total page width, triggering vertical scrollbar appearance/disappearance

### Cause 3: Ripple Animation Not Fully Contained

**File**: `src/internal/templates/base.templ` (lines 599-659)

- `.status-ripple` is 12px × 12px container
- `.hb-wave` elements scale to `transform: scale(2.5)` = **30px diameter**
- Even with `overflow: hidden` (line 607), the expansion happens during animation and may cause container to temporarily expand before clipping
- Absolute positioning doesn't guarantee layout containment
- The ripple fade is being clipped visibly instead of smoothly fading

### Cause 4: Dynamic Content Changes

**File**: `src/internal/templates/dashboard.templ` (lines 4017-4031)

- Last Seen updates every 1000ms
- Metrics update on heartbeat
- Multiple hosts heartbeating near-simultaneously compounds the issue
- Each update can trigger a reflow cycle

## Previous Fix Attempts

### Attempt 1: `overflow: hidden` on containers

- **Line 607**: `.status-ripple { overflow: hidden; }`
- **Line 1118**: `td.status-cell-with-badge { overflow: hidden; }`
- **Result**: Partial fix, but ripple still causes expansion before clipping

### Attempt 2: Force scrollbar always visible

- **Line 80**: `html { overflow-y: scroll !important; scrollbar-gutter: stable !important; }`
- **Result**: Works if page is tall enough, but doesn't help when page height changes dynamically

## Why It Still Happens

The scrollbar is forced visible, but:

1. When table reflows due to text width changes, total page height changes
2. Page height < viewport height → scrollbar becomes inactive but still visible
3. Page height > viewport height → scrollbar becomes active and functional
4. This state change causes the content area width to adjust by scrollbar width (~10-15px)
5. Result: horizontal wiggle

## Exact Fixes Required

### Fix 1: Force Table Layout Fixed ⭐ PRIMARY FIX

**File**: `src/internal/templates/base.templ`
**Location**: Line ~418, in `.host-table` style block

Add:

```css
.host-table {
  width: 100%;
  border-collapse: collapse;
  background: rgba(36, 40, 59, 0.9);
  border-radius: 8px;
  overflow: visible;
  position: relative;
  z-index: 1;
  table-layout: fixed; /* ← ADD THIS */
}
```

**Why**: Prevents columns from dynamically resizing based on content changes. All column widths are determined by first row and stay fixed.

### Fix 0: Stop Overwriting Last-Seen Cell Classes (Required for Fix 2 to Work)

**File**: `src/internal/templates/dashboard.templ`

- Replace `cell.className = result.className` with a `classList` toggle that only updates `last-seen-{ok,warn,stale}`.
- Keep `col-last-seen` intact so the fixed-width column rule remains active.

### Fix 2: Set Fixed Width for Last Seen Column

**File**: `src/internal/templates/base.templ`
**Location**: Add new rule after `.col-select` definition (around line 2173)

Add:

```css
/* Last Seen Column - Fixed width to prevent reflow */
.col-last-seen {
  width: 80px;
  min-width: 80px;
  max-width: 80px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
```

**Why**: Fixes the column width so text changes ("5s" ↔ "59s") can't cause reflow. 80px accommodates "Last Seen" header + largest expected value ("999d").

### Fix 3: Add Containment to Status Cell

**File**: `src/internal/templates/base.templ`
**Location**: Line ~1116, in `td.status-cell-with-badge` block

Change from:

```css
td.status-cell-with-badge {
  vertical-align: middle;
  overflow: hidden; /* P1000-FIX: Prevent ripple animation from causing scrollbar wiggle */
}
```

To:

```css
td.status-cell-with-badge {
  vertical-align: middle;
  overflow: hidden;
  contain: layout style paint; /* P8800: Full containment */
  width: 120px; /* P8800: Fixed width for status + badge */
  min-width: 120px;
}
```

**Why**: CSS `contain` property creates an isolation boundary. Prevents animation calculations from affecting outer layout. Fixed width prevents badge text ("pull", "switch", "test") from causing reflow.

### Fix 4: Improve Ripple Containment Strategy

**File**: `src/internal/templates/base.templ`
**Location**: Line ~599, in `.status-ripple` block

Change from:

```css
.status-ripple {
  width: 12px;
  height: 12px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--green);
  overflow: hidden; /* P1000-FIX: Prevent ripple animation from causing scrollbar wiggle */
}
```

To:

```css
.status-ripple {
  width: 12px;
  height: 12px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--green);
  overflow: hidden;
  contain: strict; /* P8800: Strict containment */
  /* Ensure explicit sizing so contain:size doesn't break layout */
  flex-shrink: 0;
}
```

**Why**: `contain: strict` (layout + style + paint + size) creates strongest isolation. Ripple animations absolutely cannot affect outside layout. `flex-shrink: 0` ensures container stays 12px even under flex pressure.

### Fix 5: Reduce Ripple Scale (Optional - Visual Polish)

**File**: `src/internal/templates/base.templ`
**Location**: Line ~649, in `@keyframes ripple-wave`

Change from:

```css
@keyframes ripple-wave {
  0% {
    transform: scale(1);
    opacity: 0.8;
  }
  100% {
    transform: scale(2.5); /* 30px diameter */
    opacity: 0;
  }
}
```

To:

```css
@keyframes ripple-wave {
  0% {
    transform: scale(1);
    opacity: 0.8;
  }
  100% {
    transform: scale(2); /* P8800: Reduced from 2.5 to 2.0 (24px) */
    opacity: 0;
  }
}
```

**Why**: Smaller ripple (2.0x = 24px vs 2.5x = 30px) has less chance of causing any layout anomalies. Still visible and effective. Better fade on all sides since it's not pushing container boundaries as hard.

### Fix 6: Set Fixed Widths for Other Dynamic Columns

**File**: `src/internal/templates/base.templ`
**Location**: Add after `.col-last-seen` rule

Add:

```css
/* Other columns with dynamic content - fixed widths */
.metrics-cell {
  width: 120px;
  min-width: 120px;
}

.agent-version-cell {
  width: 100px;
  min-width: 100px;
}

.gen-cell {
  width: 90px;
  min-width: 90px;
}

.status-cell {
  width: 140px;
  min-width: 140px;
}
```

**Why**: These cells also have dynamic content (metrics values, version strings, generation hashes, compartment states). Fix all of them to prevent ANY text change from causing reflow.

## Testing Checklist

After implementing fixes:

- [ ] Open dashboard with 5+ hosts
- [ ] Observe heartbeat ripples - no horizontal wiggle
- [ ] Check ripple animation - smooth fade on all sides, not clipped
- [ ] Let Last Seen tick from "5s" → "59s" → "1m" → "10m" - no wiggle
- [ ] Check metrics updates - no wiggle
- [ ] Check generation updates - no wiggle
- [ ] Check with browser dev tools:
  - Table maintains fixed layout
  - No reflow/repaint storms in Performance panel
  - Scrollbar stays consistently visible or hidden
- [ ] Test with browser window sized to barely need scrollbar (edge case)
- [ ] Test with multiple hosts heartbeating simultaneously

## Related Documentation

- **Issue previously documented**: Not found in +pm - this is an undocumented recurring issue
- **Previous fix comments in code**:
  - Line 607: `/* P1000-FIX: Prevent ripple animation from causing scrollbar wiggle */`
  - Line 1118: `/* P1000-FIX: Prevent ripple animation from causing scrollbar wiggle */`
- **Related work**:
  - P2600: Context bar polish (footer jumping)
  - P4370: UI Table Columns
  - P5010: Compartment Status Indicator

## Implementation Notes

- **Do ALL fixes together** - partial fixes haven't worked in previous attempts
- **Primary fix is Fix 1** (`table-layout: fixed`) - this addresses root cause
- **Fixes 2-6 are defense in depth** - prevent any text reflow
- **Test thoroughly** before considering done - this is 3rd attempt

## Dependencies

- None - pure CSS fixes

## Acceptance Criteria

- ✅ No horizontal page wiggle during heartbeats
- ✅ No horizontal page wiggle during Last Seen updates
- ✅ Ripple animation fully visible and smoothly fades on all sides
- ✅ Scrollbar stays stable (visible or hidden, but doesn't flicker)
- ✅ Table layout remains stable with dynamic content updates
- ✅ No performance regressions (check with dev tools)

## Notes for Future Self

This is the **THIRD iteration** of this fix. If it breaks again:

1. **Check** if `table-layout: fixed` got removed/overridden
2. **Check** if new dynamic content was added to table without fixed width
3. **Check** if ripple animation scale was increased
4. **Check** browser console for layout thrashing warnings
5. **Consider** disabling ripple animation entirely and using simpler pulse
6. **Last resort**: Make ripple non-animated static indicator

The fundamental issue is: **ANY dynamic content in an unfixed-width table can cause reflow → height change → scrollbar state change → horizontal wiggle**.

The **nuclear option** if this persists: Remove ripple animation entirely, use simple pulsing opacity on a static dot.
