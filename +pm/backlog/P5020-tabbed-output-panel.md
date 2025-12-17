# P5020 - Tabbed Output Panel

## Problem

Currently, the dashboard has a single output panel at the bottom that shows command output for one host at a time. When multiple hosts have operations running in parallel, users can only see output from one host, losing visibility into the others.

## Solution

Implement a tabbed output panel similar to browser tabs:

- **One tab per host** with active/recent command output
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

| State        | Visual                 | Description                         |
| ------------ | ---------------------- | ----------------------------------- |
| Active       | Highlighted background | Currently viewing this tab          |
| Running      | Pulsing dot/spinner    | Command in progress                 |
| New Output   | Badge/notification dot | Has unread output since last viewed |
| Completed OK | Green indicator        | Command finished successfully       |
| Error        | Red indicator          | Command failed                      |

**Note**: Tabs never auto-transition to "idle" or dimmed state. Completed tabs keep their green/red indicator until closed.

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

### Session Behavior

**Output is session-only.** On page refresh:

- All tabs are closed
- Output buffers are cleared
- Panel collapsed/expanded state persists (via localStorage)
- Panel height persists (via localStorage)

## UI Mockup

### Output Panel (Bottom)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ ┌─────────┐ ┌─────────┐ ┌─────────┐                        [Close All] │
│ │ hsb0  × │ │ hsb1 🔴│ │ gpc0 ●  │                           [▼ Hide]  │
│ └─────────┘ └─────────┘ └─────────┘                                     │
├─────────────────────────────────────────────────────────────────────────┤
│ $ nixos-rebuild switch --flake .#hsb0                                   │
│ building the system configuration...                                    │
│ these 12 derivations will be built:                                     │
│   /nix/store/abc123-source                                              │
│   /nix/store/def456-package                                             │
│ ...                                                                     │
│                                                          [Clear] [Copy] │
└─────────────────────────────────────────────────────────────────────────┘
```

Legend:

- `×` = Close button
- `🔴` = Error state (red dot)
- `●` = Running/has new output

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

## Technical Considerations

### State Management

```javascript
// Per-tab state
const outputTabs = {
  hsb0: {
    hostname: "hsb0",
    command: "switch",
    status: "running", // running, ok, error, idle
    output: [...lines],
    scrollPosition: 0,
    hasUnread: false,
    startedAt: "2025-12-17T18:00:00Z",
    completedAt: null,
  },
  // ...
};
```

### WebSocket Integration

- Subscribe to output for all tabs, not just active one
- Buffer output per host
- Update tab state on command start/complete

### Memory Management

- Limit output lines per tab (default: 1000 lines, oldest lines trimmed)
- No limit on number of tabs (user controls tab lifecycle)
- Output buffer persists even after command completes
- Buffer cleared only when tab is closed or "Clear" button clicked

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

3. **Phase 3: Panel Controls**
   - Collapse/expand
   - Resize by drag
   - Clear/Copy buttons
   - LocalStorage persistence for panel state

4. **Phase 4: Polish**
   - Keyboard shortcuts (Ctrl+1-9 for tabs)
   - Settings for buffer size

## Priority

**Medium** - Improves UX for parallel operations but current single-output works for sequential workflows.

## Dependencies

- P5000 (Host Update Status) - completed
- Existing WebSocket output streaming

## Related

- P5010 - Compartment Status Indicator (visual status patterns)
