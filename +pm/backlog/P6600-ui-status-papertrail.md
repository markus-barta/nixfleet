# P4375 - UI: Status Papertrail

**Priority**: Medium  
**Status**: Pending  
**Effort**: Medium  
**Updated**: 2025-12-15  
**References**: `+pm/legacy/v1.0/dashboard.html` (lines 468-568)

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

- P4300 (Live Logs) - Status updates come from command output
- T03 (Commands) - Command completion triggers status update
