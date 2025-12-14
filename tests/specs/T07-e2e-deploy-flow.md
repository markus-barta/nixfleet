# T07 - End-to-End Deploy Flow

**Backlog**: P4000, P4200 (Agent + Dashboard)
**Priority**: Must Have

---

## Purpose

Verify the complete deployment flow from user click to successful switch, including real-time output streaming.

---

## Prerequisites

- Dashboard running
- Agent connected on a real or simulated NixOS/macOS host
- Git repository with valid configuration
- User authenticated in browser

---

## Flow Diagram

```text
┌─────────┐       ┌───────────┐       ┌─────────┐       ┌──────────┐
│ Browser │       │ Dashboard │       │  Agent  │       │   Host   │
└────┬────┘       └─────┬─────┘       └────┬────┘       └────┬─────┘
     │                  │                  │                  │
     │ Click "Switch"   │                  │                  │
     │─────────────────▶│                  │                  │
     │                  │                  │                  │
     │                  │ WS: command      │                  │
     │                  │─────────────────▶│                  │
     │                  │                  │                  │
     │ WS: command_queued                  │                  │
     │◀─────────────────│                  │                  │
     │                  │                  │                  │
     │                  │                  │ nixos-rebuild    │
     │                  │                  │─────────────────▶│
     │                  │                  │                  │
     │                  │ WS: output       │ stdout lines     │
     │                  │◀─────────────────│◀─────────────────│
     │                  │                  │                  │
     │ WS: command_output                  │                  │
     │◀─────────────────│                  │                  │
     │ (repeat for each line)              │                  │
     │                  │                  │                  │
     │                  │ WS: heartbeat    │                  │
     │◀─────────────────│◀─────────────────│                  │
     │ (host still online during build)    │                  │
     │                  │                  │                  │
     │                  │ WS: status       │ exit 0           │
     │                  │◀─────────────────│◀─────────────────│
     │                  │                  │                  │
     │ WS: host_update (success)           │                  │
     │◀─────────────────│                  │                  │
     │                  │                  │                  │
```

---

## Scenarios

### Scenario 1: Successful Pull + Switch

**Given** a user is logged into the dashboard
**And** an agent for "hsb0" is connected
**And** there are new commits in the config repository
**When** the user clicks "Pull" on hsb0
**Then** the UI shows "Pulling..." and disables buttons
**And** the agent executes git pull
**And** output streams to the browser in real-time
**And** the host generation updates to new commit hash
**When** the user clicks "Switch"
**Then** the UI shows "Switching..." and disables buttons
**And** the agent executes nixos-rebuild switch
**And** build output streams to the browser
**And** heartbeats continue during the build (host stays "online")
**And** after completion, the UI shows success
**And** buttons are re-enabled

**Success Criteria:**

- [ ] Total time from click to output start < 2 seconds
- [ ] Output lines appear within 1 second of being produced
- [ ] Host never shows as "stale" during the build
- [ ] Final status correctly indicates success/failure

### Scenario 2: Pull + Switch Failure

**Given** the configuration has a syntax error
**When** the user triggers a switch
**Then** nixos-rebuild fails with an error
**And** the error output is streamed to the browser
**And** the UI shows "Error" status
**And** the host generation is NOT updated
**And** buttons are re-enabled for retry

### Scenario 3: Network Interruption During Switch

**Given** a switch is in progress
**When** the network connection to the dashboard is interrupted
**Then** the agent continues the switch to completion
**And** the agent reconnects after the switch
**And** the agent reports the final status
**And** the browser shows the result after reconnection

---

## Verification Steps

### Manual Test

```bash
# 1. Ensure dashboard is running
curl http://localhost:8000/health

# 2. Verify agent is connected
curl http://localhost:8000/api/hosts | jq '.hosts[] | select(.id == "hsb0") | .online'
# Should be true

# 3. Open dashboard in browser
open http://localhost:8000

# 4. Click "Switch" on hsb0

# 5. Observe:
#    - Button shows "Switching..."
#    - Log viewer opens with streaming output
#    - Status indicator stays "online"
#    - After completion, shows success/failure
```

### Automated Test

```go
func TestE2E_DeployFlow(t *testing.T) {
    // Setup
    dashboard := startDashboard(t)
    agent := startAgent(t, "hsb0")
    browser := connectBrowserWS(t, dashboard)

    // Wait for agent registration
    waitForHostOnline(t, dashboard, "hsb0")

    // Send pull command
    sendCommand(t, dashboard, "hsb0", "pull")

    // Verify command received by agent
    cmd := agent.waitForCommand(t)
    assert.Equal(t, "pull", cmd)

    // Agent executes and streams output
    agent.streamOutput(t, "Fetching origin\n")
    agent.streamOutput(t, "HEAD is now at abc1234\n")

    // Verify browser receives output
    output := browser.waitForMessage(t, "command_output")
    assert.Contains(t, output, "Fetching origin")

    // Agent reports success
    agent.reportStatus(t, "ok", "abc1234")

    // Verify browser receives update
    update := browser.waitForMessage(t, "host_update")
    assert.Equal(t, "ok", update.Status)
    assert.Equal(t, "abc1234", update.Generation)
}
```

---

## Performance Criteria

| Metric                   | Target               |
| ------------------------ | -------------------- |
| Command dispatch latency | < 500ms              |
| Output line latency      | < 1 second           |
| Heartbeat continuity     | No gaps > 45 seconds |
| UI update latency        | < 500ms              |

---

## Related

- T01: Agent Connection (prerequisite)
- T02: Agent Heartbeat (heartbeat during build)
- T03: Agent Commands (command execution)
- T05: Dashboard WebSocket (message routing)
