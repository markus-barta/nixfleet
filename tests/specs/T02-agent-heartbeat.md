# T02 - Agent Heartbeat

**Backlog**: P4000 (Go Agent Core)
**Priority**: Must Have

---

## Purpose

Verify that the agent sends regular heartbeats and continues heartbeating during command execution (the core problem v2.0 solves).

---

## Prerequisites

- Dashboard running and agent connected (T01 passing)
- Configurable heartbeat interval (default 30s, use 1s for tests)

---

## Protocol Details

### Heartbeat Message Format

```json
{
  "type": "heartbeat",
  "payload": {
    "generation": "abc1234",
    "nixpkgs_version": "24.11.20241201.abc1234",
    "pending_command": null,
    "command_pid": null,
    "metrics": {
      "cpu": 15.5,
      "ram": 42.0,
      "swap": 0.0,
      "load": 0.5
    }
  }
}
```

**Field Definitions:**

| Field             | Type         | Description                                             |
| ----------------- | ------------ | ------------------------------------------------------- |
| `generation`      | string       | Git commit hash of deployed config                      |
| `nixpkgs_version` | string       | Full nixpkgs version with commit                        |
| `pending_command` | string\|null | Current command: "pull", "switch", "test", etc. or null |
| `command_pid`     | int\|null    | PID of running command process, or null                 |
| `metrics`         | object\|null | StaSysMo metrics, or null if unavailable                |
| `metrics.cpu`     | float        | CPU usage percentage (0-100)                            |
| `metrics.ram`     | float        | RAM usage percentage (0-100)                            |
| `metrics.swap`    | float        | Swap usage percentage (0-100)                           |
| `metrics.load`    | float        | 1-minute load average                                   |

### Timing

| Parameter        | Value | Notes                                               |
| ---------------- | ----- | --------------------------------------------------- |
| Default interval | 30s   | Configurable via `NIXFLEET_INTERVAL`                |
| First heartbeat  | 0s    | Sent immediately after registration                 |
| Stale threshold  | 3x    | Dashboard marks offline after 3x heartbeat_interval |

### Dashboard Response

- **No response required** - Dashboard silently receives heartbeats
- Dashboard updates host `last_seen` timestamp on each heartbeat
- Dashboard broadcasts `host_update` to connected browsers

---

## Scenarios

### Scenario 1: Regular Heartbeat

**Given** the agent is connected to the dashboard
**And** the heartbeat interval is 1 second (for testing)
**When** 5 seconds pass
**Then** the dashboard receives at least 5 heartbeat messages (including immediate first)
**And** each heartbeat contains current metrics and generation

```text
Timeline:
  0.0s: Agent connects, sends register
  0.1s: Agent sends first heartbeat (immediate)
  1.1s: Agent sends heartbeat
  2.1s: Agent sends heartbeat
  3.1s: Agent sends heartbeat
  4.1s: Agent sends heartbeat
  5.0s: Test ends - at least 5 heartbeats received
```

### Scenario 2: Heartbeat During Command Execution (CRITICAL)

**Given** the agent is connected to the dashboard
**And** the heartbeat interval is 1 second
**When** a long-running command (5 seconds) is executing
**Then** the agent continues sending heartbeats during execution
**And** heartbeats include `pending_command` and `command_pid`
**And** at least 4 heartbeats are received during the 5-second command
**And** the host never appears as "stale" in the dashboard

```json
// Heartbeat during command execution:
{
  "type": "heartbeat",
  "payload": {
    "generation": "abc1234",
    "nixpkgs_version": "24.11.20241201.abc1234",
    "pending_command": "switch",
    "command_pid": 12345,
    "metrics": { "cpu": 85.0, "ram": 60.0, "swap": 0.0, "load": 2.5 }
  }
}
```

```text
Timeline:
  0s: Command starts, pending_command = "switch", command_pid = 12345
  1s: Heartbeat sent with pending_command ✓
  2s: Heartbeat sent with pending_command ✓
  3s: Heartbeat sent with pending_command ✓
  4s: Heartbeat sent with pending_command ✓
  5s: Command completes, pending_command = null, command_pid = null
```

### Scenario 3: Heartbeat with StaSysMo Metrics

**Given** StaSysMo is running on the host
**And** metrics are available in `/dev/shm/stasysmo/` (Linux) or `/tmp/stasysmo/` (macOS)
**When** the agent sends a heartbeat
**Then** the heartbeat includes CPU, RAM, swap, and load metrics
**And** metrics are fresh (< 30 seconds old)

```text
StaSysMo file locations:
  Linux:  /dev/shm/stasysmo/{cpu,ram,swap,load}
  macOS:  /tmp/stasysmo/{cpu,ram,swap,load}

File format (single line):
  cpu:  "15.5"   (percentage)
  ram:  "42.0"   (percentage)
  swap: "0.0"    (percentage)
  load: "0.5"    (1-minute average)
```

### Scenario 4: Heartbeat without StaSysMo

**Given** StaSysMo is NOT running
**When** the agent sends a heartbeat
**Then** the heartbeat is sent with `metrics: null`
**And** the dashboard handles missing metrics gracefully (shows "—")

```json
{
  "type": "heartbeat",
  "payload": {
    "generation": "abc1234",
    "nixpkgs_version": "24.11.20241201.abc1234",
    "pending_command": null,
    "command_pid": null,
    "metrics": null
  }
}
```

### Scenario 5: Host Status Based on Heartbeat

**Given** the agent is connected and heartbeating with 30s interval
**When** heartbeats stop for > 90 seconds (3x interval from registration)
**Then** the dashboard marks the host as "offline"
**And** when heartbeats resume, the host is marked "online"

```text
Timeline (30s interval):
  0s:   Heartbeat received, host online
  30s:  Heartbeat received, host online
  60s:  Heartbeat received, host online
  90s:  No heartbeat (agent stopped)
  91s:  Dashboard marks host offline (90s = 3x30s threshold exceeded)
  120s: Agent restarts, sends heartbeat
  121s: Dashboard marks host online
```

### Scenario 6: Heartbeat During Reconnection

**Given** the agent is executing a command
**And** the WebSocket connection is lost
**When** the connection is restored
**Then** the agent resumes heartbeating immediately
**And** failed heartbeats during disconnect are NOT buffered (skip, don't replay)
**And** the command continues executing (not affected by reconnect)

### Scenario 7: Concurrent Command Rejection

**Given** the agent is executing a command (`pending_command` is not null)
**When** the dashboard sends another command
**Then** the agent rejects the command with error "command already running"
**And** the agent continues executing the current command
**And** the dashboard receives the rejection and can show error to user

```json
// Dashboard sends:
{ "type": "command", "payload": { "command": "test" } }

// Agent responds:
{
  "type": "command_rejected",
  "payload": {
    "reason": "command already running",
    "current_command": "switch",
    "current_pid": 12345
  }
}
```

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
  -H "X-CSRF-Token: <token>" \
  -d '{"command": "test-long"}'
# Dashboard logs should still show heartbeats during command
# Heartbeats should include pending_command: "test-long"

# Try sending concurrent command (should be rejected)
curl -X POST http://localhost:8000/api/hosts/test-host/command \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: <token>" \
  -d '{"command": "switch"}'
# Should return error: "command already running"
```

---

## Test Implementation

```go
// tests/integration/agent_test.go

func TestAgentHeartbeat_Regular(t *testing.T) {
    // Start mock dashboard
    // Start agent with 1s interval
    // Wait 5 seconds
    // Verify at least 5 heartbeats received (including immediate first)
    // Verify each heartbeat has required fields
}

func TestAgentHeartbeat_DuringCommand(t *testing.T) {
    // Start mock dashboard
    // Start agent with 1s interval
    // Send long-running command (5s sleep)
    // Verify heartbeats continue during execution
    // Verify heartbeats include pending_command and command_pid
    // This is the CRITICAL test for v2.0
}

func TestAgentHeartbeat_WithMetrics(t *testing.T) {
    // Create mock StaSysMo files
    // Start agent
    // Verify heartbeat includes metrics
    // Verify metrics values match mock files
}

func TestAgentHeartbeat_WithoutMetrics(t *testing.T) {
    // Ensure no StaSysMo files exist
    // Start agent
    // Verify heartbeat has metrics: null
}

func TestAgentHeartbeat_HostStatus(t *testing.T) {
    // Start dashboard
    // Connect agent with 1s interval
    // Verify host is "online"
    // Stop agent (don't send heartbeats)
    // Wait 4 seconds (>3x interval)
    // Verify host is "offline"
    // Restart agent
    // Verify host is "online" again
}

func TestAgentHeartbeat_ConcurrentCommandRejection(t *testing.T) {
    // Start agent
    // Send long-running command
    // Try sending second command
    // Verify command_rejected message
    // Verify first command continues
}
```

---

## Notes

- **First heartbeat is immediate** to show host online instantly after registration
- **Stale threshold uses heartbeat_interval from registration** so dashboard knows each agent's expected timing
- **Concurrent commands are rejected** (for now) - queuing may be added later
- **Reconnect skips buffered heartbeats** - no replay of missed heartbeats, just resume
