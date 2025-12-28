# P3400: Frontend Simplification

**Priority**: P5100 (🟢 After Compartment Epic)  
**Status**: Backlog  
**Effort**: Medium (2-3 days)  
**Depends on**: P3010, P3200 (Op Engine + State Sync) - ✅ Done, Compartment Epic (P3700-P4800)  
**Updated**: 2025-12-28 (Moved after compartment work - do when foundation is solid)

---

## User Story

**As a** developer  
**I want** the frontend to be a thin dispatcher with no business logic  
**So that** all logic is testable in Go and the UI is just a view layer

---

## Scope

### Remove Business Logic from JavaScript

Currently, JavaScript contains:

- Validation logic (is host online? can run command?)
- State derivation (what actions are available?)
- Command sequencing (pull then switch)
- Error handling decisions

All of this moves to Go. JavaScript only:

- Renders state from server
- Dispatches user actions
- Applies deltas from WebSocket

### Single Dispatch Function

Replace scattered action handlers with one dispatcher:

```javascript
// Before: multiple action handlers
function handlePull(hostId) { ... }
function handleSwitch(hostId) { ... }
function handleDoAll(hostIds) { ... }

// After: single dispatcher
function dispatch(action, params) {
    ws.send(JSON.stringify({
        type: "dispatch",
        action: action,    // "op:pull", "pipeline:do-all"
        params: params     // { hostId: "hsb0" } or { hostIds: ["hsb0", "hsb1"] }
    }));
}

// Usage
dispatch("op:pull", { hostId: "hsb0" });
dispatch("pipeline:do-all", { hostIds: ["hsb0", "hsb1"] });
```

### Server-Driven UI State

Server tells client what's possible:

```json
{
  "hosts": [
    {
      "id": "hsb0",
      "availableOps": ["pull", "switch", "test"],
      "availablePipelines": ["do-all"],
      "blockedReason": null
    },
    {
      "id": "hsb1",
      "availableOps": [],
      "availablePipelines": [],
      "blockedReason": "Command in progress"
    }
  ]
}
```

Frontend just renders what server says:

```javascript
function renderHostActions(host) {
  if (host.blockedReason) {
    showBlocked(host.blockedReason);
    return;
  }

  host.availableOps.forEach((op) => {
    addActionButton(op, () => dispatch(`op:${op}`, { hostId: host.id }));
  });
}
```

### Remove Client-Side State Derivation

| Before (JS)                                  | After (Server)                   |
| -------------------------------------------- | -------------------------------- |
| `if (host.online && !host.pending)`          | `host.availableOps.includes(op)` |
| `host.status === 'idle' ? 'Switch' : 'Stop'` | Server sends correct label       |
| `host.systemDirty && host.lockOk`            | Server calculates, sends flag    |

---

## Acceptance Criteria

- [ ] All validation logic in Go
- [ ] Single `dispatch()` function in JS
- [ ] Server sends `availableOps`/`availablePipelines` per host
- [ ] Frontend renders what server says (no derivation)
- [ ] No business logic in JavaScript
- [ ] All existing UI flows work correctly
- [ ] Unit tests for available actions in Go

---

## Migration Notes

This is a significant refactor. Approach:

1. Add `availableOps` field to host state (server-side)
2. Keep old JS logic, but log warnings if it disagrees with server
3. Gradually replace JS checks with server state
4. Remove old JS logic once all flows work

---

## Related

- **P3010**: Op Engine (defines available ops)
- **P3200**: State Sync (sends available ops in state)
