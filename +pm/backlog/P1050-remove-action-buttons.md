# P1050 - Remove Action Buttons

**Created**: 2025-12-19  
**Updated**: 2025-12-19  
**Priority**: P1050 (Medium)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 1 hour  
**Depends on**: P1020 (Clickable Compartments), P1060 (Ellipsis Menu)

---

## Overview

Remove the per-row Pull/Switch/Test buttons from the Actions column. These are now replaced by:

- **Pull/Switch**: Clickable compartments (P1020)
- **Test**: Moved to ellipsis menu (P1060)
- **Stop**: Moved to Action Bar during progress (P1010)

---

## Current State (Before)

```
│ ... │ Actions                    │ ⋮ │
│     │ [Pull] [Switch] [Test] [↻] │   │
```

When command running:

```
│ ... │ Actions      │ ⋮ │
│     │ [Stop] [↻]   │   │
```

---

## Target State (After)

```
│ ... │ ⋮ │
│     │   │
```

No Actions column. Ellipsis column remains.

---

## Changes Required

### 1. Remove Table Header

**Before:**

```go
<th class="col-actions">Actions</th>
<th class="col-menu"></th>
```

**After:**

```go
<th class="col-menu"></th>
```

### 2. Remove Actions Cell

**Before (`HostRow`):**

```go
<td class="actions-cell">
    <div class="action-buttons" data-host-id={ host.ID }>
        <span class="cmd-buttons" style={ visibleWhen(host.PendingCommand == "") }>
            @CommandButton(host.ID, "pull", "", "btn", host.Online)
            @CommandButton(host.ID, "switch", "", "btn", host.Online)
            @CommandButton(host.ID, "test", "", "btn", host.Online)
        </span>
        <button class="btn btn-stop" ... />
        <button class="btn btn-refresh" ... />
        @ActionDropdown(host)
    </div>
</td>
<td class="dropdown-cell">
    <!-- Dropdown now moved inline -->
</td>
```

**After:**

```go
<td class="col-menu">
    @ActionDropdown(host)
</td>
```

### 3. Remove CSS

Delete or comment out:

```css
.actions-cell { ... }
.action-buttons { ... }
.cmd-buttons { ... }
.btn-refresh { ... }  /* If not used elsewhere */
.col-actions { ... }
```

### 4. Remove JavaScript

The following are no longer needed if only used by action buttons:

- Per-row `btn-refresh` click handler (if refresh moves elsewhere)
- Button styling classes for cmd-buttons

---

## Migration Checklist

| Step | File              | Change                                                    |
| ---- | ----------------- | --------------------------------------------------------- |
| 1    | `dashboard.templ` | Remove `<th class="col-actions">`                         |
| 2    | `dashboard.templ` | Remove `<td class="actions-cell">`                        |
| 3    | `dashboard.templ` | Remove `CommandButton` calls from row                     |
| 4    | `dashboard.templ` | Keep only `@ActionDropdown(host)` in final cell           |
| 5    | `styles.css`      | Remove `.actions-cell`, `.action-buttons`, `.cmd-buttons` |
| 6    | `styles.css`      | Remove `.col-actions`                                     |
| 7    | Verify            | Table still renders correctly                             |
| 8    | Verify            | Ellipsis menu works                                       |
| 9    | Verify            | No console errors                                         |

---

## Preserved Functionality

| Feature | New Location                            |
| ------- | --------------------------------------- |
| Pull    | Git compartment (P1020)                 |
| Switch  | System compartment (P1020)              |
| Test    | Ellipsis menu (P1060)                   |
| Stop    | Action Bar (P1010) + Ellipsis menu      |
| Refresh | Removed (handled by compartment clicks) |

---

## Testing

### Manual Test Cases

| #   | Scenario          | Steps                           | Expected                          |
| --- | ----------------- | ------------------------------- | --------------------------------- |
| 1   | Table renders     | Load dashboard                  | No Actions column, ellipsis works |
| 2   | No console errors | Check DevTools                  | Clean console                     |
| 3   | Pull works        | Click Git yellow                | Pull executes                     |
| 4   | Switch works      | Click System yellow             | Switch (via dialog if needed)     |
| 5   | Test works        | Ellipsis → Test                 | Test executes                     |
| 6   | Stop works        | Start command → Ellipsis → Stop | Command stops                     |

---

## Acceptance Criteria

- [ ] Actions column removed from table
- [ ] Pull/Switch/Test buttons removed from rows
- [ ] Ellipsis menu in final column
- [ ] No orphaned CSS/JS
- [ ] No console errors
- [ ] Compartment clicks work for Pull/Switch
- [ ] Ellipsis Test works
- [ ] Table aligns correctly

---

## Related

- **P1020**: Clickable Compartments (replaces Pull/Switch buttons)
- **P1060**: Ellipsis Menu (receives Test command)
- **P1010**: Action Bar (receives Stop button during progress)
