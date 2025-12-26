# P7240: Timeout UI (Context Bar + Status Indicator)

**Priority**: High  
**Effort**: Medium  
**Status**: Done

## Problem

Commands can timeout (e.g., slow `nix build`, network issues). The backend already:

- Detects timeout and transitions to `StateTimeoutPending`
- Broadcasts `command_state_change` event
- Has `/api/hosts/{id}/timeout-action` API ready

But there's no frontend UI to show timeout status or let users take action.

## Solution

Non-modal timeout UI with two components:

### 1. Status Column Indicator

When a command times out, the Status column shows a pulsing warning:

```
Normal:    [● ● ●]           (progress dots)
Timeout:   [⚠️ 2:30]          (pulsing, elapsed time)
```

- Pulsing animation to grab attention
- Shows elapsed time since command started
- Click opens host log panel

### 2. Context Bar Notification

The context bar (bottom of table) shows timeout notification:

```
┌──────────────────────────────────────────────────────────────────────┐
│ ⚠️ hsb1: switch timed out (2:30)   [Wait 5m] [Kill] [Ignore]   [×]  │
└──────────────────────────────────────────────────────────────────────┘
```

- Non-blocking (no modal)
- Clear actions: Wait, Kill, Ignore
- Dismissible with ×
- Multiple timeouts stack vertically

## Implementation

### Status Column Changes

In `UpdateStatusCell`, detect timeout state and render indicator:

```javascript
// In updateHostStatus or similar
if (commandState === "timeout_pending" || commandState === "running_warning") {
  statusCell.innerHTML = `<span class="timeout-indicator">${elapsedTime}</span>`;
}
```

CSS:

```css
.timeout-indicator {
  color: var(--yellow);
  animation: pulse-warning 1s ease-in-out infinite;
}

@keyframes pulse-warning {
  0%,
  100% {
    opacity: 0.6;
  }
  50% {
    opacity: 1;
  }
}
```

### Context Bar Changes

Add timeout notification section:

```html
<div class="context-bar-timeout" id="timeout-notifications">
  <!-- Dynamically populated -->
</div>
```

JavaScript handler for `command_state_change`:

```javascript
window.addEventListener("command_state_change", (e) => {
  if (e.detail.state === "timeout_pending") {
    showTimeoutNotification(
      e.detail.hostId,
      e.detail.command,
      e.detail.elapsed,
    );
  }
});

function showTimeoutNotification(hostId, command, elapsed) {
  // Add notification to context bar
  // Include Wait/Kill/Ignore buttons
  // Buttons call /api/hosts/{hostId}/timeout-action
}
```

### API Calls

```javascript
async function handleTimeoutAction(hostId, action, minutes = 5) {
  await fetch(`/api/hosts/${hostId}/timeout-action`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": CSRF_TOKEN },
    body: JSON.stringify({ action, minutes }),
  });
}
```

## Timeout States

| State             | Status Column  | Context Bar               |
| ----------------- | -------------- | ------------------------- |
| `running`         | Progress dots  | —                         |
| `running_warning` | ⚠️ + elapsed   | —                         |
| `timeout_pending` | ⚠️ + elapsed   | Notification with actions |
| `killing`         | 🔴 Killing...  | "Killing..." status       |
| `kill_failed`     | 🔴 Kill failed | Retry/Ignore options      |

## Acceptance Criteria

- [ ] Status column shows pulsing ⚠️ indicator when timeout
- [ ] Elapsed time displayed next to indicator
- [ ] Context bar shows notification for `timeout_pending` state
- [ ] Wait button extends timeout by 5 minutes
- [ ] Kill button sends SIGTERM (with SIGKILL escalation)
- [ ] Ignore button dismisses notification, leaves command running
- [ ] Multiple timeouts display as stacked notifications
- [ ] Notification auto-dismisses when command completes/fails

## Edge Cases

- **Multiple hosts timeout** — Stack notifications, most recent on top
- **User ignores, then command completes** — Notification auto-dismisses
- **Kill fails** — Show error, offer retry with SIGKILL
- **Page refresh** — Re-fetch command states, restore notifications

## Related

- P2800: Command State Machine (backend for timeout handling)
- P4020: Tabbed Output Panel (context bar lives here)
