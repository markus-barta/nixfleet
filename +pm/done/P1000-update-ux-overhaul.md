# P1000 - Update UX Overhaul

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1000 (Critical - UX)  
**Status**: ✅ DONE  
**Completed**: 2025-12-20  
**Estimated Effort**: 4-5 days  
**Depends on**: None

> **Implementation Note**: The original design was refined during implementation. Instead of separate Action Bar (P1010) and Selection Bar (P1015), a unified **Context Bar** was implemented below the table. This provides the same functionality with better UX - single bar that shows hover previews and selection actions together.

---

## Background: Problem Statement

### The Real Issue

Originally we thought macOS agents weren't restarting after switch. **Manual testing proved this wrong**:

- ✅ **macOS**: `home-manager switch` correctly restarts agent via launchd
- ✅ **NixOS**: `nixos-rebuild switch` correctly restarts agent via systemd exit(101)

**The real issue**: Users click "Pull" and think they're done, but they also need to click "Switch". The current UI presents these as separate, unrelated actions when they're actually a workflow.

### User Research Findings

| Observation                                | Root Cause               | Impact                    |
| ------------------------------------------ | ------------------------ | ------------------------- |
| Users don't click Switch after Pull        | Actions appear unrelated | Hosts stay on old config  |
| Users don't understand compartment colors  | No action mapping        | Confusion, no remediation |
| Bulk operations require clicking each host | No multi-select          | Time waste, frustration   |
| No preview of what will happen             | Immediate execution      | Accidental actions        |

---

## Executive Summary

Complete refactor of the update/action UX to make the dashboard self-explanatory. The new design introduces:

1. **Action Bar** — Shows what will happen before you click
2. **Clickable Compartments** — Direct action from status indicators
3. **Row Selection** — Multi-host operations with checkboxes
4. **Selection Bar** — Bulk action controls when hosts are selected
5. **Dependency Dialogs** — Warns when actions have unmet prerequisites

**Design Philosophy**: Preview before action, consistency across interactions, no hidden workflows.

---

## Architecture

### Component Dependency Graph

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              P1000 UX Overhaul                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
          ▼                           ▼                           ▼
    ┌──────────┐               ┌──────────┐               ┌──────────┐
    │  P1030   │               │  P1010   │               │  P1060   │
    │   Row    │               │  Action  │               │ Ellipsis │
    │Selection │               │   Bar    │               │   Menu   │
    └──────────┘               └──────────┘               └──────────┘
          │                           ▲                         │
          │                           │                         │
          ▼                           │                         ▼
    ┌──────────┐               ┌──────────┐               ┌──────────┐
    │  P1015   │               │  P1020   │               │  P1050   │
    │Selection │──────────────►│Clickable │               │  Remove  │
    │   Bar    │               │Compartmnt│               │ Buttons  │
    └──────────┘               └──────────┘               └──────────┘
          │                           │
          │                           │
          ▼                           ▼
    ┌─────────────────────────────────────┐
    │              P1040                  │
    │       Dependency Dialog             │
    └─────────────────────────────────────┘
```

### Implementation Order (Critical Path)

| Phase | Task                         | Depends On   | Blockers                    |
| ----- | ---------------------------- | ------------ | --------------------------- |
| 1     | P1030 Row Selection          | —            | Foundation for multi-select |
| 2a    | P1015 Selection Bar          | P1030        | Consumes selection state    |
| 2b    | P1010 Action Bar             | —            | Can parallel with P1015     |
| 3     | P1020 Clickable Compartments | P1010        | Sends events to Action Bar  |
| 4     | P1040 Dependency Dialog      | P1020, P1015 | Called by both              |
| 5     | P1050 Remove Action Buttons  | P1020, P1060 | After replacements work     |
| 6     | P1060 Ellipsis Menu          | P1050        | Receives Test from P1050    |

### State Management Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Global State Sources                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  hostStore (existing)        │  Alpine.store('selection')  │  Alpine.store  │
│  ─────────────────────       │  ──────────────────────────  │  ('actionBar') │
│  • Host online/offline       │  • selected: string[]       │  • state       │
│  • pendingCommand            │  • toggle(id)               │  • action      │
│  • updateStatus              │  • selectAll()              │  • result      │
│  • metrics                   │  • selectNone()             │  • timers      │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
              ┌──────────┐     ┌──────────┐     ┌──────────┐
              │ Selection │     │  Action  │     │   Host   │
              │   Bar    │     │   Bar    │     │   Rows   │
              └──────────┘     └──────────┘     └──────────┘
```

### Event Flow

```
User Hovers Compartment
         │
         ▼
┌─────────────────────┐    300ms debounce    ┌─────────────────────┐
│  onCompartmentHover │ ───────────────────► │  action-preview     │
│  (P1020)            │                      │  CustomEvent        │
└─────────────────────┘                      └─────────────────────┘
                                                       │
                                                       ▼
                                             ┌─────────────────────┐
                                             │  ActionBar.show     │
                                             │  Preview (P1010)    │
                                             └─────────────────────┘

User Clicks "DO NOW"
         │
         ▼
┌─────────────────────┐                      ┌─────────────────────┐
│  ActionBar.execute  │ ───────────────────► │  checkDependencies  │
│  (P1010)            │                      │  (P1040)            │
└─────────────────────┘                      └─────────────────────┘
         │                                             │
         │ (if no deps)                                │ (if deps needed)
         ▼                                             ▼
┌─────────────────────┐                      ┌─────────────────────┐
│  sendCommand()      │                      │  show-option-dialog │
│  (existing)         │                      │  CustomEvent        │
└─────────────────────┘                      └─────────────────────┘
         │
         ▼
┌─────────────────────┐    WebSocket         ┌─────────────────────┐
│  command_queued     │ ◄─────────────────── │  Dashboard Backend  │
│  (existing WS msg)  │                      │                     │
└─────────────────────┘                      └─────────────────────┘
         │
         ▼
┌─────────────────────┐
│  command-start      │
│  CustomEvent        │
└─────────────────────┘
         │
         ▼
┌─────────────────────┐
│  ActionBar.show     │
│  Progress (P1010)   │
└─────────────────────┘
```

---

## Design Specification

### 1. Table Layout

**Before:**

```
│ Host │ Type │ Loc │ Metrics │ Update │ Tests │ Seen │ Actions          │ ⋮ │
│      │      │     │         │[G][L][S]│       │      │[Pull][Switch][Test]│   │
```

**After:**

```
│ ☐ │ Host │ Type │ Loc │ Metrics │ Update │ Tests │ Seen │ ⋮ │
│   │      │      │     │         │[G][L][S]│       │      │   │
```

**Changes:**

| Element         | Before                   | After              |
| --------------- | ------------------------ | ------------------ |
| Checkbox column | None                     | First column, 40px |
| Actions column  | Pull/Switch/Test buttons | Removed            |
| Compartments    | Display only             | Clickable buttons  |
| Ellipsis menu   | Limited items            | Expanded with Test |

### 2. Action Bar

**Position**: Header center, between logo and user menu.

**Dimensions**:

| Property  | Value          | Responsive |
| --------- | -------------- | ---------- |
| Min width | 320px          | —          |
| Max width | 480px          | —          |
| Height    | Auto (content) | —          |
| Padding   | 12px 16px      | —          |

**States**: See P1010 for complete state machine.

### 3. Selection Bar

**Position**: Below header, above table (sticky).

**Visibility**: Only when `selection.count > 0`.

**Content**: See P1015 for complete specification.

### 4. Compartment Actions

| Compartment | Status   | Indicator | Click Action | Preview Text                      |
| ----------- | -------- | --------- | ------------ | --------------------------------- |
| Git         | ok       | 🟢        | Refresh      | "Check for updates"               |
| Git         | outdated | 🟡        | `pull`       | "Fetch latest code from GitHub"   |
| Git         | error    | 🔴        | Toast        | "Git check failed: {message}"     |
| Lock        | ok       | 🟢        | Refresh      | "Check dependencies"              |
| Lock        | outdated | 🟡        | Info toast   | "Update flake.lock via GitHub PR" |
| Lock        | error    | 🔴        | `switch`     | "Update agent to latest version"  |
| System      | ok       | 🟢        | Refresh      | "Check system status"             |
| System      | outdated | 🟡        | `switch`     | "Apply configuration"             |
| System      | error    | 🔴        | Toast        | "System check failed: {message}"  |

### 5. Keyboard Navigation

| Key         | Context     | Action                      |
| ----------- | ----------- | --------------------------- |
| Tab         | Table       | Navigate focusable elements |
| Enter       | Compartment | Execute action              |
| Space       | Checkbox    | Toggle selection            |
| Escape      | Dialog      | Close                       |
| Escape      | Selection   | Clear all                   |
| Ctrl/Cmd+A  | Table       | Select all                  |
| Shift+Click | Checkbox    | Range select                |

> **Note**: Full accessibility (ARIA, screen readers, motion preferences) deferred to [P8500](./P8500-accessibility-audit.md).

---

## Rollback Plan

If P1000 causes regressions:

### Immediate Rollback (< 5 min)

```bash
# Revert to previous commit
git revert HEAD~{n}  # where n = number of P1000 commits

# Redeploy
ssh mba@cs1.barta.cm -p 2222
cd ~/docker && docker compose build --no-cache nixfleet && docker compose up -d nixfleet
```

### Partial Rollback

Each sub-item is designed to be independently revertable:

| Component              | Revert Method                     | Side Effects             |
| ---------------------- | --------------------------------- | ------------------------ |
| Action Bar             | Remove `@ActionBar()` from header | None                     |
| Selection Bar          | Remove `@SelectionBar()`          | Bulk actions unavailable |
| Clickable Compartments | Revert to `<span>`                | Original behavior        |
| Row Selection          | Remove checkbox column            | Selection Bar breaks     |

### Feature Flags (Future)

Consider implementing feature flags for gradual rollout:

```go
// In config
type Features struct {
    ActionBar       bool `env:"FEATURE_ACTION_BAR" default:"true"`
    SelectionBar    bool `env:"FEATURE_SELECTION_BAR" default:"true"`
    ClickableComps  bool `env:"FEATURE_CLICKABLE_COMPARTMENTS" default:"true"`
}
```

---

## Implementation Sub-Items

| ID    | Item                   | Est. | Dependencies | Critical Path |
| ----- | ---------------------- | ---- | ------------ | ------------- |
| P1010 | Action Bar             | 5h   | —            | Yes           |
| P1015 | Selection Bar          | 3h   | P1030        | Yes           |
| P1020 | Clickable Compartments | 4h   | P1010, P1040 | Yes           |
| P1030 | Row Selection          | 4h   | —            | Yes           |
| P1040 | Dependency Dialog      | 3h   | —            | Yes           |
| P1050 | Remove Action Buttons  | 1h   | P1020, P1060 | No            |
| P1060 | Ellipsis Menu          | 2h   | —            | No            |

**Total**: ~22 hours (4-5 days with testing)

---

## Removed from Scope

| Original Item           | Reason                        | Alternative    |
| ----------------------- | ----------------------------- | -------------- |
| launchctl kickstart     | Agent restart works correctly | —              |
| Agent self-restart code | Works correctly               | —              |
| Activation hooks        | Not needed                    | —              |
| Mobile touch UX         | Separate ticket               | P6800          |
| Drag-and-drop reorder   | Low priority                  | Future backlog |

---

## Non-Functional Requirements

### Performance

| Metric                      | Target  | Measurement     |
| --------------------------- | ------- | --------------- |
| Action Bar render           | < 16ms  | Chrome DevTools |
| Compartment hover → preview | < 350ms | Timer in code   |
| Selection toggle            | < 8ms   | Chrome DevTools |
| Bulk action dispatch        | < 100ms | Network tab     |

### Accessibility

Basic keyboard navigation and focus visibility included. Full WCAG 2.1 AA compliance deferred to [P8500](./P8500-accessibility-audit.md).

### Browser Support

| Browser | Version | Notes          |
| ------- | ------- | -------------- |
| Chrome  | 90+     | Primary        |
| Firefox | 88+     | Secondary      |
| Safari  | 14+     | macOS users    |
| Edge    | 90+     | Chromium-based |

---

## Testing Strategy

### Unit Tests

| Component         | Test Cases                                       |
| ----------------- | ------------------------------------------------ |
| Action Bar        | State transitions, timer cleanup, event handling |
| Selection Store   | Add/remove, select all, online count             |
| Compartment Logic | Action mapping, status handling                  |
| Dependency Check  | Single/multi host, chain execution               |

### Integration Tests

| Scenario           | Steps                           | Expected                                 |
| ------------------ | ------------------------------- | ---------------------------------------- |
| Single host pull   | Hover Git → Click DO NOW        | Pull executes, Action Bar shows progress |
| Multi-host switch  | Select 3 → Click Switch All     | All 3 switch, selection cleared          |
| Dependency warning | Git outdated → Click Switch     | Dialog appears with options              |
| Offline handling   | Select host → Host goes offline | Selection kept, button disabled          |

### Manual Testing Checklist

See "Acceptance Criteria" section below.

---

## Acceptance Criteria

### Action Bar (P1010)

- [ ] Visible in header, horizontally centered
- [ ] IDLE: Shows "Hover a status to see actions" (muted, italic)
- [ ] PREVIEW: Shows action name, description, host, DO NOW button
- [ ] PREVIEW: 300ms debounce prevents flickering between compartments
- [ ] IN_PROGRESS: Shows spinner, action name, STOP button
- [ ] COMPLETE: Shows ✓/✗, result message, auto-clears after 2s
- [ ] Timer cleanup: No memory leaks on rapid state changes
- [ ] Keyboard: Enter on focused compartment updates Action Bar

### Selection Bar (P1015)

- [ ] Hidden when no hosts selected
- [ ] Shows with slide animation when 1+ hosts selected
- [ ] Displays: "{n} hosts selected" or "{n} selected ({m} online)"
- [ ] Pull/Switch/Test All buttons: Enabled only when online count > 0
- [ ] Clear button: Always enabled, clears selection
- [ ] Responsive: Stacks on narrow viewports

### Clickable Compartments (P1020)

- [ ] All 3 compartments are `<button>` elements
- [ ] Hover: pointer cursor, scale(1.1), dispatches action-preview
- [ ] Click: Executes appropriate action per status table
- [ ] Green status: Triggers refresh, not command
- [ ] Lock yellow: Shows info toast, no command
- [ ] Lock red (agent): Triggers switch
- [ ] Focus visible: 2px outline on keyboard focus
- [ ] Touch devices: Works without hover preview

### Row Selection (P1030)

- [ ] Checkbox column is first column (40px)
- [ ] Checkboxes hidden by default, visible on row hover
- [ ] Checkboxes always visible when row is selected
- [ ] Click checkbox: Toggles selection
- [ ] Click row background: Toggles selection
- [ ] Click text/compartment/menu: Does NOT toggle
- [ ] Shift+Click: Range selection
- [ ] Header checkbox: Select all / deselect all
- [ ] Header checkbox: Indeterminate when partially selected
- [ ] Ctrl/Cmd+A: Select all (when not in input)
- [ ] Escape: Clear selection

### Dependency Dialog (P1040)

- [ ] Appears when Switch clicked with Git = outdated
- [ ] Single host: 4 buttons (Cancel, Pull Only, Switch Anyway, Pull + Switch)
- [ ] Multi host: Shows host list, 3 buttons
- [ ] Pull + Switch: Executes sequentially, shows progress
- [ ] Chain error: Shows error, allows retry
- [ ] Cancel mid-chain: Stops pending commands
- [ ] Escape/click outside: Closes dialog
- [ ] Focus trap: Tab cycles within dialog
- [ ] Autofocus: Primary button focused on open

### Remove Action Buttons (P1050)

- [ ] Pull/Switch/Test buttons removed from table
- [ ] Actions column header removed
- [ ] Ellipsis menu still works
- [ ] Refresh button moved to minimal actions cell
- [ ] No console errors

### Ellipsis Menu (P1060)

- [ ] Test at top of menu
- [ ] All icons are SVG (no emojis)
- [ ] Groups separated by dividers
- [ ] Test disabled when host offline
- [ ] Stop only shows when command running
- [ ] Copy Hostname: Copies to clipboard, shows toast
- [ ] SSH Command: Copies ssh command, shows toast
- [ ] Keyboard: Arrow keys navigate, Enter activates
- [ ] Focus management: Focus returns to trigger on close

---

## QA Checklist

### Pre-Merge

- [ ] All acceptance criteria checked
- [ ] No TypeScript/ESLint errors
- [ ] No console errors in browser
- [ ] Tested in Chrome, Firefox, Safari
- [ ] Tested at 375px, 768px, 1920px, 2560px widths
- [ ] Keyboard-only navigation works
- [ ] Memory profiler shows no leaks (5 min test)

### Post-Deploy

- [ ] Production dashboard loads without errors
- [ ] WebSocket connection stable
- [ ] All hosts appear and update correctly
- [ ] Execute pull on test host
- [ ] Execute switch on test host
- [ ] Bulk select and pull on multiple hosts
- [ ] Verify Action Bar states cycle correctly

---

## Related

- [P1010](./P1010-action-bar-component.md) — Action Bar implementation
- [P1015](./P1015-selection-bar.md) — Selection Bar
- [P1020](./P1020-clickable-compartments.md) — Clickable compartments
- [P1030](./P1030-row-selection.md) — Row selection & multi-select
- [P1040](./P1040-option-dialog.md) — Dependency warning dialog
- [P1050](./P1050-remove-action-buttons.md) — Remove old buttons
- [P1060](./P1060-ellipsis-menu-cleanup.md) — Ellipsis menu reorganization
- [P6800](./P6800-mobile-card-view.md) — Mobile-specific card view
- [P8500](./P8500-accessibility-audit.md) — Full accessibility (ARIA, screen readers)
