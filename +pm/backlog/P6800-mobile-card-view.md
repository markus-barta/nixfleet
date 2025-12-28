# P6800 - Mobile Card View

**Created**: 2025-12-19  
**Updated**: 2025-12-28  
**Priority**: P9100 (⚪ Very Low Priority - Future)  
**Status**: Backlog  
**Depends on**: P1000 (Action Bar refactor)

**Note**: Priority lowered - desktop UI works on mobile, optimization not urgent

---

## Problem

The dashboard has a responsive breakpoint that switches to card view on mobile, but:

- Cards are empty/non-functional
- No actions available in card view
- Touch interactions not designed

---

## Current State

```
Desktop (>1024px): Table view with columns
Tablet/Mobile (<1024px): Card view (broken)
```

The card view exists but has no content or functionality.

---

## Solution

Design and implement a touch-optimized card view for mobile devices.

### Card Layout (Concept)

```
┌─────────────────────────────────────┐
│  hsb1                    🟢 Online  │
│  ─────────────────────────────────  │
│  CPU: 12%    RAM: 45%    2.1.0     │
│  ─────────────────────────────────  │
│  ┌────┐ ┌────┐ ┌────┐              │
│  │ 🟢 │ │ 🟢 │ │ 🟡 │  [Update]    │
│  │ Git│ │Lock│ │ Sys│              │
│  └────┘ └────┘ └────┘              │
└─────────────────────────────────────┘
```

### Touch Interactions

- Tap compartment → Shows action sheet (bottom drawer)
- Tap [Update] → Runs pull-switch
- Swipe left → Reveal more actions (Stop, Restart, Test)
- Long press → Multi-select mode

---

## Acceptance Criteria

- [ ] Card view shows host status, metrics, compartments
- [ ] Compartments are tappable with action feedback
- [ ] Action sheet appears on tap (not hover)
- [ ] Multi-select works with long press
- [ ] Smooth animations (iOS-like feel)
- [ ] Works on phones (375px+) and tablets

---

## Design Considerations

- Touch targets ≥ 44px
- No hover states (touch only)
- Bottom sheet for actions (thumb-friendly)
- Pull-to-refresh for status update

---

## Related

- **P1000**: Action Bar refactor (desktop-first)
- **NFR-3**: Responsive design requirements
