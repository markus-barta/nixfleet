# P4370 - UI: Complete Table Columns

**Priority**: High  
**Status**: Done  
**Effort**: Large  
**Updated**: 2025-12-16  
**References**: `+pm/legacy/v1.0/dashboard.html`

## Problem

v2 table is missing several columns from v1:

- Location (Loc) - cloud/home/work
- Device Type - server/desktop/laptop/gaming
- Metrics - CPU/RAM with visual indicators
- Config hash - with outdated indicator
- Tests - progress during test, results after

Also missing:

- Proper column widths and alignment
- Visual dimming of offline hosts
- Heartbeat ripple animation

## Solution

### Full Column Set

| Column    | Width | Content                         |
| --------- | ----- | ------------------------------- |
| Host      | 100px | Status dot/ripple + hostname    |
| OS        | 60px  | Icon + version (5 chars)        |
| Loc       | 40px  | Location icon                   |
| Type      | 40px  | Device type icon                |
| Last Seen | 70px  | Relative time, full on hover    |
| Metrics   | 60px  | CPU + RAM with icons            |
| Config    | 60px  | Hash badge + ↓/✓ indicator      |
| Tests     | 50px  | Progress or results             |
| Status    | 100px | Papertrail (expandable history) |
| Actions   | 160px | Buttons + dropdown              |

- "Last Seen" shows time since last heartbeat in relative terms (e.g., "3h ago", "12m ago").
- If offline ≥ 1 day, show days (e.g., "2") in a calendar SVG icon (normal color) beside the relative time.
- Support up to 99 days; for >99, show "99+" in a larger, red calendar icon.
- Omit the calendar icon if offline < 1 day.

### Status Indicator (Host cell)

Online hosts get animated ripple:

```html
<span class="status-ripple">
  <span class="hb-wave"></span>
  <span class="hb-wave"></span>
  <span class="hb-wave"></span>
  <span class="hb-core"></span>
</span>
```

Offline hosts get static dot:

```html
<span class="status-dot offline"></span>
```

### Metrics Cell

```html
<span class="metric cpu {{ 'high' if cpu >= 80 }}">
  <svg class="metric-icon"><use href="#icon-cpu" /></svg>
  <span class="metric-val">{{ cpu }}</span>%
</span>
<span class="metric ram {{ 'high' if ram >= 80 }}">
  <svg class="metric-icon"><use href="#icon-ram" /></svg>
  <span class="metric-val">{{ ram }}</span>%
</span>
```

### Config Cell

```html
<code class="hash-badge {{ 'outdated' if outdated else 'current' }}">
  {{ hash[:7] }}
</code>
<span class="update-indicator">{{ '↓' if outdated else '✓' }}</span>
```

### Tests Cell

During test:

```html
<span class="test-progress">3/10</span>
```

After test:

```html
<span class="test-result {{ 'pass' if passed else 'fail' }}">8/10</span>
```

### Offline Host Overlay

```css
tr[data-online="false"] td::before {
  content: "";
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  pointer-events: none;
  z-index: 10;
}
```

### Requirements

- [x] Add Location column with icons (home/work/cloud)
- [x] Add Device Type column with icons (server/desktop/laptop/gaming)
- [x] Add Metrics column with CPU/RAM
- [x] Add Update Status column with three compartments (see P5000)
- [x] Add Tests column with progress/results
- [x] Implement heartbeat ripple animation
- [x] Add 50% overlay for offline hosts
- [x] Set proper column widths
- [x] Add hover titles for full info (metrics)
- [x] Add OS type icons (NixOS/macOS) to Type column

### Completion Notes (2025-12-16)

All table columns implemented. Config hash replaced with three-compartment Update Status indicator (P5000). Git compartment working via GitHub Pages version comparison. Lock and System compartments show placeholder status pending agent-side implementation.

### Implementation Notes (2025-12-15)

Full-stack implementation completed:

1. **Nix module** (`modules/shared.nix`): Added `location` and `deviceType` options
2. **Agent config** (`config.go`): Parse env vars with defaults
3. **Protocol** (`messages.go`): Added to `RegisterPayload`
4. **Database** (`database.go`): Added columns with migrations
5. **UI** (`dashboard.templ`): Added Loc, Dev, Tests columns with icons
6. **CSS** (`base.templ`): Styled icons and test progress/results

Hosts need to be configured with `location` and `deviceType` in their nixfleet config.
Default values: `location = "home"`, `deviceType = "desktop"`.

## Related

- P4350 (Icons) - Needs icon system
- P4365 (Theme Colors) - Icons inherit color
- T02 (Heartbeat) - Metrics data source
