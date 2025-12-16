# Bug: Old Agents Don't Reset on Pull

**Created**: 2025-12-16  
**Priority**: P4050 (Critical)  
**Status**: Backlog  
**Discovered on**: gpc0, potentially affects all hosts with old flake.lock

---

## Problem

Agents built from nixfleet commits **before** `e579804` (P5500: Implement isolated repo mode with clean-slate pull) do not have the `runIsolatedPull()` function. When the "Pull" button is clicked in the dashboard:

- **Old agents**: Only do `git fetch`, leaving the local repo behind `origin/main`
- **New agents**: Do `git fetch` + `git reset --hard origin/main` + `git clean -fd`

This results in:

1. Dashboard shows "Pull completed successfully" (exit code 0)
2. But the repo is still behind origin (no reset happened)
3. Host continues to show "behind" status in the dashboard
4. User confusion: "I clicked pull but it still shows behind"

### Root Cause

The host's `flake.lock` has an old nixfleet input that predates the isolated pull fix:

```
# Old (broken):
nixfleet: ace61cb (2025-12-15) - before isolated pull

# New (fixed):
nixfleet: dab6664 (2025-12-16) - has isolated pull
```

### Symptoms

1. Click "Pull" in dashboard → shows success
2. Host still shows "behind" status
3. Agent logs show `command completed command=pull exit_code=0 status=ok`
4. But NO log line for `running isolated pull (fetch + reset --hard)`
5. Manual check: `git -C /var/lib/nixfleet-agent/repo log --oneline -1` shows old commit

---

## Diagnosis Steps

```bash
# 1. Check agent logs for isolated pull message
ssh host "sudo journalctl -u nixfleet-agent | grep 'isolated pull'"
# If empty → old agent

# 2. Check nixfleet version in flake.lock
ssh host "cat /var/lib/nixfleet-agent/repo/flake.lock | jq '.nodes.nixfleet.locked.rev'"

# 3. Compare with the isolated pull commit
# e579804 = P5500: Implement isolated repo mode with clean-slate pull
git merge-base --is-ancestor e579804 <agent_commit> && echo "OK" || echo "OLD"
```

---

## Solution (Manual Fix per Host)

### For NixOS Hosts

```bash
# 1. Fix repo permissions (may be root-owned after sudo operations)
ssh host "sudo chown -R mba:users /var/lib/nixfleet-agent/repo"

# 2. Update nixfleet in flake.lock
ssh host "cd /var/lib/nixfleet-agent/repo && nix flake update nixfleet"

# 3. Rebuild with new agent
ssh host "cd /var/lib/nixfleet-agent/repo && sudo nixos-rebuild switch --flake .#hostname"

# 4. Restart agent (it won't auto-restart due to X-RestartIfChanged=false)
ssh host "sudo systemctl restart nixfleet-agent"

# 5. Verify
ssh host "sudo journalctl -u nixfleet-agent -n 5"
# Should show: "isolated_mode=true repo_dir=/var/lib/nixfleet-agent/repo"
```

### For macOS Hosts

```bash
# 1. Update nixfleet in flake.lock
cd ~/.local/state/nixfleet-agent/repo
nix flake update nixfleet

# 2. Rebuild home-manager
home-manager switch --flake .#hostname

# 3. Agent auto-restarts via launchd
# Verify:
tail -5 /tmp/nixfleet-agent.err
# Should show: "isolated_mode=true"
```

---

## Permanent Fix (TODO)

### Option A: Self-Healing Agent

Add logic to the agent to detect when its own version is outdated and:

1. Update nixfleet input in flake.lock automatically
2. Trigger a switch to install new agent
3. Restart itself

### Option B: Dashboard Warning

When dashboard detects agent version mismatch:

1. Show warning icon on the host row
2. Provide "Update Agent" button that runs the fix commands
3. Already partially implemented with the "A" compartment in status

### Option C: Prevent Old Agents from Running

In the NixOS/Home-Manager modules:

1. Check if the configured nixfleet version is recent enough
2. Fail the build with helpful error if too old
3. Force users to update flake.lock before deploying

---

## Acceptance Criteria

- [ ] All online hosts have agents with isolated pull (version >= e579804)
- [ ] Document which hosts were affected and fixed
- [ ] Consider implementing permanent fix (Option A, B, or C)

---

## Affected Hosts (2025-12-16)

| Host          | Status     | Notes                                              |
| ------------- | ---------- | -------------------------------------------------- |
| gpc0          | ✅ Fixed   | Had ace61cb, updated to dab6664                    |
| imac0         | ✅ Fixed   | Was running v1 bash agent, switched to v2 Go agent |
| hsb0          | ✅ OK      | Already had new agent                              |
| hsb1          | ✅ OK      | Already had new agent                              |
| csb0          | ✅ OK      | Already had new agent                              |
| csb1          | ✅ OK      | Already had new agent                              |
| hsb8          | ⏳ Pending | Needs check when online                            |
| mba-mbp-work  | ⏳ Pending | Needs check when online                            |
| mba-imac-work | ⏳ Pending | Needs check when online                            |

---

## Related

- P5500: Implement isolated repo mode (the fix commit: e579804)
- P5000: Version/generation tracking (shows the mismatch in dashboard)
