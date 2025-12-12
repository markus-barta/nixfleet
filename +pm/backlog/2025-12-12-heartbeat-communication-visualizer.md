# Heartbeat & Communication Visualizer

**Created**: 2025-12-12
**Priority**: Medium
**Status**: Ready (Spec Complete)

---

## Goal

Visualize real-time communication between each host and the NixFleet backend:

- **Incoming** (host → dashboard): Heartbeats, status updates
- **Outgoing** (dashboard → host): Commands with ACK/NACK and timeout
- **Connection health**: Online → Stale → Offline states

---

## Configuration

### New Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `poll_interval` | 30s | How often agent polls for commands |
| `command_timeout` | 60s | How long to wait for command ACK before timeout |
| `offline_multiplier` | 3x | Offline after `3 × poll_interval` without heartbeat |

---

## Design Specification

### Layout (Per Host Row)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│   [INCOMING]        [CENTER]        [OUTGOING]                          │
│   · · · · · · · ●    ⬡             ● · · · · · · ·                      │
│   ←←←←←←←←←←←←←←                    →→→→→→→→→→→→→→                      │
│                                                                         │
│   15 dots (2px)                     15 dots (2px)                       │
│   1px spacing                       1px spacing                         │
│   Right-to-Left                     Left-to-Right                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Dimensions:**

- 15 dots per column
- 2px × 2px per dot
- 1px spacing between dots
- Center symbol: ⬡ (hexagon, static)

---

## Incoming Heartbeat (Left Column)

### Animation Flow (Right → Left)

Timer is **relative to last heartbeat** — resets to 0 when heartbeat received.

### States

| Time Since Last | Visual State | Description |
|-----------------|--------------|-------------|
| **0s** | Ripple effect | Heartbeat just received |
| **0–10s** | Green ripple animation | 1/3 of interval (10s for 30s poll) |
| **10s** | Static green center dot | Ripple stops, dot remains |
| **10–30s** | Dots light up right→left | Gray-blue dots animate, ~1 every 2s |
| **30s** | All dots lit, waiting | Expected heartbeat window |
| **30–90s** | Yellow stale | All dots + center turn yellow |
| **90s+** | Dark gray offline | 3× interval missed |

### Color Palette (Tokyo Night)

| State | Element | Color | Hex |
|-------|---------|-------|-----|
| Unlit | Dots | Very dark gray | `#1a1b26` |
| Animating | Dots | Blue-gray | `#565f89` |
| Active | Center dot | Green | `#9ece6a` |
| Stale | All dots + center | Amber | `#e0af68` |
| Offline | Center dot | Dark gray | `#414868` |

---

## Outgoing Commands (Right Column)

### Command Lifecycle

Commands now have explicit states with ACK/NACK:

```
┌─────────┐     ┌──────────┐     ┌───────────┐     ┌──────────┐
│ QUEUED  │────▶│ RECEIVED │────▶│ EXECUTING │────▶│ COMPLETE │
└─────────┘     └──────────┘     └───────────┘     └──────────┘
     │               │                 │                 │
     ▼               ▼                 ▼                 ▼
  Timeout         Timeout           Error            Success
  (no ACK)        (no exec)         (failed)         (done)
```

### API Changes (Agent)

**New endpoint or message types:**

1. **ACK** — Agent received command: `POST /api/hosts/{id}/command-ack`
2. **EXEC** — Agent started executing: `POST /api/hosts/{id}/command-exec`
3. **DONE** — Command completed: (existing status endpoint)

### Animation Flow (Left → Right)

| Dot Position | State | Trigger |
|--------------|-------|---------|
| Dot 1 | **Queued** | User clicks action button |
| Dot 2 | **Received (ACK)** | Agent acknowledges receipt |
| Dot 3+ | **Executing** | Agent confirms execution started |
| All fade | **Complete** | Command finished (success or error) |

### Timeout Handling

Based on `command_timeout` setting (default 60s):

| Time Since Queued | Visual State | Action |
|-------------------|--------------|--------|
| **0s** | Dot 1 lights (queued) | Command stored |
| **0–60s** | Waiting for ACK | Dots animate toward timeout |
| **60s (no ACK)** | Dot turns red | Command timed out (NACK) |
| **ACK received** | Dot 2 lights | Proceed to execution |
| **60s after ACK (no EXEC)** | Dots turn red | Execution timeout |

### Dot Animation (Timeout Visualization)

Similar to incoming, the outgoing column shows time remaining:

- 15 dots animate **left → right** over `command_timeout` period
- If command progresses (ACK, EXEC), dots reset/advance
- If timeout reached without progress, dots turn red

### Color Palette (Outgoing)

| State | Color | Hex |
|-------|-------|-----|
| Queued | Blue | `#7aa2f7` |
| ACK received | Cyan | `#7dcfff` |
| Executing | Bright blue | `#7aa2f7` |
| Success | Green (fade) | `#9ece6a` |
| Error | Red (flash) | `#f7768e` |
| Timeout | Red (stays) | `#f7768e` |

---

## SSE Events (Backend → Browser)

### New Events Required

| Event | Trigger | Payload |
|-------|---------|---------|
| `heartbeat` | Agent polls/registers | `{host_id, last_seen, metrics}` |
| `command_queued` | User queues command | `{host_id, command, queued_at}` |
| `command_ack` | Agent ACKs command | `{host_id, command, acked_at}` |
| `command_exec` | Agent starts execution | `{host_id, command, started_at}` |
| `command_done` | Command finished | `{host_id, command, status, output}` |
| `command_timeout` | No ACK/progress | `{host_id, command, timeout_type}` |

---

## Agent Changes

### New API Calls

```bash
# After receiving command from poll
api_call POST "/api/hosts/${HOST_ID}/command-ack" '{"command": "switch"}'

# Before executing command
api_call POST "/api/hosts/${HOST_ID}/command-exec" '{"command": "switch"}'

# After completion (existing)
report_status "ok" "$gen" "$output"
```

### Updated Command Flow

```bash
# In agent main loop:
command=$(poll_command)
if [[ -n "$command" ]]; then
    # ACK: Tell dashboard we received the command
    api_call POST "/api/hosts/${HOST_ID}/command-ack" "{\"command\": \"$command\"}"
    
    case "$command" in
        switch)
            # EXEC: Tell dashboard we're starting
            api_call POST "/api/hosts/${HOST_ID}/command-exec" "{\"command\": \"$command\"}"
            do_switch
            ;;
        # ... etc
    esac
fi
```

---

## Backend Changes

### Database

```sql
-- Add command tracking columns
ALTER TABLE hosts ADD COLUMN command_acked_at TEXT;
ALTER TABLE hosts ADD COLUMN command_exec_at TEXT;
ALTER TABLE hosts ADD COLUMN command_timeout_at TEXT;
```

### New Endpoints

```python
@app.post("/api/hosts/{host_id}/command-ack")
async def command_ack(host_id: str, ...):
    """Agent acknowledges receiving a command."""
    # Update command_acked_at
    # Broadcast SSE event
    
@app.post("/api/hosts/{host_id}/command-exec")
async def command_exec(host_id: str, ...):
    """Agent confirms execution started."""
    # Update command_exec_at
    # Broadcast SSE event
```

### Timeout Detection

Background task or on-poll check:

```python
# If command_queued_at is set but command_acked_at is NULL
# and now() - command_queued_at > command_timeout:
#   → Set command_timeout, broadcast SSE
```

---

## Frontend (JavaScript)

### State Machine Per Host

```javascript
const hostState = {
    // Incoming
    lastHeartbeat: Date,
    pollInterval: 30,
    heartbeatPhase: 'active' | 'waiting' | 'stale' | 'offline',
    
    // Outgoing
    commandState: null | 'queued' | 'acked' | 'executing' | 'complete' | 'timeout',
    commandQueuedAt: Date,
    commandAckedAt: Date,
    commandExecAt: Date,
    commandTimeout: 60,
};
```

### Animation Loop

```javascript
// Every 100ms or requestAnimationFrame
function updateHostVisualizer(host) {
    // Incoming: calculate which dots should be lit
    const sinceHeartbeat = (Date.now() - host.lastHeartbeat) / 1000;
    const incomingDots = Math.floor(sinceHeartbeat / 2); // 1 dot per 2s
    
    // Outgoing: calculate based on command state
    // ...
}
```

---

## Mobile Behavior

On screens < 768px width:

- **Hide** the dot animation columns
- **Show only** the center status indicator (⬡ with color)
- Fallback to current simple status dot behavior

---

## Multiple Commands

If a new command is queued while one is executing:

- **Overwrite** the pending command (current behavior)
- **Log** the override in command_log table
- **Show** in Comments column: "Command overwritten"

---

## Acceptance Criteria

### Incoming (Heartbeat)

- [ ] 15 dots (2px, 1px spacing) animate right→left
- [ ] Ripple effect on heartbeat (10s duration for 30s interval)
- [ ] Static green dot after ripple
- [ ] Yellow stale after 1× interval (30s)
- [ ] Dark gray offline after 3× interval (90s)

### Outgoing (Commands)

- [ ] 15 dots animate left→right based on command_timeout
- [ ] Agent sends ACK on command receipt
- [ ] Agent sends EXEC on execution start
- [ ] Timeout detection with red indicator
- [ ] Success/error states with appropriate colors

### Center Symbol

- [ ] Static hexagon ⬡ between columns
- [ ] Color reflects overall host state

### SSE Events

- [ ] `command_ack` event broadcast
- [ ] `command_exec` event broadcast
- [ ] `command_timeout` event broadcast
- [ ] Browser updates in real-time

### Agent

- [ ] Sends ACK after receiving command
- [ ] Sends EXEC before running command
- [ ] Graceful handling if dashboard unreachable

### Mobile

- [ ] Dots hidden on narrow screens
- [ ] Simple status dot shown instead

---

## Implementation Order

1. **Backend**: Add new API endpoints (command-ack, command-exec)
2. **Agent**: Add ACK/EXEC calls to command flow
3. **Backend**: Add timeout detection logic
4. **Frontend**: Build dot visualizer component
5. **Frontend**: Wire up SSE events to visualizer
6. **Testing**: Verify all state transitions

---

## References

- Dashboard template: `app/templates/dashboard.html`
- SSE implementation: `app/main.py` (broadcast_event)
- Agent: `agent/nixfleet-agent.sh`
- Current status column: search for "ripple" in dashboard.html
