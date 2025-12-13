# T01: Pull Command

Test the git pull command dispatch from dashboard to agent.

## Prerequisites

- NixFleet dashboard running
- Agent registered and online
- Git repository with remote configured
- Network access to git remote

## What This Test Verifies

| Component      | Verification                                    |
| -------------- | ----------------------------------------------- |
| Dashboard UI   | Pull button dispatches command                  |
| API Endpoint   | Command stored for agent polling                |
| Agent Function | `do_pull()` executes git operations             |
| Status Report  | Agent reports success/failure back to dashboard |
| Dashboard UI   | Status column updates with result               |

## Command Flow

```
┌─────────────┐     POST /command      ┌─────────────┐
│  Dashboard  │ ──────────────────────>│   Server    │
│  (Browser)  │                        │   (API)     │
└─────────────┘                        └─────────────┘
                                              │
                                              │ stores command
                                              ▼
┌─────────────┐     GET /poll          ┌─────────────┐
│    Agent    │ <──────────────────────│   Server    │
│   (Host)    │                        │   (API)     │
└─────────────┘                        └─────────────┘
       │
       │ executes: git pull
       │
       ▼
┌─────────────┐     POST /status       ┌─────────────┐
│    Agent    │ ──────────────────────>│   Server    │
│   (Host)    │                        │   (API)     │
└─────────────┘                        └─────────────┘
```

## Manual Test Procedures

### Test 1: UI Pull Button

**Steps:**

1. Open dashboard: <https://fleet.barta.cm>
2. Find a host row with agent online
3. Click "Pull" button
4. Observe status column

**Expected Results:**

- Button shows loading state briefly
- Status shows "⧖ Pulling..."
- After completion: "✓ Pull successful" or "✗ Pull failed: reason"

**Status:** ⏳ Pending

### Test 2: Agent Receives Command

**Steps:**

1. Trigger pull from dashboard
2. Check agent logs:

   ```bash
   tail -f /tmp/nixfleet-agent.err
   ```

**Expected Results:**

- Log shows: `[INFO] Received command: pull`
- Log shows: `[INFO] Executing: pull`
- Log shows git output or error

**Status:** ⏳ Pending

### Test 3: Git Pull Execution

**Steps:**

1. SSH to target host
2. Check nixcfg directory:

   ```bash
   cd ~/Code/nixcfg
   git log --oneline -1
   ```

3. Trigger pull from dashboard
4. Check git log again

**Expected Results:**

- Git HEAD matches remote after pull
- No uncommitted changes lost
- Merge conflicts reported if any

**Status:** ⏳ Pending

### Test 4: Status Reporting

**Steps:**

1. Trigger pull from dashboard
2. Watch status column in real-time
3. Expand status papertrail (▼ button)

**Expected Results:**

- Status updates via SSE in real-time
- Papertrail shows timestamp and result
- Multiple pulls create history entries

**Status:** ⏳ Pending

### Test 5: Error Handling

**Steps:**

1. Create a situation that causes pull to fail:
   - Disconnect network
   - Or create uncommitted changes that conflict
2. Trigger pull from dashboard
3. Check status

**Expected Results:**

- Status shows "✗ Pull failed: ..."
- Error message is descriptive
- Agent continues running (doesn't crash)

**Status:** ⏳ Pending

## Agent Function Reference

The agent's `do_pull()` function:

```bash
do_pull() {
    log_info "Pulling latest changes..."
    cd "$NIXCFG_PATH" || { log_error "Cannot cd to $NIXCFG_PATH"; return 1; }

    if git pull 2>&1; then
        report_status "success" "Pull successful"
        return 0
    else
        report_status "error" "Pull failed: git error"
        return 1
    fi
}
```

## Summary

- Total Tests: 5
- Passed: 0
- Pending: 5

## Related

- Automated: [T01-command-pull.sh](./T01-command-pull.sh)
- Agent: [nixfleet-agent.sh](../agent/nixfleet-agent.sh) - `do_pull()` function
- Dashboard: [main.py](../app/main.py) - `/api/hosts/{host_id}/command` endpoint
