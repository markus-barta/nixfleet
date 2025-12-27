# P3300: Logs on Page Load

**Priority**: P3300 (High - v3 Phase 4)  
**Status**: Backlog  
**Effort**: Small-Medium (1-2 days)  
**Implements**: [CORE-003](../spec/CORE-003-state-store.md), [CORE-004](../spec/CORE-004-state-sync.md)  
**Depends on**: P3100, P3200 (State Persistence + Sync)

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

- [ ] `/api/event-log` returns recent events
- [ ] `/api/hosts/{id}/output` returns recent command output
- [ ] `init` payload includes events
- [ ] System log tab populated on page load
- [ ] Host output tabs restored from server
- [ ] Test: refresh page → logs still visible
- [ ] Test: close/reopen tab → output restored

---

## Related

- **CORE-003**: State Store (event_log table)
- **CORE-004**: State Sync (init payload)
- **P3200**: State Sync Protocol (sends init)
