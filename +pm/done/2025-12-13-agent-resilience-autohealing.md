# Agent Resilience & Autohealing ✅

**Completed**: 2025-12-13  
**Priority**: Medium → **Done**

## Summary

Made the NixFleet agent more resilient by preventing duplicate processes and ensuring reliable startup on macOS.

## Problem Encountered

The agent was becoming stuck on macOS with these symptoms:

1. **DNS timeout at boot**: Launchd starts the agent before network is ready
2. **Duplicate processes**: During `home-manager switch`, multiple agents could run
3. **Buttons stayed locked in UI**: Agent couldn't report status back

## Solutions Implemented

### 1. Network Readiness Check

Added a network wait loop using curl to check the actual fleet endpoint:

```bash
while ! /usr/bin/curl -sf --connect-timeout 2 --max-time 5 "${cfg.url}/health" >/dev/null 2>&1; do
  # Wait up to 60s
done
```

**Note**: Initially tried `/usr/bin/host` for DNS check but it doesn't work in launchd context.

### 2. Cross-Platform Process Locking

Added PID-based locking (works on both Linux and macOS):

```bash
# Try to create lock file atomically
if ! ( set -o noclobber; echo $$ > "$LOCK_FILE" ) 2>/dev/null; then
  log_error "Another agent instance is already running"
  return 1
fi
```

Features:

- Detects stale locks (dead PIDs)
- Cleans up on exit via trap
- Works without flock (not available on macOS)

## Critical Review of Original Proposal

The original backlog (from another LLM) proposed a multi-layered approach. Here's my verdict:

### ✅ Kept (Simple & Effective)

- Process locking - prevents duplicates
- Network wait for launchd - fixes boot timing

### ❌ Rejected (Over-Engineered)

- **Duplicate detection + kill logic**: Redundant with proper locking, and dangerous
- **Health monitoring with heartbeat files**: Dashboard already tracks `last_seen`
- **Self-healing via process killing**: Let launchd/systemd handle restarts
- **Child process timeouts**: Not needed - curl already has timeouts

## Files Modified

1. `modules/home-manager.nix` - Added network wait, uses curl instead of host
2. `agent/nixfleet-agent.sh` - Added `acquire_lock()` with PID-based locking

## Testing Results

Tested on `imac0` (macOS):

- ✅ Agent starts immediately (no 60s network wait)
- ✅ Lock file created with correct PID
- ✅ Heartbeats working (dashboard shows "just now")
- ✅ Commands executing properly
- ✅ No duplicate processes

## Lessons Learned

1. **Start simple**: PID locking + network wait solved the problem
2. **Trust the service manager**: Launchd handles restarts - don't duplicate that
3. **Dashboard is the authority**: Status tracking belongs in dashboard, not agent files
4. **Test on real hardware**: DNS timeout only appeared on actual macOS boot
5. **Use curl, not host**: `/usr/bin/host` doesn't work in launchd context
