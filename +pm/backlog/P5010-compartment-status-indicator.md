# P5010 - Compartment Status Indicator (Mini Badge)

**Created**: 2025-12-17  
**Priority**: P5010 (UI Polish)  
**Status**: Backlog  
**Depends on**: US-7 (Update Status compartments must exist)  
**Related**: P5000 (Update Status feature)

---

## Summary

Add a small, polished status indicator (colored dot) to each of the three status compartments (Git, Lock, System). The indicator provides at-a-glance status using color alone — no icons needed at this scale.

---

## Design Specification

### Compartment Context

- Compartment size: ~30×25px
- Three compartments: Git | Lock | System

### Indicator Properties

| Property           | Value                                                    |
| ------------------ | -------------------------------------------------------- |
| **Size**           | 3-4px circular (core)                                    |
| **Anti-aliasing**  | Subtle box-shadow blur to smooth edges                   |
| **Position**       | Bottom-right of compartment, 1px offset toward center    |
| **Shape**          | Circle                                                   |
| **Pointer events** | `none` (clicks/hovers pass through to compartment below) |

### 3D Effect (Gradient)

The indicator has a subtle depth effect simulating light from above:

- **Top half**: Base color + 20-30% brightness
- **Bottom half**: Base color − 20-30% brightness

This creates a gentle "bubble" appearance.

### Anti-Aliasing

At 3-4px, raw pixels can look jagged. Use a subtle outer glow/blur:

```css
box-shadow: 0 0 1px rgba(base-color, 0.5);
```

This softens the edges and makes the dot feel smooth and polished.

### Color States

| State      | Base Color                         | When                                      |
| ---------- | ---------------------------------- | ----------------------------------------- |
| ✅ OK      | Green (`hsl(142, 71%, 45%)`)       | Status is current/healthy                 |
| ⚠️ Warning | Yellow/Amber (`hsl(45, 90%, 50%)`) | Status needs attention (e.g., stale lock) |
| ❌ Error   | Red (`hsl(0, 70%, 50%)`)           | Status mismatch or failure                |

All states use the same 3D gradient treatment.

---

## Visual Mockup (ASCII)

```
Compartment layout (~30×25px each):
┌──────────┬──────────┬──────────┐
│   Git    │   Lock   │  System  │
│          │          │          │
│        · │        · │        · │  ← 3-4px dots, 1px from edges
└──────────┴──────────┴──────────┘

Zoomed indicator (3×3 core + anti-alias blur):
    ░░░
   ░▓▓▓░    ← brighter top half
   ░███░    ← darker bottom half
    ░░░     ← anti-alias blur (box-shadow)
```

---

## Technical Implementation

### CSS

```css
.compartment-indicator {
  position: absolute;
  bottom: 1px;
  right: 1px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  pointer-events: none;
}

/* OK state (green) */
.compartment-indicator--ok {
  background: linear-gradient(
    to bottom,
    hsl(142, 71%, 55%),
    /* brighter top */ hsl(142, 71%, 35%) /* darker bottom */
  );
  box-shadow: 0 0 1px hsla(142, 71%, 45%, 0.5);
}

/* Warning state (yellow) */
.compartment-indicator--warning {
  background: linear-gradient(to bottom, hsl(45, 90%, 60%), hsl(45, 90%, 40%));
  box-shadow: 0 0 1px hsla(45, 90%, 50%, 0.5);
}

/* Error state (red) */
.compartment-indicator--error {
  background: linear-gradient(to bottom, hsl(0, 70%, 60%), hsl(0, 70%, 40%));
  box-shadow: 0 0 1px hsla(0, 70%, 50%, 0.5);
}
```

---

## Acceptance Criteria

- [ ] **AC-1**: Each compartment shows a 3-4px circular indicator in bottom-right (1px from edges)
- [ ] **AC-2**: OK state shows green dot with 3D gradient (brighter top, darker bottom)
- [ ] **AC-3**: Warning state shows yellow/amber dot with same treatment
- [ ] **AC-4**: Error state shows red dot with same treatment
- [ ] **AC-5**: Anti-aliasing blur makes dots look smooth (not jagged)
- [ ] **AC-6**: Indicator does not block compartment tooltip (pointer-events: none)
- [ ] **AC-7**: Looks polished at 100% zoom and on retina displays

---

## Out of Scope

- Icons inside the indicator (checkmarks, !, X) — color alone is sufficient
- Animated state transitions
- Indicator size customization

---

## Future Enhancement (Optional)

If we later want icons inside the dots, consider increasing size to 6-8px and adding:

- ✓ for OK
- ! for Warning
- ✕ for Error

But for v1, plain colored dots are cleaner and more practical at this scale.

---

## Definition of Done

1. Indicator renders correctly in all three compartments
2. All three states (OK/Warning/Error) implemented with correct colors
3. 3D gradient effect visible and subtle
4. Anti-aliasing makes dots smooth
5. Tooltip interaction unaffected
6. Tested on Chrome, Firefox, Safari
