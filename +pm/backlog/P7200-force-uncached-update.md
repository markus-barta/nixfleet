# P7200: Force Uncached Agent & System Update

**Priority**: Medium  
**Effort**: Small  
**Status**: Backlog

## Problem

When deploying agent fixes, Nix may use cached binaries instead of rebuilding from the updated source. This caused the P6750 incident where csb0 kept getting the old 2.1.0 agent despite the flake.lock pointing to the fixed commit.

The current workaround requires a complex command chain:

```bash
cd ~/Code/nixcfg && \
git pull && \
nix flake update nixfleet && \
sudo systemctl stop nixfleet-agent && \
sudo nixos-rebuild switch --flake .#$(hostname) --option narinfo-cache-negative-ttl 0
```

## Proposed Solution

Add a dashboard command (or agent self-update command) that:

1. **Stops the agent** gracefully
2. **Pulls nixcfg** (or triggers remote pull)
3. **Updates the nixfleet flake input**
4. **Rebuilds with cache bypass** (`--option narinfo-cache-negative-ttl 0`)
5. **Restarts with new binary**

### Option A: Dashboard "Force Update" Button

Add a "Force Update" option in the host's action menu that:

- Sends a special command to the agent
- Agent executes the full update chain
- Dashboard shows progress via log streaming
- Agent restarts with new version after rebuild

### Option B: Agent `--self-update` Flag

```bash
nixfleet-agent --self-update --force-rebuild
```

Agent would:

1. Fork a detached process to run the update
2. Exit gracefully
3. Detached process runs rebuild
4. Systemd restarts with new binary

### Option C: Dedicated `nixfleet-update` Script

A NixOS module that provides `/run/current-system/sw/bin/nixfleet-update`:

```bash
nixfleet-update [--force] [--dry-run]
```

This script would be:

- Installed alongside the agent
- Runnable manually or via dashboard command
- Include all the cache-bypass logic

## Implementation Notes

- Must handle the "agent dies mid-update" scenario (use `setsid` or systemd transient units)
- Should report version before/after for verification
- Consider adding `--verify` flag to check if update succeeded

## Acceptance Criteria

- [ ] Single command/button to force-update agent without cache
- [ ] Works even when current agent has bugs
- [ ] Shows clear before/after version
- [ ] Handles network failures gracefully
- [ ] Works on both NixOS and macOS (Home Manager)

## Related

- P6750: csb0 Load Spike Investigation (the incident that revealed this need)
- P2810: Binary Freshness Detection (version verification)
