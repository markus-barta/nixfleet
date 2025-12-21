# P2900 - Host Theme Colors in Dashboard

**Created**: 2025-12-21  
**Completed**: 2025-12-21  
**Priority**: P2900 (Medium - UI Polish)  
**Status**: ✅ Done (Dashboard Ready)

---

## Summary

Each host displays its unique theme color (from starship prompt) as a subtle row gradient in the dashboard.

---

## What Was Done

### Dashboard Implementation ✅ COMPLETE

Row gradients are implemented in `dashboard.templ`:

```
0% (2%) → 25% (10%) → 50% (2%) → 100% (0%)
```

The `hostRowStyle()` function generates inline CSS using `ThemeColor` from heartbeat.

### Bug Fix (2025-12-21)

Fixed missing `ThemeColor` assignment in `getHostByID()` (`handlers.go:688-690`).
The theme color was being read from DB but not assigned to the host struct.

### Fallback Colors (hub.go:573-579)

```go
if themeColor == "" {
    if payload.HostType == "macos" {
        themeColor = "#bb9af7" // Tokyo Night purple
    } else {
        themeColor = "#7aa2f7" // Tokyo Night blue
    }
}
```

---

## What's Still Needed (In nixcfg)

**P7200 (nixcfg)** must wire `theme-palettes.nix` → `nixfleet-agent.themeColor`:

```nix
# In theme-hm.nix:
services.nixfleet-agent.themeColor = palette.gradient.primary;
```

Until P7200 is complete:

- ALL NixOS hosts show blue (fallback)
- ALL macOS hosts show purple (fallback)

---

## Acceptance Criteria

- [x] **Dashboard displays row gradients** using ThemeColor
- [x] **Bug fix**: `getHostByID` now assigns ThemeColor correctly
- [x] **Fallback works** for hosts without theme color set
- [ ] **P7200 Phase 1 complete** (nixcfg wiring) — blocks per-host colors

---

## Technical Reference

| Component         | File                                    | Notes                    |
| ----------------- | --------------------------------------- | ------------------------ |
| Row gradient      | `v2/internal/templates/dashboard.templ` | `hostRowStyle()`         |
| Color fallback    | `v2/internal/dashboard/hub.go:573-579`  | OS-based default         |
| Agent reads color | `v2/internal/config/config.go:132`      | `NIXFLEET_THEME_COLOR`   |
| Module option     | `modules/shared.nix:99-108`             | `themeColor` option      |
| Bug fix           | `v2/internal/dashboard/handlers.go:688` | `getHostByID` ThemeColor |

---

## Why This Moved to Done

The NixFleet dashboard side is fully ready. The remaining work (wiring theme colors from nixcfg to the agent) is tracked in P7200 (nixcfg), which another agent is working on.
