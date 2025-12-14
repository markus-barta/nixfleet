# T05 - Dashboard WebSocket

**Backlog**: P4200 (Go Dashboard Core)
**Priority**: Must Have

---

## Purpose

Verify that the dashboard correctly handles WebSocket connections from both agents and browsers.

---

## Prerequisites

- Dashboard running
- Valid agent token for agent connections
- Valid session for browser connections

---

## Scenarios

### Scenario 1: Agent WebSocket Connection

**Given** the dashboard is running
**When** an agent connects to `/ws` with a valid Bearer token
**Then** the connection is accepted
**And** the agent is registered in the WebSocket hub
**And** the agent can send/receive messages

```http
GET /ws HTTP/1.1
Upgrade: websocket
Authorization: Bearer <agent-token>
```

### Scenario 2: Browser WebSocket Connection

**Given** the dashboard is running
**And** a user is logged in with a valid session
**When** the browser connects to `/ws` with the session cookie
**Then** the connection is accepted
**And** the browser is registered in the WebSocket hub
**And** the browser receives host update broadcasts

### Scenario 3: Reject Unauthenticated WebSocket

**Given** the dashboard is running
**When** a WebSocket connection is attempted without authentication
**Then** the connection is rejected with 401

### Scenario 4: Agent Message Routing

**Given** an agent is connected
**And** a browser is connected
**When** the agent sends a "heartbeat" message
**Then** the dashboard updates the host state
**And** the dashboard broadcasts "host_update" to all browsers

### Scenario 5: Command Message Routing

**Given** an agent is connected
**And** the browser dispatches a command for that host
**When** the command is queued
**Then** the dashboard sends "command" message to the specific agent
**And** the dashboard broadcasts "command_queued" to all browsers

### Scenario 6: Output Streaming

**Given** an agent is connected
**And** browsers are connected
**When** the agent sends "output" messages during command execution
**Then** the dashboard forwards "command_output" to all browsers
**And** messages arrive in order with < 100ms latency

### Scenario 7: Connection Cleanup

**Given** an agent is connected
**When** the agent disconnects (network failure or close)
**Then** the agent is removed from the WebSocket hub
**And** the dashboard marks the host as potentially offline (after timeout)

### Scenario 8: Multiple Browsers

**Given** multiple browsers are connected
**When** a host update occurs
**Then** all browsers receive the update simultaneously

### Scenario 9: Ping/Pong Keepalive

**Given** an agent is connected
**When** no messages are exchanged for 30 seconds
**Then** the dashboard sends a ping
**And** the agent responds with a pong
**And** the connection stays alive

---

## Verification Commands

```bash
# Test agent WebSocket (using websocat)
websocat -H "Authorization: Bearer $TOKEN" ws://localhost:8000/ws

# Send register message:
{"type": "register", "payload": {"hostname": "test"}}

# Test browser WebSocket (with cookie)
websocat -H "Cookie: nixfleet_session=$SESSION" ws://localhost:8000/ws

# Observe host updates
```

---

## Test Implementation

```go
// tests/integration/dashboard_test.go

func TestDashboardWebSocket_AgentConnection(t *testing.T) {
    // Connect with valid token
    // Send register message
    // Verify agent in hub
}

func TestDashboardWebSocket_BrowserConnection(t *testing.T) {
    // Login to get session
    // Connect WebSocket with cookie
    // Verify browser in hub
}

func TestDashboardWebSocket_MessageRouting(t *testing.T) {
    // Connect agent and browser
    // Agent sends heartbeat
    // Verify browser receives host_update
}

func TestDashboardWebSocket_OutputStreaming(t *testing.T) {
    // Connect agent and browser
    // Agent sends output messages
    // Verify browser receives all in order
    // Verify latency < 100ms
}
```
