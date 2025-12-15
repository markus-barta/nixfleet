# P4400 - Clear Stale Pending Commands for Offline Hosts

**Priority**: Medium (downgraded from High)
**Status**: Pending
**Effort**: Small
**References**: Analysis 2025-12-15, P2000 (Hub Resilience)

## Problem

When a host goes offline with a `pending_command` set, that command badge persists indefinitely because:

1. The host can't send heartbeats to clear it
2. The host can't re-register to trigger the cleanup
3. The dashboard has no timeout mechanism for stale commands

### What ALREADY Works

The system correctly clears `pending_command` when hosts are **online**:

1. **On Registration** (line 445 in hub.go):

   ```go
   ON CONFLICT(hostname) DO UPDATE SET
       ...
       pending_command = NULL
   ```

2. **On Heartbeat** (line 494 in hub.go):

   ```go
   pending_command = ?,  // Agent reports its actual state
   ```

3. **On Command Completion** (line 531 in hub.go):
   ```go
   UPDATE hosts SET pending_command = NULL WHERE hostname = ?
   ```

### The Narrow Problem

Only **OFFLINE hosts** have stale commands because they can't communicate to clear them.

**Example** (2025-12-15):

```
hostname | status  | pending_command | last_seen
---------|---------|-----------------|-------------------
csb0     | online  | NULL            | 2025-12-15 13:45  ✅ Cleared on reconnect
csb1     | online  | NULL            | 2025-12-15 13:45  ✅ Cleared on reconnect
gpc0     | offline | switch          | 2025-12-14 20:56  ❌ Still stale (host offline)
```

## Solution

### Option A: Timeout-Based Cleanup (Recommended)

Add a background job that clears `pending_command` for hosts that have been offline for a configurable duration (e.g., 10 minutes).

**Rationale**: If a host has been offline for 10+ minutes, any "pending" command is certainly not running anymore. The agent either crashed, was rebooted, or the network is down.

```go
func (h *Hub) startStaleCommandCleanup(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            h.cleanupStaleCommands()
        }
    }
}

func (h *Hub) cleanupStaleCommands() {
    result, err := h.db.Exec(`
        UPDATE hosts
        SET pending_command = NULL
        WHERE pending_command IS NOT NULL
        AND status = 'offline'
        AND last_seen < datetime('now', '-10 minutes')
    `)
    if err != nil {
        h.log.Error().Err(err).Msg("failed to cleanup stale commands")
        return
    }

    if rows, _ := result.RowsAffected(); rows > 0 {
        h.log.Info().Int64("count", rows).Msg("cleared stale commands for offline hosts")
        // Optionally broadcast updates to browsers
    }
}
```

### Option B: Clear on Offline Transition

When a host transitions to offline (disconnect detected), optionally clear `pending_command` after a grace period.

**Downside**: May clear commands that could resume if the host quickly reconnects.

### Option C: Visual Indication (No DB Change)

Instead of clearing the command, show it with a "stale" indicator in the UI after X minutes.

**Downside**: Command remains in DB, button stays disabled.

## Recommended Implementation

**Option A only** - simple, effective, low risk:

1. Add `startStaleCommandCleanup()` goroutine in `hub.Run()`
2. Run every 1 minute, clear commands for hosts offline > 10 minutes
3. Log when commands are cleared (for debugging)

This is the **minimum viable fix** that solves the actual problem without over-engineering.

## Why NOT the Original Three-Option Approach

The original P4400 proposed:

- Option A: Clear on reconnect → **Already implemented!**
- Option B: Timeout cleanup → **Still needed for offline hosts**
- Option C: Heartbeat sync → **Already implemented!**

Two of three options were already working. Only the timeout for offline hosts is needed.

## Requirements

- [ ] Add `cleanupStaleCommands()` function
- [ ] Start cleanup goroutine in `hub.Run()`
- [ ] Make timeout configurable (default 10 minutes)
- [ ] Log when stale commands are cleared
- [ ] Consider broadcasting `host_update` to refresh UI

## Test Scenarios

### Manual Test: M-P4400-1 - Timeout Clears Offline Host Command

**Preconditions:**

- Dashboard running
- At least one host with stale `pending_command` that is offline

**Steps:**

1. Verify an offline host shows a command badge (e.g., "switch")
2. Ensure the host has been offline > 10 minutes
3. Wait for cleanup job to run (check logs for "cleared stale commands")
4. Refresh dashboard

**Expected:** Badge is cleared for the offline host

### Automated Test: T-P4400-stale-cleanup.sh

```bash
#!/bin/bash
# Test that cleanup job clears stale commands for offline hosts

# This test verifies the cleanup logic by checking DB after the job runs
# Requires: access to the nixfleet database

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

# Check cleanup is started in Run()
if grep -q "startStaleCommandCleanup\|cleanupStaleCommands" v2/internal/dashboard/hub.go | grep -q "go "; then
    echo "[PASS] Cleanup goroutine started"
    ((PASS++))
else
    echo "[WARN] Cleanup goroutine not clearly started (check manually)"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
```

### Integration Test: TestHub_CleansStaleOfflineCommands

```go
func TestHub_CleansStaleOfflineCommands(t *testing.T) {
    // Setup: Create offline host with stale command
    _, err := db.Exec(`
        INSERT INTO hosts (id, hostname, host_type, status, pending_command, last_seen)
        VALUES ('test', 'test', 'nixos', 'offline', 'switch', datetime('now', '-15 minutes'))
    `)
    require.NoError(t, err)

    // Act: Run cleanup
    hub.cleanupStaleCommands()

    // Assert: pending_command should be cleared
    var pending sql.NullString
    err = db.QueryRow(`SELECT pending_command FROM hosts WHERE hostname = 'test'`).Scan(&pending)
    require.NoError(t, err)

    assert.False(t, pending.Valid, "expected pending_command to be NULL")
}
```

## Edge Cases

1. **Host comes online during cleanup**: No issue - next heartbeat will set correct state
2. **Short offline period**: 10-minute threshold prevents premature clearing
3. **Legitimate long-running command**: If a switch takes 15 minutes and host stays online, heartbeats keep `pending_command` set correctly

## Files to Modify

| File                           | Changes         |
| ------------------------------ | --------------- |
| `v2/internal/dashboard/hub.go` | Add cleanup job |

## Related

- P2000 (Hub Resilience) - Context/graceful shutdown patterns to follow
- P4395 (Stop Command) - Stop already clears pending_command correctly
