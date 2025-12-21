# P2900 - Host Theme Colors in Dashboard

**Created**: 2025-12-21  
**Priority**: P2900 (Medium - UI Polish)  
**Status**: Blocked by P7200 (nixcfg)  
**Effort**: Dashboard-side: Done | nixcfg wiring: 1-2h

---

## Summary

Each host should display its unique theme color (from starship prompt) as a subtle row gradient in the dashboard.

---

## Current State

### Dashboard Implementation ✅ DONE

Row gradients are implemented in `dashboard.templ`:

```
0% (2%) → 25% (10%) → 50% (2%) → 100% (0%)
```

The `hostRowStyle()` function generates inline CSS using `ThemeColor` from heartbeat.

### What's Missing ❌

**Hosts don't report their colors!**

The agent reads `NIXFLEET_THEME_COLOR`, but:

- NixOS module doesn't set it
- Home Manager module doesn't set it
- No wiring from `theme-palettes.nix` to agent config

### Current Fallback (hub.go:573-579)

```go
if themeColor == "" {
    if payload.HostType == "macos" {
        themeColor = "#bb9af7" // Tokyo Night purple
    } else {
        themeColor = "#7aa2f7" // Tokyo Night blue
    }
}
```

Result: ALL NixOS hosts show blue, ALL macOS hosts show purple.

---

## Fix Required (in nixcfg, not nixfleet)

### The Color Data Exists

```
theme-palettes.nix
    │
    │ hostPalette.hsb0 = "yellow"
    │ palettes.yellow.gradient.primary = "#d4c060"
    ▼
Starship prompt uses this color ✅
Zellij frame uses this color ✅
```

### Missing Wire

```nix
# In theme-hm.nix (or similar):
services.nixfleet-agent.themeColor = palette.gradient.primary;
```

This auto-populates the color from the existing palette data.

---

## See Also

**P7200 (nixcfg)** - Host Colors: Single Source of Truth

This is the master task that covers:

1. **Phase 1**: Wire `theme-palettes.nix` → `nixfleet-agent.themeColor`
2. **Phase 2**: Dashboard UI to SET colors → writes to nixcfg

---

## Acceptance Criteria

- [ ] **P7200 Phase 1 complete** (nixcfg wiring)
- [ ] Each host row shows its unique gradient color
- [ ] Colors match the host's starship prompt
- [ ] Fallback still works for hosts without uzumaki

---

## Technical Reference

| Component         | File                                    | Notes                  |
| ----------------- | --------------------------------------- | ---------------------- |
| Row gradient      | `v2/internal/templates/dashboard.templ` | `hostRowStyle()`       |
| Color fallback    | `v2/internal/dashboard/hub.go:573-579`  | OS-based default       |
| Agent reads color | `v2/internal/config/config.go:132`      | `NIXFLEET_THEME_COLOR` |
| Module option     | `modules/shared.nix:99-108`             | `themeColor` option    |

---

## Why This Task Exists

The dashboard side is ready. This task tracks the dependency on nixcfg P7200 for the full feature to work.
