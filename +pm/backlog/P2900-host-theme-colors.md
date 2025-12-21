# P2900 - Host Theme Colors

**Created**: 2025-12-21  
**Priority**: P2900 (Medium - UI Polish)  
**Status**: Backlog  
**Effort**: Medium (2-4 hours)

---

## Summary

Each host should display its unique theme color (from starship config) as a subtle row gradient in the dashboard.

**Current approach**: Row background gradient from left to 50%, using host's `ThemeColor`.

---

## Problem: Colors Not Working

**Root Cause**: The NixOS/macOS modules do NOT pass the starship color to the agent.

### Current Data Flow (BROKEN)

```
starship.toml → ??? → Agent reads NIXFLEET_THEME_COLOR (empty!) → Fallback by OS type
```

The agent reads `NIXFLEET_THEME_COLOR` from environment (`config.go:132`), but the NixOS/macOS modules don't set this variable from starship.

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

This means ALL NixOS hosts show blue, ALL macOS hosts show purple, regardless of their starship config.

---

## Required Fix: NixOS/macOS Modules

### NixOS Module

The module needs to read starship's palette and export it:

```nix
# In nixfleet agent module
environment.variables.NIXFLEET_THEME_COLOR =
  config.programs.starship.settings.palette.primary or "#7aa2f7";
```

### macOS Module (launchd)

Similar - read from starship.toml and set in the launchd plist.

---

## Dashboard Implementation (DONE)

### Row Gradient

Each `<tr>` gets an inline style with a gradient using the host's color:

```
0% (2%) → 25% (10%) → 50% (2%) → 100% (0%)
```

Implemented in `hostRowStyle()` function.

### Text/Icons

All text and icons use the bright default color (`--fg: #e8ecf5`), NOT the host color.

---

## Acceptance Criteria

- [ ] **NixOS module** sets `NIXFLEET_THEME_COLOR` from starship palette
- [ ] **macOS module** sets `NIXFLEET_THEME_COLOR` from starship palette
- [ ] Each host row shows its unique gradient color
- [ ] Hosts without starship config fall back to OS-based colors
- [ ] Dashboard correctly displays per-host gradients

---

## Related

- **P2700** - Table Column Redesign
- **nixcfg** - NixOS module updates needed
- Agent config: `v2/internal/config/config.go:132`
- Hub fallback: `v2/internal/dashboard/hub.go:573-579`
