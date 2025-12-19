# P1030 - Row Selection & Multi-Select

**Created**: 2025-12-19  
**Priority**: P1030 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 2-3 hours

---

## Scope

Add row selection with checkboxes for multi-host operations.

---

## Requirements

### Checkbox Column

- New column at end of table (after ellipsis menu)
- Header: Toggle button (select all / select none)
- Rows: Checkbox, visible on hover or when selected

### Selection Triggers

- Click checkbox → Toggle selection
- Click row background → Toggle selection
- NOT: Text (allow copy), compartment buttons, ellipsis menu

### Selected Row Styling

- Brighter background color
- Checkbox always visible and checked

### Multi-Select State

- Track selected host IDs in Alpine.js store
- Action Bar shows "DO ALL" when multiple selected
- Hovering individual compartment still does single-host action

---

## Implementation

### Table Header

```html
<th class="col-select">
  <button
    class="select-toggle"
    @click="toggleSelectAll()"
    :title="allSelected ? 'Deselect all' : 'Select all'"
  >
    <span x-show="!allSelected">☐</span>
    <span x-show="allSelected">☑</span>
  </button>
</th>
```

### Table Row

```html
<tr
  class="host-row"
  :class="{ 'selected': isSelected(host.id) }"
  @click.self="toggleSelect(host.id)"
>
  <!-- ... other columns ... -->
  <td class="col-select">
    <input
      type="checkbox"
      :checked="isSelected(host.id)"
      @click.stop="toggleSelect(host.id)"
      class="row-checkbox"
    />
  </td>
</tr>
```

### CSS

```css
.row-checkbox {
  opacity: 0;
  transition: opacity 0.2s;
}

.host-row:hover .row-checkbox,
.host-row.selected .row-checkbox {
  opacity: 1;
}

.host-row.selected {
  background: var(--bg-highlight);
}

.select-toggle {
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 4px;
}
```

### Alpine Store

```js
Alpine.store("selection", {
  selected: [],

  toggle(id) {
    const idx = this.selected.indexOf(id);
    if (idx === -1) {
      this.selected.push(id);
    } else {
      this.selected.splice(idx, 1);
    }
  },

  isSelected(id) {
    return this.selected.includes(id);
  },

  selectAll(ids) {
    this.selected = [...ids];
  },

  selectNone() {
    this.selected = [];
  },
});
```

---

## Acceptance Criteria

- [ ] Checkbox column added to table
- [ ] Checkboxes visible on hover
- [ ] Checkboxes always visible when selected
- [ ] Click row background toggles selection
- [ ] Click text does NOT toggle (allows copy)
- [ ] Header toggle selects/deselects all
- [ ] Selected rows have highlight styling
- [ ] Selection state available to Action Bar

---

## Related

- **P1010**: Action Bar (uses selection for "DO ALL")
