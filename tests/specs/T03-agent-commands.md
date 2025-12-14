# T03 - Agent Commands

**Backlog**: P4000 (Go Agent Core)
**Priority**: Must Have

---

## Purpose

Verify that the agent correctly executes commands and streams output to the dashboard.

---

## Prerequisites

- Dashboard running and agent connected
- Git repository accessible (for pull commands)
- nixos-rebuild or home-manager available (for switch commands)

---

## Scenarios

### Scenario 1: Execute Pull Command

**Given** the agent is connected
**And** the config repository is configured
**When** the dashboard sends a "pull" command
**Then** the agent executes `git fetch && git reset --hard` (isolated mode)
**And** the agent streams git output to the dashboard
**And** the agent reports success with new generation hash

```json
// Dashboard sends:
{"type": "command", "payload": {"command": "pull"}}

// Agent streams:
{"type": "output", "payload": {"line": "Fetching origin", "command": "pull"}}
{"type": "output", "payload": {"line": "HEAD is now at abc1234", "command": "pull"}}

// Agent reports:
{"type": "status", "payload": {"status": "ok", "generation": "abc1234", "message": "Pull: Updated to abc1234"}}
```

### Scenario 2: Execute Switch Command (NixOS)

**Given** the agent is connected on a NixOS host
**When** the dashboard sends a "switch" command
**Then** the agent executes `nixos-rebuild switch --flake .#hostname`
**And** the agent streams build output to the dashboard
**And** heartbeats continue during the build
**And** the agent reports success or failure

### Scenario 3: Execute Switch Command (macOS)

**Given** the agent is connected on a macOS host
**When** the dashboard sends a "switch" command
**Then** the agent executes `home-manager switch --flake .#user@hostname`
**And** the agent streams build output to the dashboard
**And** the agent reports success or failure

### Scenario 4: Execute Test Command

**Given** the agent is connected
**And** test scripts exist in `hosts/{hostname}/tests/T*.sh`
**When** the dashboard sends a "test" command
**Then** the agent runs each test script in order
**And** the agent reports progress (current/total/passed)
**And** the agent reports final results (pass/fail counts)

```json
// Progress updates:
{"type": "test_progress", "payload": {"current": 1, "total": 5, "passed": 1, "running": true}}
{"type": "test_progress", "payload": {"current": 2, "total": 5, "passed": 2, "running": true}}
...
{"type": "test_progress", "payload": {"current": 5, "total": 5, "passed": 4, "running": false, "result": "4/5 passed"}}
```

### Scenario 5: Stop Running Command

**Given** a command is currently executing
**When** the dashboard sends a "stop" command
**Then** the agent kills the running process (SIGTERM, then SIGKILL)
**And** the agent reports "Stopped by user"
**And** the agent is ready for new commands

### Scenario 6: Command Output Streaming

**Given** a command produces output
**When** each line is written to stdout/stderr
**Then** the line is sent to the dashboard within 100ms
**And** stderr lines are marked as errors
**And** output is also saved to local log file

### Scenario 7: Command Failure

**Given** a command fails (non-zero exit code)
**When** the command completes
**Then** the agent reports status "error"
**And** the error message includes relevant output
**And** the agent is ready for new commands

---

## Verification Commands

```bash
# Test pull command
curl -X POST http://localhost:8000/api/hosts/test-host/command \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{"command": "pull"}'

# Watch output in dashboard logs or WebSocket

# Test stop command (while long command running)
curl -X POST http://localhost:8000/api/hosts/test-host/command \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{"command": "stop"}'
```

---

## Test Implementation

```go
// tests/integration/agent_test.go

func TestAgentCommand_Pull(t *testing.T) {
    // Setup: mock git repo
    // Send pull command
    // Verify git operations executed
    // Verify output streamed
    // Verify status reported
}

func TestAgentCommand_OutputStreaming(t *testing.T) {
    // Send command that produces many lines
    // Verify each line received in order
    // Verify latency < 100ms per line
}

func TestAgentCommand_Stop(t *testing.T) {
    // Start long-running command
    // Send stop command
    // Verify process killed
    // Verify agent still responsive
}

func TestAgentCommand_Failure(t *testing.T) {
    // Send command that will fail
    // Verify error status reported
    // Verify error message present
}
```
