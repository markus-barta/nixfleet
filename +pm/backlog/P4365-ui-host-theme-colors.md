# P4365 - UI: Per-Host Theme Colors

**Priority**: Medium  
**Status**: Pending  
**Effort**: Medium  
**References**: `+pm/legacy/v1.0/dashboard.html`, `nixcfg/modules/uzumaki/theme/theme-palettes.nix`

## Problem

v2 shows all hosts in the same color. v1 had per-host theme colors that:

- Made each host visually distinct
- Matched the host's actual terminal/editor theme color
- Applied to hostname, OS icon, location icon, device icon

## Current v1 Implementation

```css
/* Generated per-host */
.host-color-hsb0 {
  color: #f7768e;
} /* Tokyo Night Storm - Red */
.host-color-gpc0 {
  color: #9ece6a;
} /* Tokyo Night - Green */
.host-color-imac0 {
  color: #bb9af7;
} /* Tokyo Night - Purple */
```

```html
<span class="hostname host-color-{{ host.id }}">{{ host.hostname }}</span>
<td class="os-cell host-color-{{ host.id }}">...</td>
<td class="loc-cell host-color-{{ host.id }}">...</td>
<td class="type-cell host-color-{{ host.id }}">...</td>
```

## Solution

### 1. Store theme_color in database

Already have `theme_color` column in hosts table.

### 2. Generate CSS classes dynamically

In Templ template, generate inline style or CSS classes:

```go
templ HostRow(host Host) {
  <tr style={ fmt.Sprintf("--host-color: %s", host.ThemeColor) }>
    <td class="host-cell">
      <span class="hostname" style="color: var(--host-color)">{ host.Hostname }</span>
    </td>
    ...
  </tr>
}
```

### 3. Default colors from palette

If no theme_color set, use defaults:

- NixOS hosts: `#7aa2f7` (blue)
- macOS hosts: `#bb9af7` (purple)

### Requirements

- [ ] Ensure theme_color stored on agent registration
- [ ] Pass theme_color to template
- [ ] Apply color to hostname
- [ ] Apply color to OS icon
- [ ] Apply color to location icon
- [ ] Apply color to device type icon
- [ ] Default colors if not set

## Related

- P4350 (Icons) - Icons need to inherit color
- nixcfg theme-palettes.nix - Source of truth for host colors
