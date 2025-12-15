# T-P2000: Hub Resilience Test

**Feature**: P2000 - Hub Resilience & Deadlock Fix
**Priority**: CRITICAL
**Last Verified**: 2025-12-15

---

## Preconditions

- [ ] NixFleet dashboard is running at https://fleet.barta.cm/
- [ ] User is logged in to the dashboard
- [ ] At least one agent is online
- [ ] SSH access to csb1 for log viewing

---

## Test Steps

### Step 1: Verify Hub Is Processing Messages

1. Open the dashboard
2. Verify at least one host shows as online

**Expected**: Hosts display with green status indicators

3. Click "Pull" on any online host

**Expected**: Command is sent and status changes to show pending

### Step 2: Verify No Deadlock on Agent Disconnect

1. SSH to csb1 and watch logs:
   ```bash
   ssh mba@cs1.barta.cm -p 2222
   cd ~/docker && docker compose logs -f nixfleet
   ```
2. On another machine with an agent, stop the agent:
   ```bash
   systemctl --user stop nixfleet-agent
   ```
3. Watch the dashboard logs

**Expected**:

- Log shows "client unregistered" message
- No freeze or hang
- Other hosts still update normally

4. Restart the agent:
   ```bash
   systemctl --user start nixfleet-agent
   ```

**Expected**: Agent reconnects and shows online again

### Step 3: Verify Graceful Shutdown

1. SSH to csb1
2. Restart the dashboard container:
   ```bash
   cd ~/docker && docker compose restart nixfleet
   ```
3. Watch the logs

**Expected**:

- Log shows "hub shutting down gracefully"
- Container restarts cleanly
- Agents reconnect within seconds

### Step 4: Verify Broadcast Queue (Stress Test)

1. Open multiple browser tabs with the dashboard (5+)
2. Trigger several commands in quick succession

**Expected**:

- All browsers receive updates
- No "broadcast queue full" warnings in logs
- Dashboard remains responsive

---

## Pass/Fail Criteria

| Criterion                         | Pass | Fail |
| --------------------------------- | ---- | ---- |
| Hub processes commands            | [ ]  | [ ]  |
| No deadlock on agent disconnect   | [ ]  | [ ]  |
| Agent reconnects after restart    | [ ]  | [ ]  |
| Graceful shutdown logged          | [ ]  | [ ]  |
| Multiple browsers receive updates | [ ]  | [ ]  |
| No queue overflow warnings        | [ ]  | [ ]  |

**Overall Result**: [ ] PASS / [ ] FAIL

---

## Key Log Messages to Look For

### Good Signs

- `"client registered"` - Agent/browser connected
- `"client unregistered"` - Clean disconnect
- `"hub shutting down gracefully"` - Clean shutdown
- `"heartbeat received"` - Agents communicating

### Bad Signs

- No logs at all (frozen hub)
- `"hub panic"` - Crash (should recover)
- `"broadcast queue full"` - Backpressure hit

---

## Notes

_Record any observations or issues here_
