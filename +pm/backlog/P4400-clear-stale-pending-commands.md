# P4400 - Clear Stale Pending Commands for Offline Hosts

**Priority**: Medium
**Status**: Pending
**Effort**: Small
**References**: PRD FR-2.13, P2000 (Hub Resilience)

## Problem

When a host goes offline with a `pending_command` set, that command badge persists indefinitely because:

1. The host can't send heartbeats to clear it
2. The host can't re-register to trigger the cleanup
3. The dashboard has no timeout mechanism for stale commands

### What ALREADY Works

The system correctly clears `pending_command` when hosts are **online**:

1. **On Registration** (hub.go): `pending_command = NULL`
2. **On Heartbeat** (hub.go): Agent reports its actual state
3. **On Command Completion** (hub.go): `pending_command = NULL`

### The Narrow Problem

Only **OFFLINE hosts** have stale commands because they can't communicate to clear them.

## Solution: Multiplier-Based Stale Detection

Following industry patterns (Kubernetes liveness probes, etcd, AWS ALB), use a **heartbeat multiplier** with a floor:

```
stale_threshold = max(heartbeat_interval × multiplier, minimum_floor)
```

### Configuration

| Parameter                   | Default | Description                                |
| --------------------------- | ------- | ------------------------------------------ |
| `NIXFLEET_STALE_MULTIPLIER` | 120     | Number of missed heartbeats                |
| `NIXFLEET_STALE_MINIMUM`    | 5m      | Floor to prevent overly aggressive cleanup |

### Effective Timeouts

| Heartbeat    | Multiplier | Calculated | Floor | **Effective** |
| ------------ | ---------- | ---------- | ----- | ------------- |
| 5s           | 120        | 10 min     | 5 min | **10 min**    |
| 10s          | 120        | 20 min     | 5 min | **20 min**    |
| 30s          | 120        | 60 min     | 5 min | **60 min**    |
| 1s (testing) | 120        | 2 min      | 5 min | **5 min**     |

### Why This Approach

1. **Industry standard**: Kubernetes, etcd, Consul all use multiplier-based health detection
2. **Self-adjusting**: Automatically scales with heartbeat interval changes
3. **Semantic clarity**: "Stale after 120 missed heartbeats" is intuitive
4. **Safe floor**: Prevents aggressive cleanup during testing (1s heartbeat)

## Implementation

### 1. Add Configuration (dashboard config)

```go
// In internal/dashboard/config.go or hub.go
type HubConfig struct {
    // Existing fields...

    // Stale command detection
    HeartbeatInterval    time.Duration // Reference: 5s default
    StaleMultiplier      int           // Default: 120
    StaleMinimum         time.Duration // Default: 5 minutes
    CleanupCheckInterval time.Duration // Default: 1 minute
}

func (c *HubConfig) StaleCommandTimeout() time.Duration {
    calculated := c.HeartbeatInterval * time.Duration(c.StaleMultiplier)
    if calculated < c.StaleMinimum {
        return c.StaleMinimum
    }
    return calculated
}
```

### 2. Add Cleanup Goroutine (hub.go)

```go
func (h *Hub) Run(ctx context.Context) {
    // Start cleanup loop
    go h.staleCommandCleanupLoop(ctx)

    // Existing run loop...
}

func (h *Hub) staleCommandCleanupLoop(ctx context.Context) {
    ticker := time.NewTicker(h.cfg.CleanupCheckInterval) // 1 minute
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            h.log.Info().Msg("stale command cleanup loop shutting down")
            return
        case <-ticker.C:
            h.cleanupStaleCommands()
        }
    }
}

func (h *Hub) cleanupStaleCommands() {
    timeout := h.cfg.StaleCommandTimeout()
    thresholdMinutes := int(timeout.Minutes())

    result, err := h.db.Exec(`
        UPDATE hosts
        SET pending_command = NULL
        WHERE pending_command IS NOT NULL
        AND status = 'offline'
        AND last_seen < datetime('now', '-' || ? || ' minutes')
    `, thresholdMinutes)

    if err != nil {
        h.log.Error().Err(err).Msg("failed to cleanup stale commands")
        return
    }

    if rows, _ := result.RowsAffected(); rows > 0 {
        h.log.Info().
            Int64("count", rows).
            Dur("threshold", timeout).
            Msg("cleared stale commands for offline hosts")

        // Broadcast to refresh browser UI
        h.broadcastStaleCommandsCleared()
    }
}

func (h *Hub) broadcastStaleCommandsCleared() {
    // Query which hosts were affected and broadcast updates
    rows, err := h.db.Query(`
        SELECT hostname FROM hosts
        WHERE status = 'offline' AND pending_command IS NULL
    `)
    if err != nil {
        return
    }
    defer rows.Close()

    for rows.Next() {
        var hostname string
        if err := rows.Scan(&hostname); err != nil {
            continue
        }
        h.queueBroadcast(map[string]any{
            "type": "host_update",
            "payload": map[string]any{
                "host_id":         hostname,
                "pending_command": nil,
            },
        })
    }
}
```

### 3. Environment Variables

```bash
# docker-compose.yml / nixfleet.env
NIXFLEET_STALE_MULTIPLIER=120      # 120 missed heartbeats
NIXFLEET_STALE_MINIMUM=5m          # Minimum 5 minutes
NIXFLEET_CLEANUP_INTERVAL=1m       # Check every minute
```

## Requirements Checklist

- [ ] Add stale detection config to Hub
- [ ] Implement `StaleCommandTimeout()` with multiplier + floor
- [ ] Add `staleCommandCleanupLoop()` goroutine
- [ ] Add `cleanupStaleCommands()` with proper SQL
- [ ] Broadcast `host_update` when commands cleared
- [ ] Add env var parsing for configuration
- [ ] Log when stale commands are cleared
- [ ] Write integration test

## Test Scenarios

### Manual Test: M-P4400-1 - Timeout Clears Offline Host Command

**Preconditions:**

- Dashboard running with `NIXFLEET_STALE_MINIMUM=1m` (for faster testing)
- At least one host that can go offline

**Steps:**

1. Send a command to an online host (e.g., "switch")
2. Stop the agent or disconnect the host before command completes
3. Verify dashboard shows host as offline with command badge
4. Wait for cleanup interval + stale threshold (e.g., 2 minutes with 1m minimum)
5. Check dashboard logs for "cleared stale commands"
6. Verify badge is removed in UI

**Expected:** Badge is cleared for the offline host

### Automated Test: T-P4400-stale-cleanup.sh

```bash
#!/bin/bash
set -e

PASS=0
FAIL=0

# Check cleanup function exists
if grep -q "cleanupStaleCommands" v2/internal/dashboard/hub.go; then
    echo "[PASS] cleanupStaleCommands function exists"
    ((PASS++))
else
    echo "[FAIL] cleanupStaleCommands function NOT FOUND"
    ((FAIL++))
fi

# Check cleanup loop exists
if grep -q "staleCommandCleanupLoop" v2/internal/dashboard/hub.go; then
    echo "[PASS] staleCommandCleanupLoop exists"
    ((PASS++))
else
    echo "[FAIL] staleCommandCleanupLoop NOT FOUND"
    ((FAIL++))
fi

# Check multiplier-based calculation
if grep -q "StaleMultiplier\|StaleCommandTimeout" v2/internal/dashboard/hub.go; then
    echo "[PASS] Multiplier-based timeout calculation exists"
    ((PASS++))
else
    echo "[FAIL] Multiplier-based timeout NOT FOUND"
    ((FAIL++))
fi

# Check SQL targets offline hosts only
if grep -q "status = 'offline'" v2/internal/dashboard/hub.go | grep -q "pending_command"; then
    echo "[PASS] Cleanup targets offline hosts only"
    ((PASS++))
else
    echo "[WARN] Check cleanup SQL manually"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
```

### Integration Test: TestHub_CleansStaleOfflineCommands

```go
func TestHub_CleansStaleOfflineCommands(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    hub := NewHub(db, zerolog.Nop(), &HubConfig{
        HeartbeatInterval: 1 * time.Second,
        StaleMultiplier:   3,  // 3 seconds
        StaleMinimum:      0,  // No floor for testing
    })

    // Insert offline host with stale command (5 seconds ago)
    _, err := db.Exec(`
        INSERT INTO hosts (id, hostname, host_type, status, pending_command, last_seen)
        VALUES ('test', 'test', 'nixos', 'offline', 'switch', datetime('now', '-5 seconds'))
    `)
    require.NoError(t, err)

    // Run cleanup
    hub.cleanupStaleCommands()

    // Verify pending_command is cleared
    var pending sql.NullString
    err = db.QueryRow(`SELECT pending_command FROM hosts WHERE hostname = 'test'`).Scan(&pending)
    require.NoError(t, err)
    assert.False(t, pending.Valid, "expected pending_command to be NULL")
}

func TestHub_DoesNotClearOnlineHostCommands(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    hub := NewHub(db, zerolog.Nop(), &HubConfig{
        HeartbeatInterval: 1 * time.Second,
        StaleMultiplier:   3,
        StaleMinimum:      0,
    })

    // Insert ONLINE host with pending command
    _, err := db.Exec(`
        INSERT INTO hosts (id, hostname, host_type, status, pending_command, last_seen)
        VALUES ('test', 'test', 'nixos', 'online', 'switch', datetime('now', '-5 seconds'))
    `)
    require.NoError(t, err)

    // Run cleanup
    hub.cleanupStaleCommands()

    // Verify pending_command is NOT cleared (host is online)
    var pending sql.NullString
    err = db.QueryRow(`SELECT pending_command FROM hosts WHERE hostname = 'test'`).Scan(&pending)
    require.NoError(t, err)
    assert.True(t, pending.Valid, "expected pending_command to remain for online host")
    assert.Equal(t, "switch", pending.String)
}
```

## Edge Cases

1. **Host comes online during cleanup**: No issue - registration clears command anyway
2. **Very short heartbeat (1s testing)**: Floor of 5 min prevents aggressive cleanup
3. **Legitimate long command on online host**: Agent heartbeats keep it "online", cleanup doesn't touch it
4. **Dashboard restart**: Cleanup loop starts fresh, uses DB timestamps

## Files to Modify

| File                              | Changes                                         |
| --------------------------------- | ----------------------------------------------- |
| `v2/internal/dashboard/hub.go`    | Add cleanup goroutine, config, cleanup function |
| `v2/internal/dashboard/server.go` | Pass config to Hub                              |
| `v2/tests/integration/`           | Add integration tests                           |

## Related

- **PRD FR-2.13**: Stale command cleanup requirement
- **P2000** (Hub Resilience): Context/graceful shutdown patterns to follow
- **P4395** (Stop Command): Stop already clears pending_command correctly
