# P4400 - Clear Stale Pending Commands

**Priority**: High
**Status**: Pending
**Effort**: Medium
**References**: Analysis 2025-12-15, P2000 (Hub Resilience)

## Problem

When an agent goes offline while a command is running (or shortly after receiving one), the `pending_command` field in the database is never cleared. This causes:

1. Stale "switch"/"pull"/"test" badges that never disappear
2. Buttons remaining disabled for offline hosts
3. Confusion about whether a command is actually running

### Root Cause

The hub only clears `pending_command` when it receives a `status` message with completion info (`complete`, `error`, `stopped`). But if the agent:

- Crashes during command execution
- Is removed by a `nixos-rebuild switch`
- Disconnects from the network
- Reboots mid-command

...then the status message is never sent, and `pending_command` remains set forever.

### Observed Behavior

```
hostname | pending_command | last_seen           | Notes
---------|-----------------|---------------------|---------------------------
csb0     | switch          | 2025-12-14 20:56:25 | Stale for 16+ hours
csb1     | switch          | 2025-12-14 20:56:29 | Stale for 16+ hours
gpc0     | switch          | 2025-12-14 20:56:32 | Stale for 16+ hours
```

## Solution

### Option A: Clear on Agent Reconnect (Recommended)

When an agent registers/reconnects, check if it's reporting as idle. If the agent has no command running but the DB shows `pending_command`, clear it.

```go
func (h *Hub) handleRegister(client *Client) {
    // ... existing registration logic ...

    // If agent reconnects and reports no command running,
    // clear any stale pending_command
    if msg.Status == "" || msg.Status == "idle" {
        _, err := h.db.Exec(`
            UPDATE hosts
            SET pending_command = NULL
            WHERE hostname = ? AND pending_command IS NOT NULL
        `, msg.Hostname)
        if err != nil {
            h.log.Error().Err(err).Str("host", msg.Hostname).Msg("failed to clear stale pending_command")
        } else {
            h.log.Info().Str("host", msg.Hostname).Msg("cleared stale pending_command on reconnect")
        }
    }
}
```

### Option B: Timeout-Based Cleanup (Background Job)

Run a periodic job that clears `pending_command` for hosts that:

- Have been offline for > X minutes (e.g., 10 minutes)
- Have had a pending command for > Y minutes without status update

```go
func (h *Hub) cleanupStaleCommands() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        _, err := h.db.Exec(`
            UPDATE hosts
            SET pending_command = NULL
            WHERE pending_command IS NOT NULL
            AND (
                status = 'offline'
                OR last_seen < datetime('now', '-10 minutes')
            )
        `)
        if err != nil {
            h.log.Error().Err(err).Msg("failed to cleanup stale commands")
        }
    }
}
```

### Option C: Agent Reports "Busy" State in Heartbeat

Have the agent include its current busy state in every heartbeat. If the dashboard receives a heartbeat showing `busy: false` but has `pending_command` set, clear it.

```go
// In heartbeat handling
if heartbeat.Busy == false && host.PendingCommand != "" {
    // Agent says it's not busy, but we think it is - clear stale state
    clearPendingCommand(host.ID)
}
```

## Recommended Implementation

Implement **all three** for defense in depth:

1. **Option A** (Primary): Clear on reconnect - handles the common case
2. **Option B** (Backup): Timeout cleanup - catches edge cases
3. **Option C** (Real-time): Heartbeat sync - keeps state accurate

## Requirements

- [ ] Clear `pending_command` when agent reconnects with idle status
- [ ] Add background job to clear stale commands (configurable timeout)
- [ ] Include `busy` field in heartbeat messages
- [ ] Sync `pending_command` state from heartbeat
- [ ] Log when stale commands are cleared (for debugging)
- [ ] Broadcast host_update when pending_command changes

## Test Scenarios

### Manual Test: M-P4400-1 - Reconnect Clears Stale Command

**Preconditions:**

- Dashboard running
- At least one agent online

**Steps:**

1. Click "Switch" on an online host
2. While command is running, stop the agent service:
   ```bash
   systemctl --user stop nixfleet-agent  # or system service
   ```
3. Wait 30 seconds
4. Verify dashboard shows host as offline with "switch" badge
5. Start the agent service:
   ```bash
   systemctl --user start nixfleet-agent
   ```
6. Verify dashboard shows host as online WITHOUT "switch" badge

**Expected:** Badge is cleared when agent reconnects

### Manual Test: M-P4400-2 - Timeout Clears Stale Command

**Preconditions:**

- Dashboard running
- Agent stopped (simulating offline)
- Host has stale `pending_command` in DB

**Steps:**

1. Verify host shows badge (e.g., "switch")
2. Wait for timeout period (e.g., 10 minutes)
3. Refresh dashboard

**Expected:** Badge is cleared by timeout cleanup job

### Manual Test: M-P4400-3 - Heartbeat Sync

**Preconditions:**

- Dashboard running
- Agent online and idle
- Manually set `pending_command` in DB (to simulate stale state)

**Steps:**

1. Manually insert stale pending_command:
   ```bash
   sqlite3 /data/nixfleet.db "UPDATE hosts SET pending_command='test' WHERE hostname='hsb0';"
   ```
2. Wait for next heartbeat (5-30 seconds)
3. Refresh dashboard

**Expected:** Badge is cleared because agent reported `busy: false`

### Automated Test: T-P4400-reconnect-clears-pending.sh

```bash
#!/bin/bash
# Test that agent reconnect clears stale pending_command

# Setup: Set a pending_command in DB
sqlite3 /data/nixfleet.db "UPDATE hosts SET pending_command='switch' WHERE hostname='test-host';"

# Simulate agent reconnect (send register message via test endpoint or wait for real agent)
# ...

# Verify: pending_command should be NULL
RESULT=$(sqlite3 /data/nixfleet.db "SELECT pending_command FROM hosts WHERE hostname='test-host';")
if [ -z "$RESULT" ]; then
    echo "[PASS] pending_command cleared on reconnect"
else
    echo "[FAIL] pending_command still set: $RESULT"
    exit 1
fi
```

### Integration Test: TestHub_ClearsStalePendingOnReconnect

```go
func TestHub_ClearsStalePendingOnReconnect(t *testing.T) {
    // Setup: Create host with stale pending_command
    db.Exec(`INSERT INTO hosts (hostname, pending_command) VALUES ('test', 'switch')`)

    // Act: Simulate agent registration
    hub.handleRegister(&Client{
        clientType: "agent",
        clientID:   "test",
    }, RegisterMessage{
        Hostname: "test",
        Status:   "idle",
    })

    // Assert: pending_command should be cleared
    var pending sql.NullString
    db.QueryRow(`SELECT pending_command FROM hosts WHERE hostname = 'test'`).Scan(&pending)

    if pending.Valid {
        t.Errorf("expected pending_command to be NULL, got %s", pending.String)
    }
}
```

## Edge Cases

1. **Command actually running**: Don't clear if agent reports `busy: true`
2. **Network flap**: Short disconnects shouldn't clear mid-execution commands
3. **Multiple agents same host**: Shouldn't happen, but handle gracefully
4. **Race condition**: Reconnect happens just as command completes - order matters

## Related

- P2000 (Hub Resilience) - Related hub stability improvements
- P4395 (Stop Command) - Stop should also clear pending_command
- P5100 (Queue Offline Commands) - Future: queue commands for offline hosts

## Files to Modify

| File                               | Changes                            |
| ---------------------------------- | ---------------------------------- |
| `v2/internal/dashboard/hub.go`     | Add reconnect cleanup, timeout job |
| `v2/internal/agent/heartbeat.go`   | Add busy field to heartbeat        |
| `v2/internal/protocol/messages.go` | Update HeartbeatMessage struct     |
