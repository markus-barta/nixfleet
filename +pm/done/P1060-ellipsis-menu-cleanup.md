# P1060 - Ellipsis Menu Cleanup

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1060 (Medium)  
**Status**: ✅ DONE  
**Completed**: 2025-12-20  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 2 hours  
**Depends on**: None

> **Completion Note**: Menu reorganized with Test at top, all SVG icons (no emojis), grouped sections with dividers. Added Copy Hostname, SSH Command, Download Logs, Remove Host. Dropdown toggle styled with border. Arrow key navigation deferred to P2100.

---

## Overview

Reorganize the ellipsis (more actions) dropdown menu to include Test and improve organization. All icons must use SVG (no emojis per NFR-2).

---

## Current State (Before)

```
⋮
├── Restart Agent
├── ───────────
├── Copy Hostname
└── SSH Command
```

---

## Target State (After)

```
⋮
├── Test                    [icon-flask]
├── ───────────
├── Stop                    [icon-stop] (only when running)
├── Restart Agent           [icon-refresh-cw]
├── ───────────
├── Copy Hostname           [icon-copy]
└── SSH Command             [icon-terminal]
```

---

## Menu Structure

### Groups

| Group     | Items                      | Purpose       |
| --------- | -------------------------- | ------------- |
| Actions   | Test, Stop, Restart        | Host commands |
| Utilities | Copy Hostname, SSH Command | Quick access  |

### Item Details

| Item          | Icon               | Enabled         | Click Action                     |
| ------------- | ------------------ | --------------- | -------------------------------- |
| Test          | `#icon-flask`      | Online only     | `sendCommand(hostId, 'test')`    |
| Stop          | `#icon-stop`       | Command running | `sendCommand(hostId, 'stop')`    |
| Restart Agent | `#icon-refresh-cw` | Online only     | `sendCommand(hostId, 'restart')` |
| Copy Hostname | `#icon-copy`       | Always          | Copy to clipboard, show toast    |
| SSH Command   | `#icon-terminal`   | Always          | Copy `ssh user@host`, show toast |

---

## Implementation

### Template (`dashboard.templ`)

```go
templ ActionDropdown(host Host) {
    <div class="dropdown" x-data="{ open: false }">
        <button
            type="button"
            class="dropdown-toggle"
            @click="open = !open"
            @click.outside="open = false"
            title="More actions"
        >
            <svg class="icon"><use href="#icon-more-vertical"></use></svg>
        </button>

        <div class="dropdown-menu" x-show="open" x-cloak @click="open = false">
            <!-- Actions Group -->
            <button
                type="button"
                class="dropdown-item"
                onclick={ templ.JSOnClick(fmt.Sprintf("sendCommand('%s', 'test')", host.ID)) }
                disabled?={ !host.Online }
            >
                <svg class="icon"><use href="#icon-flask"></use></svg>
                <span>Test</span>
            </button>

            if host.PendingCommand != "" {
                <button
                    type="button"
                    class="dropdown-item"
                    onclick={ templ.JSOnClick(fmt.Sprintf("sendCommand('%s', 'stop')", host.ID)) }
                >
                    <svg class="icon"><use href="#icon-stop"></use></svg>
                    <span>Stop</span>
                </button>
            }

            <button
                type="button"
                class="dropdown-item"
                onclick={ templ.JSOnClick(fmt.Sprintf("sendCommand('%s', 'restart')", host.ID)) }
                disabled?={ !host.Online }
            >
                <svg class="icon"><use href="#icon-refresh-cw"></use></svg>
                <span>Restart Agent</span>
            </button>

            <hr class="dropdown-divider" />

            <!-- Utilities Group -->
            <button
                type="button"
                class="dropdown-item"
                onclick={ templ.JSOnClick(fmt.Sprintf("copyToClipboard('%s', 'Hostname copied')", host.Hostname)) }
            >
                <svg class="icon"><use href="#icon-copy"></use></svg>
                <span>Copy Hostname</span>
            </button>

            <button
                type="button"
                class="dropdown-item"
                onclick={ templ.JSOnClick(fmt.Sprintf("copyToClipboard('%s', 'SSH command copied')", getSSHCommand(host))) }
            >
                <svg class="icon"><use href="#icon-terminal"></use></svg>
                <span>SSH Command</span>
            </button>
        </div>
    </div>
}
```

### Go Helper Function

```go
// getSSHCommand returns the SSH command for a host
func getSSHCommand(host Host) string {
    // Default user based on host type
    user := "mba"
    if host.HostType == "macos" {
        user = "markus"
    }

    // Use hostname, which should resolve via DNS or /etc/hosts
    return fmt.Sprintf("ssh %s@%s", user, host.Hostname)
}
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   ELLIPSIS DROPDOWN (P1060)
   ═══════════════════════════════════════════════════════════════════════════ */

.col-menu {
  width: 48px;
  min-width: 48px;
  text-align: center;
  padding: 0 !important;
}

.dropdown {
  position: relative;
  display: inline-block;
}

.dropdown-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: var(--fg-muted);
  cursor: pointer;
  transition:
    color 150ms ease,
    background 150ms ease;
}

.dropdown-toggle:hover {
  color: var(--fg);
  background: var(--bg-highlight);
}

.dropdown-toggle:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.dropdown-toggle .icon {
  width: 18px;
  height: 18px;
}

.dropdown-menu {
  position: absolute;
  right: 0;
  top: 100%;
  margin-top: 4px;
  min-width: 180px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 100;
  overflow: hidden;
}

/* Ensure menu opens upward if near bottom */
.dropdown-menu.dropdown-up {
  top: auto;
  bottom: 100%;
  margin-top: 0;
  margin-bottom: 4px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 14px;
  font-size: 0.875rem;
  text-align: left;
  background: transparent;
  border: none;
  color: var(--fg);
  cursor: pointer;
  transition: background 100ms ease;
}

.dropdown-item:hover:not(:disabled) {
  background: var(--bg-highlight);
}

.dropdown-item:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dropdown-item:focus-visible {
  outline: none;
  background: var(--bg-highlight);
}

.dropdown-item .icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: var(--fg-muted);
}

.dropdown-item:hover:not(:disabled) .icon {
  color: var(--fg);
}

.dropdown-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--border);
  border: none;
}
```

### JavaScript

```js
// ═══════════════════════════════════════════════════════════════════════════
// CLIPBOARD UTILITY (P1060)
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Copy text to clipboard and show toast
 */
async function copyToClipboard(text, message) {
  try {
    await navigator.clipboard.writeText(text);
    showToast(message, "success");
  } catch (err) {
    console.error("Failed to copy:", err);
    showToast("Failed to copy to clipboard", "error");
  }
}
```

### New SVG Icons

Add to sprite:

```html
<symbol id="icon-flask" viewBox="0 0 24 24">
  <path
    d="M9 3h6m-5 0v6l-4 8a3 3 0 0 0 2.67 4h6.66a3 3 0 0 0 2.67-4l-4-8V3m-4 0h6"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
  ></path>
</symbol>

<symbol id="icon-copy" viewBox="0 0 24 24">
  <rect
    x="9"
    y="9"
    width="13"
    height="13"
    rx="2"
    ry="2"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></rect>
  <path
    d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></path>
</symbol>

<symbol id="icon-terminal" viewBox="0 0 24 24">
  <polyline
    points="4 17 10 11 4 5"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
  ></polyline>
  <line
    x1="12"
    y1="19"
    x2="20"
    y2="19"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
  ></line>
</symbol>

<symbol id="icon-more-vertical" viewBox="0 0 24 24">
  <circle cx="12" cy="12" r="1" fill="currentColor"></circle>
  <circle cx="12" cy="5" r="1" fill="currentColor"></circle>
  <circle cx="12" cy="19" r="1" fill="currentColor"></circle>
</symbol>
```

---

## Keyboard Navigation

| Key        | Action                   |
| ---------- | ------------------------ |
| Tab        | Move focus between items |
| Enter      | Activate focused item    |
| Space      | Activate focused item    |
| Escape     | Close menu               |
| Arrow Down | Focus next item          |
| Arrow Up   | Focus previous item      |

**Implementation**: Alpine.js handles with `@keydown.escape`, `@keydown.arrow-down`, `@keydown.arrow-up` on the menu container.

```js
// Add to dropdown menu
@keydown.arrow-down.prevent="focusNext()"
@keydown.arrow-up.prevent="focusPrev()"

// In Alpine data
focusNext() {
    const items = [...this.$el.querySelectorAll('.dropdown-item:not(:disabled)')];
    const current = document.activeElement;
    const idx = items.indexOf(current);
    const next = items[idx + 1] || items[0];
    next?.focus();
},
focusPrev() {
    const items = [...this.$el.querySelectorAll('.dropdown-item:not(:disabled)')];
    const current = document.activeElement;
    const idx = items.indexOf(current);
    const prev = items[idx - 1] || items[items.length - 1];
    prev?.focus();
}
```

---

## Edge Cases

### Menu Position Near Bottom

If the dropdown would extend past viewport bottom:

```js
// Simple detection in Alpine
init() {
    const rect = this.$el.getBoundingClientRect();
    const menuHeight = 250; // Approximate
    if (rect.bottom + menuHeight > window.innerHeight) {
        this.$refs.menu.classList.add('dropdown-up');
    }
}
```

### Stop Only Shows When Running

Template uses conditional:

```go
if host.PendingCommand != "" {
    // Show Stop button
}
```

This means the menu structure changes dynamically. When command completes, Stop disappears.

---

## Testing

### Manual Test Cases

| #   | Scenario      | Steps                       | Expected                         |
| --- | ------------- | --------------------------- | -------------------------------- |
| 1   | Open menu     | Click ellipsis              | Menu opens                       |
| 2   | Test command  | Click Test                  | Test executes, menu closes       |
| 3   | Test disabled | With offline host           | Test item is disabled            |
| 4   | Stop shows    | Start command, open menu    | Stop item visible                |
| 5   | Stop hides    | Complete command, open menu | Stop item gone                   |
| 6   | Restart       | Click Restart Agent         | Restart executes                 |
| 7   | Copy hostname | Click Copy Hostname         | Toast: "Hostname copied"         |
| 8   | SSH command   | Click SSH Command           | Toast: "SSH command copied"      |
| 9   | Escape        | Menu open, press Escape     | Menu closes                      |
| 10  | Click outside | Menu open, click elsewhere  | Menu closes                      |
| 11  | Arrow keys    | Menu open, press arrows     | Focus moves                      |
| 12  | Focus return  | Close menu                  | Focus returns to ellipsis button |

---

## Acceptance Criteria

- [ ] Test at top of menu
- [ ] All icons are SVG (no emojis)
- [ ] Groups separated by dividers
- [ ] Stop only shows when command running
- [ ] Test disabled when host offline
- [ ] Restart disabled when host offline
- [ ] Copy Hostname copies and shows toast
- [ ] SSH Command copies correct format
- [ ] Escape closes menu
- [ ] Click outside closes menu
- [ ] Arrow keys navigate items
- [ ] Focus returns to trigger on close

---

## Related

- **P1050**: Remove Action Buttons (Test moves here)
- **P1010**: Action Bar (also has Stop during progress)
- **P1020**: Clickable Compartments (handles Pull/Switch)
