# P5100 - Queue Commands for Offline Hosts

**Created**: 2025-12-14  
**Updated**: 2025-12-15  
**Priority**: P5100 (Medium)  
**Status**: Backlog  
**Depends on**: P4000-P4400 (Core rewrite)

---

## Overview

Queue commands for offline hosts; execute when they come back online.

---

## Design

### Database

```sql
ALTER TABLE hosts ADD COLUMN queued_commands TEXT;  -- JSON array
ALTER TABLE hosts ADD COLUMN queued_at TEXT;
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
- **P5300**: Automated updates may queue for offline hosts
