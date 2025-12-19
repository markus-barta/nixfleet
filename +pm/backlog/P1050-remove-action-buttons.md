# P1050 - Remove Action Buttons

**Created**: 2025-12-19  
**Priority**: P1050 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-reliable-agent-updates.md)  
**Estimated Effort**: 30 minutes

---

## Scope

Remove the separate Pull, Switch, Test buttons from the host table.

---

## What to Remove

### Current Layout

```
│ Update    │ Actions           │ ⋮ │
│ [G][L][S] │ [Pull][Switch][Test] │ ... │
```

### New Layout

```
│ Update    │ ⋮ │ ☐ │
│ [G][L][S] │ ... │   │
```

The "Actions" column is completely removed.

---

## Files to Modify

| File              | Change                                            |
| ----------------- | ------------------------------------------------- |
| `dashboard.templ` | Remove Actions column, remove CommandButton calls |
| `styles.css`      | Remove `.host-actions` styles (cleanup)           |

---

## Code to Remove

### Template

```go
// REMOVE this entire section
<td class="host-actions">
    @CommandButton(host.ID, "pull", "Pull", "btn", host.Online && host.PendingCommand == "")
    @CommandButton(host.ID, "switch", "Switch", "btn", host.Online && host.PendingCommand == "")
    @CommandButton(host.ID, "test", "Test", "btn", host.Online && host.PendingCommand == "")
</td>
```

### Header

```go
// REMOVE
<th>Actions</th>
```

---

## Acceptance Criteria

- [ ] Pull, Switch, Test buttons removed from table
- [ ] Actions column header removed
- [ ] No JavaScript errors from removed elements
- [ ] CSS cleaned up

---

## Related

- **P1020**: Compartments now handle pull/switch
- **P1060**: Test moved to ellipsis menu
