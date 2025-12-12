# Update Agent from UI

## Summary

Add "Update Agent" action to the ellipsis menu that updates the nixfleet flake input
and rebuilds the host, bringing the agent to the latest version.

## Problem

Currently, to update a host's NixFleet agent to the latest version:

1. User must manually run `nix flake update nixfleet` in nixcfg repo
2. Commit and push the flake.lock change
3. Then Pull + Switch on each host

This is cumbersome and requires SSH access or local terminal work.

## Solution

Add "Update Agent" action that automates this process from the dashboard UI.

## Technical Analysis

### Command Flow

```
User clicks "Update Agent" on host X
    ↓
Dashboard queues "update" command for host X
    ↓
Agent receives "update" command
    ↓
Agent executes:
    1. cd $NIXFLEET_NIXCFG
    2. nix flake update nixfleet
    3. git add flake.lock
    4. git commit -m "chore: Update nixfleet to $(date +%Y-%m-%d)"
    5. git push
    6. nixos-rebuild switch / home-manager switch
    ↓
Agent reports status back to dashboard
    ↓
Dashboard shows new Fleet hash (should match target)
```

### Git Commit Strategy

**Option A: Commit & Push (Recommended)**

- First host to update commits the new flake.lock
- Other hosts get it automatically on next Pull
- Ensures fleet-wide consistency
- Requires: git push permissions on agent

**Option B: Local Only**

- Each host updates independently
- No git commits
- Hosts may have different nixfleet versions
- Simpler but inconsistent

**Recommendation:** Option A with fallback - try to commit/push, but don't fail
the update if push fails (e.g., another host already pushed the same change).

### Agent Changes

New function in `agent/nixfleet-agent.sh`:

```bash
do_update() {
  log_info "Executing: update agent (flake update + switch)"
  cd "$NIXFLEET_NIXCFG"

  # Update nixfleet input
  log_info "Updating nixfleet flake input..."
  if ! nix flake update nixfleet 2>&1; then
    report_status "error" "$(get_generation)" "Flake update failed" "update failed"
    return 1
  fi

  # Check if flake.lock changed
  if git diff --quiet flake.lock; then
    log_info "No changes to flake.lock (already up to date)"
  else
    # Commit and push the change
    log_info "Committing flake.lock update..."
    git add flake.lock
    git commit -m "chore: Update nixfleet flake input"

    log_info "Pushing to remote..."
    if ! git push 2>&1; then
      log_warn "Push failed (may already be updated by another host)"
      git reset --soft HEAD~1  # Undo commit if push failed
      git checkout flake.lock  # Restore original
      git pull --rebase        # Get latest
    fi
  fi

  # Now rebuild
  do_switch
}
```

### Command Routing

In the main command handler, add:

```bash
case "$command" in
  pull)   do_pull ;;
  switch) do_switch ;;
  test)   do_test ;;
  restart) do_restart ;;
  update) do_update ;;  # NEW
  *) log_warn "Unknown command: $command" ;;
esac
```

### Backend Changes

1. Add "update" to allowed commands in `queue_command` validation
2. No other backend changes needed (reuses existing command infrastructure)

### Frontend Changes

1. Add "Update Agent" button to ellipsis dropdown menu:

```html
<button class="dropdown-item" type="button"
        data-action="command" data-command="update"
        data-host-id="{{ host.id }}">
    <svg><use href="#icon-download"/></svg>
    Update Agent
</button>
```

2. Add command label for status display:

```javascript
const commandLabels = {
    'pull': 'Pulling...',
    'switch': 'Switching...',
    'test': 'Testing...',
    'restart': 'Restarting...',
    'update': 'Updating agent...',  // NEW
};
```

### Edge Cases

| Scenario | Handling |
| --- | --- |
| Flake update fails (network) | Report error, don't proceed to switch |
| Git push fails (conflict) | Reset commit, pull latest, proceed to switch |
| Git push fails (permissions) | Log warning, proceed to switch anyway |
| Switch fails after update | Report switch error (flake.lock may be updated) |
| Already up to date | Skip commit, proceed to switch |
| Host offline | Reject command (existing behavior) |

### Security Considerations

- Agent already has git push access (for commits during switch)
- No new credentials needed
- `nix flake update` only updates from configured flake inputs (safe)

### Testing Plan

1. Test on single NixOS host (e.g., csb0)
2. Test on single macOS host (e.g., imac0)
3. Test conflict scenario (two hosts update simultaneously)
4. Test when already up-to-date
5. Test with network failure during flake update
6. Verify Fleet hash updates in UI after successful update

## UI/UX

### Button Placement

In ellipsis dropdown, after "Restart Agent":

```
├── Unlock Actions
├── Restart Agent
├── Update Agent      ← NEW
├── ─────────────
├── Download Logs
├── Remove Host
```

### Status Messages

- During: "Updating agent..."
- Success: "Agent updated to [hash]"
- Failure: "Update failed: [reason]"

## Implementation Tasks

- [ ] Add `do_update` function to agent script
- [ ] Add "update" case to command handler in agent
- [ ] Add "update" to allowed commands in backend
- [ ] Add "Update Agent" button to ellipsis dropdown
- [ ] Add "Updating agent..." label to JS command labels
- [ ] Test on NixOS host
- [ ] Test on macOS host
- [ ] Test conflict/edge cases
- [ ] Deploy and verify

## Priority

Medium - Quality of life improvement, not blocking

## Dependencies

- None (uses existing command infrastructure)

## Estimated Effort

2-3 hours implementation + testing
