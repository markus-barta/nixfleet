# 2025-12-12 - Queue commands for offline hosts

**Created**: 2025-12-12
**Priority**: Medium
**Status**: Backlog

---

## Goal

Allow users to queue commands for offline hosts. When the host comes back online, the NixFleet backend should automatically trigger the queued action(s).

---

## Description

Currently, action buttons (Pull, Switch, Test) only work when a host is online. If a host is offline, users cannot schedule actions to run when it comes back online.

This feature adds a "Queue command" option in the ellipsis menu for each host, allowing users to:
- Queue one or more actions (pull, update/switch, test)
- Queue all three actions in sequence
- Have the backend automatically execute queued commands when the host reconnects

---

## Scope

Applies to: dashboard, backend, agent

---

## Design

### UI Changes

**Ellipsis Menu Addition:**
- Add "Queue command..." option in the per-host ellipsis dropdown
- Opens a modal/dialog with checkboxes for:
  - [ ] Pull
  - [ ] Update/Switch
  - [ ] Test
- "Queue" button to submit
- "Cancel" button to close

**Visual Indicators:**
- Show queued command status in host row (e.g., badge or icon)
- Display queued actions in Comments column or status area
- Clear indicator when host is offline with queued commands

### Backend Changes

**Database Schema:**
```sql
-- Add queued_commands column to hosts table
ALTER TABLE hosts ADD COLUMN queued_commands TEXT; -- JSON array: ["pull", "switch", "test"]
ALTER TABLE hosts ADD COLUMN queued_at TEXT; -- Timestamp when queued
```

**New Endpoint:**
```python
@app.post("/api/hosts/{host_id}/queue-command")
async def queue_command(host_id: str, commands: List[str]):
    """Queue one or more commands for execution when host comes online."""
    # Store queued_commands and queued_at
    # Broadcast SSE event: command_queued
```

**Command Execution Logic:**
- On agent registration/poll (when host comes online):
  - Check if `queued_commands` is set and not empty
  - If host was offline and now online, execute queued commands in order
  - Clear `queued_commands` after execution starts
  - Broadcast SSE events for each queued command execution

**Command Order:**
1. Pull (if queued)
2. Switch/Update (if queued)
3. Test (if queued)

### Agent Changes

No changes required - agent continues to poll and execute commands as normal. The backend handles queuing and execution timing.

---

## Acceptance Criteria

- [ ] "Queue command..." option appears in ellipsis menu for all hosts
- [ ] Modal allows selecting one or more actions (pull, switch, test)
- [ ] Can queue all three actions at once
- [ ] Queued commands are stored in database
- [ ] Queued commands persist across backend restarts
- [ ] When offline host comes online, queued commands execute automatically
- [ ] Commands execute in correct order: pull → switch → test
- [ ] Visual indicator shows when host has queued commands
- [ ] Queued commands clear after execution starts
- [ ] SSE events broadcast when commands are queued and executed
- [ ] Works correctly if multiple commands are queued
- [ ] Works correctly if host goes offline again before executing queued commands

---

## Edge Cases

- **Host goes offline again**: Queued commands remain until host comes back online
- **Multiple queue operations**: Latest queue overwrites previous queue (or append? - TBD)
- **Backend restart**: Queued commands persist in database
- **Host never comes back online**: Queued commands remain indefinitely (consider timeout? - TBD)
- **Command fails**: Next queued command still executes (or stop? - TBD)

---

## Test Plan

### Manual Test

1. **Queue command for offline host:**
   - Take a host offline (stop agent or disconnect network)
   - Click ellipsis menu → "Queue command..."
   - Select "Pull" and "Switch"
   - Click "Queue"
   - Verify visual indicator shows queued commands
   - Verify database has `queued_commands` set

2. **Execute queued commands:**
   - Bring host back online (start agent or reconnect network)
   - Verify agent polls and receives commands
   - Verify commands execute in order: pull → switch
   - Verify queued commands clear after execution starts
   - Verify SSE events broadcast correctly

3. **Queue all three actions:**
   - Queue pull, switch, and test for offline host
   - Bring host online
   - Verify all three execute in sequence

4. **Visual indicators:**
   - Verify queued command badge/icon appears in host row
   - Verify queued actions visible in Comments/status area
   - Verify indicators clear after execution

### Automated Test

```bash
# Test queue endpoint
curl -X POST http://localhost:8000/api/hosts/test-host/queue-command \
  -H "Content-Type: application/json" \
  -d '{"commands": ["pull", "switch"]}'

# Verify database
sqlite3 nixfleet.db "SELECT queued_commands, queued_at FROM hosts WHERE id='test-host';"

# Simulate host coming online (agent registration)
curl -X POST http://localhost:8000/api/hosts/test-host/register \
  -H "Content-Type: application/json" \
  -d '{"hostname": "test-host", "status": "ok"}'

# Verify commands executed and queued_commands cleared
sqlite3 nixfleet.db "SELECT queued_commands FROM hosts WHERE id='test-host';"
```

---

## Implementation Order

1. **Backend**: Add database schema (queued_commands, queued_at columns)
2. **Backend**: Add `/api/hosts/{id}/queue-command` endpoint
3. **Backend**: Add logic to execute queued commands on host reconnect
4. **Frontend**: Add "Queue command..." to ellipsis menu
5. **Frontend**: Create queue command modal/dialog
6. **Frontend**: Add visual indicators for queued commands
7. **Backend**: Add SSE events for queue operations
8. **Testing**: Manual and automated tests

---

## References

- Dashboard template: `app/templates/dashboard.html`
- Backend API: `app/main.py`
- Agent: `agent/nixfleet-agent.sh`
- Related: `2025-12-11-nixfleet-action-button-locking.md` (button locking during commands)

