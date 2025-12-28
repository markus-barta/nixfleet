# P3300: Logs on Page Load

**Priority**: P3300 (🔴 Critical Path - Sprint 1)  
**Status**: ✅ Done (Needs Manual Testing)  
**Effort**: Small-Medium (1-2 days) → Actual: 2 hours  
**Implements**: [CORE-003](../spec/CORE-003-state-store.md), [CORE-004](../spec/CORE-004-state-sync.md)  
**Depends on**: P3100, P3200 (State Persistence + Sync) - ✅ Done  
**Updated**: 2025-12-28 (Implemented - ready for testing)

---

## User Story

**As a** user  
**I want** to see recent logs when I open the dashboard  
**So that** I don't lose context after a page refresh

---

## Scope

### API Endpoints

```go
// Recent system/audit events
GET /api/event-log?limit=100

// Per-host command output
GET /api/hosts/{id}/output?lines=500
```

### Include in Init Payload

When browser connects, include recent data:

```go
type InitPayload struct {
    Hosts     []Host           `json:"hosts"`
    Commands  []Command        `json:"commands"`   // Active/recent
    Pipelines []Pipeline       `json:"pipelines"`  // Active/recent
    Events    []EventLogEntry  `json:"events"`     // Last 100
    Version   uint64           `json:"version"`
}
```

### Frontend Restoration

On `init` message:

```javascript
function handleInit(payload) {
  // Restore host list
  renderHosts(payload.hosts);

  // Restore system log
  payload.events.forEach((event) => {
    addSystemLogEntry(event);
  });

  // Restore active commands
  payload.commands.forEach((cmd) => {
    if (cmd.status === "EXECUTING") {
      showCommandProgress(cmd);
    }
  });

  stateVersion = payload.version;
}
```

### Host Output Tabs

When user opens a host output tab:

```javascript
async function openHostTab(hostId) {
  // Fetch recent output from server
  const resp = await fetch(`/api/hosts/${hostId}/output?lines=500`);
  const output = await resp.text();

  // Populate tab with historical output
  tabContent.innerHTML = output;

  // Continue receiving live updates via WebSocket
}
```

---

## Acceptance Criteria

- [x] `/api/event-log` returns recent events (already existed as `/api/events`)
- [x] `/api/hosts/{id}/output` returns recent command output
- [x] `init` payload includes events (already implemented in State Sync)
- [x] System log tab populated on page load
- [x] Host output tabs restored from server
- [ ] Test: refresh page → logs still visible (needs manual browser test)
- [ ] Test: close/reopen tab → output restored (needs manual browser test)

---

## Implementation Summary (2025-12-28)

### Backend

- Added `GET /api/hosts/{hostID}/output?lines=N` endpoint
- Added `LogStore.GetLatestLogContent()` - reads most recent log file
- Added `LogStore.GetCurrentCommandOutput()` - reads active command output
- `/api/events` endpoint already existed

### Frontend

- Added `restoreEventLog()` - fetches last 50 events on page load
- Added `restoreHostOutput()` - fetches last 500 lines when tab opens
- Modified `switchTab()` - triggers restore on first tab open
- Added `getCategoryIcon()` - maps event categories to icons

### Flow

1. **Page Load**: `restoreEventLog()` → populates system log
2. **Open Host Tab**: `switchTab()` → `restoreHostOutput()` → populates output
3. **Live Updates**: WebSocket continues to stream new output

---

## Related

- **CORE-003**: State Store (event_log table) - ✅ Used
- **CORE-004**: State Sync (init payload) - ✅ Events included
- **P3200**: State Sync Protocol (sends init) - ✅ Done
