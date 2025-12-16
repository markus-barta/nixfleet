# T09: Isolated Repository Mode

**Status**: Active  
**Priority**: High  
**Related**: FR-1.9, P5500

## Overview

The agent must manage its own isolated repository clone to ensure "clean slate" operations.
This prevents user-managed repos from interfering with agent operations.

## Test Cases

### T09-01: Auto-Clone on Startup

**Preconditions:**

- `NIXFLEET_REPO_URL` is set to a valid git URL
- `NIXFLEET_REPO_DIR` is either unset (uses default) or set to a path that doesn't exist

**Steps:**

1. Start agent with `NIXFLEET_REPO_URL=https://github.com/markus-barta/nixcfg.git`
2. Agent should auto-clone the repository

**Expected:**

- Repository is cloned to the default path (or `NIXFLEET_REPO_DIR` if set)
- Directory permissions are 0700
- Agent logs "cloning repository (isolated mode)"
- Agent connects to dashboard successfully

---

### T09-02: Pull Performs Clean Reset

**Preconditions:**

- Agent is running in isolated mode (REPO_URL set)
- Repository exists with local modifications (dirty working tree)

**Steps:**

1. Create a local file in the repo: `echo "test" > /var/lib/nixfleet-agent/repo/DIRTY_FILE`
2. Modify a tracked file: `echo "changed" >> /var/lib/nixfleet-agent/repo/flake.nix`
3. Issue "pull" command from dashboard

**Expected:**

- `git fetch origin main` runs
- `git reset --hard origin/main` runs (reverts tracked changes)
- `git clean -fd` runs (removes untracked files)
- DIRTY_FILE is removed
- flake.nix is restored to remote state
- Pull completes with status "ok"

---

### T09-03: Pull-Switch Uses Clean Repo

**Preconditions:**

- Agent is running in isolated mode
- Repository has pending remote changes

**Steps:**

1. Push a change to the remote repository
2. Issue "pull-switch" command from dashboard

**Expected:**

- Pull phase: fetch + reset --hard + clean
- Switch phase: home-manager switch or nixos-rebuild switch
- Generation updates to match new commit

---

### T09-04: Generation Detection Uses Isolated Path

**Preconditions:**

- Agent running in isolated mode with repo at default path
- User also has a separate clone in their home directory with different commit

**Steps:**

1. Check generation reported in dashboard heartbeat
2. Compare with commit hash in isolated repo

**Expected:**

- Generation matches isolated repo commit, NOT user's clone
- Format: 7-character short hash

---

### T09-05: Corrupt Repo Recovery

**Preconditions:**

- Agent running in isolated mode
- Repository exists but `.git` directory is corrupted

**Steps:**

1. Corrupt the repo: `rm -rf /var/lib/nixfleet-agent/repo/.git/objects`
2. Restart agent

**Expected:**

- Agent detects corruption via `git rev-parse --git-dir`
- Agent removes corrupted repo
- Agent re-clones from `REPO_URL`
- Agent logs "repository appears corrupt, re-cloning"

---

### T09-06: SSH Key Support

**Preconditions:**

- `NIXFLEET_REPO_URL` is an SSH URL: `git@github.com:org/private-repo.git`
- `NIXFLEET_SSH_KEY` points to a valid deploy key

**Steps:**

1. Start agent with SSH repo URL and key
2. Observe clone operation

**Expected:**

- Clone uses `GIT_SSH_COMMAND=ssh -i <key> -o StrictHostKeyChecking=no`
- Clone succeeds for private repository

---

### T09-07: Legacy Mode Still Works

**Preconditions:**

- `NIXFLEET_REPO_DIR` set to existing user directory
- `NIXFLEET_REPO_URL` is NOT set

**Steps:**

1. Start agent with only REPO_DIR set
2. Issue "pull" command

**Expected:**

- Agent uses legacy mode (plain `git pull`)
- No fetch + reset --hard
- Agent logs normal git pull output

---

## Platform-Specific Paths

| Platform | Default Repo Path                    |
| -------- | ------------------------------------ |
| NixOS    | `/var/lib/nixfleet-agent/repo`       |
| macOS    | `~/.local/state/nixfleet-agent/repo` |

## Related Tests

- T01: Agent connection (must work after auto-clone)
- T02: Heartbeat (generation from isolated repo)
- T03: Commands (pull/switch use correct path)
