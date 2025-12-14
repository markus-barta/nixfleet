# P4385 - UI: Button States & Locking

**Priority**: High  
**Status**: Pending  
**Effort**: Small  
**References**: `+pm/legacy/v1.0/dashboard.html` (lines 2143-2188), PRD FR-3.6

## Problem

v2 shows `pending_command` in the row but doesn't actually disable buttons. This allows:

- Double-clicking to send multiple commands
- Clicking other buttons while command is running
- Confusing UI state

PRD FR-3.6: "Disable buttons while command running" - **Must** priority

## Solution

### Button Disable Logic

```javascript
function applyActionState(row) {
  const online = row.dataset.online === "true";
  const pending = row.dataset.pendingCommand;
  const testRunning = row.dataset.testRunning === "true";
  const busy = pending || testRunning;

  const pullBtn = row.querySelector(".btn-pull");
  const switchBtn = row.querySelector(".btn-switch");
  const testBtn = row.querySelector(".btn-test");
  const stopBtn = row.querySelector(".btn-stop");

  // If offline: all disabled
  if (!online) {
    pullBtn?.disabled = true;
    switchBtn?.disabled = true;
    testBtn?.disabled = true;
    stopBtn?.disabled = true;
    return;
  }

  // If busy: disable primary actions, keep stop enabled
  if (busy) {
    pullBtn?.disabled = true;
    switchBtn?.disabled = true;
    testBtn?.disabled = true;
    stopBtn?.disabled = !testRunning; // Only stop during test
    return;
  }

  // Online and idle: all enabled
  pullBtn?.disabled = false;
  switchBtn?.disabled = false;
  testBtn?.disabled = false;
}
```

### CSS for Disabled State

```css
.btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
  pointer-events: none;
}
```

### Immediate UI Feedback

When button clicked:

1. Immediately set `row.dataset.pendingCommand = command`
2. Call `applyActionState(row)`
3. Send API request
4. On success: wait for WebSocket update
5. On error: clear pending and re-enable

### Requirements

- [ ] Track pendingCommand in row dataset
- [ ] Apply disabled state on click (immediate)
- [ ] Clear on WebSocket status update
- [ ] Clear on API error
- [ ] Add disabled CSS styling
- [ ] Keep Stop enabled during tests

## Related

- P4300 (Live Logs) - Triggers pending state
- P4380 (Dropdown) - "Unlock actions" to manually clear
