# P5100: Frontend Simplification

**Priority**: P5100 (🟢 After Compartment Epic)  
**Status**: 90% Done (Testing Needed)  
**Effort**: Small (1h remaining)  
**Depends on**: P3010, P3200 (Op Engine + State Sync) - ✅ Done  
**Updated**: 2025-12-28 (Implemented server-driven UI with availableOps)

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

- [x] All validation logic in Go
- [x] Single `dispatch()` function in JS (stateSync.dispatchOp)
- [x] Server sends `availableOps` per host
- [x] Frontend renders what server says (dropdown buttons use isOpAvailable)
- [x] No business logic in JavaScript (removed host.online checks)
- [ ] All existing UI flows work correctly (needs browser testing)
- [ ] Unit tests for available actions in Go

---

## Implementation Summary (2025-12-28)

### ✅ Completed

**Backend:**

- Added `calculateAvailableOps()` in `handlers.go` and `state_provider.go`
- Server logic: `online + no pending = [pull, switch, test, reboot]`
- Added `AvailableOps []string` to `templates.Host` struct
- Included in both initial page render and State Sync messages

**Frontend:**

- Updated dropdown menu to use `isOpAvailable(host, op)` instead of `!host.Online`
- Removed client-side "host is offline" check in `showRebootModal()`
- Added `data-available-ops` attribute to `HostRow` for JS hydration
- Updated `hostStore._parseAvailableOps()` to cache server state

### 🔄 Remaining Work (~1h)

1. **Browser Testing**: Verify buttons enable/disable correctly
   - Test with online host → buttons enabled
   - Test with offline host → buttons disabled
   - Test with pending command → buttons disabled
   - Test reboot modal only opens when button is enabled

2. **State Sync Updates**: Ensure `availableOps` updates on state changes
   - When host goes offline → buttons should disable
   - When command completes → buttons should re-enable

---

## Related

- **P3010**: Op Engine (defines available ops) - ✅ Done
- **P3200**: State Sync (sends available ops in state) - ✅ Done
