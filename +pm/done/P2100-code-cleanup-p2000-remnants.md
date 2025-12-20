# P2100: Code Cleanup - Polish & Remnants

**Priority**: Low  
**Complexity**: Low  
**Status**: ✅ DONE  
**Created**: 2025-12-20  
**Completed**: 2025-12-20  
**Depends On**: None

---

## Summary

Minor cleanup and polish items remaining after P1000 (Update UX Overhaul) and P2000 (Unified Host State Management) were completed. These are non-critical improvements.

---

## Tasks

### 1. Per-Host Refresh Button ✅

**Status**: DONE

**Decision**: Remove dead CSS, keep the `refreshHost()` function.

- The `.btn-refresh` CSS was never used (no button element rendered)
- The `refreshHost()` JS function is still used as fallback in `handleCompartmentClick()`
- Removed ~30 lines of dead CSS from `base.templ`

---

### 2. Update Integration Tests ✅

**Status**: DONE

Updated all references from `host_update` → `host_heartbeat` in:

- `v2/tests/integration/t05_dashboard_websocket_test.go` (8 occurrences)

---

### 3. Dropdown Arrow Key Navigation (P1060) ✅

**Status**: DONE

Added keyboard navigation to both dropdown menus:

1. **Per-host ActionDropdown** (Alpine.js):
   - Arrow Down/Up: Navigate between items
   - Escape: Close menu and return focus to button
   - Tab: Close menu

2. **Header "More" menu** (vanilla JS):
   - Same keyboard navigation via `handleBulkMenuKeydown()`
   - `focusBulkMenuItem()` function for focus management

---

## Acceptance Criteria

- [x] Decision made on refresh button (add or remove dead CSS/JS)
- [x] Integration tests updated to use `host_heartbeat`
- [x] Dropdown arrow key navigation added
- [x] No dead code remaining

---

## Notes

- `handleFlakeUpdateJob()` moved to P2500 (context bar deploy progress display)
