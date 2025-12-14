# T02 - Agent Heartbeat

**Backlog**: P4000 (Go Agent Core)
**Priority**: Must Have

---

## Purpose

Verify that the agent sends regular heartbeats and continues heartbeating during command execution (the core problem v2.0 solves).

---

## Prerequisites

- Dashboard running and agent connected
- Configurable heartbeat interval (default 30s, use 1s for tests)

---

## Scenarios

### Scenario 1: Regular Heartbeat

**Given** the agent is connected to the dashboard
**And** the heartbeat interval is 1 second (for testing)
**When** 5 seconds pass
**Then** the dashboard receives at least 4 heartbeat messages
**And** each heartbeat contains current metrics and generation

```json
// Agent sends every interval:
{
  "type": "heartbeat",
  "payload": {
    "generation": "abc1234",
    "metrics": {
      "cpu": 15.5,
      "ram": 42.0,
      "swap": 0.0,
      "load": "0.5"
    }
  }
}
```

### Scenario 2: Heartbeat During Command Execution (CRITICAL)

**Given** the agent is connected to the dashboard
**And** the heartbeat interval is 1 second
**When** a long-running command (5 seconds) is executing
**Then** the agent continues sending heartbeats during execution
**And** at least 4 heartbeats are received during the 5-second command
**And** the host never appears as "stale" in the dashboard

```text
Timeline:
  0s: Command starts
  1s: Heartbeat sent ✓
  2s: Heartbeat sent ✓
  3s: Heartbeat sent ✓
  4s: Heartbeat sent ✓
  5s: Command completes
```

### Scenario 3: Heartbeat with StaSysMo Metrics

**Given** StaSysMo is running on the host
**And** metrics are available in `/dev/shm/stasysmo/` (Linux) or `/tmp/stasysmo/` (macOS)
**When** the agent sends a heartbeat
**Then** the heartbeat includes CPU, RAM, swap, and load metrics
**And** metrics are fresh (< 30 seconds old)

### Scenario 4: Heartbeat without StaSysMo

**Given** StaSysMo is NOT running
**When** the agent sends a heartbeat
**Then** the heartbeat is sent without metrics
**And** the dashboard handles missing metrics gracefully

### Scenario 5: Host Status Based on Heartbeat

**Given** the agent is connected and heartbeating
**When** heartbeats stop for > 90 seconds (3x interval)
**Then** the dashboard marks the host as "offline"
**And** when heartbeats resume, the host is marked "online"

---

## Verification Commands

```bash
# Start dashboard with debug logging
NIXFLEET_LOG_LEVEL=debug go run ./cmd/nixfleet-dashboard &

# Start agent with 1s heartbeat
NIXFLEET_URL=ws://localhost:8000/ws \
NIXFLEET_TOKEN=test-token \
NIXFLEET_INTERVAL=1 \
go run ./cmd/nixfleet-agent &

# Watch heartbeats in dashboard logs
# Should see "heartbeat from test-host" every ~1 second

# Trigger a long command and verify heartbeats continue
curl -X POST http://localhost:8000/api/hosts/test-host/command \
  -H "Content-Type: application/json" \
  -d '{"command": "test-long"}'
# Dashboard logs should still show heartbeats during command
```

---

## Test Implementation

```go
// tests/integration/agent_test.go

func TestAgentHeartbeat_Regular(t *testing.T) {
    // Start mock dashboard
    // Start agent with 1s interval
    // Wait 5 seconds
    // Verify at least 4 heartbeats received
}

func TestAgentHeartbeat_DuringCommand(t *testing.T) {
    // Start mock dashboard
    // Start agent with 1s interval
    // Send long-running command (5s sleep)
    // Verify heartbeats continue during execution
    // This is the CRITICAL test for v2.0
}

func TestAgentHeartbeat_HostStatus(t *testing.T) {
    // Start dashboard
    // Connect agent
    // Verify host is "online"
    // Stop agent heartbeats
    // Wait 90+ seconds
    // Verify host is "offline"
}
```
