# P2600: Context Bar Polish & Remaining P2500 Tasks

**Priority**: Low  
**Complexity**: Low  
**Depends On**: P2500 (Streamline Commands) - completed  
**Status**: Backlog  
**Created**: 2025-12-20

---

## Problem Statement

The context bar redesign (P2500) is functionally complete, but there are polish items and deferred tasks that would improve the UX:

1. **Footer jumping** - When a 3rd row appears (PR + Hover + Selection), the footer moves. The space for 2 rows is reserved but not 3.
2. **Visual boundary** - The context bar area is invisible when empty, making it unclear where content will appear.
3. **Deploy progress** - `handleFlakeUpdateJob()` still just logs to console instead of showing progress.
4. **Documentation** - RUNBOOK not updated with command reference.

---

## Tasks

### Visual Polish

- [ ] Add subtle hairline border (30% opacity) around context bar area so it's always visible
- [ ] Reserve space for 3 rows instead of 2 to prevent footer jumping
- [ ] Consider: show faint placeholder text when context bar is empty (e.g., "Hover over compartments or select hosts")

### Deploy Progress Display

- [ ] Replace `handleFlakeUpdateJob()` console.log with toast/banner
- [ ] Show states: "Merging PR #42...", "Deploying to 3 hosts...", "Complete"
- [ ] Consider: show progress in context bar PR row during merge

### Compartment UX (deferred from P2500 Phase 2)

- [ ] Green compartments: clicking shows "No action needed, status refreshed" toast
- [ ] Lock yellow: show clearer message about GitHub PR requirement in context bar

### Documentation

- [ ] Update RUNBOOK with command reference table
- [ ] Document context bar behavior

---

## Technical Notes

### Context Bar Height Issue

Current CSS reserves 120px (`min-height: 120px`), which fits ~2-3 rows comfortably. However:

- Each row is ~36px (content) + 8px (gap) = ~44px
- 3 rows = ~132px → slightly taller than reserved space

**Fix options:**

1. Increase `min-height` to 140px (simple but wastes space when empty)
2. Keep 120px but add subtle border so footer jump is less jarring
3. Dynamic height with CSS transition for smooth animation

### Subtle Border CSS

```css
.context-bar {
  /* Add always-visible boundary */
  border: 1px solid rgba(var(--fg-rgb), 0.1);
  border-radius: 8px;
}

/* When empty, just show border outline */
.context-bar-empty {
  opacity: 1; /* Keep visible */
  border-color: rgba(var(--fg-rgb), 0.05);
}

.context-bar-empty * {
  opacity: 0; /* Hide content but keep container */
}
```

---

## Success Criteria

1. Footer never jumps when context bar content changes
2. Context bar area is subtly visible even when empty
3. Deploy progress shown to user (not just console)
4. RUNBOOK documents all available commands

---

## Related Items

- P2500: Streamline and Unify Commands (completed, parent task)
- P1000: Update UX Overhaul (completed)
