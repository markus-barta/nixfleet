# T01 - Agent Connection

**Backlog**: P4000 (Go Agent Core)
**Priority**: Must Have

---

## Purpose

Verify that the agent can connect to the dashboard via WebSocket and handle connection lifecycle properly.

---

## Prerequisites

- Dashboard running on `ws://localhost:8000/ws`
- Valid agent token configured
- Network connectivity

---

## Scenarios

### Scenario 1: Successful Connection

**Given** the dashboard is running and accepting WebSocket connections
**And** the agent has a valid token
**When** the agent starts
**Then** the agent connects to the dashboard WebSocket endpoint
**And** the agent sends a "register" message with host information
**And** the dashboard acknowledges the registration

```json
// Agent sends:
{
  "type": "register",
  "payload": {
    "hostname": "test-host",
    "host_type": "nixos",
    "agent_version": "2.0.0",
    "os_version": "24.11",
    "generation": "abc1234"
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

### Scenario 2: Invalid Token

**Given** the dashboard is running
**And** the agent has an invalid token
**When** the agent attempts to connect
**Then** the connection is rejected with 401 Unauthorized
**And** the agent logs the authentication failure
**And** the agent retries with exponential backoff

### Scenario 3: Dashboard Unavailable

**Given** the dashboard is not running
**When** the agent starts
**Then** the agent logs a connection failure
**And** the agent retries with exponential backoff (1s, 2s, 4s, 8s, max 60s)
**And** the agent successfully connects when dashboard becomes available

### Scenario 4: Connection Lost

**Given** the agent is connected to the dashboard
**When** the network connection is interrupted
**Then** the agent detects the disconnection (via ping/pong timeout)
**And** the agent attempts to reconnect with backoff
**And** the agent successfully reconnects when network is restored

### Scenario 5: Graceful Shutdown

**Given** the agent is connected to the dashboard
**When** the agent receives SIGTERM
**Then** the agent sends a close message to the dashboard
**And** the agent exits cleanly within 5 seconds

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
```

---

## Test Implementation

```go
// tests/integration/agent_test.go

func TestAgentConnection_Success(t *testing.T) {
    // Start mock dashboard
    // Start agent
    // Verify registration message received
    // Verify agent is listed in hosts
}

func TestAgentConnection_InvalidToken(t *testing.T) {
    // Start mock dashboard
    // Start agent with bad token
    // Verify 401 response
    // Verify agent retries with backoff
}

func TestAgentConnection_Reconnect(t *testing.T) {
    // Start mock dashboard
    // Start agent
    // Kill dashboard
    // Verify agent detects disconnect
    // Restart dashboard
    // Verify agent reconnects
}
```
