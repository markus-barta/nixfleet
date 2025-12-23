# Agent-Managed Isolated Repository

**Created**: 2025-12-13
**Completed**: 2025-12-13
**Status**: Done
**Priority**: High

## Problem

Currently, each host's NixFleet agent uses a shared user-managed Git repository (e.g., `/home/admin/Code/nixcfg` or `/Users/markus/Code/nixcfg`). This creates several issues:

1. **External interference**: Users can make manual changes, create conflicts, leave dirty working trees
2. **Permission complexity**: The agent runs as a specific user who must own the repo
3. **Conflict resolution**: When `nix flake update` runs on one host and pushes, other hosts get merge conflicts on `flake.lock`
4. **Shared state**: Agent operations compete with user operations (IDE, manual git commands)
5. **Non-deterministic**: The repo state depends on what the user last did

## Goal

The NixFleet agent should manage its own **isolated, dedicated repository clone** that:

1. **Is exclusively owned by the agent** – no external writes allowed
2. **Can be fully reset** at any time without consequences (fresh clone approach)
3. **Is configured via a remote URL**, not a local path
4. **Lives in a system-controlled location** (e.g., `/var/lib/nixfleet-agent/repo` or similar)

## Proposed Solution

### New Configuration Options

```nix
services.nixfleet-agent = {
  enable = true;
  url = "https://fleet.barta.cm";
  tokenFile = config.age.secrets.nixfleet-token.path;

  # NEW: Remote repo to clone (replaces configRepo)
  repoUrl = "git@github.com:markus-barta/nixcfg.git";
  # or HTTPS: "https://github.com/markus-barta/nixcfg.git"

  # OPTIONAL: SSH key for private repos
  sshKeyFile = config.age.secrets.nixfleet-deploy-key.path;

  # OPTIONAL: Branch to track (default: main)
  branch = "main";

  # DEPRECATED: configRepo (local path)
};
```

### Agent Behavior Changes

1. **On startup**: Check if `/var/lib/nixfleet-agent/repo` exists and is valid
2. **If missing or corrupt**: `git clone --branch $BRANCH $REPO_URL /var/lib/nixfleet-agent/repo`
3. **On `pull` command**:
   - `git fetch origin`
   - `git reset --hard origin/$BRANCH` (no merge, just reset to remote state)
4. **On `update` command**:
   - `nix flake update`
   - `git add flake.lock && git commit && git push`
   - If push fails (someone else pushed): reset and retry
5. **On `switch` command**: Build from the isolated repo path

### Directory Structure

```
/var/lib/nixfleet-agent/
├── repo/           # Git clone (owned by agent, mode 0700)
│   ├── .git/
│   ├── flake.nix
│   ├── hosts/
│   └── ...
├── token           # Cached per-host token
└── state.json      # Agent state (optional)
```

### Permissions

- **NixOS**: Directory owned by `nixfleet-agent` service user (not the configured `user`)
- **macOS**: Directory owned by the user running the launchd agent, mode 0700
- **No group/world access**: Only the agent process can read/write

### SSH Key Management

For private repos, the agent needs an SSH key:

```nix
# In agenix secrets
age.secrets.nixfleet-deploy-key = {
  file = ./secrets/nixfleet-deploy-key.age;
  owner = "nixfleet-agent";  # NixOS
  mode = "0600";
};

services.nixfleet-agent = {
  repoUrl = "git@github.com:org/nixcfg.git";
  sshKeyFile = config.age.secrets.nixfleet-deploy-key.path;
};
```

The agent would use `GIT_SSH_COMMAND="ssh -i $SSH_KEY_FILE"` for Git operations.

## Migration Path

1. **Phase 1**: Add new `repoUrl` option alongside existing `configRepo`
2. **Phase 2**: If `repoUrl` is set, use isolated repo; if `configRepo` is set, use legacy mode
3. **Phase 3**: Deprecation warning when using `configRepo`
4. **Phase 4**: Remove `configRepo` in future version

## Benefits

- ✅ **No conflicts**: Fresh reset on each pull, no merge issues
- ✅ **No interference**: User activity in their own repo doesn't affect agent
- ✅ **Clean separation**: Agent has its own controlled environment
- ✅ **Simpler debugging**: Known state, can be blown away and recreated
- ✅ **Better security**: Isolated directory with restricted permissions

## Considerations

- **Disk usage**: Each host has a full clone (mitigated by shallow clones or sparse checkout if needed)
- **SSH keys**: Need deploy key management per-host or shared read-only key
- **Initial clone time**: First startup takes longer (can pre-seed in NixOS activation)

## Acceptance Criteria

- [x] Agent clones from remote URL on first run
- [x] Pull command uses `git reset --hard` instead of `git pull`
- [x] Repo directory has restricted permissions (0700, agent-only)
- [x] SSH key authentication works for private repos
- [x] Update command handles push conflicts gracefully (falls back to pull+switch in HTTPS mode)
- [x] Legacy `configRepo` still works with deprecation warning
- [x] Documentation updated with new configuration
