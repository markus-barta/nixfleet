# P1010 - Action Bar Component

**Created**: 2025-12-19  
**Priority**: P1010 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 3-4 hours

---

## Scope

Create the Action Bar component that displays in the header and shows action previews.

---

## Requirements

### Position

- Fixed in header, center (between logo and user menu)
- Always visible (content changes based on hover state)

### States

1. **Idle**: "Hover a status to see actions"
2. **Single host**: Shows action name, description, host name, [▶ DO NOW] button
3. **Multi host**: Shows action name, description, host count, [▶ DO ALL] button
4. **In progress**: Shows spinner, action name, host
5. **Complete**: Shows checkmark, success message (briefly)

### Animation

- 1s fade in when content changes
- 1s fade out before new content
- CSS transitions, no JavaScript animation

### Data Flow

- Receives hover events from compartments
- Receives selected hosts list from row selection
- Triggers command execution on button click

---

## Implementation

### Template (dashboard.templ)

```go
templ ActionBar() {
    <div id="action-bar" class="action-bar" x-data="actionBar()">
        <div class="action-bar-content" :class="{ 'fade-in': visible, 'fade-out': !visible }">
            <template x-if="!action">
                <span class="action-bar-idle">Hover a status to see actions</span>
            </template>
            <template x-if="action">
                <div class="action-bar-preview">
                    <div class="action-bar-header">
                        <span class="action-icon" x-text="action.icon"></span>
                        <span class="action-name" x-text="action.name"></span>
                        <button class="action-btn" @click="execute()" x-text="action.btnText"></button>
                    </div>
                    <div class="action-bar-description" x-text="action.description"></div>
                    <div class="action-bar-target" x-text="action.target"></div>
                </div>
            </template>
        </div>
    </div>
}
```

### CSS (styles.css)

```css
.action-bar {
  position: fixed;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
  min-width: 300px;
  max-width: 500px;
}

.action-bar-content {
  background: var(--bg-highlight);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
  transition: opacity 1s ease;
}

.action-bar-content.fade-out {
  opacity: 0;
}

.action-bar-content.fade-in {
  opacity: 1;
}

.action-bar-idle {
  color: var(--fg-muted);
  font-style: italic;
}

.action-btn {
  background: var(--green);
  color: var(--bg);
  padding: 4px 12px;
  border-radius: 4px;
  font-weight: 600;
}
```

---

## Acceptance Criteria

- [ ] Action Bar visible in header center
- [ ] Shows idle message when nothing hovered
- [ ] Shows action preview on compartment hover
- [ ] Shows "DO ALL" when multiple hosts selected
- [ ] 1s fade in/out animation works
- [ ] Button executes action on click

---

## Related

- **P1020**: Clickable compartments (sends hover events)
- **P1030**: Row selection (provides selected hosts list)
