# P1015 - Selection Bar Component

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1015 (Critical)  
**Status**: ✅ DONE (superseded by Context Bar)  
**Completed**: 2025-12-20  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 3 hours  
**Depends on**: P1030 (Row Selection)

> **Implementation Note**: This spec was superseded by the **unified Context Bar**. Instead of a separate Selection Bar below the header, selection actions were merged with hover preview functionality into a single Context Bar below the table. All bulk action buttons (Pull All, Switch All, Test All, Do All, Clear) are implemented in the Context Bar.

---

## Overview

The Selection Bar appears below the header when hosts are selected, providing bulk action controls. It consumes selection state from `Alpine.store('selection')` defined in P1030.

---

## Requirements

### Position & Layout

| Property   | Value                           |
| ---------- | ------------------------------- |
| Position   | Below header, above table       |
| Width      | 100% (matches header)           |
| Height     | Auto (single row)               |
| Visibility | Only when `selection.count > 0` |
| Z-index    | 40 (below header, above table)  |

### Responsive Behavior

| Breakpoint | Layout                                     |
| ---------- | ------------------------------------------ |
| > 768px    | Horizontal: info left, buttons right       |
| ≤ 768px    | Stacked: info top, buttons below (wrapped) |

---

## Content

### Left Side (Info)

```
☑ 3 hosts selected (2 online)
```

| Element | Condition    | Display                           |
| ------- | ------------ | --------------------------------- |
| Icon    | Always       | `#icon-check-square`              |
| Count   | All online   | "{n} hosts selected"              |
| Count   | Some offline | "{n} hosts selected ({m} online)" |

### Right Side (Actions)

| Button     | Icon             | Enabled When    | Click Action                        |
| ---------- | ---------------- | --------------- | ----------------------------------- |
| Pull All   | `#icon-download` | onlineCount > 0 | `bulkCommand('pull')`               |
| Switch All | `#icon-refresh`  | onlineCount > 0 | `bulkCommand('switch')` (via P1040) |
| Test All   | `#icon-flask`    | onlineCount > 0 | `bulkCommand('test')`               |
| Clear (✗)  | `#icon-x`        | Always          | `clearSelection()`                  |

---

## Implementation

### Template (`dashboard.templ`)

Add after header, before table:

```go
templ SelectionBar() {
    <div
        id="selection-bar"
        class="selection-bar"
        x-data="selectionBar()"
        x-show="$store.selection.selected.length > 0"
        x-transition:enter="selection-bar-enter"
        x-transition:enter-start="selection-bar-enter-start"
        x-transition:enter-end="selection-bar-enter-end"
        x-transition:leave="selection-bar-leave"
        x-transition:leave-start="selection-bar-leave-start"
        x-transition:leave-end="selection-bar-leave-end"
        x-cloak
    >
        <div class="selection-info">
            <svg class="icon"><use href="#icon-check-square"></use></svg>
            <span x-text="selectionText"></span>
        </div>
        <div class="selection-actions">
            <button
                class="btn btn-selection"
                :disabled="onlineCount === 0"
                @click="bulkCommand('pull')"
            >
                <svg class="icon"><use href="#icon-download"></use></svg>
                <span class="btn-label">Pull All</span>
            </button>
            <button
                class="btn btn-selection"
                :disabled="onlineCount === 0"
                @click="bulkCommand('switch')"
            >
                <svg class="icon"><use href="#icon-refresh"></use></svg>
                <span class="btn-label">Switch All</span>
            </button>
            <button
                class="btn btn-selection"
                :disabled="onlineCount === 0"
                @click="bulkCommand('test')"
            >
                <svg class="icon"><use href="#icon-flask"></use></svg>
                <span class="btn-label">Test All</span>
            </button>
            <button
                class="btn btn-clear"
                @click="clearSelection()"
                title="Clear selection"
            >
                <svg class="icon"><use href="#icon-x"></use></svg>
            </button>
        </div>
    </div>
}
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   SELECTION BAR (P1015)
   ═══════════════════════════════════════════════════════════════════════════ */

.selection-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.625rem 1rem;
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border);
  gap: 1rem;
  position: sticky;
  top: 60px; /* Below header */
  z-index: 40;
}

@media (max-width: 768px) {
  .selection-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
  }
}

/* Animation */
.selection-bar-enter {
  transition: all 200ms ease-out;
}
.selection-bar-enter-start {
  opacity: 0;
  transform: translateY(-100%);
}
.selection-bar-enter-end {
  opacity: 1;
  transform: translateY(0);
}
.selection-bar-leave {
  transition: all 150ms ease-in;
}
.selection-bar-leave-start {
  opacity: 1;
  transform: translateY(0);
}
.selection-bar-leave-end {
  opacity: 0;
  transform: translateY(-100%);
}

/* Info Section */
.selection-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
  font-size: 0.875rem;
}

.selection-info .icon {
  width: 18px;
  height: 18px;
  color: var(--blue);
}

/* Actions Section */
.selection-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .selection-actions {
    justify-content: center;
  }
}

/* Buttons */
.btn-selection {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem 0.75rem;
  font-size: 0.8125rem;
  font-weight: 500;
  background: var(--bg-highlight);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--fg);
  cursor: pointer;
  transition:
    background 150ms ease,
    border-color 150ms ease;
}

.btn-selection:hover:not(:disabled) {
  background: var(--bg-highlight-bright);
  border-color: var(--border-bright);
}

.btn-selection:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-selection .icon {
  width: 14px;
  height: 14px;
}

@media (max-width: 480px) {
  .btn-selection .btn-label {
    display: none;
  }
  .btn-selection {
    padding: 0.5rem;
  }
  .btn-selection .icon {
    width: 18px;
    height: 18px;
  }
}

.btn-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.375rem;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px;
  cursor: pointer;
  color: var(--fg-muted);
  transition:
    color 150ms ease,
    background 150ms ease;
}

.btn-clear:hover {
  color: var(--red);
  background: var(--bg-highlight);
}

.btn-clear .icon {
  width: 18px;
  height: 18px;
}
```

### JavaScript (Alpine.js Component)

```js
// ═══════════════════════════════════════════════════════════════════════════
// SELECTION BAR COMPONENT (P1015)
// ═══════════════════════════════════════════════════════════════════════════

Alpine.data("selectionBar", () => ({
  /**
   * Get count of selected hosts
   */
  get selectedCount() {
    return Alpine.store("selection").selected.length;
  },

  /**
   * Get count of selected hosts that are online
   */
  get onlineCount() {
    const selected = Alpine.store("selection").selected;
    return selected.filter((id) => {
      const host = hostStore.get(id);
      return host && host.online;
    }).length;
  },

  /**
   * Get display text for selection count
   */
  get selectionText() {
    const total = this.selectedCount;
    const online = this.onlineCount;

    if (total === 0) return "";

    const hostWord = total === 1 ? "host" : "hosts";

    if (online === total) {
      return `${total} ${hostWord} selected`;
    }

    return `${total} ${hostWord} selected (${online} online)`;
  },

  /**
   * Execute bulk command on all selected online hosts
   */
  bulkCommand(command) {
    const selected = Alpine.store("selection").selected;
    const onlineHostIds = selected.filter((id) => {
      const host = hostStore.get(id);
      return host && host.online;
    });

    if (onlineHostIds.length === 0) return;

    // Use dependency check for switch (delegates to P1040)
    if (command === "switch") {
      checkDependenciesAndExecute(onlineHostIds, "switch");
    } else {
      // For pull/test, execute directly
      onlineHostIds.forEach((hostId) => {
        sendCommand(hostId, command);
      });
    }
  },

  /**
   * Clear all selections
   */
  clearSelection() {
    Alpine.store("selection").selectNone();
  },
}));
```

---

## Edge Cases

### All Selected Hosts Go Offline

| Scenario                    | Behavior                             |
| --------------------------- | ------------------------------------ |
| 3 selected, all go offline  | Buttons disabled, shows "(0 online)" |
| User clicks disabled button | Nothing happens                      |

### Selection During Bulk Action

| Scenario                                   | Behavior                                         |
| ------------------------------------------ | ------------------------------------------------ |
| Bulk switch running, user selects more     | Selection changes, but running commands continue |
| Bulk switch running, user clears selection | Selection clears, running commands continue      |

### Maximum Selection

No hard limit, but consider:

- With 50+ hosts, button text might need "Switch All (50)" format
- Currently shows count in info text, not on buttons

---

## Testing

### Manual Test Cases

| #   | Scenario          | Steps                           | Expected                            |
| --- | ----------------- | ------------------------------- | ----------------------------------- |
| 1   | Appears on select | Click one row checkbox          | Selection Bar slides in             |
| 2   | Count updates     | Select 3 hosts                  | Shows "3 hosts selected"            |
| 3   | Offline count     | Select 2 (1 offline)            | Shows "2 hosts selected (1 online)" |
| 4   | Pull All          | Select 2 online, click Pull All | Both hosts start pull               |
| 5   | Disabled buttons  | Select 1 offline host           | All action buttons disabled         |
| 6   | Clear             | Click ✗                         | Selection Bar slides out            |
| 7   | Switch All        | Select hosts with Git outdated  | P1040 dialog appears                |
| 8   | Responsive        | Resize to mobile                | Buttons stack, labels hide          |

---

## Acceptance Criteria

- [ ] Hidden when no hosts selected
- [ ] Appears with slide animation when 1+ hosts selected
- [ ] Shows correct selected count
- [ ] Shows "(N online)" when some offline
- [ ] Pull/Switch/Test buttons enabled when onlineCount > 0
- [ ] Buttons disabled when onlineCount = 0
- [ ] Clear button always enabled
- [ ] Clear button deselects all, bar hides
- [ ] Switch All triggers dependency check (P1040)
- [ ] Responsive: buttons stack on mobile
- [ ] Responsive: button labels hide on narrow screens

---

## Related

- **P1030**: Row Selection (provides selection state)
- **P1040**: Dependency Dialog (called by Switch All)
- **P1010**: Action Bar (separate single-host UI)
