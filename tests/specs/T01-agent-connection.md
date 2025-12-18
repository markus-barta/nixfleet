# T01 - Agent Connection

**Backlog**: P4000 (Go Agent Core)
**Priority**: Must Have

---

## Purpose

Verify that the agent can connect to the dashboard via WebSocket and handle connection lifecycle properly.

---

## Prerequisites

- Dashboard running on `ws://localhost:8000/ws` (or `wss://` in production)
- Valid agent token configured
- Network connectivity

---

## Protocol Details

### WebSocket Endpoint

Single endpoint `/ws` handles both agents and browsers. Client type is determined by the first message:

- Agents send `register` → dashboard tracks as agent connection
- Browsers send `subscribe` → dashboard tracks as browser connection

### Token Transmission

Agent token is sent via HTTP header during WebSocket upgrade:

```http
GET /ws HTTP/1.1
Host: localhost:8000
Upgrade: websocket
Connection: Upgrade
Authorization: Bearer <agent-token>
```

### Connection Parameters

| Parameter         | Value  | Notes                                      |
| ----------------- | ------ | ------------------------------------------ |
| Ping/Pong timeout | 45s    | 1.5x default heartbeat interval            |
| Reconnect backoff | 1s-60s | Exponential: 1, 2, 4, 8, 16, 32, 60, 60... |
| Close timeout     | 5s     | Max time to wait for graceful close        |
| Read buffer size  | 4KB    | Default WebSocket read buffer              |
| Write buffer size | 4KB    | Default WebSocket write buffer             |

### Duplicate Hostname Behavior

If an agent connects with a hostname that already exists:

- The new connection **replaces** the old one (last one wins)
- The old connection is closed with code 1008 (Policy Violation)
- This handles agent restarts and network reconnects gracefully

---

## Scenarios

### Scenario 1: Successful Connection

**Given** the dashboard is running and accepting WebSocket connections
**And** the agent has a valid token in the `Authorization` header
**When** the agent starts
**Then** the agent connects to the dashboard WebSocket endpoint
**And** the agent sends a "register" message with host information
**And** the dashboard acknowledges the registration

```json
// Agent sends (immediately after connection):
{
  "type": "register",
  "payload": {
    "hostname": "test-host",
    "host_type": "nixos",
    "agent_version": "2.1.0",
    "os_version": "24.11",
    "nixpkgs_version": "24.11.20241201.abc1234",
    "generation": "abc1234",
    "heartbeat_interval": 30
  }
}

// Dashboard responds:
{
  "type": "registered",
  "payload": {
    "host_id": "test-host"
  }
}
```

**Field Definitions:**

| Field                | Type   | Description                                                        |
| -------------------- | ------ | ------------------------------------------------------------------ |
| `hostname`           | string | System hostname (identifier)                                       |
| `host_type`          | string | "nixos" or "macos"                                                 |
| `agent_version`      | string | Agent software version (e.g., "2.1.0")                             |
| `os_version`         | string | OS version (e.g., "24.11" or "15.2")                               |
| `nixpkgs_version`    | string | Full nixpkgs version with commit                                   |
| `generation`         | string | Git commit hash of deployed config (7+ chars)                      |
| `heartbeat_interval` | int    | Seconds between heartbeats (dashboard uses 3x for stale detection) |

### Scenario 2: Invalid Token

**Given** the dashboard is running
**And** the agent has an invalid token
**When** the agent attempts to connect
**Then** the WebSocket upgrade is rejected with HTTP 401 Unauthorized
**And** the agent logs the authentication failure
**And** the agent retries with exponential backoff

```text
Agent logs:
ERROR connection failed: 401 Unauthorized
INFO  retrying in 1s...
ERROR connection failed: 401 Unauthorized
INFO  retrying in 2s...
```

### Scenario 3: Dashboard Unavailable

**Given** the dashboard is not running
**When** the agent starts
**Then** the agent logs a connection failure
**And** the agent retries with exponential backoff (1s, 2s, 4s, 8s, 16s, 32s, max 60s)
**And** the agent successfully connects when dashboard becomes available

### Scenario 4: Connection Lost

**Given** the agent is connected to the dashboard
**When** the network connection is interrupted
**Then** the agent detects the disconnection via ping/pong timeout (45s)
**And** the agent attempts to reconnect with backoff
**And** the agent successfully reconnects when network is restored
**And** the agent re-sends the register message after reconnecting

### Scenario 5: Graceful Shutdown

**Given** the agent is connected to the dashboard
**When** the agent receives SIGTERM
**Then** the agent sends a WebSocket close frame (code 1000, "shutdown")
**And** the agent waits up to 5 seconds for close acknowledgment
**And** the agent exits cleanly

### Scenario 6: Malformed Messages

**Given** the agent is connected to the dashboard
**When** the dashboard sends an invalid JSON message
**Then** the agent logs the parse error
**And** the agent continues operating (does not disconnect)
**And** the agent ignores the malformed message

---

## Verification Commands

```bash
# Start test dashboard
go run ./cmd/nixfleet-dashboard &

# Start agent with debug logging
NIXFLEET_URL=ws://localhost:8000/ws \
NIXFLEET_TOKEN=test-token \
NIXFLEET_LOG_LEVEL=debug \
go run ./cmd/nixfleet-agent

# Check dashboard logs for registration
curl http://localhost:8000/api/hosts | jq '.hosts[] | select(.id == "test-host")'

# Test reconnection: kill dashboard, wait, restart
kill %1
sleep 5
go run ./cmd/nixfleet-dashboard &
# Agent should reconnect within 10 seconds

# Test invalid token
NIXFLEET_TOKEN=bad-token go run ./cmd/nixfleet-agent
# Should see 401 errors and backoff retries
```

---

## Test Implementation

```go
// tests/integration/agent_test.go

func TestAgentConnection_Success(t *testing.T) {
    // Start mock dashboard
    // Start agent with valid token
    // Verify registration message received with all fields
    // Verify agent is listed in hosts
    // Verify heartbeat_interval is recorded
}

func TestAgentConnection_InvalidToken(t *testing.T) {
    // Start mock dashboard
    // Start agent with bad token
    // Verify 401 response (upgrade rejected)
    // Verify agent retries with backoff (check timing)
}

func TestAgentConnection_Reconnect(t *testing.T) {
    // Start mock dashboard
    // Start agent
    // Kill dashboard
    // Verify agent detects disconnect within 45s
    // Restart dashboard
    // Verify agent reconnects and re-registers
}

func TestAgentConnection_DuplicateHostname(t *testing.T) {
    // Start mock dashboard
    // Start agent 1 as "test-host"
    // Start agent 2 as "test-host"
    // Verify agent 1's connection is closed
    // Verify agent 2 is registered
}

func TestAgentConnection_MalformedMessage(t *testing.T) {
    // Start mock dashboard
    // Start agent
    // Send malformed JSON from dashboard
    // Verify agent logs error but continues operating
}
```

---

## Notes

- **Production**: Always use `wss://` (TLS) in production environments
- **Testing**: Examples use `ws://` for simplicity
- **Token format**: Opaque string, typically 32+ characters, stored hashed in dashboard DB
