# Pickup Document: hsb1 Agent Not Reconnecting After Switch

**Date**: 2025-12-25 ~23:45 CET  
**Priority**: High  
**Status**: Needs Investigation

---

## Problem Summary

After running `pull` → `switch` on hsb1:

1. Pull completed successfully (git up to date)
2. Switch completed with exit code 0
3. Dashboard shows "switch" badge stuck (stale)
4. Agent disconnected but **never reconnected**

---

## Root Cause (Identified)

**The "old agent" caching problem strikes again!**

```bash
# Agent is running OLD version 2.1.0, not 2.3.x!
Process: /nix/store/lr9vxnaphgb371fihydg90s15s3i9jxv-nixfleet-agent-2.1.0/bin/nixfleet-agent
Status: activating (auto-restart) - exit code 1/FAILURE
```

The `nixos-rebuild switch` completed but Nix is using a **cached old agent binary** from the store instead of building/fetching the new one.

---

## Timeline from Dashboard Logs

```
22:41:28  pull completed, exit_code=0
22:41:28  git status: ok (up to date with remote)
22:42:15  switch command sent
22:42:42  switch completed, exit_code=0
22:42:42  state → AWAITING_RECONNECT (waiting for agent restart)
22:42:47  client unregistered (agent disconnected)
          ❌ NO MORE HEARTBEATS - agent never reconnected
```

---

## Why Agent is Failing

The old 2.1.0 agent likely:

1. Has incompatible message formats with 2.3.x dashboard
2. Missing required fields/handlers
3. Exit code 1 = crash on startup

---

## Fix Required

SSH to hsb1 and force uncached rebuild:

```bash
# SSH to hsb1
ssh mba@hsb1

# Force fresh agent build (bypass Nix cache)
cd ~/Code/nixcfg
git pull
nix flake update nixfleet
sudo nixos-rebuild switch --flake .#hsb1 --option narinfo-cache-negative-ttl 0

# Verify new agent version
nixfleet-agent --version  # Should be 2.3.x
systemctl status nixfleet-agent
```

---

## Related Issues

This is the same caching problem from P7200. The force-uncached-update command chain was created specifically for this:

```bash
cd ~/Code/nixcfg && \
git pull && \
nix flake update nixfleet && \
sudo nixos-rebuild switch --flake .#$(hostname) --option narinfo-cache-negative-ttl 0
```

---

## Dashboard State Cleanup

After agent reconnects, may need to:

1. Clear the "switch" pending command badge manually (if stuck)
2. Verify state machine exits AWAITING_RECONNECT state

---

## Prevention (Future Work)

- [ ] P7200: Add "Force Update Agent" button to dashboard
- [ ] Add agent version check before switch - warn if cache might be stale
- [ ] Consider: auto-retry with `--option narinfo-cache-negative-ttl 0` if version mismatch detected

---

## Files to Review

- `v2/internal/agent/commands.go` - switch command execution
- `v2/internal/dashboard/hub.go` - AWAITING_RECONNECT handling
- `v2/internal/dashboard/command_state.go` - state machine transitions
- `packages/nixfleet-agent-v2.nix` - Nix package definition

---

**Next Action**: SSH to hsb1, run force-uncached rebuild command chain
