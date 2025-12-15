# P4380 - UI: Per-Host Action Dropdown

**Priority**: Medium  
**Status**: Pending  
**Effort**: Medium  
**Updated**: 2025-12-15  
**References**: `+pm/legacy/v1.0/dashboard.html` (lines 886-970, 1650-1692)

## Problem

v2 only has inline action buttons. v1 had a "More" dropdown per host with:

- Unlock actions (manual override for stuck UI)
- Restart Agent
- Download Logs
- Remove Host (dangerous, requires confirmation)
- ... and more

Also missing: Stop button that replaces Test during test execution.

## Solution

### Dropdown Structure

```html
<div class="dropdown">
  <button class="btn btn-more" data-action="toggle-dropdown">
    <svg><use href="#icon-more" /></svg>
  </button>
  <div class="dropdown-menu">
    <button
      class="dropdown-item"
      data-action="unlock-actions"
      data-host-id="{{ host.id }}"
    >
      <svg><use href="#icon-refresh" /></svg>
      Unlock actions
    </button>
    <button
      class="dropdown-item"
      data-action="send-command"
      data-host-id="{{ host.id }}"
      data-command="restart"
    >
      <svg><use href="#icon-refresh" /></svg>
      Restart Agent
    </button>
    <div class="dropdown-divider"></div>
    <button
      class="dropdown-item"
      data-action="download-logs"
      data-host-id="{{ host.id }}"
    >
      <svg><use href="#icon-file" /></svg>
      Download Logs
    </button>
    <div class="dropdown-divider"></div>
    <button
      class="dropdown-item danger"
      data-action="confirm-remove-host"
      data-host-id="{{ host.id }}"
    >
      <svg><use href="#icon-trash" /></svg>
      Remove Host
    </button>
  </div>
</div>
```

### CSS

```css
.dropdown {
  position: relative;
}

.dropdown-menu {
  display: none;
  position: absolute;
  right: 0;
  top: 100%;
  margin-top: 4px;
  background: var(--bg-dark);
  border: 1px solid var(--border);
  border-radius: 6px;
  min-width: 140px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 9999;
}

.dropdown.open .dropdown-menu {
  display: block;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  width: 100%;
}

.dropdown-item:hover {
  background: var(--bg-highlight);
}

.dropdown-item.danger {
  color: var(--red);
}

.dropdown-item.danger:hover {
  background: rgba(247, 118, 142, 0.1);
}
```

### Stop Button (replaces Test during test)

```javascript
if (data.test_running && testBtn && !stopBtn) {
  testBtn.outerHTML = `<button class="btn btn-stop" data-action="send-command" 
    data-host-id="${host.id}" data-command="stop">
    <svg><use href="#icon-stop"/></svg>
    Stop
  </button>`;
}
```

### Requirements

- [ ] Create dropdown component
- [ ] Add "More" button to actions
- [ ] Implement Unlock actions (clears local busy state)
- [ ] Implement Restart Agent command
- [ ] Implement Download Logs
- [ ] Implement Remove Host with confirmation modal
- [ ] Close dropdown on click outside
- [ ] Close dropdown on Escape key
- [ ] Dynamic Stop/Test button swap

## Related

- P4350 (Icons) - Icon system _(historical reference, may be completed)_
- P4390 (Modals) - Confirmation for Remove Host
- P4395 (Stop Command) - Stop implementation _(historical reference)_
- **P5000** (Update Status) - Bulk actions design (Update All, Pull All, etc.)
