# Status Column Papertrail

**Created**: 2025-12-12
**Priority**: Medium
**Scope**: Dashboard UI + Backend (in-memory storage)

---

## Overview

Transform the status column from a single-line text display into a scrollable, expandable papertrail showing timestamped history of status changes.

**Dual-history approach:**

1. **Status history** — Short, truncated messages for UI display in status column
2. **Full log** — Complete command output (no truncation) downloadable via ellipsis menu

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

1. **Dual in-memory history stores**

   | Store | Purpose | Message size | Used by |
   |-------|---------|--------------|---------|
   | `status_history` | UI display | Truncated (≤100 chars) | Status column |
   | `full_log` | Download | Complete output | Ellipsis menu → Download Logs |

   - Both keyed by host_id → list of entries
   - NOT persisted to DB (lost on restart)

2. **History entry structure**

   ```python
   # Status history (for UI)
   {"timestamp": "2025-12-12T14:32:05Z", "icon": "✓", "message": "Switch complete"}
   
   # Full log (for download)
   {"timestamp": "2025-12-12T14:32:05Z", "type": "switch_complete", "message": "Switch complete", "output": "<full multi-line output>"}
   ```

3. **History retention**
   - Time-based: configurable via `NIXFLEET_STATUS_HISTORY_DAYS` (default: 30)
   - Prune old entries on each append
   - Same retention for both stores

4. **Events that create history entries**
   - ❌ Heartbeats (too noisy)
   - ✅ Command queued ("⏳ Pulling...")
   - ✅ Command started (agent picked it up)
   - ✅ Command completed ("✓ Switch complete") + full output in log
   - ✅ Command failed ("✗ Switch failed: <truncated>") + full error in log
   - ✅ Test progress ("🧪 Testing 3/8")
   - ✅ Test result ("✓ Tests: 8/8 passed") + full test output in log

5. **API changes**
   - `GET /api/hosts` → include `status_history: [...]` (last N entries, truncated)
   - `GET /api/hosts/{id}/logs` → return full log for download (JSON or plain text)
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

6. **Download Logs (ellipsis menu)**
   - New menu item: "📥 Download Logs"
   - Calls `GET /api/hosts/{id}/logs`
   - Downloads as `{hostname}-logs-{date}.txt` or `.json`
   - Contains full output (not truncated)

---

## Acceptance Criteria

### Status Column UI

- [ ] Status column shows scrollable history instead of single line
- [ ] Collapsed by default, shows 1-2 lines with scroll indicator
- [ ] Expand button expands to 10x height
- [ ] Only one row can be expanded at a time
- [ ] New status entries appear via SSE with animation
- [ ] Timestamps in HH:MM format, full datetime on hover

### History Content

- [ ] History entries include: command start/complete/fail, test progress/results
- [ ] History excludes: heartbeats
- [ ] Status history has truncated messages (≤100 chars)
- [ ] Full log has complete output (no truncation)

### Download Logs

- [ ] Ellipsis menu has "Download Logs" option
- [ ] Downloads full log as text/JSON file
- [ ] Filename includes hostname and date

### Configuration

- [ ] History retention configurable via `NIXFLEET_STATUS_HISTORY_DAYS` (default 30 days)
- [ ] Both histories are in-memory only (acceptable to lose on restart)

---

## Technical Notes

### Files to modify

| File | Changes |
|------|---------|
| `app/main.py` | Dual in-memory dicts, append on status change, new `/logs` endpoint |
| `app/templates/dashboard.html` | Status cell structure, CSS for scroll/expand, JS for toggle, download menu item |

### In-memory structure

```python
# In main.py

# Status history (for UI - truncated messages)
status_history: dict[str, list[dict]] = {}
# Example:
# {
#   "hsb1": [
#     {"timestamp": "2025-12-12T14:32:05Z", "icon": "✓", "message": "Switch complete"},
#     {"timestamp": "2025-12-12T14:31:42Z", "icon": "⏳", "message": "Switching..."},
#   ]
# }

# Full log (for download - complete output)
full_log: dict[str, list[dict]] = {}
# Example:
# {
#   "hsb1": [
#     {
#       "timestamp": "2025-12-12T14:32:05Z",
#       "type": "switch_complete",
#       "message": "Switch complete",
#       "output": "building '/nix/store/...-nixos-system-hsb1-25.05...'\n..."
#     },
#   ]
# }

def append_history(host_id: str, icon: str, message: str, full_output: str = None, event_type: str = None):
    """Append to both histories. Truncates message for status_history."""
    now = datetime.utcnow().isoformat() + "Z"
    
    # Status history (truncated)
    truncated = message[:100] + "..." if len(message) > 100 else message
    status_history.setdefault(host_id, []).insert(0, {
        "timestamp": now, "icon": icon, "message": truncated
    })
    
    # Full log (complete)
    full_log.setdefault(host_id, []).insert(0, {
        "timestamp": now, "type": event_type or "unknown",
        "message": message, "output": full_output
    })
    
    # Prune old entries (older than HISTORY_DAYS)
    prune_history(host_id)
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
- Filter history by type (e.g., only errors)
- Search within history
- Per-host history page with full details (web UI viewer)
- Log streaming (live tail)
