# P4020 - Tabbed Output Panel

## Problem

Currently, the dashboard has a single output panel at the bottom that shows command output for one host at a time. When multiple hosts have operations running in parallel, users can only see output from one host, losing visibility into the others.

Additionally, ephemeral UI events (toast notifications, connection events, errors) disappear without a trace. Users have no way to review what happened if they missed a notification.

## Solution

Implement a tabbed output panel similar to browser tabs:

- **One tab per host** with active/recent command output
- **System Log tab** captures all ephemeral events (toasts, errors, system messages)
- **Tabs persist** until the user explicitly dismisses them (X button)
- **Visual indicators** for tab state (active, has new output, completed, error)
- **Auto-scroll** within each tab, independent of other tabs

## User Stories

### US-1: View Multiple Outputs

**As a** fleet administrator  
**I want to** see command output from multiple hosts simultaneously  
**So that** I can monitor parallel operations across my fleet

### US-2: Dismiss Completed Outputs

**As a** fleet administrator  
**I want to** close output tabs when I'm done reviewing them  
**So that** the output panel stays clean and focused

### US-3: Track Output State

**As a** fleet administrator  
**I want to** see which tabs have new output or errors  
**So that** I can quickly identify hosts that need attention

### US-4: Review System Events

**As a** fleet administrator  
**I want to** see a history of system events and toast notifications  
**So that** I can review what happened even if I missed a notification

## Functional Requirements

### FR-1: Tab Management

| ID     | Requirement                                                  |
| ------ | ------------------------------------------------------------ |
| FR-1.1 | New tab auto-created on first output for a host              |
| FR-1.2 | Tab shows hostname as label                                  |
| FR-1.3 | Tab has X button to close/dismiss                            |
| FR-1.4 | Clicking tab switches to that host's output                  |
| FR-1.5 | Tabs persist until explicitly closed by user (NO auto-close) |
| FR-1.6 | Ellipsis menu has "Show Output" option to open/reopen tab    |
| FR-1.7 | "Show Output" reopens tab with buffered output history       |
| FR-1.8 | Tabs ordered by creation time (first command = leftmost)     |
| FR-1.9 | New command on existing tab appends with visual separator    |

### FR-1.9 Detail: Command Separator

When a new command starts on a host that already has an open tab, insert a visual separator:

```
...previous output...
───────────────── switch (19:05:23) ─────────────────
$ nixos-rebuild switch --flake .#hsb0
building the system configuration...
```

The separator shows command name and timestamp.

### FR-1.5 Critical: No Auto-Close

**The output panel and tabs must NEVER auto-close.** This is a deliberate design decision:

- ❌ Do NOT close on command completion
- ❌ Do NOT close on timeout
- ❌ Do NOT close when switching to another host
- ✅ Only close when user clicks X on the tab
- ✅ Only close when user clicks "Close All"

### FR-2: Tab States

| State              | Visual                 | Description                              |
| ------------------ | ---------------------- | ---------------------------------------- |
| Active             | Highlighted background | Currently viewing this tab               |
| Running            | Pulsing dot/spinner    | Command in progress                      |
| Awaiting Reconnect | Pulsing orange dot     | Switch complete, waiting for agent       |
| New Output         | Badge/notification dot | Has unread output since last viewed      |
| Completed OK       | Green indicator        | Command finished successfully            |
| Warning            | Orange indicator       | Partial success or stale binary detected |
| Error              | Red indicator          | Command failed                           |
| Timeout            | Yellow indicator       | Awaiting user action (timeout)           |

**Note**: Tabs never auto-transition to "idle" or dimmed state. Completed tabs keep their indicator until closed.

**State Mapping from P2800 Command State Machine:**

| P2800 State        | Tab State          | Indicator |
| ------------------ | ------------------ | --------- |
| IDLE               | (no tab)           | -         |
| VALIDATING         | Running            | Spinner   |
| QUEUED             | Running            | Spinner   |
| RUNNING            | Running            | Spinner   |
| AWAITING_RECONNECT | Awaiting Reconnect | 🟠        |
| SUCCESS            | Completed OK       | 🟢        |
| PARTIAL            | Warning            | 🟠        |
| STALE_BINARY       | Warning            | 🟠        |
| FAILED             | Error              | 🔴        |
| BLOCKED            | Error              | 🔴        |
| TIMEOUT_PENDING    | Timeout            | 🟡        |
| KILL_FAILED        | Timeout            | 🟡        |
| ABORTED_BY_REBOOT  | Warning            | 🟠        |

### FR-3: Output Panel Behavior

| ID     | Requirement                                                          |
| ------ | -------------------------------------------------------------------- |
| FR-3.1 | Each tab maintains independent scroll position                       |
| FR-3.2 | Auto-scroll to bottom when new output arrives (if already at bottom) |
| FR-3.3 | Output buffer per tab (last N lines, configurable)                   |
| FR-3.4 | Clear button to clear output for current tab                         |
| FR-3.5 | Copy button to copy output to clipboard                              |

### FR-4: Panel Controls

| ID     | Requirement                                                           |
| ------ | --------------------------------------------------------------------- |
| FR-4.1 | Collapse/expand the entire output panel                               |
| FR-4.2 | Resize panel height by dragging                                       |
| FR-4.3 | "Close All" button to dismiss all tabs                                |
| FR-4.4 | Panel state (collapsed/expanded, height) persists across page refresh |

### FR-5: Tab Overflow (Desktop + Mobile)

| ID     | Requirement                                                           |
| ------ | --------------------------------------------------------------------- |
| FR-5.1 | When tabs exceed available width, show "more tabs" dropdown           |
| FR-5.2 | Dropdown lists hidden tabs with hostname and state indicator          |
| FR-5.3 | Clicking dropdown item switches to that tab (moves to visible area)   |
| FR-5.4 | On mobile (<640px), only active tab visible + dropdown for all others |

### FR-6: System Log Tab

A special tab that captures all ephemeral UI events (toasts, errors, system messages).

| ID     | Requirement                                                    |
| ------ | -------------------------------------------------------------- |
| FR-6.1 | Tab auto-created on first system message                       |
| FR-6.2 | Tab labeled "Log" with 📋 icon                                 |
| FR-6.3 | Tab is closable like any other tab                             |
| FR-6.4 | If closed, re-created automatically on next system message     |
| FR-6.5 | New entries appear at the top (reverse chronological)          |
| FR-6.6 | Each entry shows: icon, timestamp, message                     |
| FR-6.7 | Timestamp format: "14:23:05 (2 min ago)" - absolute + relative |
| FR-6.8 | Scrollable with same buffer limit as host tabs (1000 entries)  |

### FR-7: System Log Message Categories

| Icon | Color  | Category | Examples                                                |
| ---- | ------ | -------- | ------------------------------------------------------- |
| ✓    | Green  | Success  | Command completed, host connected, PR merged            |
| ⚠   | Orange | Warning  | Partial success, stale binary, timeout, host disconnect |
| ✗    | Red    | Error    | Command failed, connection lost, kill failed            |
| ℹ   | Blue   | Info     | Command started, system events, state transitions       |
| ⧖    | Yellow | Pending  | Awaiting reconnect, awaiting user action                |

### FR-8: Events Logged to System Log

| Event Type                           | Logged? | Source        |
| ------------------------------------ | ------- | ------------- |
| Toast notifications                  | ✅      | UI            |
| Command start (any host)             | ✅      | P2800         |
| Command end (any host)               | ✅      | P2800         |
| Pre-validation blocked               | ✅      | P2800         |
| Post-validation result               | ✅      | P2800         |
| Timeout warning                      | ✅      | P2800         |
| Timeout pending (user action needed) | ✅      | P2800         |
| Kill process attempt                 | ✅      | P2800         |
| Kill failed (agent unresponsive)     | ✅      | P2800         |
| Awaiting agent reconnect             | ✅      | P2800         |
| Stale binary detected                | ✅      | P2800/P2810   |
| Post-reboot recovery                 | ✅      | P2800/P6900   |
| Host connect                         | ✅      | Agent         |
| Host disconnect                      | ✅      | Agent         |
| WebSocket connection issues          | ✅      | Dashboard     |
| Flake update PR events               | ✅      | P5300         |
| Agent version mismatch               | ✅      | Agent         |
| Git/Lock/System status change        | ❌      | (too verbose) |
| Heartbeats                           | ❌      | (too verbose) |

### FR-9: Status History in Host Tabs (merged from P6600)

Each host tab displays a compact status history summary at the top, showing recent status updates before the full command output.

| ID     | Requirement                                                      |
| ------ | ---------------------------------------------------------------- |
| FR-9.1 | Status history appears at top of host tab (above command output) |
| FR-9.2 | Shows last N status entries (default: 10, configurable)          |
| FR-9.3 | Each entry shows: timestamp (HH:MM), icon, truncated message     |
| FR-9.4 | Entries scrollable if history exceeds visible area               |
| FR-9.5 | Most recent entry highlighted (bold or brighter color)           |
| FR-9.6 | Error entries shown in red (matching System Log styling)         |
| FR-9.7 | Status history updates in real-time via WebSocket                |
| FR-9.8 | History persists across tab close/reopen (session storage)       |

**Status History Icons** (aligned with P2800 verbose logging):

| Event   | Icon | Color  | Description                    |
| ------- | ---- | ------ | ------------------------------ |
| Success | ✓    | Green  | Command completed successfully |
| Warning | ⚠   | Orange | Partial, stale binary, timeout |
| Error   | ✗    | Red    | Command failed, blocked        |
| Pending | ⧖    | Yellow | Running, awaiting reconnect    |
| Info    | ℹ   | Blue   | State transitions, progress    |
| Testing | ✦    | Cyan   | Test execution in progress     |

**Status History Events** (from P2800 state machine):

- ✅ Command queued (VALIDATING → QUEUED)
- ✅ Pre-check passed/failed
- ✅ Command started (RUNNING)
- ✅ Awaiting reconnect (switch-specific)
- ✅ Command completed (SUCCESS)
- ✅ Partial success (PARTIAL)
- ✅ Command failed (FAILED)
- ✅ Command blocked (BLOCKED + reason)
- ✅ Timeout warning (RUNNING_WARNING)
- ✅ Timeout pending (TIMEOUT_PENDING)
- ✅ Kill attempt/failed
- ✅ Stale binary detected (STALE_BINARY)
- ✅ Post-reboot recovery (ABORTED_BY_REBOOT → IDLE)
- ✅ Test progress ("Testing 3/8")
- ✅ Test result ("Tests: 8/8 passed")
- ❌ Heartbeats (too noisy, excluded)

### Session Behavior

**Output is session-only.** On page refresh:

- All tabs are closed
- Output buffers are cleared
- Panel collapsed/expanded state persists (via localStorage)
- Panel height persists (via localStorage)

## UI Mockup

### Output Panel (Bottom) - Host Tab

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            [Close All] │
│ │ 📋 Log ×│ │ hsb0  × │ │ hsb1 🔴│ │ gpc0 ●  │               [▼ Hide] │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘                         │
├─────────────────────────────────────────────────────────────────────────┤
│ Status History (last 10):                                                │
│   14:23 ✓ Switch complete                                                │
│   14:22 ⧖ Switching...                                                  │
│   14:20 ✓ Pull complete                                                  │
│   14:19 ⧖ Pulling...                                                     │
│ ──────────────────────────────────────────────────────────────────────── │
│ Command Output:                                                          │
│ $ nixos-rebuild switch --flake .#hsb0                                   │
│ building the system configuration...                                    │
│ these 12 derivations will be built:                                     │
│   /nix/store/abc123-source                                              │
│   /nix/store/def456-package                                             │
│ ...                                                                     │
│                                                          [Clear] [Copy] │
└─────────────────────────────────────────────────────────────────────────┘
```

**Note**: Status history section is collapsible (click to expand/collapse) and shows truncated messages (≤100 chars). Full details available in command output below.

### Output Panel (Bottom) - System Log Tab

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            [Close All] │
│ │ 📋 Log ×│ │ hsb0  × │ │ hsb1 🔴│ │ gpc0 ●  │               [▼ Hide] │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘                         │
├─────────────────────────────────────────────────────────────────────────┤
│ ✗  14:23:05 (just now)   hsb1 command failed: switch (exit 1)          │
│ ✓  14:22:47 (38s ago)    gpc0 connected                                │
│ ℹ  14:22:45 (40s ago)    Pull started on 3 hosts                       │
│ ⚠  14:21:30 (2 min ago)  imac0 disconnected (timeout)                  │
│ ✓  14:20:00 (3 min ago)  Flake update PR #42 merged                    │
│ ⚠  14:19:58 (3 min ago)  Agent version mismatch: gpc0 (2.0.0 → 2.1.0)  │
│ ℹ  14:19:55 (3 min ago)  Dashboard started (v2.1.0)                    │
│                                                          [Clear] [Copy] │
└─────────────────────────────────────────────────────────────────────────┘
```

Legend:

- `×` = Close button
- `🔴` = Error state (red dot)
- `●` = Running/has new output
- `📋` = System Log tab

### Ellipsis Menu (Per-Host)

```
┌─────────────────┐
│ Pull            │
│ Switch          │
│ Test            │
│ Pull + Switch   │
│ ─────────────── │
│ Show Output     │  ← Opens/reopens output tab for this host
│ ─────────────── │
│ Remove Host     │
└─────────────────┘
```

The "Show Output" option:

- Opens the output panel if collapsed
- Creates a new tab for the host (or switches to existing tab)
- Shows buffered output history from previous commands

### Bulk Actions Dropdown (Header)

Rename "Bark Actions" → "Bulk Actions" with sections:

```
┌───────────────────────┐
│ Fleet Actions         │
│   Pull All            │
│   Switch All          │
│   Bark at All         │
│ ──────────────────────│
│ View                  │
│   Show/Hide Output    │  ← Toggles entire output panel
└───────────────────────┘
```

Note: No separate "Show System Log" option needed - it's just a tab in the output panel.

## Technical Considerations

### State Management

```javascript
// Per-tab state (host tabs)
const outputTabs = {
  hsb0: {
    hostname: "hsb0",
    command: "switch",
    status: "running", // running, ok, error, idle
    output: [...lines],
    statusHistory: [
      // Merged from P6600: compact status history
      {
        timestamp: "2025-12-17T14:23:05Z",
        icon: "✓",
        message: "Switch complete",
        category: "success",
      },
      {
        timestamp: "2025-12-17T14:22:31Z",
        icon: "⧖",
        message: "Switching...",
        category: "pending",
      },
      // ... last N entries (default: 10)
    ],
    scrollPosition: 0,
    hasUnread: false,
    startedAt: "2025-12-17T18:00:00Z",
    completedAt: null,
  },
  // ...
};

// System Log state
const systemLog = {
  entries: [
    {
      id: "uuid",
      category: "error", // success, warning, error, info
      timestamp: "2025-12-17T14:23:05Z",
      message: "hsb1 command failed: switch (exit 1)",
    },
    // ... newest first
  ],
  scrollPosition: 0,
  hasUnread: false,
};

// Helper for relative time display
function formatRelativeTime(timestamp) {
  // Returns "just now", "38s ago", "2 min ago", "1 hour ago", etc.
}
```

### WebSocket Integration

- Subscribe to output for all tabs, not just active one
- Buffer output per host
- Update tab state on command start/complete
- Broadcast status history updates (from P2800 state machine)

**P2800 State Machine Log Messages:**

The dashboard broadcasts `state_machine_log` messages that P4020 consumes:

```json
{
  "type": "state_machine_log",
  "payload": {
    "timestamp": "2025-12-21T14:23:05Z",
    "level": "info",
    "host_id": "hsb1",
    "state": "PRE-CHECK",
    "message": "CanSwitch: PASS (git=ok, system=outdated)",
    "code": "outdated"
  }
}
```

**Level Mapping to Status History Icons:**

| P2800 Level | P4020 Icon | Color  |
| ----------- | ---------- | ------ |
| SUCCESS     | ✓          | Green  |
| WARNING     | ⚠         | Orange |
| ERROR       | ✗          | Red    |
| INFO        | ℹ         | Blue   |
| DEBUG       | (skip)     | -      |

**Events Consumed from P2800:**

- All state transitions (IDLE → VALIDATING → QUEUED → RUNNING → ...)
- Pre-check validation results
- Post-check validation results
- Timeout events
- Kill command events
- Reboot abort/recovery events
- Binary freshness checks

### Memory Management

- Limit output lines per tab (default: 1000 lines, oldest lines trimmed)
- Limit status history entries per host (default: 50 entries, oldest trimmed) (merged from P6600)
- No limit on number of tabs (user controls tab lifecycle)
- Output buffer persists even after command completes
- Status history persists across tab close/reopen (session storage)
- Buffers cleared only when tab is closed or "Clear" button clicked

## Implementation Order

1. **Phase 1: Basic Tabs**
   - Tab bar with hostname labels
   - Click to switch tabs
   - X button to close
   - Tab overflow dropdown (required for mobile)
   - Command separator when new command starts on existing tab

2. **Phase 2: State Indicators**
   - Running/completed/error indicators
   - Unread output badge

3. **Phase 3: System Log Tab**
   - Auto-create Log tab on first system message
   - Message categories with colored icons
   - Dual timestamp format (absolute + relative)
   - Reverse chronological order (newest at top)
   - Hook into toast system to capture messages

4. **Phase 4: Status History in Host Tabs** (merged from P6600)
   - Backend: Store status_history array per host (in-memory, session-only)
   - Backend: Broadcast status history updates via WebSocket
   - Frontend: Render status history summary at top of host tabs
   - Frontend: Status history icons and styling
   - Frontend: Collapsible status history section
   - Frontend: Real-time updates as commands progress

5. **Phase 5: Panel Controls**
   - Collapse/expand
   - Resize by drag
   - Clear/Copy buttons
   - LocalStorage persistence for panel state
   - Rename "Bark Actions" → "Bulk Actions" with sections

6. **Phase 6: Polish**
   - Keyboard shortcuts (Ctrl+1-9 for tabs)
   - Settings for buffer size
   - Settings for status history retention (default: 50 entries)

## Priority

**Medium** - Improves UX for parallel operations but current single-output works for sequential workflows.

## Dependencies

- Host Update Status - completed (built into dashboard)
- Existing WebSocket output streaming
- **P2800 Command State Machine** - REQUIRED (provides state transitions, validation results, timeout events)

## Related

- **P2800** - Command State Machine (comprehensive spec - source of all state machine events)
- **P6600** - Status Papertrail (merged into this item - status history in host tabs)
- **P6900** - Forced Reboot (reboot abort/recovery events displayed in System Log)
- P5010 - Compartment Status Indicator (visual status patterns)
