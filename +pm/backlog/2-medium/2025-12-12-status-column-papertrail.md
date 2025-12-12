# Status Column Papertrail

**Created**: 2025-12-12
**Priority**: Medium
**Scope**: Dashboard UI + Backend (in-memory storage)

---

## Overview

Transform the status column from a single-line text display into a scrollable, expandable papertrail showing timestamped history of status changes.

## Current State

- Single `comment` field in DB (one line)
- Status cell shows: pending command badge OR test badge OR icon+comment
- Max-width 200px, truncated with ellipsis
- No history preserved

## Target State

```
┌─────────────────────────────────────────────┐
│ Status (collapsed - default)                │
├─────────────────────────────────────────────┤
│ 14:32 ✓ Switch complete                [▼]  │  ← tiny expand button
│ ▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒ (scroll indicator)    │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ Status (expanded - 10x height)              │
├─────────────────────────────────────────────┤
│ 14:32 ✓ Switch complete                [▲]  │
│ 14:31 ⏳ Switching...                       │
│ 14:30 ✓ Pull complete                       │
│ 14:30 ⏳ Pulling...                         │
│ 14:15 ✓ Tests: 8/8 passed                   │
│ 14:14 🧪 Testing 8/8                        │
│ 14:14 🧪 Testing 7/8                        │
│ ...                                         │
│ 09:45 ✓ Switch complete                     │
│ ▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒ (scrollable)          │
└─────────────────────────────────────────────┘
```

---

## Requirements

### Backend

1. **In-memory history store**
   - Dict keyed by host_id → list of status entries
   - Each entry: `{ timestamp: ISO8601, icon: str, message: str }`
   - NOT persisted to DB (lost on restart)

2. **History retention**
   - Time-based: configurable via `NIXFLEET_STATUS_HISTORY_DAYS` (default: 30)
   - Prune old entries on each append

3. **Events that create history entries**
   - ❌ Heartbeats (too noisy)
   - ✅ Command queued ("⏳ Pulling...")
   - ✅ Command started (agent picked it up)
   - ✅ Command completed ("✓ Switch complete")
   - ✅ Command failed ("✗ Switch failed: <truncated error>")
   - ✅ Test progress ("🧪 Testing 3/8")
   - ✅ Test result ("✓ Tests: 8/8 passed" or "✗ Tests: 5/8 failed")

4. **API changes**
   - `GET /api/hosts` → include `status_history: [...]` (last N entries)
   - SSE events → include new history entry when status changes

### Frontend

1. **Collapsed state (default)**
   - Show 1-2 lines max (most recent entry)
   - Tiny font (0.65rem or smaller)
   - Scroll indicator if more entries exist
   - Tiny expand button (▼) on the right

2. **Expanded state**
   - 10x normal row height
   - Scrollable container
   - All entries visible (newest on top)
   - Tiny collapse button (▲)

3. **Expand/collapse behavior**
   - Only ONE row expanded at a time
   - Clicking expand on row B collapses row A
   - Smooth transition animation

4. **Timestamps**
   - Format: `HH:MM` (24h, local time)
   - Full datetime on hover (tooltip)

5. **SSE live updates**
   - New entries animate in (prepend with slide-down)
   - Auto-scroll to top if already at top

---

## Acceptance Criteria

- [ ] Status column shows scrollable history instead of single line
- [ ] Collapsed by default, shows 1-2 lines with scroll indicator
- [ ] Expand button expands to 10x height
- [ ] Only one row can be expanded at a time
- [ ] New status entries appear via SSE with animation
- [ ] Timestamps in HH:MM format, full datetime on hover
- [ ] History entries include: command start/complete/fail, test progress/results
- [ ] History excludes: heartbeats
- [ ] History retention configurable via env var (default 30 days)
- [ ] History is in-memory only (acceptable to lose on restart)

---

## Technical Notes

### Files to modify

| File | Changes |
|------|---------|
| `app/main.py` | In-memory history dict, append on status change, include in API responses |
| `app/templates/dashboard.html` | New status cell structure, CSS for scroll/expand, JS for toggle |

### In-memory structure

```python
# In main.py
status_history: dict[str, list[dict]] = {}
# Example:
# {
#   "hsb1": [
#     {"timestamp": "2025-12-12T14:32:05Z", "icon": "✓", "message": "Switch complete"},
#     {"timestamp": "2025-12-12T14:31:42Z", "icon": "⏳", "message": "Switching..."},
#     ...
#   ]
# }
```

### CSS approach

```css
.status-cell {
  max-height: 2.5em;  /* collapsed */
  overflow-y: auto;
  font-size: 0.65rem;
  transition: max-height 0.3s ease;
}

.status-cell.expanded {
  max-height: 25em;  /* ~10x */
}

.status-entry {
  white-space: nowrap;
  padding: 0.1rem 0;
}

.status-timestamp {
  color: var(--fg-gutter);
  margin-right: 0.3rem;
}
```

---

## Out of Scope (Future)

- Persist history to DB (separate task)
- Filter history by type
- Search within history
- Export history
- Per-host history page with full details
