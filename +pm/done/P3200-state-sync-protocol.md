# P3200: State Sync Protocol

**Priority**: P3200 (Critical - v3 Phase 3)  
**Status**: ✅ Done  
**Effort**: Medium (2-3 days)  
**Implements**: [CORE-004](../spec/CORE-004-state-sync.md)  
**Depends on**: P3100 (State Persistence)

---

## User Story

**As a** user  
**I want** the dashboard UI to always show current state  
**So that** I never need to manually refresh (CMD+R)

---

## Scope

### Server-Side State Manager

```go
type StateManager struct {
    mu      sync.RWMutex
    version uint64
    store   *StateStore
    clients map[*Client]struct{}
}

func (sm *StateManager) ApplyChange(change Change) {
    sm.mu.Lock()
    sm.version++
    v := sm.version
    sm.mu.Unlock()

    sm.store.ApplyChange(change)
    sm.broadcast(Message{Type: "delta", Version: v, Payload: change})
}
```

### WebSocket Messages

| Message      | Direction       | Purpose                       |
| ------------ | --------------- | ----------------------------- |
| `init`       | Server → Client | Full state on connect         |
| `delta`      | Server → Client | Incremental change            |
| `sync`       | Server → Client | Version beacon every 30s      |
| `get_state`  | Client → Server | Request full state (on drift) |
| `full_state` | Server → Client | Response to get_state         |

### Client-Side StateSync

```javascript
class StateSync {
  constructor(ws) {
    this.ws = ws;
    this.version = 0;
  }

  handleMessage(msg) {
    const { type, version, payload } = JSON.parse(msg.data);

    switch (type) {
      case "init":
      case "full_state":
        this.state = payload;
        this.version = version;
        this.render();
        break;

      case "delta":
        if (version !== this.version + 1) {
          this.requestFullState();
          return;
        }
        this.applyDelta(payload);
        this.version = version;
        this.render();
        break;

      case "sync":
        if (version !== this.version) {
          this.requestFullState();
        }
        break;
    }
  }

  requestFullState() {
    this.ws.send(JSON.stringify({ type: "get_state" }));
  }
}
```

### Sync Beacon

Every 30s, server broadcasts current version:

```go
func (sm *StateManager) StartSyncBeacon() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        sm.broadcast(Message{Type: "sync", Version: sm.version})
    }
}
```

### Delta Change Types

```go
type Change struct {
    Type   string      // "host_added", "command_started", etc.
    Target string      // Host ID or empty for global
    Data   interface{} // Change-specific payload
}
```

---

## Acceptance Criteria

- [x] StateManager tracks global version (persisted to SQLite)
- [x] `init` sent on WebSocket connect with full state
- [x] `delta` sent on every state change
- [x] `sync` beacon every 30s
- [x] Client detects version gap → auto resync
- [x] Client detects drift via sync → auto resync
- [x] StateSync JS class with `dispatchOp()` and `dispatchPipeline()`
- [x] CSRF token properly included in all dispatches

---

## Related

- **CORE-004**: State Sync Protocol spec
- **P3100**: State Persistence (prerequisite)
- **P3300**: Logs on Page Load (uses init payload)
