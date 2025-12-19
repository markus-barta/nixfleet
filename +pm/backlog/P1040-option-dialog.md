# P1040 - Dependency Warning Dialog

**Created**: 2025-12-19  
**Priority**: P1040 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-reliable-agent-updates.md)  
**Estimated Effort**: 1-2 hours

---

## Scope

Create a dialog that appears when user tries to execute an action with unmet dependencies.

---

## Requirements

### When to Show

| Action | Condition                    | Show Dialog            |
| ------ | ---------------------------- | ---------------------- |
| Switch | Git is yellow (behind)       | Yes                    |
| Switch | Lock is red (agent outdated) | No (switch fixes this) |
| Pull   | Any                          | No (always allowed)    |

### Dialog Content

```
┌─────────────────────────────────────────────────┐
│  ⚠️ Git is behind on hsb1                       │
│  ─────────────────────────────────────────────  │
│  Running Switch without Pull may use old code.  │
│                                                 │
│  [Cancel]  [Pull]  [Switch]  [Pull + Switch]    │
└─────────────────────────────────────────────────┘
```

### Button Actions

- **Cancel**: Close dialog, do nothing
- **Pull**: Run pull only
- **Switch**: Run switch anyway (user's choice)
- **Pull + Switch**: Run pull, then switch (command: `pull-switch`)

### Dialog Behavior

- Modal overlay (blocks background)
- Escape key closes
- Click outside closes
- Keyboard navigation (Tab between buttons)

---

## Implementation

### Template

```go
templ OptionDialog() {
    <div id="option-dialog" class="dialog-overlay" x-show="optionDialog.show" x-cloak>
        <div class="dialog-content">
            <div class="dialog-header">
                <span class="dialog-icon">⚠️</span>
                <span class="dialog-title" x-text="optionDialog.title"></span>
            </div>
            <div class="dialog-message" x-text="optionDialog.message"></div>
            <div class="dialog-actions">
                <button class="btn" @click="optionDialog.cancel()">Cancel</button>
                <template x-for="opt in optionDialog.options">
                    <button class="btn" @click="optionDialog.choose(opt.action)"
                            x-text="opt.label"></button>
                </template>
            </div>
        </div>
    </div>
}
```

### CSS

```css
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog-content {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  max-width: 450px;
}

.dialog-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 16px;
}
```

---

## Acceptance Criteria

- [ ] Dialog appears when Switch clicked with Git behind
- [ ] All 4 buttons work correctly
- [ ] Cancel/Escape closes dialog
- [ ] Click outside closes dialog
- [ ] Keyboard navigation works
- [ ] Pull + Switch executes both in order

---

## Related

- **P1020**: Clickable compartments (triggers dialog check)
