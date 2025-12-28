# P6700: Queue Commands for Offline Hosts

**Created**: 2025-12-14  
**Updated**: 2025-12-28  
**Priority**: P8100 (⚪ Low Priority - Future)  
**Status**: Backlog  
**Related Specs**: [CORE-003](../spec/CORE-003-state-store.md) (commands table)  
**Depends on**: P3100 (State Persistence) - ✅ Done

**Note**: Priority lowered - edge case feature, not critical

---

## Overview

Queue commands for offline hosts; execute when they come back online.

---

## Design

### Database

Uses the `commands` table from CORE-003 with status `QUEUED`:

```sql
-- Queue a command for an offline host
INSERT INTO commands (id, host_id, op, status)
VALUES (uuid(), 'hsb0', 'pull', 'QUEUED');

-- On agent connect, find queued commands
SELECT * FROM commands WHERE host_id = ? AND status = 'QUEUED' ORDER BY created_at;
```

### UI

Ellipsis menu → "Queue command..." → Modal with checkboxes:

- [ ] Pull
- [ ] Switch
- [ ] Test

### Execution

When agent connects via WebSocket:

1. Check `queued_commands`
2. If non-empty, send commands in order
3. Clear `queued_commands`

Order: pull → switch → test

---

## Acceptance Criteria

- [ ] Queue UI in ellipsis menu
- [ ] Commands stored in database
- [ ] Commands execute on reconnect
- [ ] Visual indicator for queued hosts
- [ ] Commands persist across dashboard restart

---

## Related

- Post-MVP feature
- **P5000**: Update status shows last known state for offline hosts
- **P4300**: Automated updates may queue for offline hosts
