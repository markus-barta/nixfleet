# P1030 - Row Selection & Multi-Select

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1030 (Critical)  
**Status**: ✅ DONE  
**Completed**: 2025-12-20  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 4 hours  
**Depends on**: None (foundation component)

> **Completion Note**: Implemented with checkbox column on the **right side** (per user feedback, differs from original spec). Uses subtle button-style checkboxes matching header toggle. Shift+click range selection, Ctrl/Cmd+A select all, Escape to clear all work. Header shows correct icons per state (none/some/all).

---

## Overview

Add row selection with checkboxes for multi-host operations. This component provides the `Alpine.store('selection')` that P1015 (Selection Bar) consumes.

---

## Requirements

### Checkbox Column

| Property       | Value                           |
| -------------- | ------------------------------- |
| Position       | First column                    |
| Width          | 40px fixed                      |
| Header content | Toggle button (select all/none) |
| Row content    | Checkbox input                  |

### Header Button States

| State         | Icon                 | Tooltip           |
| ------------- | -------------------- | ----------------- |
| None selected | `#icon-square`       | "Select all"      |
| Some selected | `#icon-minus-square` | "Clear selection" |
| All selected  | `#icon-check-square` | "Deselect all"    |

### Selection Triggers

| Action                 | Result                           |
| ---------------------- | -------------------------------- |
| Click checkbox         | Toggle that row                  |
| Click row background   | Toggle that row                  |
| Click text content     | NO toggle (allow text selection) |
| Click compartment      | NO toggle (execute action)       |
| Click ellipsis menu    | NO toggle (open menu)            |
| Shift + Click checkbox | Range select                     |
| Ctrl/Cmd + A           | Select all                       |
| Escape                 | Clear selection                  |

### Visual States

| State            | Checkbox             | Row Background        |
| ---------------- | -------------------- | --------------------- |
| Default          | Hidden (opacity: 0)  | Normal                |
| Row hover        | Visible (opacity: 1) | Slight highlight      |
| Selected         | Visible, checked     | `var(--bg-highlight)` |
| Selected + hover | Visible, checked     | Brighter highlight    |

---

## Implementation

### Alpine Store

Add before component definitions:

```js
// ═══════════════════════════════════════════════════════════════════════════
// SELECTION STORE (P1030)
// ═══════════════════════════════════════════════════════════════════════════

document.addEventListener("alpine:init", () => {
  Alpine.store("selection", {
    // State
    selected: [],
    lastSelected: null, // For shift+click range selection

    /**
     * Toggle selection for a host
     */
    toggle(id) {
      const idx = this.selected.indexOf(id);
      if (idx === -1) {
        this.selected.push(id);
      } else {
        this.selected.splice(idx, 1);
      }
      this.lastSelected = id;
    },

    /**
     * Select a single host (used for shift+click anchor)
     */
    select(id) {
      if (!this.selected.includes(id)) {
        this.selected.push(id);
      }
      this.lastSelected = id;
    },

    /**
     * Deselect a single host
     */
    deselect(id) {
      const idx = this.selected.indexOf(id);
      if (idx !== -1) {
        this.selected.splice(idx, 1);
      }
    },

    /**
     * Check if host is selected
     */
    isSelected(id) {
      return this.selected.includes(id);
    },

    /**
     * Select all hosts
     */
    selectAll() {
      const allIds = this._getAllHostIds();
      this.selected = [...allIds];
      this.lastSelected = allIds[allIds.length - 1] || null;
    },

    /**
     * Deselect all hosts
     */
    selectNone() {
      this.selected = [];
      this.lastSelected = null;
    },

    /**
     * Select range from lastSelected to targetId
     */
    selectRange(targetId) {
      if (!this.lastSelected) {
        this.toggle(targetId);
        return;
      }

      const allIds = this._getAllHostIds();
      const startIdx = allIds.indexOf(this.lastSelected);
      const endIdx = allIds.indexOf(targetId);

      if (startIdx === -1 || endIdx === -1) {
        this.toggle(targetId);
        return;
      }

      const minIdx = Math.min(startIdx, endIdx);
      const maxIdx = Math.max(startIdx, endIdx);

      for (let i = minIdx; i <= maxIdx; i++) {
        if (!this.selected.includes(allIds[i])) {
          this.selected.push(allIds[i]);
        }
      }
    },

    /**
     * Get count of selected hosts
     */
    get count() {
      return this.selected.length;
    },

    /**
     * Get count of selected hosts that are online
     */
    get onlineCount() {
      return this.selected.filter((id) => {
        const host = hostStore.get(id);
        return host && host.online;
      }).length;
    },

    /**
     * Get selection state for header checkbox
     * Returns: 'none' | 'some' | 'all'
     */
    get headerState() {
      const allIds = this._getAllHostIds();
      if (allIds.length === 0) return "none";
      if (this.selected.length === 0) return "none";
      if (this.selected.length === allIds.length) return "all";
      return "some";
    },

    /**
     * Helper: Get all host IDs from DOM
     */
    _getAllHostIds() {
      const ids = [];
      document.querySelectorAll("tr[data-host-id]").forEach((row) => {
        ids.push(row.dataset.hostId);
      });
      return ids;
    },
  });
});
```

### Template Changes (`dashboard.templ`)

#### Table Header

Add as first `<th>`:

```go
<th class="col-select">
    <button
        type="button"
        class="select-toggle"
        @click="handleHeaderCheckboxClick()"
        x-data
        :title="$store.selection.headerState === 'all' ? 'Deselect all' : ($store.selection.headerState === 'some' ? 'Clear selection' : 'Select all')"
    >
        <svg class="icon" x-show="$store.selection.headerState === 'none'"><use href="#icon-square"></use></svg>
        <svg class="icon" x-show="$store.selection.headerState === 'some'"><use href="#icon-minus-square"></use></svg>
        <svg class="icon" x-show="$store.selection.headerState === 'all'"><use href="#icon-check-square"></use></svg>
    </button>
</th>
```

#### Table Row

Modify `HostRow` template:

```go
templ HostRow(host Host, csrfToken string) {
    <tr
        data-host-id={ host.ID }
        data-hostname={ host.Hostname }
        // ... other data attributes ...
        class={ templ.KV("host-offline", !host.Online) }
        x-data
        :class="{ 'selected': $store.selection.isSelected('" + host.ID + "') }"
        @click="handleRowClick($event, '" + host.ID + "')"
    >
        <td class="col-select">
            <input
                type="checkbox"
                class="row-checkbox"
                :checked="$store.selection.isSelected('" + host.ID + "')"
                @click.stop="handleCheckboxClick($event, '" + host.ID + "')"
            />
        </td>
        // ... rest of row ...
    </tr>
}
```

### JavaScript

```js
// ═══════════════════════════════════════════════════════════════════════════
// ROW SELECTION HANDLERS (P1030)
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Handle header checkbox click
 */
function handleHeaderCheckboxClick() {
  const store = Alpine.store("selection");
  if (store.headerState === "none") {
    store.selectAll();
  } else {
    store.selectNone();
  }
}

/**
 * Handle individual checkbox click
 */
function handleCheckboxClick(event, hostId) {
  const store = Alpine.store("selection");

  if (event.shiftKey && store.lastSelected) {
    // Shift+click: range selection
    store.selectRange(hostId);
  } else {
    // Normal click: toggle
    store.toggle(hostId);
  }
}

/**
 * Handle row background click
 */
function handleRowClick(event, hostId) {
  // Don't toggle if clicking on interactive elements
  const target = event.target;
  const interactiveSelectors = [
    "button",
    "input",
    "a",
    ".compartment-btn",
    ".dropdown",
    ".dropdown-menu",
    "[data-cell]", // Allow text selection in data cells
    ".host-name", // Allow text selection on hostname
  ];

  for (const selector of interactiveSelectors) {
    if (target.closest(selector)) {
      return;
    }
  }

  // Toggle selection
  const store = Alpine.store("selection");

  if (event.shiftKey && store.lastSelected) {
    store.selectRange(hostId);
  } else {
    store.toggle(hostId);
  }
}

// ═══════════════════════════════════════════════════════════════════════════
// KEYBOARD SHORTCUTS (P1030)
// ═══════════════════════════════════════════════════════════════════════════

document.addEventListener("keydown", (e) => {
  // Skip if in input/textarea
  if (e.target.matches("input, textarea, [contenteditable]")) {
    return;
  }

  // Ctrl/Cmd + A: Select all
  if ((e.ctrlKey || e.metaKey) && e.key === "a") {
    e.preventDefault();
    Alpine.store("selection").selectAll();
  }

  // Escape: Clear selection (if any)
  if (e.key === "Escape") {
    const store = Alpine.store("selection");
    if (store.count > 0) {
      e.preventDefault();
      store.selectNone();
    }
  }
});
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   ROW SELECTION (P1030)
   ═══════════════════════════════════════════════════════════════════════════ */

/* Checkbox Column */
.col-select {
  width: 40px;
  min-width: 40px;
  max-width: 40px;
  padding: 0 8px !important;
  text-align: center;
}

/* Header Toggle */
.select-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: var(--fg-muted);
  transition:
    color 150ms ease,
    background 150ms ease;
}

.select-toggle:hover {
  color: var(--fg);
  background: var(--bg-highlight);
}

.select-toggle:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.select-toggle .icon {
  width: 18px;
  height: 18px;
}

/* Row Checkbox */
.row-checkbox {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--blue);
  opacity: 0;
  transition: opacity 150ms ease;
}

/* Show checkbox on row hover or when selected */
tr:hover .row-checkbox,
tr.selected .row-checkbox {
  opacity: 1;
}

/* Selected Row */
tr.selected {
  background: var(--bg-highlight) !important;
}

tr.selected:hover {
  background: var(--bg-highlight-bright) !important;
}

/* Prevent text selection when clicking row */
tr[data-host-id] {
  user-select: none;
}

/* But allow text selection on specific content */
tr[data-host-id] [data-cell],
tr[data-host-id] .host-name {
  user-select: text;
  cursor: text;
}
```

### New SVG Icons

Add to sprite in `base.templ`:

```html
<symbol id="icon-square" viewBox="0 0 24 24">
  <rect
    x="3"
    y="3"
    width="18"
    height="18"
    rx="2"
    ry="2"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></rect>
</symbol>

<symbol id="icon-check-square" viewBox="0 0 24 24">
  <rect
    x="3"
    y="3"
    width="18"
    height="18"
    rx="2"
    ry="2"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></rect>
  <polyline
    points="9 11 12 14 22 4"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></polyline>
</symbol>

<symbol id="icon-minus-square" viewBox="0 0 24 24">
  <rect
    x="3"
    y="3"
    width="18"
    height="18"
    rx="2"
    ry="2"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
  ></rect>
  <line
    x1="8"
    y1="12"
    x2="16"
    y2="12"
    stroke="currentColor"
    stroke-width="2"
  ></line>
</symbol>
```

---

## Edge Cases

### Host Added/Removed

| Scenario              | Behavior                              |
| --------------------- | ------------------------------------- |
| Host added (via WS)   | Not selected, header state may change |
| Host removed (via WS) | Removed from selection if selected    |

Add to host removal handler:

```js
// In delete host flow:
Alpine.store("selection").deselect(hostId);
```

### Host Goes Offline While Selected

| Scenario                       | Behavior                           |
| ------------------------------ | ---------------------------------- |
| Selected host goes offline     | Remains selected, shown as offline |
| Bulk action on mixed selection | Only online hosts receive commands |

### Selection Persistence

| Scenario            | Behavior                         |
| ------------------- | -------------------------------- |
| Page refresh        | Selection cleared (session-only) |
| WebSocket reconnect | Selection preserved (in-memory)  |

---

## Testing

### Manual Test Cases

| #   | Scenario          | Steps                           | Expected                            |
| --- | ----------------- | ------------------------------- | ----------------------------------- |
| 1   | Single select     | Click checkbox                  | Row selected, Selection Bar appears |
| 2   | Deselect          | Click selected checkbox         | Row deselected                      |
| 3   | Row click         | Click row background            | Row toggles                         |
| 4   | Text click        | Click hostname text             | NO toggle (text selectable)         |
| 5   | Compartment click | Click compartment button        | NO toggle (action executes)         |
| 6   | Select all        | Click header when none selected | All selected                        |
| 7   | Deselect all      | Click header when all selected  | All deselected                      |
| 8   | Clear partial     | Click header when some selected | All deselected                      |
| 9   | Shift+click       | Select row 1, Shift+click row 4 | Rows 1-4 selected                   |
| 10  | Ctrl+A            | Press Ctrl+A                    | All hosts selected                  |
| 11  | Escape            | Press Escape with selection     | Selection cleared                   |
| 12  | Keyboard checkbox | Tab to checkbox, Space          | Row toggles                         |

---

## Acceptance Criteria

- [ ] Checkbox column is first column (40px)
- [ ] Header button shows correct icon per state
- [ ] Checkboxes hidden by default
- [ ] Checkboxes visible on row hover
- [ ] Checkboxes always visible when selected
- [ ] Click checkbox toggles selection
- [ ] Click row background toggles selection
- [ ] Click text does NOT toggle
- [ ] Click compartment does NOT toggle
- [ ] Click ellipsis does NOT toggle
- [ ] Shift+Click selects range
- [ ] Ctrl/Cmd+A selects all
- [ ] Escape clears selection
- [ ] Header click toggles all/none
- [ ] Header shows indeterminate (minus) when partially selected
- [ ] Selected rows have highlight styling

---

## Related

- **P1015**: Selection Bar (consumes selection store)
- **P1010**: Action Bar (separate, single-host UI)
- **P1040**: Dependency Dialog (called from P1015 bulk actions)
