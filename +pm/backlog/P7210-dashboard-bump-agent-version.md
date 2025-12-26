# P7210: Dashboard Bump Agent Version

**Priority**: Medium  
**Effort**: Medium  
**Status**: Backlog

## Problem

To update an agent to a new version, operators must SSH to each host and run:

```bash
cd ~/Code/nixcfg
git pull
nix flake update nixfleet
sudo nixos-rebuild switch --flake .#$(hostname)
```

This is manual, error-prone, and doesn't scale.

## Solution

Add a dashboard command that triggers a flake lock update on the host, then rebuilds.

### Flow

```
Dashboard "Update Agent" button
  → Agent receives "update-agent" command
  → Agent forks detached script
  → Agent exits gracefully
  → Script executes:
      cd /path/to/nixcfg
      git pull
      nix flake update nixfleet
      nixos-rebuild switch --flake .#$(hostname)
  → systemd restarts agent with new binary
  → New agent connects, reports new version
```

### Dashboard UI

In the host action menu:

- **Update Agent** — runs `nix flake update nixfleet` + rebuild

Shows:

- Current agent version (before)
- Command output streaming
- New agent version (after reconnect)

## Implementation Notes

### Agent Side

1. Receive `update-agent` command
2. Fork detached process (`setsid` or systemd transient unit)
3. Exit gracefully (so systemd can restart with new binary)
4. Detached script runs the update sequence

### Script Requirements

- Must survive agent exit
- Must handle failures gracefully (network, build errors)
- Should log to a file for debugging
- Must work on both NixOS (systemd) and macOS (launchd)

### Path Discovery

Agent needs to know where nixcfg lives:

- NixOS: Could be `/etc/nixos` or user home
- macOS: Usually `~/Code/nixcfg`

Options:

1. Environment variable: `NIXFLEET_NIXCFG_PATH`
2. Convention: `~/Code/nixcfg` or `/etc/nixos`
3. Config in nixcfg module

## Acceptance Criteria

- [ ] Dashboard has "Update Agent" action for each host
- [ ] Command runs `nix flake update nixfleet` on target host
- [ ] Host rebuilds with new flake.lock
- [ ] Agent restarts and reports new version
- [ ] Works on NixOS (systemd)
- [ ] Works on macOS (launchd + Home Manager)
- [ ] Failure shows clear error message

## Open Questions

- Where does nixcfg live on each host? (env var vs convention)
- Should we show a diff of what changed in flake.lock?
- Timeout handling for slow rebuilds?

## Related

- P7200: Agent CLI Interface
- P7220: Dashboard Force Uncached Rebuild
- P4700: Automated Flake Lock Updates (broader automation)
