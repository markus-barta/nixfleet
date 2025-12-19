# P5100 - Update Column UI Simplification

**Created**: 2025-12-19  
**Priority**: P5100 (Medium - UI Polish)  
**Status**: Backlog  
**Effort**: Small (1-2 hours)

---

## Problem

The update column compartments have inconsistent visual states that were never intended:

1. **Lock compartment turns red** when agent is outdated or in error state
2. **Offline hosts show inconsistent compartments** — some have backgrounds, some don't
3. **Icons light up white** in `needs-update` and `error` states
4. **`unknown` state makes compartment invisible** (transparent background)

The current design has too many visual signals competing for attention.

---

## Root Cause Analysis

### CSS Classes Currently Applied

| State        | Class Added     | Background       | Icon Color             |
| ------------ | --------------- | ---------------- | ---------------------- |
| OK           | (none)          | `#374151` gray   | `#1f2937` dark         |
| Needs Update | `.needs-update` | unchanged        | `#f9fafb` WHITE ❌     |
| Error        | `.error`        | `#7f1d1d` RED ❌ | `#fca5a5` light red ❌ |
| Unknown      | `.unknown`      | `transparent` ❌ | `#6b7280` gray         |
| PR Pending   | `.pr-pending`   | blue gradient ❌ | `#f9fafb` WHITE ❌     |

### Problem Code Locations

**CSS (base.templ, lines ~884-920):**

```css
.update-compartment.needs-update .update-icon {
  fill: #f9fafb; /* WHITE - remove */
}

.update-compartment.error {
  background: #7f1d1d; /* RED - remove */
}

.update-compartment.error .update-icon {
  fill: #fca5a5; /* LIGHT RED - remove */
}

.update-compartment.unknown {
  background: transparent; /* INVISIBLE - remove */
  opacity: 0.4;
}
```

**Go functions (dashboard.templ, lines ~1638-1743):**

- `updateCompartmentClass()` — adds `.needs-update`, `.error`, `.unknown`
- `lockCompartmentClass()` — adds `.error` when `AgentOutdated`

**JavaScript (dashboard.templ, lines ~614-644):**

- `renderUpdateStatus()` — applies same class logic client-side

---

## Solution: Simplified Design

### Design Principle

> **One signal per compartment**: Only the indicator dot changes. Everything else stays constant.

### Target State

| Component              | Behavior                              | Rationale                    |
| ---------------------- | ------------------------------------- | ---------------------------- |
| Compartment background | **Always** `#374151` gray             | Consistent visual weight     |
| Compartment on hover   | Lighten to `#4b5563`                  | Keep hover feedback          |
| Icon color             | **Always** `#1f2937` dark             | No visual noise              |
| Indicator dot          | Status colors (green/yellow/red/gray) | Single source of status info |

### CSS Changes Required

**REMOVE these rules from `base.templ`:**

```css
/* DELETE - icon should never light up */
.update-compartment.needs-update .update-icon { ... }

/* DELETE - background should never turn red */
.update-compartment.error { background: #7f1d1d; }
.update-compartment.error .update-icon { ... }

/* MODIFY - keep icon gray but restore background */
.update-compartment.unknown {
    background: #374151;  /* Same as base, not transparent */
    /* Remove opacity: 0.4 OR keep for subtle "stale data" hint */
}

/* DELETE or MODIFY - pr-pending icon should stay dark */
.update-compartment.pr-pending .update-icon { ... }
```

**KEEP these rules:**

```css
/* Base compartment styling - unchanged */
.update-compartment { ... }
.update-compartment:hover { ... }
.update-compartment .update-icon { ... }

/* Indicator dot styling - this is the ONLY status signal */
.compartment-indicator--ok { ... }
.compartment-indicator--warning { ... }
.compartment-indicator--error { ... }
.compartment-indicator--unknown { ... }
```

### Go Function Changes

**Simplify `updateCompartmentClass()`:**

```go
func updateCompartmentClass(status *UpdateStatus, compartment string) string {
    // Always return base class - no state-based classes
    return "update-compartment"
}
```

**Simplify `lockCompartmentClass()`:**

```go
func lockCompartmentClass(host Host) string {
    // Always return base class - no error state
    return "update-compartment"
}
```

**Keep indicator functions unchanged** — they correctly set dot colors.

### JavaScript Changes

**Simplify `renderUpdateStatus()` (lines ~614-644):**

```javascript
["git", "lock", "system"].forEach((type, i) => {
  const comp = compartments[i];
  if (!comp) return;

  const check = status[type];
  if (!check) return;

  // Remove: comp.className = 'update-compartment';
  // Remove: switch (check.status) { case 'outdated': comp.classList.add('needs-update'); ... }

  // ONLY update the indicator dot
  const indicator = comp.querySelector(".compartment-indicator");
  if (indicator) {
    indicator.className = "compartment-indicator";
    const cssStatus =
      check.status === "outdated" ? "warning" : check.status || "unknown";
    indicator.classList.add(`compartment-indicator--${cssStatus}`);
  }

  // Lock-specific: agent outdated affects indicator only
  if (type === "lock" && host.agentOutdated) {
    const indicator = comp.querySelector(".compartment-indicator");
    if (indicator) {
      indicator.className =
        "compartment-indicator compartment-indicator--error";
    }
  }
});
```

---

## Files to Modify

| File                                    | Changes                                                                                      |
| --------------------------------------- | -------------------------------------------------------------------------------------------- |
| `v2/internal/templates/base.templ`      | Remove state-based CSS for compartments/icons (lines ~884-920)                               |
| `v2/internal/templates/dashboard.templ` | Simplify `updateCompartmentClass()`, `lockCompartmentClass()`, and JS `renderUpdateStatus()` |

---

## Acceptance Criteria

- [ ] All compartments always have consistent gray background (`#374151`)
- [ ] All icons always dark (`#1f2937`) — never white, never colored
- [ ] Offline/unknown hosts show compartments with same background as online hosts
- [ ] Only indicator dot changes color based on status
- [ ] Hover effect on compartments still works
- [ ] PR-pending state: keep subtle indicator (dot or border), icon stays dark
- [ ] Manual test: verify all host states look consistent (online, offline, error, unknown)

---

## Visual Reference

### Before (Current)

```
┌─────────────────────────────────┐
│ Online host (ok):               │
│ [■ git] [■ lock] [■ system]    │  <- gray bg, dark icons, green dots
│                                 │
│ Online host (outdated):         │
│ [■ GIT] [■ LOCK] [■ SYSTEM]    │  <- icons WHITE ❌
│                                 │
│ Online host (error/agent old):  │
│ [■ git] [🔴LOCK] [■ system]    │  <- lock bg RED ❌
│                                 │
│ Offline host (unknown):         │
│ [■ git] [  lock] [  system]    │  <- some compartments INVISIBLE ❌
└─────────────────────────────────┘
```

### After (Target)

```
┌─────────────────────────────────┐
│ Online host (ok):               │
│ [■ git•] [■ lock•] [■ sys•]    │  <- gray bg, dark icons, green dots
│                                 │
│ Online host (outdated):         │
│ [■ git•] [■ lock•] [■ sys•]    │  <- SAME bg, SAME icons, YELLOW dots
│                                 │
│ Online host (error/agent old):  │
│ [■ git•] [■ lock•] [■ sys•]    │  <- SAME bg, SAME icons, RED dot on lock
│                                 │
│ Offline host (unknown):         │
│ [■ git•] [■ lock•] [■ sys•]    │  <- SAME bg, SAME icons, GRAY dots
└─────────────────────────────────┘

Legend: • = indicator dot (only thing that changes color)
```

---

## Related

- P5000 - Original three-compartment design
- P5010 - Indicator dot implementation
- P2000 - Unified Host State (larger refactor, this can land first)
