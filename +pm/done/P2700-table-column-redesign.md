# P2700: Table Column Redesign (TYPE + STATUS)

**Priority**: Medium  
**Complexity**: Medium-High  
**Depends On**: P2500 (Streamline Commands) - completed  
**Status**: Backlog  
**Created**: 2025-12-20

---

## Overview

Redesign the host table to:

1. **Consolidate 3 columns → 1**: Merge LOC, DEV, TYPE into a single composite "TYPE" column
2. **Replace TESTS with STATUS**: New segmented progress bar showing operation status + test results

**Before (9 columns):**

```
| HOSTS | LOC | DEV | TYPE | METRICS | UPDATE | TESTS | LAST SEEN | ⋮ |
```

**After (7 columns):**

```
| HOSTS | TYPE | METRICS | UPDATE | STATUS | LAST SEEN | ⋮ |
```

---

## Part 1: Composite TYPE Column

### Design

Combine location, device, and OS type into a single cell with layered icons:

```
┌────────────────┐
│         🖥️     │  ← DEV icon (25%, top-right superscript)
│.   ☁️          │  ← LOC icon (80%, center-Y, left-aligned)
│         ❄️     │  ← TYPE icon (25%, bottom-right subscript)
└────────────────┘
```

### Specifications

| Element                         | Size | Position                 |
| ------------------------------- | ---- | ------------------------ |
| **LOC** (cloud/home/office)     | 80%  | Center Y, left-aligned   |
| **DEV** (server/desktop/laptop) | 25%  | Top-right (superscript)  |
| **TYPE** (NixOS/macOS)          | 25%  | Bottom-right (subscript) |

### Column Header

`TYPE`

### Context Bar on Hover

```
hsb0: Cloud server running NixOS
imac0: Home desktop running macOS
```

### CSS Structure

```css
.type-composite {
  position: relative;
  display: flex;
  align-items: center;
  width: 48px;
  height: 40px;
}

.type-loc {
  width: 80%;
  height: 80%;
  /* Left-aligned, center Y */
}

.type-dev {
  position: absolute;
  top: 0;
  right: 0;
  width: 25%;
  height: 25%;
  opacity: 0.8;
}

.type-os {
  position: absolute;
  bottom: 0;
  right: 0;
  width: 25%;
  height: 25%;
  opacity: 0.8;
}
```

---

## Part 2: STATUS Column (replaces TESTS)

### Segmented Progress Bar

```
  PULL      LOCK     SYSTEM       TESTS
  ●●●○      ●●       ●●○          ●●●○○○○○
  (4)       (2)      (3)          (max 8)
```

### Segments

| Segment    | Dots | Meaning                                         |
| ---------- | ---- | ----------------------------------------------- |
| **PULL**   | 4    | fetch → merge → verify → complete               |
| **LOCK**   | 2    | flake.lock sync → agent version check           |
| **SYSTEM** | 3    | Started → Built → Validated                     |
| **TESTS**  | 1-8  | One per test file (capped at 8), or `—` if none |

**Separator**: space character between segments

### Dot States

| Symbol | Meaning          | Color/Style                  |
| ------ | ---------------- | ---------------------------- |
| `○`    | Not started      | dim gray (30% opacity)       |
| `◐`    | In progress      | cyan + PS5 shimmer animation |
| `●`    | Complete/success | green                        |
| `✗`    | Failed           | red                          |

### Visual States

```
Idle (all good):      ●●●● ●● ●●● ●●●●●     (90% transparent green, tests visible)
Idle (test failed):   ●●●● ●● ●●● ●●✗●●     (red X stands out)
Idle (no tests):      ●●●● ●● ●●● —         (dash for no tests)
Pull in progress:     ●●◐○ ○○ ○○○ ○○○○○     (shimmer on current)
Lock syncing:         ●●●● ◐○ ○○○ ○○○○○     (1st lock dot)
Agent updating:       ●●●● ●◐ ○○○ ○○○○○     (2nd lock dot)
System building:      ●●●● ●● ●◐○ ○○○○○     (shimmer)
Tests running:        ●●●● ●● ●●● ●●●◐○     (current test shimmers)
```

### Idle State Behavior

When nothing is running:

- **Previous segments** (PULL, LOCK, SYSTEM): 90% transparent green `●`
- **TESTS segment**: Actual results visible (e.g., `●●●✗●` = 4 passed, 1 failed)
- Focus stays on what matters (test results) while showing "everything else is fine"

### PS5-Style Shimmer Animation

```css
@keyframes shimmer {
  0% {
    opacity: 0.4;
    transform: scale(0.95);
  }
  50% {
    opacity: 1;
    transform: scale(1.05);
  }
  100% {
    opacity: 0.4;
    transform: scale(0.95);
  }
}

.dot-in-progress {
  animation: shimmer 1.2s ease-in-out infinite;
  color: var(--cyan);
  text-shadow: 0 0 8px var(--cyan);
}
```

### Context Bar on Hover

**Idle state:**

```
hsb0: All systems nominal • Tests: 5/5 passed
```

**During operation:**

```
hsb0: Pull complete • Lock synced • Switch building (45s elapsed) • Tests pending
```

**During test run:**

```
hsb0: Running test 3/5: T03-network.sh (started 12s ago)
```

### Column Width

Flexible/responsive:

- Minimum: fits 4+2+3+8 dots with spaces (~17 characters)
- Maximum: expand to fill available space

---

## Implementation Tasks

### Phase 1: TYPE Column Consolidation

- [ ] Create composite TYPE cell component with layered icons
- [ ] Update table header (remove LOC, DEV columns; rename TYPE)
- [ ] Add context bar hover handler for TYPE column
- [ ] Remove old LOC, DEV, TYPE column renderers
- [ ] Test responsive behavior at various widths

### Phase 2: STATUS Column Structure

- [ ] Create STATUS cell component with dot segments
- [ ] Define data structure for progress state per host
- [ ] Implement dot rendering (○, ●, ✗, ◐)
- [ ] Add separator spacing between segments
- [ ] Handle variable test count (1-8 dots, or `—`)

### Phase 3: Progress Tracking

- [ ] Update WebSocket messages to include granular progress
- [ ] Track PULL progress (4 steps)
- [ ] Track LOCK progress (2 steps: lock sync, agent)
- [ ] Track SYSTEM progress (3 steps: started, built, validated)
- [ ] Track TESTS progress (per-test status)

### Phase 4: Animation & Polish

- [ ] Implement PS5-style shimmer animation for in-progress dots
- [ ] Style idle state (90% transparent previous segments)
- [ ] Add context bar hover details for STATUS
- [ ] Test animation performance (many hosts updating)

### Phase 5: Backend Support

- [ ] Extend agent protocol to report granular progress
- [ ] Add progress fields to host state
- [ ] Handle progress timeout/stale states

---

## Technical Notes

### Progress State Structure

```javascript
hostProgress: {
  pull: { current: 2, total: 4, status: 'in_progress' },
  lock: { current: 1, total: 2, status: 'complete' },
  system: { current: 0, total: 3, status: 'pending' },
  tests: {
    current: 0,
    total: 5,
    results: [null, null, null, null, null],  // null=pending, true=pass, false=fail
    status: 'pending'
  }
}
```

### Dot Rendering Logic

```javascript
function renderDot(step, current, status) {
  if (status === "error") return "✗";
  if (step < current) return "●"; // completed
  if (step === current && status === "in_progress") return "◐"; // animating
  return "○"; // pending
}
```

---

## Success Criteria

1. Table has 7 columns instead of 9
2. TYPE column shows all 3 icons in layered layout
3. STATUS shows real-time progress during operations
4. PS5-style shimmer animates smoothly
5. Idle state shows test results prominently
6. Context bar provides detailed hover info
7. Responsive layout works on mobile

---

## Related Items

- P2500: Streamline and Unify Commands (parent, completed)
- P2600: Context Bar Polish (sibling)
- P1000: Update UX Overhaul (ancestor, completed)
