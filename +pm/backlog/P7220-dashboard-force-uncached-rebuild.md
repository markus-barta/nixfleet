# P7220: Dashboard Force Uncached Rebuild

**Created**: 2025-12-19  
**Updated**: 2025-12-28  
**Priority**: P9200 (⚪ Very Low Priority - Future)  
**Effort**: Small  
**Status**: Backlog

**Note**: Priority lowered - niche development tool, workaround exists  
**Implements**: Pipeline `force-update` from [CORE-002](../spec/CORE-002-pipeline-executor.md)  
**Depends on**: P3020 (Pipeline Executor)

## Problem

Sometimes Nix cache serves stale binaries even when flake.lock points to the correct commit. This happened in the P6750 incident where csb0 kept getting the old 2.1.0 agent.

The workaround requires:

```bash
sudo nixos-rebuild switch --flake .#$(hostname) --option narinfo-cache-negative-ttl 0
```

Operators shouldn't need to remember this incantation.

## Solution

Add a dashboard command that triggers a rebuild with cache bypass.

### Flow

```
Dashboard "Force Rebuild" button
  → Agent receives "force-rebuild" command
  → Agent forks detached script
  → Agent exits gracefully
  → Script executes:
      nixos-rebuild switch --flake .#$(hostname) \
        --option narinfo-cache-negative-ttl 0
  → systemd restarts agent with fresh binary
```

### Dashboard UI

In the host action menu (under "Advanced" or similar):

- **Force Rebuild** — rebuilds with cache bypass

This is distinct from "Update Agent" (P7210):

- **Update Agent** = bump flake.lock + rebuild
- **Force Rebuild** = rebuild with current flake.lock, bypass cache

## Implementation Notes

### When to Use

Use Force Rebuild when:

- Flake.lock already updated but agent still shows old version
- Suspecting cache corruption
- Binary substitution failing silently

### Cache Bypass Options

```bash
# Bypass negative cache (most common fix)
--option narinfo-cache-negative-ttl 0

# Force rebuild from source (nuclear option)
--option substitute false
```

Start with `narinfo-cache-negative-ttl 0`. Add `--substitute false` as a separate "Rebuild from Source" option if needed.

### NixOS vs macOS

| OS    | Command                                                 |
| ----- | ------------------------------------------------------- |
| NixOS | `sudo nixos-rebuild switch --flake .#host --option ...` |
| macOS | `home-manager switch --flake .#host --option ...`       |

Agent must detect OS and use correct command.

## Acceptance Criteria

- [ ] Dashboard has "Force Rebuild" action for each host
- [ ] Command runs rebuild with `--option narinfo-cache-negative-ttl 0`
- [ ] Agent restarts with freshly-built binary
- [ ] Works on NixOS
- [ ] Works on macOS (Home Manager)
- [ ] Shows command output in real-time

## Out of Scope

- Full "rebuild from source" (`--substitute false`) — too slow for normal use
- Automatic detection of when cache bypass is needed

## Related

- P7200: Agent CLI Interface
- P7210: Dashboard Bump Agent Version
- P6750: csb0 Load Spike Investigation (the incident that revealed this need)
