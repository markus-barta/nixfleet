# T03: Update Agent Command

Test the agent self-update functionality via dashboard.

## Prerequisites

- NixFleet dashboard running
- Agent registered and online
- Git repository with push access
- nixfleet flake input defined in nixcfg

## What This Test Verifies

| Component      | Verification                                   |
| -------------- | ---------------------------------------------- |
| Flake Update   | `nix flake update nixfleet` updates flake.lock |
| Git Commit     | Changes committed with descriptive message     |
| Git Push       | Changes pushed to remote repository            |
| Switch         | New agent version applied via switch           |
| Agent Restart  | New agent process running after update         |
| Version Report | Dashboard shows new agent hash                 |

## Update Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Update Agent Process                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. Dashboard sends "update" command                                         │
│                     │                                                        │
│                     ▼                                                        │
│  2. Agent receives command via poll                                          │
│                     │                                                        │
│                     ▼                                                        │
│  3. nix flake update nixfleet                                                │
│     └─► Updates flake.lock with latest nixfleet                             │
│                     │                                                        │
│                     ▼                                                        │
│  4. git commit -m "chore: Update nixfleet agent"                             │
│     └─► Commits flake.lock change                                           │
│                     │                                                        │
│                     ▼                                                        │
│  5. git push origin master                                                   │
│     └─► Pushes to remote (triggers sync on other hosts)                     │
│                     │                                                        │
│                     ▼                                                        │
│  6. do_switch()                                                              │
│     └─► Applies new configuration with updated agent                        │
│                     │                                                        │
│                     ▼                                                        │
│  7. Agent restarts (launchd/systemd)                                         │
│     └─► New agent version begins running                                    │
│                     │                                                        │
│                     ▼                                                        │
│  8. New agent registers with new hash                                        │
│     └─► Dashboard shows updated agent version                               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Manual Test Procedures

### Test 1: UI Update Agent Button

**Steps:**

1. Open dashboard: https://fleet.barta.cm
2. Find a host row with outdated agent (↓ indicator)
3. Click "More actions" (⋮) → "Update Agent"
4. Observe status column

**Expected Results:**

- Status shows "↑ Updating agent..."
- After completion: "✓ Agent updated" or "✗ Update failed: reason"
- Agent column shows new hash (↓ becomes ✓)

**Status:** ⏳ Pending

### Test 2: Flake Lock Update

**Steps:**

1. Check current nixfleet hash in flake.lock:
   ```bash
   cd ~/Code/nixcfg
   cat flake.lock | jq '.nodes.nixfleet.locked.rev'
   ```
2. Trigger update from dashboard
3. Check flake.lock again

**Expected Results:**

- flake.lock updated with new nixfleet revision
- Revision matches latest nixfleet master

**Status:** ⏳ Pending

### Test 3: Git Commit Created

**Steps:**

1. Check git log before update:
   ```bash
   git log --oneline -3
   ```
2. Trigger update from dashboard
3. Check git log after

**Expected Results:**

- New commit: "chore: Update nixfleet agent"
- Commit modifies only flake.lock
- Commit has valid author/email

**Status:** ⏳ Pending

### Test 4: Git Push Successful

**Steps:**

1. Check remote status:
   ```bash
   git fetch origin
   git log origin/master --oneline -1
   ```
2. Trigger update from dashboard
3. Check remote again

**Expected Results:**

- Remote updated with new commit
- Push completes without errors
- Other hosts can now pull the update

**Status:** ⏳ Pending

### Test 5: Agent Version Updated

**Steps:**

1. Note current agent version:
   ```bash
   grep "Agent:" /tmp/nixfleet-agent.err | tail -1
   ```
2. Trigger update from dashboard
3. Wait for agent restart
4. Check agent version again

**Expected Results:**

- Agent version shows new hash
- Agent successfully registered with new version
- Dashboard shows ✓ for agent column

**Status:** ⏳ Pending

### Test 6: Cascade to Other Hosts

**Steps:**

1. Trigger update on one host (e.g., imac0)
2. Wait for completion
3. On another host, trigger "Pull"
4. Trigger "Switch" on that host

**Expected Results:**

- Second host receives the flake.lock update
- Second host gets same new agent version
- Both hosts show matching agent hashes

**Status:** ⏳ Pending

## Agent Function Reference

The agent's `do_update()` function:

```bash
do_update() {
    report_status "running" "↑ Updating nixfleet agent..."
    cd "$NIXCFG_PATH" || return 1

    # Update nixfleet flake input
    log_info "Updating nixfleet flake input..."
    nix flake update nixfleet

    # Commit changes
    log_info "Committing flake.lock update..."
    git add flake.lock
    git commit -m "chore: Update nixfleet agent"

    # Push to remote
    log_info "Pushing to remote..."
    git push origin master

    # Switch to new configuration
    do_switch

    report_status "success" "✓ Agent updated"
}
```

## Pre-commit Hooks

The update process respects pre-commit hooks configured in the repository:

- deadnix
- nixfmt-rfc-style
- statix
- shellcheck

If hooks fail, the commit will be aborted and the update will fail.

## Summary

- Total Tests: 6
- Passed: 0
- Pending: 6

## Related

- Automated: [T03-command-update-agent.sh](./T03-command-update-agent.sh)
- Agent: [nixfleet-agent.sh](../agent/nixfleet-agent.sh) - `do_update()` function
- Backlog: [Update Agent from UI](../+pm/done/2025-12-12-update-agent-from-ui.md)
