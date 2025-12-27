# CORE-004: State Sync Protocol

> **Spec Type**: Core Building Block  
> **Status**: Stable  
> **Last Updated**: 2025-12-27

---

## Purpose

The **State Sync Protocol** ensures the browser UI is **always live**—never stale, no manual refresh required. It solves the fundamental problem of WebSocket message loss and client-server state divergence.

---

## Problem Statement (v2)

| Issue                        | Impact                        |
| ---------------------------- | ----------------------------- |
| Messages missed on reconnect | Client shows stale data       |
| No reconciliation mechanism  | Divergence goes undetected    |
| Long-running tabs drift      | Must CMD+R to fix             |
| Server restart loses version | Client doesn't know to resync |

---

## Solution: Version-Based State Sync

Every state change increments a monotonic `state_version`. Clients track their version and detect drift.

---

## Protocol Messages

### Server → Client

| Message Type | Payload             | When Sent                     |
| ------------ | ------------------- | ----------------------------- |
| `init`       | `{state, version}`  | On connect, on resync request |
| `delta`      | `{change, version}` | On any state change           |
| `sync`       | `{version}`         | Every 30s (heartbeat beacon)  |
| `full_state` | `{state, version}`  | Response to `get_state`       |

### Client → Server

| Message Type | Payload | When Sent                              |
| ------------ | ------- | -------------------------------------- |
| `get_state`  | `{}`    | On version mismatch or explicit resync |

---

## Message Flow

```
Server                                              Browser
   │                                                    │
   │──────────── init(state, version=100) ─────────────▶│  [1. Initial connect]
   │                                                    │
   │──────────── delta(change, version=101) ───────────▶│  [2. State changes]
   │──────────── delta(change, version=102) ───────────▶│
   │                                                    │
   │         [connection lost, missed 103-105]          │  [3. Reconnection]
   │                                                    │
   │──────────── init(state, v=106) ───────────────────▶│  [4. Always full sync on reconnect]
   │                                                    │
   │         [periodic sync beacon every 30s]           │  [5. Drift detection]
   │                                                    │
   │──────────── sync(version=110) ────────────────────▶│
   │                                                    │
   │         [client: my_version=106 != 110 → resync]   │  [6. Auto-recovery]
   │                                                    │
   │◀─────────── get_state() ──────────────────────────│
   │──────────── full_state(state, v=110) ─────────────▶│
```

---

## State Version Rules

| Rule                     | Description                             |
| ------------------------ | --------------------------------------- |
| Monotonically increasing | Never decreases (except server restart) |
| Increments on ANY change | Hosts, commands, pipelines, event_log   |
| Persisted to disk        | Survives dashboard restart              |
| Reset detection          | Version < client version → force resync |

---

## Server Implementation

```go
type StateManager struct {
    mu       sync.RWMutex
    version  uint64
    store    *StateStore
    clients  map[*Client]struct{}
}

func (sm *StateManager) ApplyChange(change Change) {
    sm.mu.Lock()
    sm.version++
    currentVersion := sm.version
    sm.mu.Unlock()

    // Persist change to store
    sm.store.ApplyChange(change)

    // Broadcast delta to all clients
    sm.broadcast(Message{
        Type:    "delta",
        Version: currentVersion,
        Payload: change,
    })
}

func (sm *StateManager) StartSyncBeacon() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        sm.mu.RLock()
        v := sm.version
        sm.mu.RUnlock()

        sm.broadcast(Message{
            Type:    "sync",
            Version: v,
        })
    }
}

func (sm *StateManager) HandleConnect(client *Client) {
    sm.mu.RLock()
    state := sm.store.GetFullState()
    v := sm.version
    sm.mu.RUnlock()

    client.Send(Message{
        Type:    "init",
        Version: v,
        Payload: state,
    })
}

func (sm *StateManager) HandleGetState(client *Client) {
    sm.mu.RLock()
    state := sm.store.GetFullState()
    v := sm.version
    sm.mu.RUnlock()

    client.Send(Message{
        Type:    "full_state",
        Version: v,
        Payload: state,
    })
}
```

---

## Client Implementation

```javascript
class StateSync {
  constructor(ws) {
    this.ws = ws;
    this.version = 0;
    this.state = null;
  }

  handleMessage(msg) {
    const { type, version, payload } = JSON.parse(msg.data);

    switch (type) {
      case "init":
      case "full_state":
        // Full state replacement
        this.state = payload;
        this.version = version;
        this.render();
        break;

      case "delta":
        // Check for missed messages
        if (version !== this.version + 1) {
          console.warn(
            `Version gap: expected ${this.version + 1}, got ${version}`,
          );
          this.requestFullState();
          return;
        }
        // Apply delta
        this.applyDelta(payload);
        this.version = version;
        this.render();
        break;

      case "sync":
        // Drift detection
        if (version !== this.version) {
          console.warn(
            `Drift detected: local=${this.version}, server=${version}`,
          );
          this.requestFullState();
        }
        break;
    }
  }

  requestFullState() {
    this.ws.send(JSON.stringify({ type: "get_state" }));
  }

  applyDelta(change) {
    // Apply incremental change to this.state
    // Implementation depends on change structure
  }
}
```

---

## Delta Change Format

```typescript
type Change =
  | { type: "host_added"; host: Host }
  | { type: "host_updated"; id: string; fields: Partial<Host> }
  | { type: "host_removed"; id: string }
  | { type: "command_started"; command: Command }
  | { type: "command_progress"; id: string; output: string }
  | { type: "command_finished"; id: string; status: string; exit_code: number }
  | { type: "event"; event: EventLogEntry };
```

---

## Full State Format

```typescript
interface FullState {
  hosts: Host[];
  commands: Command[]; // Only active/recent
  pipelines: Pipeline[]; // Only active/recent
  events: EventLogEntry[]; // Last 100
}
```

---

## Guarantees

| Scenario              | Behavior                                   |
| --------------------- | ------------------------------------------ |
| Normal operation      | Deltas applied in order, UI always current |
| Missed single message | Version gap detected → auto resync         |
| Reconnection          | Always full resync on connect              |
| Tab inactive (30s+)   | Sync beacon catches drift                  |
| Server restart        | Version resets → clients detect and resync |
| Client desync         | Self-healing within 30s max                |

---

## Configuration

| Setting                         | Default | Description                       |
| ------------------------------- | ------- | --------------------------------- |
| `NIXFLEET_SYNC_BEACON_INTERVAL` | 30s     | Time between sync beacons         |
| `NIXFLEET_INIT_EVENT_LIMIT`     | 100     | Event log entries in init payload |

---

## Implementation Location

```
src/internal/sync/
├── sync.go           # StateManager, sync protocol

src/static/js/
├── state-sync.js     # Client-side StateSync class
```

---

## Implementing Backlog Items

> Updated as backlog items are created/completed.

| Backlog Item | Description                       | Status |
| ------------ | --------------------------------- | ------ |
| (pending)    | Add state_version to StateManager | —      |
| (pending)    | Implement init message            | —      |
| (pending)    | Implement delta messages          | —      |
| (pending)    | Implement sync beacon             | —      |
| (pending)    | Client-side StateSync class       | —      |

---

## Changelog

| Date       | Change                          |
| ---------- | ------------------------------- |
| 2025-12-27 | Initial spec extracted from PRD |
