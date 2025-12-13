# Live Command Output Streaming & Async Heartbeats

**Created**: 2025-12-13  
**Priority**: HIGH  
**Status**: Backlog

---

## Problem

Currently, when a long-running command like `switch` runs (can take 20+ minutes), the dashboard shows no progress. Users must wait blindly without knowing:

- If the command is still running
- What the current build status is
- If there are any errors

Additionally, the agent is single-threaded - it cannot send heartbeats while executing a command. This causes:

- Dashboard shows host as "stale" during long commands
- No way to distinguish "busy building" from "agent crashed"

---

## Requirements

### 1. Live Command Output Streaming

- [ ] Stream command output (stdout/stderr) to dashboard in real-time
- [ ] Show build progress (e.g., "building derivation X of Y")
- [ ] Update dashboard UI with live log tail
- [ ] Store full command output for later review
- [ ] Handle large outputs gracefully (truncate old lines, keep recent)

### 2. Async Heartbeats During Command Execution

- [ ] Agent must send heartbeats while commands run
- [ ] Use background process or async mechanism (bash subshell, coprocess, or separate process)
- [ ] Heartbeat should include current command status: `running`, `building`, etc.
- [ ] Dashboard should show "busy" state, not "stale"

### 3. Progress Indicators

- [ ] Parse nixos-rebuild output for progress indicators:
  - "evaluating..." → evaluation phase
  - "building '/nix/store/...'" → build phase (count derivations)
  - "activating..." → activation phase
  - "switching to configuration..." → switch phase
- [ ] Report phase + progress to dashboard

---

## Technical Approaches

### Option A: Background Heartbeat Process

```bash
# Start heartbeat in background before command
(
  while true; do
    send_heartbeat "running" "Building..."
    sleep 10
  done
) &
HEARTBEAT_PID=$!

# Run command, capture output
nixos-rebuild switch 2>&1 | tee /tmp/switch.log | while read line; do
  # Stream lines to dashboard
  send_log_line "$line"
done

# Stop heartbeat
kill $HEARTBEAT_PID
```

### Option B: Named Pipe + Async Reader

```bash
mkfifo /tmp/nixfleet-output
# Reader in background sends lines to dashboard
cat /tmp/nixfleet-output | while read line; do send_log_line "$line"; done &

# Command writes to pipe
nixos-rebuild switch > /tmp/nixfleet-output 2>&1
```

### Option C: Rewrite Agent in Python/Go (WE WANT THIS)

- More complex but cleaner async handling
- Proper threading/async I/O
- Better error handling
- When going this route, we should also change the communication protocol to use WebSocket instead of SSE.
- Consider for v2.0

---

## Dashboard Changes

### API Endpoints

- `POST /api/hosts/{hostname}/logs` - Receive streaming log lines
- `GET /api/hosts/{hostname}/logs?since=TIMESTAMP` - Poll for new lines
- Or: WebSocket for true real-time streaming

### UI Components

- Log viewer panel (collapsible)
- Auto-scroll with pause on hover
- "View full log" link
- Phase indicator (Evaluating → Building → Activating → Done)

---

## Acceptance Criteria

- [ ] User can see live output from `switch` command in dashboard
- [ ] Dashboard shows "busy/building" instead of "stale" during long commands
- [ ] Heartbeats continue during command execution (every 10-30s)
- [ ] Full command output stored and retrievable after completion
- [ ] Works for all commands: switch, pull, test, update

---

## Related

- Agent script: `agent/nixfleet-agent.sh`
- Dashboard: `app/main.py`, `app/templates/dashboard.html`
- Existing heartbeat logic: `heartbeat()` function in agent

---

## Notes

- Bash coprocess (`coproc`) could work but has portability concerns
- Background subshell with trap cleanup is simplest
- Consider rate-limiting log streaming (batch lines, max 1 update/sec)
- WebSocket would be ideal but adds complexity (consider SSE as middle ground)
