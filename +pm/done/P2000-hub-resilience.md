# P2000: Hub Resilience & Deadlock Fix

**Priority:** CRITICAL  
**Date:** 2025-12-15  
**Status:** ✅ IMPLEMENTED & DEPLOYED  
**Subsumes:** P3000 (Switch Commands Not Delivered)

---

## Summary

The WebSocket hub has a critical deadlock bug that causes it to stop processing messages while appearing healthy. This was the root cause of P3000 (switch commands not being delivered).

---

## Root Cause Analysis

### The Bug: Deadlock in Hub Unregister

When an agent disconnects, the hub enters a deadlock:

```go
// hub.go lines 100-119
case client := <-h.unregister:
    h.mu.Lock()                              // ← Acquires WRITE lock
    // ...
    h.broadcastHostOffline(client.clientID)  // ← Called WHILE holding lock
    // ...
    h.mu.Unlock()

// broadcastHostOffline → BroadcastToBrowsers:
func (h *Hub) BroadcastToBrowsers(msg map[string]any) {
    // ...
    h.mu.RLock()   // ← Tries READ lock → DEADLOCK
```

**Go's `sync.RWMutex` does not support recursive locking.** The hub goroutine blocks forever.

### Why It Appeared to Work

- Health endpoint: Different goroutine → Still responded
- WebSocket upgrades: HTTP handler → Still worked
- Container status: "healthy" based on health checks
- But hub's main loop: **Frozen**

### Timeline (2025-12-14)

1. 8:52 PM: Dashboard started, agents registered
2. 8:53-8:54 PM: Pull commands processed successfully
3. 8:57 PM: Agent disconnected → Hub deadlocked
4. 8:57 PM onwards: No logs, no message processing
5. Next day: Switch commands clicked but never delivered

---

## Implementation Plan

### Phase 1: Critical Fixes (P0)

#### 1.1 Fix Deadlock - Move External Calls Outside Locks

**File:** `v2/internal/dashboard/hub.go`

```go
case client := <-h.unregister:
    var (
        shouldNotify bool
        hostID       string
        sendChan     chan []byte
    )

    h.mu.Lock()
    if _, ok := h.clients[client]; ok {
        delete(h.clients, client)
        delete(h.browsers, client)
        if client.clientType == "agent" && client.clientID != "" {
            if h.agents[client.clientID] == client {
                delete(h.agents, client.clientID)
                shouldNotify = true
                hostID = client.clientID
            }
        }
        sendChan = client.send
    }
    h.mu.Unlock()

    // ALL external operations OUTSIDE the lock
    if hostID != "" {
        _, _ = h.db.Exec(`UPDATE hosts SET status = 'offline' WHERE hostname = ?`, hostID)
    }
    if sendChan != nil {
        close(sendChan)
    }
    if shouldNotify {
        h.broadcastHostOffline(hostID)
    }

    h.log.Debug().Str("type", client.clientType).Str("id", client.clientID).Msg("client unregistered")
```

#### 1.2 Add Panic Recovery to Hub

```go
func (h *Hub) Run(ctx context.Context) {
    for {
        if err := h.runLoop(ctx); err != nil {
            if errors.Is(err, context.Canceled) {
                h.log.Info().Msg("hub shutting down gracefully")
                return
            }
            h.log.Error().Err(err).Msg("hub loop crashed, restarting in 100ms...")
            time.Sleep(100 * time.Millisecond)
        }
    }
}

func (h *Hub) runLoop(ctx context.Context) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("hub panic: %v\n%s", r, debug.Stack())
        }
    }()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case client := <-h.register:
            h.handleRegister(client)
        case client := <-h.unregister:
            h.handleUnregister(client)
        case msg := <-h.agentMessages:
            h.handleAgentMessage(msg)
        }
    }
}
```

### Phase 2: Prevent Send-on-Closed-Channel (P1)

#### 2.1 Add Safe Channel Handling to Client

```go
type Client struct {
    conn       *websocket.Conn
    clientType string
    clientID   string
    send       chan []byte
    hub        *Hub
    server     *Server

    // Safe close handling
    closeOnce sync.Once
    closed    atomic.Bool
}

// SafeSend sends without panicking on closed channel
func (c *Client) SafeSend(data []byte) bool {
    if c.closed.Load() {
        return false
    }
    select {
    case c.send <- data:
        return true
    default:
        return false // Buffer full
    }
}

// Close safely closes the send channel exactly once
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        c.closed.Store(true)
        close(c.send)
    })
}
```

#### 2.2 Update BroadcastToBrowsers

```go
func (h *Hub) BroadcastToBrowsers(msg map[string]any) {
    data, err := json.Marshal(msg)
    if err != nil {
        return
    }

    h.mu.RLock()
    browsers := make([]*Client, 0, len(h.browsers))
    for client := range h.browsers {
        browsers = append(browsers, client)
    }
    h.mu.RUnlock()

    for _, client := range browsers {
        client.SafeSend(data)  // Never panics
    }
}
```

### Phase 3: Async Broadcast Queue (P2)

Decouple state changes from broadcasts for maximum resilience.

#### 3.1 Add Broadcast Channel to Hub

```go
type Hub struct {
    // ... existing fields ...

    // Async broadcast queue - decouples state from notification
    broadcasts chan []byte
}

func NewHub(log zerolog.Logger, db *sql.DB) *Hub {
    return &Hub{
        // ... existing ...
        broadcasts: make(chan []byte, 1024),
    }
}
```

#### 3.2 Separate Broadcast Goroutine

```go
func (h *Hub) Run(ctx context.Context) {
    // Separate goroutine for broadcasts - can't block main loop
    go h.broadcastLoop(ctx)

    // Main loop handles state only
    for {
        if err := h.runLoop(ctx); err != nil {
            // ... recovery logic ...
        }
    }
}

func (h *Hub) broadcastLoop(ctx context.Context) {
    defer func() {
        if r := recover(); r != nil {
            h.log.Error().Interface("panic", r).Msg("broadcast loop crashed")
        }
    }()

    for {
        select {
        case <-ctx.Done():
            return
        case data := <-h.broadcasts:
            h.mu.RLock()
            browsers := make([]*Client, 0, len(h.browsers))
            for client := range h.browsers {
                browsers = append(browsers, client)
            }
            h.mu.RUnlock()

            for _, client := range browsers {
                client.SafeSend(data)
            }
        }
    }
}

// Queue broadcast instead of inline execution
func (h *Hub) QueueBroadcast(msg map[string]any) {
    data, err := json.Marshal(msg)
    if err != nil {
        return
    }
    select {
    case h.broadcasts <- data:
    default:
        h.log.Warn().Msg("broadcast queue full, dropping message")
    }
}
```

---

## Acceptance Criteria

### Phase 1 (Critical) ✅

- [x] Hub no longer deadlocks when agent disconnects
- [x] Hub automatically recovers from panics with logging
- [x] Context cancellation enables graceful shutdown
- [x] All external calls (DB, broadcast) happen outside mutex locks

### Phase 2 (Important) ✅

- [x] `SafeSend` prevents send-on-closed-channel panics
- [x] `Client.Close()` uses `sync.Once` to prevent double-close
- [x] No panics possible in broadcast path

### Phase 3 (Resilience) ✅

- [x] Broadcasts run in separate goroutine
- [x] State changes cannot be blocked by slow broadcasts
- [x] Broadcast queue has backpressure (drops with warning if full)

---

## Testing

### Unit Tests

```go
func TestHub_UnregisterDoesNotDeadlock(t *testing.T) {
    // Register agent, then unregister while browsers connected
    // Must complete within 1 second (not deadlock)
}

func TestHub_RecoverFromPanic(t *testing.T) {
    // Inject panic in message handler
    // Hub should log error and continue processing
}

func TestClient_SafeSendOnClosedChannel(t *testing.T) {
    // Close client, then SafeSend
    // Must return false, not panic
}

func TestHub_BroadcastQueueBackpressure(t *testing.T) {
    // Fill broadcast queue
    // New broadcasts should be dropped with warning, not block
}
```

### Integration Tests

```go
func TestHub_AgentDisconnectWhileBroadcasting(t *testing.T) {
    // Connect agent and browsers
    // Agent sends heartbeat (triggers broadcast)
    // Simultaneously disconnect agent
    // Must not deadlock
}
```

---

## Files to Modify

| File                                | Changes                                        |
| ----------------------------------- | ---------------------------------------------- |
| `v2/internal/dashboard/hub.go`      | Deadlock fix, panic recovery, async broadcasts |
| `v2/internal/dashboard/server.go`   | Pass context to hub.Run()                      |
| `v2/cmd/nixfleet-dashboard/main.go` | Context setup for graceful shutdown            |

---

## Risk Assessment

| Risk                           | Mitigation                            |
| ------------------------------ | ------------------------------------- |
| Regression in broadcast timing | Phase 3 queues messages; test latency |
| Lost broadcasts on queue full  | Log warning, monitor in production    |
| Panic recovery masking bugs    | Stack trace in logs, alerting         |

---

## Related

- **Caused:** P3000 (Switch Commands Not Delivered) - now closed as duplicate
- **PRD:** FR-2.8 (Broadcast host updates to connected browsers)
- **PRD:** NFR-2.1 (Agent uptime 99.9%)

---

## Changelog

| Date       | Change                                                   |
| ---------- | -------------------------------------------------------- |
| 2025-12-15 | Created, subsuming P3000 after root cause analysis       |
| 2025-12-15 | ✅ IMPLEMENTED: All 3 phases, deployed to csb1, verified |
