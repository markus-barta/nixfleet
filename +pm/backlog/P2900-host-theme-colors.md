# P2900 - Host Theme Colors

**Created**: 2025-12-21  
**Priority**: P2900 (Medium - UI Polish)  
**Status**: Backlog  
**Effort**: Small (1-2 hours)

---

## Summary

Apply each host's primary theme color (from starship config) consistently across the dashboard UI. The color is already passed from the agent to the dashboard and stored in `host.ThemeColor`.

---

## Requirements

### 1. Hostname Text

The hostname text in the HOSTS column should use the host's theme color.

**Current**: All hostnames use the same color  
**Target**: Each hostname uses its `ThemeColor` (e.g., `#7aa2f7` for NixOS blue, `#bb9af7` for macOS purple)

### 2. TYPE Column Icons

All icons in the composite TYPE column should use the host's theme color:

- **Location icon** (home/cloud/office) - main icon
- **Device icon** (server/desktop/laptop) - top-right badge
- **OS icon** (NixOS/Apple) - bottom-right badge

**Status**: ✅ Partially implemented in P2700 polish commit (`e9d0449`)

### 3. Fallback Behavior

If `ThemeColor` is empty or not set, fall back to the default color (`var(--fg-dark)` or similar).

---

## Data Flow

```
starship.toml → Agent reads palette.primary → Agent sends to Dashboard → Stored in host.ThemeColor
```

The `ThemeColor` field already exists in the `Host` struct and is populated by agents.

---

## Implementation Notes

### Hostname (not yet done)

The hostname is rendered in `HostRow` template. Currently uses:

```html
<span style="{" hostColorStyle(host) }>{ host.Hostname }</span>
```

This should already work if `hostColorStyle()` returns `color: #xxxxxx`. Verify it's being applied.

### TYPE Column Icons (done)

Added in polish commit:

```html
<span class="type-loc" style="{" hostColorStyle(host) }>
  <span class="type-dev" style="{" hostColorStyle(host) }>
    <span class="type-os" style="{" hostColorStyle(host) }></span></span
></span>
```

### CSS Inheritance

The `style="color: #xxx"` on the span should cascade to child SVG icons via `fill: currentColor` or `color: currentColor`.

---

## Acceptance Criteria

- [ ] Hostname text uses host's theme color
- [ ] Location icon uses host's theme color
- [ ] Device badge icon uses host's theme color
- [ ] OS badge icon uses host's theme color
- [ ] Hosts without a theme color fall back gracefully
- [ ] Colors match starship config (NixOS = blue, macOS = purple, etc.)

---

## Related

- **P2700** - Table Column Redesign (parent task)
- **Starship integration** - Theme colors come from starship palette
