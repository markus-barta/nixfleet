# T-P4400: Manual Test - Stale Command Cleanup

**Feature**: PRD FR-2.13 - Clear stale pending_command for offline hosts
**Backlog**: P4400

## Overview

This test verifies that stale `pending_command` entries are automatically cleared for hosts that have been offline longer than the configured threshold.

## Preconditions

- Dashboard is running
- At least one host is registered and online
- For faster testing, consider setting shorter thresholds:
  ```bash
  NIXFLEET_STALE_MINIMUM=1m  # 1 minute floor instead of 5 minutes
  NIXFLEET_STALE_CLEANUP_INTERVAL=30s  # Check every 30 seconds
  ```

---

## Test Case 1: Stale Command Cleared for Offline Host

**Objective**: Verify that `pending_command` is cleared after a host goes offline for longer than the threshold.

### Steps

1. **Identify a test host** that can be safely disconnected (e.g., stop the agent)

2. **Send a command** to the online host:
   - Click "Switch" or "Pull" button
   - Verify the command badge appears (yellow "switch" or "pull" badge)

3. **Stop the agent** before the command completes:

   ```bash
   # NixOS
   sudo systemctl stop nixfleet-agent

   # macOS
   launchctl unload ~/Library/LaunchAgents/com.nixfleet.agent.plist
   ```

4. **Verify host shows offline with command badge**:
   - Host row should be dimmed (50% opacity)
   - Status should show red offline dot
   - Command badge should still be visible

5. **Wait for cleanup threshold** (default 10 minutes, or shorter if configured):
   - Check dashboard logs for: `"cleared stale pending_command for offline host"`
   - Or wait for `StaleCleanupInterval` × cycles

6. **Verify badge is cleared**:
   - Refresh dashboard (or wait for WebSocket update)
   - Command badge should be removed
   - Host still shows offline but with no pending command

### Expected Results

- [ ] Command badge appears when command is sent
- [ ] Host shows offline with badge after agent stops
- [ ] Dashboard logs show cleanup message after threshold
- [ ] Badge is cleared in UI (via WebSocket or refresh)

---

## Test Case 2: Online Host Commands Not Affected

**Objective**: Verify that the cleanup job does NOT clear commands for online hosts.

### Steps

1. **Send a long-running command** to an online host:

   ```bash
   # Use test command that takes time, or just switch
   ```

2. **Verify the command badge appears** while host is online

3. **Wait past the stale threshold** (e.g., 10+ minutes)

4. **Verify the badge remains** (host is online, so not affected by cleanup)

5. **Let the command complete normally**

6. **Verify badge clears** only when command finishes

### Expected Results

- [ ] Badge appears immediately on command send
- [ ] Badge persists for online host past cleanup threshold
- [ ] Badge only clears when command completes (not by cleanup job)

---

## Test Case 3: Host Reconnects Before Cleanup

**Objective**: Verify that reconnection clears `pending_command` before the cleanup job runs.

### Steps

1. **Send a command** to an online host

2. **Stop the agent** (host goes offline with badge)

3. **Restart the agent** before cleanup threshold:

   ```bash
   # NixOS
   sudo systemctl start nixfleet-agent

   # macOS
   launchctl load ~/Library/LaunchAgents/com.nixfleet.agent.plist
   ```

4. **Verify host reconnects** and badge is cleared immediately

5. **Check that reconnection (not cleanup job) cleared the badge**:
   - Look for log message: `"agent registered"` (not "cleared stale pending_command")

### Expected Results

- [ ] Host reconnects and shows online
- [ ] Badge is cleared immediately on reconnection
- [ ] Cleanup job did NOT clear it (registration did)

---

## Test Case 4: Verify Logging

**Objective**: Verify that the cleanup job logs appropriate messages.

### Steps

1. **Check dashboard logs on startup** for cleanup loop initialization:

   ```
   stale command cleanup loop started
   ```

   With logged values for interval, threshold, and multiplier

2. **Trigger a cleanup** (per Test Case 1 steps)

3. **Verify cleanup log entry**:
   ```
   clearing stale pending_command for offline host  host=<hostname> command=<cmd> threshold=10m0s
   cleared stale commands for offline hosts  count=1 threshold=10m0s
   ```

### Expected Results

- [ ] Startup log shows cleanup loop initialized with config values
- [ ] Cleanup events are logged with host, command, and threshold

---

## Verification Commands

### Check database directly (on csb1):

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker
docker compose exec nixfleet cat /data/nixfleet.db > /tmp/nf.db
sqlite3 /tmp/nf.db "SELECT hostname, status, pending_command, last_seen FROM hosts"
```

### Check dashboard logs:

```bash
docker compose logs nixfleet --tail=100 -f | grep -E "(stale|cleanup|pending_command)"
```

---

## Configuration Reference

| Environment Variable              | Default | Description                        |
| --------------------------------- | ------- | ---------------------------------- |
| `NIXFLEET_HEARTBEAT_INTERVAL`     | 5s      | Reference heartbeat for multiplier |
| `NIXFLEET_STALE_MULTIPLIER`       | 120     | Missed heartbeats before stale     |
| `NIXFLEET_STALE_MINIMUM`          | 5m      | Floor for aggressive cleanup       |
| `NIXFLEET_STALE_CLEANUP_INTERVAL` | 1m      | How often cleanup runs             |

**Effective timeout**: `max(120 × 5s, 5m) = max(10m, 5m) = 10m`
