# P6600 - UI: Status Papertrail

**Priority**: P6600 (Low)  
**Status**: Merged into P4020  
**Effort**: Medium  
**Created**: 2025-12-15  
**Merged**: 2025-01-XX

> **⚠️ This item has been merged into P4020 - Tabbed Output Panel.**  
> Status history functionality is now part of P4020's host tabs (see FR-9).  
> The inline papertrail concept was replaced with status history summary at the top of each host tab in the output panel.

## Problem

v2 status column is basic. v1 had a "papertrail" showing:

- Scrollable history of status updates
- Expand/collapse per host
- Timestamps for each entry
- Icons for different event types
- Tiny font for compact display

## Solution

### Structure

```html
<td class="status-cell {{ 'has-error' if status == 'error' }}">
  <div class="status-papertrail" data-host="{{ host.id }}">
    <div class="status-entries">
      <div class="status-entry">
        <span class="status-ts">14:23:05</span>
        <span class="status-icon">✓</span>
        <span class="status-msg">Switch complete</span>
      </div>
      <div class="status-entry muted">
        <span class="status-ts">14:22:31</span>
        <span class="status-icon">⧖</span>
        <span class="status-msg">Switching...</span>
      </div>
      <!-- More entries... -->
    </div>
    <button class="status-expand-btn" data-action="toggle-status-expand">
      ▼
    </button>
  </div>
</td>
```

### CSS

```css
.status-cell {
  font-size: 0.45rem; /* Tiny font */
  max-width: 180px;
  padding: 0.15rem 0.3rem !important;
}

.status-entries {
  max-height: 5.5em; /* ~5 lines collapsed */
  overflow-y: auto;
  transition: max-height 0.2s ease;
}

.status-papertrail.expanded .status-entries {
  max-height: 20em; /* Expanded */
}

.status-entry {
  display: flex;
  gap: 0.2rem;
  white-space: nowrap;
  line-height: 1.1;
}

.status-ts {
  color: var(--fg-gutter);
  min-width: 3.5em;
}

.status-cell.has-error .status-entry:first-child {
  color: var(--red);
}
```

### Icons

| Event   | Icon |
| ------- | ---- |
| Success | ✓    |
| Error   | ✗    |
| Pending | ⧖    |
| Testing | ✦    |
| Info    | •    |

### Data Flow

1. Dashboard stores `status_history` array per host
2. WebSocket broadcasts history updates
3. JavaScript renders entries client-side

### Requirements

- [ ] Create status_history table/field
- [ ] Store last N status updates per host
- [ ] Broadcast history via WebSocket
- [ ] Render papertrail in template
- [ ] Implement expand/collapse
- [ ] Add scrollbar styling
- [ ] Color first entry red if error

## Related

- **P4020** - Tabbed Output Panel (status history merged here - see FR-9)
- **P2800** - Command State Machine (provides validation results and command history)
- P4300 (Live Logs) - Status updates come from command output
- T03 (Commands) - Command completion triggers status update

## Merge Notes

This item has been merged into P4020 - Tabbed Output Panel.

The status history papertrail concept has been integrated into P4020's host tabs:

- **Status history** now appears at the top of each host tab (not inline in Status column)
- **Icons and event types** preserved (✓ ✗ ⧖ ✦ •)
- **Backend storage** (`status_history` array) included in P4020 requirements
- **Real-time updates** via WebSocket integrated into P4020's tab system

**Rationale**: The inline papertrail in the Status column was redundant with the output panel tabs. Showing status history at the top of host tabs provides better context while viewing command output, and avoids cluttering the table view.
