# P4021 - Tabbed Output Panel Polish & Completion

## Parent

Continuation of **P4020** (Tabbed Output Panel) - completing remaining features.

## Status

**Complete** - All features implemented.

## Context

P4020 was implemented with core functionality (Phases 1-2 complete), but several features were deferred.

**All features now implemented:**

- ✅ Tab overflow dropdown for mobile/many hosts (FR-5)
- ✅ Resize panel by dragging (FR-4.2)
- ✅ Panel height persistence (FR-4.4)
- ✅ Status History section (FR-9, merged from P6600)
- ✅ Dual timestamp format (FR-6.7)
- ✅ Show Output menu option (FR-1.6/1.7)
- ✅ Keyboard shortcuts (Ctrl+1-9, Ctrl+W, Escape)

## Remaining Features

### High Priority

#### 1. Tab Overflow Dropdown (FR-5) - Mobile Critical

When tabs exceed available width, show "more tabs" dropdown instead of horizontal scroll.

| ID     | Requirement                                                           |
| ------ | --------------------------------------------------------------------- |
| FR-5.1 | When tabs exceed available width, show "more tabs" dropdown           |
| FR-5.2 | Dropdown lists hidden tabs with hostname and state indicator          |
| FR-5.3 | Clicking dropdown item switches to that tab (moves to visible area)   |
| FR-5.4 | On mobile (<640px), only active tab visible + dropdown for all others |

**Effort**: Medium (4-6 hours)

#### 2. Status History in Host Tabs (FR-9)

Each host tab displays a compact status history summary at the top, showing recent state transitions from P2800.

| ID     | Requirement                                                      |
| ------ | ---------------------------------------------------------------- |
| FR-9.1 | Status history appears at top of host tab (above command output) |
| FR-9.2 | Shows last N status entries (default: 10)                        |
| FR-9.3 | Each entry shows: timestamp (HH:MM), icon, truncated message     |
| FR-9.4 | Entries scrollable if history exceeds visible area               |
| FR-9.5 | Most recent entry highlighted                                    |
| FR-9.6 | Error entries shown in red                                       |
| FR-9.7 | Status history updates in real-time via WebSocket                |
| FR-9.8 | History persists across tab close/reopen (session storage)       |

**Dependency**: P2800 must be emitting `state_machine_log` WebSocket messages.

**Effort**: High (6-8 hours)

### Medium Priority

#### 3. Panel Resize by Dragging (FR-4.2)

Allow user to resize output panel height by dragging a handle.

**Effort**: Medium (3-4 hours)

#### 4. Panel Height Persistence (FR-4.4)

Save panel height to localStorage and restore on page load.

**Effort**: Low (1 hour)

### Low Priority

#### 5. Dual Timestamp Format (FR-6.7)

System Log entries should show both absolute and relative time:
`14:23:05 (2 min ago)`

**Effort**: Low (1-2 hours)

#### 6. "Show Output" Menu Option (FR-1.6/1.7)

Add "Show Output" to host ellipsis menu to open/reopen output tab.

**Effort**: Low (1 hour)

#### 7. Keyboard Shortcuts

- `Ctrl+1-9` to switch tabs
- `Ctrl+W` to close active tab
- `Ctrl+Shift+L` to focus System Log

**Effort**: Low (2 hours)

## Implementation Order

1. Tab overflow dropdown (critical for mobile)
2. Status History section (requires P2800)
3. Resize by dragging
4. Height persistence
5. Polish items (dual timestamp, menu option, shortcuts)

## Effort Estimate

| Item                  | Effort     |
| --------------------- | ---------- |
| Tab overflow dropdown | 4-6 hours  |
| Status History (FR-9) | 6-8 hours  |
| Resize by dragging    | 3-4 hours  |
| Height persistence    | 1 hour     |
| Dual timestamp        | 1-2 hours  |
| Show Output menu      | 1 hour     |
| Keyboard shortcuts    | 2 hours    |
| **Total**             | **18-24h** |

## Dependencies

- **P2800**: Status History requires P2800 state machine to emit WebSocket messages
- P4020 (complete)

## Priority

**Low** - Core functionality works. These are UX polish items.

## Notes

The current implementation is production-ready for core use cases. These remaining features are nice-to-have polish that can be added incrementally.
