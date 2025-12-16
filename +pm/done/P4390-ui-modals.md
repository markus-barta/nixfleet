# P4390 - UI: Modals (Add Host, Remove Host)

**Priority**: Medium  
**Status**: Done  
**Effort**: Medium  
**Updated**: 2025-12-16  
**References**: `+pm/legacy/v1.0/dashboard.html` (lines 972-1103, 1705-1776)

## Problem

v2 has no modals. v1 had:

- **Remove Host confirmation** - Prevents accidental deletion
- **Add Host form** - Manually add hosts before agent connects

## Solution

### Modal Component Base

```html
<div class="modal-overlay" id="removeHostModal">
  <div class="modal">
    <div class="modal-title">Remove Host</div>
    <div class="modal-body">
      Are you sure you want to remove <code id="removeHostName"></code>?
    </div>
    <div class="modal-actions">
      <button class="modal-btn modal-btn-cancel" data-action="close-modal">
        Cancel
      </button>
      <button class="modal-btn modal-btn-danger" data-action="remove-host">
        Remove
      </button>
    </div>
  </div>
</div>
```

### Add Host Modal

```html
<div class="modal-overlay" id="addHostModal">
  <div class="modal modal-wide">
    <div class="modal-title">Add Host</div>
    <div class="modal-body">
      <form id="addHostForm">
        <div class="form-group">
          <label>Hostname *</label>
          <input
            type="text"
            name="hostname"
            required
            pattern="[a-zA-Z][a-zA-Z0-9\-]{0,62}"
          />
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>OS Type</label>
            <select name="host_type">
              <option value="nixos">NixOS</option>
              <option value="macos">macOS</option>
            </select>
          </div>
          <div class="form-group">
            <label>Location</label>
            <select name="location">
              <option value="home">Home</option>
              <option value="work">Work</option>
              <option value="cloud">Cloud</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>Device Type</label>
            <select name="device_type">
              <option value="server">Server</option>
              <option value="desktop">Desktop</option>
              <option value="laptop">Laptop</option>
              <option value="gaming">Gaming</option>
            </select>
          </div>
          <div class="form-group">
            <label>Theme Color</label>
            <input type="color" name="theme_color" value="#769ff0" />
          </div>
        </div>
      </form>
    </div>
    <div class="modal-actions">
      <button class="modal-btn modal-btn-cancel" data-action="close-add-modal">
        Cancel
      </button>
      <button class="modal-btn modal-btn-primary" data-action="submit-add-host">
        Add Host
      </button>
    </div>
  </div>
</div>
```

### CSS

```css
.modal-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  z-index: 1000;
  align-items: center;
  justify-content: center;
}

.modal-overlay.open {
  display: flex;
}

.modal {
  background: var(--bg-dark);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}

.modal-btn-danger {
  background: rgba(247, 118, 142, 0.15);
  color: var(--red);
  border-color: rgba(247, 118, 142, 0.3);
}
```

### API Endpoints Required

- `POST /api/hosts` - Create host manually
- `DELETE /api/hosts/{id}` - Remove host

### Requirements

- [x] Create modal overlay component
- [x] Implement Remove Host modal with confirmation
- [x] Implement Add Host modal with form
- [x] Close on Escape key
- [x] Close on overlay click
- [x] Create API endpoints
- [x] Validate hostname pattern
- [x] Reload page after add/remove

### Completion Notes (2025-12-16)

Both modals (Remove Host and Add Host) implemented with proper confirmation flows, form validation, and API integration. Validated via browser testing on fleet.barta.cm.

## Related

- P4380 (Dropdown) - Remove Host triggered from dropdown
- P4355 (Header) - Add Host triggered from Actions menu _(historical reference)_
