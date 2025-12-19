# P1020 - Clickable Compartments

**Created**: 2025-12-19  
**Priority**: P1020 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-reliable-agent-updates.md)  
**Estimated Effort**: 2-3 hours

---

## Scope

Make the Update column compartments (Git, Lock, System) clickable to trigger actions.

---

## Requirements

### Compartment Actions

| Compartment | Color     | Click Action          |
| ----------- | --------- | --------------------- |
| Git         | 🟡 Yellow | `pull` command        |
| Git         | 🟢 Green  | Refresh status        |
| Lock        | 🟡 Yellow | Info only (no action) |
| Lock        | 🔴 Red    | `switch` command      |
| Lock        | 🟢 Green  | Refresh status        |
| System      | 🟡 Yellow | `switch` command      |
| System      | 🟢 Green  | Refresh status        |

### Hover Behavior

- On hover: Update Action Bar with preview
- Cursor: pointer
- Visual feedback: slight highlight

### Click Behavior

- Execute action immediately (desktop)
- On touch: First tap shows Action Bar with DO NOW, second tap executes

---

## Implementation

### Template Changes

Current:

```go
<span class={ updateCompartmentClass(host.UpdateStatus, "git") }
      title={ gitTooltip(host) }>
```

New:

```go
<button class={ "compartment " + updateCompartmentClass(host.UpdateStatus, "git") }
        @mouseenter="showAction('pull', 'hsb1')"
        @mouseleave="hideAction()"
        @click="executeAction('pull', 'hsb1')">
```

### CSS

```css
.compartment {
  cursor: pointer;
  transition:
    transform 0.1s,
    background 0.1s;
}

.compartment:hover {
  transform: scale(1.1);
  background: var(--bg-highlight);
}

.compartment:active {
  transform: scale(0.95);
}
```

---

## Acceptance Criteria

- [ ] All 3 compartments are clickable buttons
- [ ] Hover updates Action Bar
- [ ] Click executes appropriate command
- [ ] Green compartments trigger refresh
- [ ] Lock yellow shows info (no action)
- [ ] Visual hover feedback

---

## Related

- **P1010**: Action Bar (receives hover events)
- **P1040**: Option Dialog (shown for dependency warnings)
