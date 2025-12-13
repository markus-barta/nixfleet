# Agent Resilience & Autohealing

**Created**: 2025-12-13  
**Priority**: Medium → **Partially Complete**  
**Status**: In Progress

## Summary

Make the NixFleet agent more resilient by preventing duplicate processes and ensuring reliable startup on macOS.

## Problem Encountered (2025-12-13)

The agent was becoming stuck on `mba-mbp-work` with these symptoms:

1. **DNS timeout at boot**: Launchd starts the agent before network is ready, causing DNS resolution to time out
2. **Duplicate processes**: During `home-manager switch`, launchd restarts the agent while the old one is still running
3. **Buttons stay locked in UI**: Agent can't report status back due to network failures

**Root causes identified**:

- Launchd starts agents at login before network is fully initialized
- No process locking to prevent multiple agent instances
- Agent has no startup delay to wait for network readiness

## Solutions Implemented ✅

### 1. Network Readiness Check (launchd plist)

Added a network wait loop at the start of the launchd launch script:

```bash
# Wait for network to be ready (DNS resolution working)
MAX_WAIT=60
WAITED=0
while ! /usr/bin/host -W 2 fleet.barta.cm >/dev/null 2>&1; do
  if [ $WAITED -ge $MAX_WAIT ]; then
    echo "Warning: Network not ready after ${MAX_WAIT}s, starting anyway..." >&2
    break
  fi
  sleep 2
  WAITED=$((WAITED + 2))
done
```

**File**: `modules/home-manager.nix`

### 2. Process Locking (flock)

Added file-based locking using `flock` to prevent duplicate instances:

```bash
LOCK_FILE="/tmp/nixfleet-agent-${HOST_ID}.lock"

acquire_lock() {
  exec 200>"$LOCK_FILE"
  if ! flock -n 200; then
    log_error "Another agent instance is already running"
    return 1
  fi
  trap 'flock -u 200 2>/dev/null; rm -f "$LOCK_FILE"' EXIT
  return 0
}
```

**File**: `agent/nixfleet-agent.sh`

## Critical Review of Original Backlog

The original backlog (created by another LLM) proposed a multi-layered approach. Here's my assessment:

### ✅ Good Ideas (Implemented)

| Feature | Status | Notes |
|---------|--------|-------|
| Process Locking (flock) | ✅ Done | Simple and effective |
| Network wait for launchd | ✅ Done | Fixes the actual boot problem |

### ❌ Over-Engineered / Not Needed

| Feature | Why Not Needed |
|---------|----------------|
| **Duplicate Detection + Kill** | If you have proper locking, you don't need to detect and kill duplicates. Killing a locked process would be wrong. |
| **Health Monitoring (heartbeat file)** | The dashboard already tracks `last_seen` from heartbeats. Agent-side tracking is redundant. |
| **Self-healing via process killing** | Dangerous - could create race conditions. Let launchd/systemd handle restarts. |

### ⏳ Maybe Later (Low Priority)

| Feature | Notes |
|---------|-------|
| **Child Process Timeouts** | Could prevent truly stuck operations, but curl already has timeouts. Only consider if we see commands hanging. |

## Remaining Work

### High Priority

- [x] Add network wait to launchd plist
- [x] Add flock-based process locking

### Testing Needed

- [ ] Test agent startup at macOS login (cold boot)
- [ ] Test home-manager switch doesn't create duplicates
- [ ] Verify lock file cleanup on normal exit
- [ ] Test lock prevents second agent from starting

### Optional (Not Planned)

- [ ] Child process timeouts (only if needed)
- [ ] NixOS systemd service improvements (already has `After=network-online.target`)

## Files Modified

1. `modules/home-manager.nix` - Added network wait loop to launchd plist
2. `agent/nixfleet-agent.sh` - Added `acquire_lock()` function with flock

## Lessons Learned

1. **Start simple**: flock + network wait solved the problem. No need for complex self-healing.
2. **Trust the service manager**: Launchd/systemd handles restarts - don't duplicate that logic.
3. **Dashboard is the authority**: Status tracking belongs in the dashboard, not agent-side files.
4. **Test on real hardware**: The DNS timeout only appeared on actual macOS boot, not in SSH sessions.
