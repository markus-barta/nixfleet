# P5500: Complete Isolated Repo Mode in v2 Agent

**Priority**: High (blocks reliable pull/switch operations)  
**Created**: 2025-12-16  
**Completed**: 2025-12-16  
**Status**: Done

## Problem

The v2 agent has partial isolated repo mode support but is missing critical pieces:

1. **No auto-clone on startup** - Agent doesn't create the repo if it doesn't exist
2. **No dedicated directory** - Uses arbitrary `RepoDir` (could be user's home)
3. **Missing `git reset --hard`** - Only does `fetch`, no reset to clean state
4. **No "clean slate" guarantee** - Repo could have local changes

This causes:

- `pull` command may fail if directory doesn't exist
- User's working directory can interfere with agent operations
- Merge conflicts possible if user made local changes
- Compartment indicators (generation detection) may read wrong directory

## Solution

### 1. Dedicated Directory Paths

| Platform                     | Isolated Repo Path                   |
| ---------------------------- | ------------------------------------ |
| NixOS (systemd)              | `/var/lib/nixfleet-agent/repo`       |
| macOS (launchd/Home Manager) | `~/.local/state/nixfleet-agent/repo` |

### 2. Auto-Clone on Startup

When `NIXFLEET_REPO_URL` is set:

1. Derive `RepoDir` automatically (don't require user to set it)
2. On startup, check if repo exists
3. If missing, clone from `RepoURL`
4. If exists but corrupt, delete and re-clone

### 3. Clean Pull Command

```bash
# Current (wrong):
git fetch origin main

# Fixed:
git fetch origin main
git reset --hard origin/main
git clean -fd  # Optional: remove untracked files
```

### 4. Generation Detection

The `detectGeneration()` function must use the isolated repo path, not user's config repo.

## Acceptance Criteria

- [x] Agent auto-clones repo on startup when `REPO_URL` is set
- [x] Agent uses dedicated directory paths by default
- [x] Pull command does `fetch + reset --hard` (not just fetch)
- [x] Repo directory has correct permissions (0700)
- [x] Generation detection uses isolated repo
- [x] Works on both NixOS and macOS
- [x] Existing `REPO_DIR` override still works
- [x] Test validates isolated repo behavior

## Implementation

### Files to Modify

1. `v2/internal/config/config.go` - Add isolated dir logic
2. `v2/internal/agent/agent.go` - Add ensureRepoExists() call
3. `v2/internal/agent/commands.go` - Add git reset --hard after fetch
4. `v2/internal/agent/repo.go` - New file for repo management
5. `modules/nixos.nix` - Update default repoDir
6. `modules/home-manager.nix` - Update default repoDir

### Tests

- `tests/specs/T09-isolated-repo.md` - New test spec
- Integration test for clone/fetch/reset flow

## References

- PRD FR-1.9: "Support isolated repo mode (agent-managed git clone)"
- Legacy design: `+pm/legacy/v1-backlog/2025-12-13-agent-isolated-repo.md`
