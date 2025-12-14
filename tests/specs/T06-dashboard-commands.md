# T06 - Dashboard Commands

**Backlog**: P4200 (Go Dashboard Core)
**Priority**: Must Have

---

## Purpose

Verify that the dashboard correctly dispatches commands to agents and tracks their status.

---

## Prerequisites

- Dashboard running with agent connected
- User authenticated with valid session
- CSRF token available

---

## Scenarios

### Scenario 1: Queue Pull Command

**Given** an agent is connected for host "hsb0"
**And** the user is authenticated
**When** the user sends POST `/api/hosts/hsb0/command` with `{"command": "pull"}`
**And** includes a valid CSRF token
**Then** the dashboard sends a "command" message to the agent
**And** the dashboard returns `{"status": "queued", "command": "pull"}`
**And** a "command_queued" event is broadcast to browsers

### Scenario 2: Queue Switch Command

**Given** an agent is connected for host "hsb0"
**When** the user sends a "switch" command
**Then** the dashboard sends the command to the agent
**And** the host status shows "Switching..."

### Scenario 3: Reject Command for Offline Host

**Given** no agent is connected for host "hsb0"
**And** the host's last_seen is > 5 minutes ago
**When** the user sends a command
**Then** the dashboard returns 409 Conflict
**And** the error message indicates the host is offline

### Scenario 4: Reject Without Authentication

**Given** the user is not authenticated
**When** the user sends a command request
**Then** the dashboard returns 401 Unauthorized

### Scenario 5: Reject Without CSRF Token

**Given** the user is authenticated
**When** the user sends a command without X-CSRF-Token header
**Then** the dashboard returns 403 Forbidden
**And** the command is not executed

### Scenario 6: Command Status Update

**Given** a command was sent to an agent
**When** the agent reports status "ok" with output
**Then** the dashboard updates the host record
**And** the dashboard broadcasts "host_update" to browsers
**And** the command log is updated with result

### Scenario 7: Test Command Progress

**Given** a "test" command is running
**When** the agent sends test_progress updates
**Then** the dashboard broadcasts "test_progress" to browsers
**And** the host record shows current test progress

### Scenario 8: Stop Command

**Given** a command is currently running
**When** the user sends a "stop" command
**Then** the dashboard sends "stop" to the agent
**And** the previous command is interrupted
**And** the host status returns to normal

---

## Verification Commands

```bash
# Get CSRF token (from login response or dashboard page)
CSRF_TOKEN="..."

# Queue a command
curl -b cookies.txt -X POST http://localhost:8000/api/hosts/hsb0/command \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{"command": "pull"}'

# Check host status
curl -b cookies.txt http://localhost:8000/api/hosts | jq '.hosts[] | select(.id == "hsb0")'

# Check command log
curl -b cookies.txt http://localhost:8000/api/hosts/hsb0/logs
```

---

## Test Implementation

```go
// tests/integration/dashboard_test.go

func TestDashboardCommand_Queue(t *testing.T) {
    // Login
    // Connect mock agent
    // POST command with CSRF
    // Verify agent receives command
    // Verify response is "queued"
}

func TestDashboardCommand_OfflineHost(t *testing.T) {
    // Create host without agent
    // POST command
    // Verify 409 response
}

func TestDashboardCommand_StatusUpdate(t *testing.T) {
    // Send command
    // Agent reports status
    // Verify host record updated
    // Verify browsers notified
}

func TestDashboardCommand_TestProgress(t *testing.T) {
    // Send test command
    // Agent sends progress updates
    // Verify browsers receive progress
}
```
