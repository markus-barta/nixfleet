# P4010 - Agent Resilience Verification & v1 Cleanup

**Created**: 2025-12-17  
**Priority**: P4010 (Critical - Blocking)  
**Status**: Backlog  
**Depends on**: None  
**Blocks**: Everything else - this is the foundation

---

## Problem

We have "fixed" agent issues **10+ times** across the fleet. Each time:

- "Now it's fixed for all hosts" → Still broken on some
- "The agent survives switch now" → Agent runs old binary after switch
- "macOS launchd restarts it" → Agent dead after home-manager switch
- "All hosts updated" → Some still running v1 bash agent

**This cycle ends now.**

This backlog item ensures:

1. All resilience requirements (RES-1 through RES-8) are verified on every host
2. All v1 agent remnants are removed
3. A verification process exists to prevent regressions

---

## Scope

### Part 1: Remove ALL v1 Agent Remnants

The v1 agent was a bash script. It must be completely gone from all hosts.

**Files/artifacts to remove:**

| Type       | Path                                                      | Platform |
| ---------- | --------------------------------------------------------- | -------- |
| v1 script  | `/usr/local/bin/nixfleet-agent.sh`                        | All      |
| v1 script  | `~/.local/bin/nixfleet-agent.sh`                          | All      |
| v1 config  | `/etc/nixfleet-agent/`                                    | NixOS    |
| v1 config  | `~/.config/nixfleet/` (if contains v1 config)             | All      |
| v1 systemd | `/etc/systemd/system/nixfleet-agent.service` (if v1)      | NixOS    |
| v1 launchd | `~/Library/LaunchAgents/com.nixfleet.agent.plist` (if v1) | macOS    |
| v1 logs    | `/var/log/nixfleet-agent.log`                             | All      |
| v1 state   | Any `.pid` files, lock files                              | All      |

**How to identify v1:**

- Agent binary is a bash script, not a Go binary
- Service runs `nixfleet-agent.sh` instead of `nixfleet-agent`
- URL contains `https://` instead of `wss://`
- No WebSocket connection in logs

### Part 2: Verify v2 Agent on All Hosts

Every host must have:

1. **Go agent binary** in nix store (not bash script)
2. **Correct URL** (`wss://fleet.barta.cm/ws`)
3. **Isolated repo mode** (`/var/lib/nixfleet-agent/repo` or `~/.local/state/nixfleet-agent/repo`)
4. **Latest nixfleet in flake.lock** (matches current master)
5. **Correct systemd/launchd config**

### Part 3: Verify All Resilience Requirements

Run the verification checklist from the PRD on EVERY host:

| Test                   | Command                                                                     | Expected Result                |
| ---------------------- | --------------------------------------------------------------------------- | ------------------------------ |
| Agent running          | `systemctl is-active nixfleet-agent` / `launchctl list \| grep nixfleet`    | active / running               |
| Using Go binary        | `file $(readlink /proc/$(pgrep nixfleet-agent)/exe)`                        | ELF executable, not script     |
| Correct URL            | `systemctl show nixfleet-agent --property=Environment \| grep NIXFLEET_URL` | `wss://fleet.barta.cm/ws`      |
| Isolated repo exists   | `ls -la /var/lib/nixfleet-agent/repo/.git`                                  | exists                         |
| Repo on correct branch | `git -C /var/lib/nixfleet-agent/repo branch`                                | main                           |
| Repo matches origin    | `git -C /var/lib/nixfleet-agent/repo log -1 --format=%H`                    | matches origin/main            |
| Latest nixfleet        | `cat flake.lock \| jq -r .nodes.nixfleet.locked.rev`                        | matches current master         |
| Kill test              | `sudo kill -9 $(pgrep nixfleet-agent); sleep 5; pgrep nixfleet-agent`       | PID exists                     |
| Switch test            | Click Switch in dashboard                                                   | Agent restarts with new binary |

### Part 4: Create Automated Verification Script

Create a script that can be run on any host to verify compliance:

```bash
#!/usr/bin/env bash
# nixfleet-verify.sh - Verify agent resilience requirements

set -euo pipefail

ERRORS=0

check() {
    local name="$1"
    local cmd="$2"
    local expected="$3"

    result=$(eval "$cmd" 2>/dev/null || echo "FAILED")
    if [[ "$result" == *"$expected"* ]]; then
        echo "✅ $name"
    else
        echo "❌ $name: expected '$expected', got '$result'"
        ((ERRORS++))
    fi
}

echo "=== NixFleet Agent Verification ==="
echo ""

# Platform detection
if [[ "$(uname)" == "Darwin" ]]; then
    PLATFORM="macos"
    REPO_DIR="$HOME/.local/state/nixfleet-agent/repo"
else
    PLATFORM="nixos"
    REPO_DIR="/var/lib/nixfleet-agent/repo"
fi

echo "Platform: $PLATFORM"
echo "Repo dir: $REPO_DIR"
echo ""

# v1 remnants check
echo "=== v1 Remnants (must be empty) ==="
v1_files=$(ls /usr/local/bin/nixfleet-agent.sh ~/.local/bin/nixfleet-agent.sh /etc/nixfleet-agent 2>/dev/null || true)
if [[ -z "$v1_files" ]]; then
    echo "✅ No v1 remnants found"
else
    echo "❌ v1 remnants found: $v1_files"
    ((ERRORS++))
fi
echo ""

# v2 agent checks
echo "=== v2 Agent Checks ==="
check "Agent running" "pgrep -x nixfleet-agent" ""
check "Go binary (not script)" "file \$(readlink /proc/\$(pgrep -o nixfleet-agent)/exe) 2>/dev/null | grep -c ELF" "1"
check "Isolated repo exists" "test -d $REPO_DIR/.git && echo exists" "exists"
check "Repo on main branch" "git -C $REPO_DIR rev-parse --abbrev-ref HEAD" "main"
echo ""

# Config checks
echo "=== Configuration ==="
if [[ "$PLATFORM" == "nixos" ]]; then
    check "WebSocket URL" "systemctl show nixfleet-agent --property=Environment | grep NIXFLEET_URL" "wss://"
else
    check "WebSocket URL" "launchctl print gui/\$(id -u)/com.nixfleet.agent 2>/dev/null | grep NIXFLEET_URL || cat /tmp/nixfleet-agent.err | grep -m1 url" "wss://"
fi
echo ""

# Summary
echo "=== Summary ==="
if [[ $ERRORS -eq 0 ]]; then
    echo "✅ All checks passed!"
    exit 0
else
    echo "❌ $ERRORS check(s) failed"
    exit 1
fi
```

### Part 5: Add to CI/Deployment Process

- [ ] Verification script runs after every `nixos-rebuild switch`
- [ ] Verification script runs after every `home-manager switch`
- [ ] Dashboard shows verification status per host
- [ ] Alert if any host fails verification

---

## Acceptance Criteria

- [ ] **AC-1**: No v1 agent files exist on any host (verified by script)
- [ ] **AC-2**: All hosts run Go v2 agent (verified by binary type check)
- [ ] **AC-3**: All hosts use `wss://` URL (verified by config check)
- [ ] **AC-4**: All hosts have isolated repo mode enabled
- [ ] **AC-5**: All hosts pass kill-restart test (agent restarts within 30s)
- [ ] **AC-6**: All hosts pass switch test (agent restarts with new binary)
- [ ] **AC-7**: Verification script exists and can be run on any host
- [ ] **AC-8**: Host checklist in this file shows all hosts verified

---

## Host Verification Checklist

**Run verification script on each host and update status:**

| Host          | v1 Removed | v2 Running | Correct URL | Isolated Repo | Kill Test | Switch Test | Verified By | Date |
| ------------- | ---------- | ---------- | ----------- | ------------- | --------- | ----------- | ----------- | ---- |
| hsb0          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| hsb1          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| hsb8          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| csb0          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| csb1          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| gpc0          | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| imac0         | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| mba-mbp-work  | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |
| mba-imac-work | ⬜         | ⬜         | ⬜          | ⬜            | ⬜        | ⬜          |             |      |

**Legend:** ⬜ = Not verified, ✅ = Passed, ❌ = Failed

---

## Definition of Done

1. All hosts have ✅ in every column of the verification checklist
2. Verification script is committed to the repo
3. No manual fixes were needed (or if needed, the fix is documented and automated)
4. PRD resilience requirements (RES-1 through RES-8) are verified
5. This backlog item is moved to `done/`

---

## Post-Completion: Regression Prevention

After this is done, to prevent regression:

1. **New host onboarding**: Must pass verification before considered "deployed"
2. **Agent updates**: Must include verification step
3. **Monitoring**: Dashboard shows agent health prominently
4. **Documentation**: RUNBOOK includes verification steps

---

## Related

- PRD: Critical Requirement - Agent Resilience
- P4050: Bug - Old agents pull not resetting
- P5500: Isolated repo mode implementation
