# Update Architecture

How NixFleet handles updates across your fleet.

---

## The Update Problem

Keeping a fleet updated involves multiple components that must all be in sync:

| Component       | What it is                                              | How it's updated                    |
| --------------- | ------------------------------------------------------- | ----------------------------------- |
| **nixcfg repo** | Your NixOS/Home Manager configurations                  | git pull                            |
| **flake.lock**  | Pinned versions of all inputs (nixpkgs, nixfleet, etc.) | nix flake update                    |
| **System**      | Running NixOS or Home Manager generation                | nixos-rebuild / home-manager switch |
| **Agent**       | NixFleet agent binary                                   | Rebuilt during switch               |
| **Dashboard**   | Web UI container on csb1                                | Docker build & restart              |

The challenge: These components have **dependencies** and must be updated in the **right order**.

---

## The Update Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           COMPLETE UPDATE FLOW                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  STEP 1: Update flake.lock (optional - for new package versions)   │     │
│  │                                                                    │     │
│  │  Where: GitHub (PR) or local machine                               │     │
│  │  Command: nix flake update                                         │     │
│  │  Creates: New flake.lock with updated input revisions              │     │
│  │  Commit & push to origin/main                                      │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                    │                                        │
│                                    ▼                                        │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  STEP 2: Pull (State: IDLE → VALIDATING → QUEUED → RUNNING)        │     │
│  │                                                                    │     │
│  │  Where: Each host's isolated repo                                  │     │
│  │  Command: git fetch && git reset --hard origin/main                │     │
│  │  Result: Host has latest nixcfg code + flake.lock                  │     │
│  │  Post-Check: Verify git status is now "ok"                         │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                    │                                        │
│                                    ▼                                        │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  STEP 3: Switch (State: IDLE → VALIDATING → QUEUED → RUNNING)      │     │
│  │                                                                    │     │
│  │  Where: Each host                                                  │     │
│  │  Command: nixos-rebuild switch OR home-manager switch              │     │
│  │  Result: New system generation, switch exit code 0                 │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                    │                                        │
│                                    ▼                                        │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  STEP 4: Agent Restart (State: RUNNING → AWAITING_RECONNECT)       │     │
│  │                                                                    │     │
│  │  NixOS: Agent exits with code 101 → systemd restarts with new bin  │     │
│  │  macOS: Agent detaches switch → launchd restarts with new binary   │     │
│  │                                                                    │     │
│  │  Result: Agent disconnected, Dashboard waits for reconnection      │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                    │                                        │
│                                    ▼                                        │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  STEP 5: Verify (State: AWAITING_RECONNECT → SUCCESS/STALE)        │     │
│  │                                                                    │     │
│  │  Verification: 3-layer freshness (Commit, Path, Hash)              │     │
│  │  Post-Check: Confirm system status = "ok"                          │     │
│  │  Final: Goal achieved, host returns to IDLE                        │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Agent Lifecycle Integration

NixFleet agents self-restart after a successful switch to pick up the new binary.

#### NixOS (systemd)

The agent module configures systemd to restart on exit code `101`:

```nix
systemd.services.nixfleet-agent = {
  restartIfChanged = false;  # Don't restart DURING switch
  serviceConfig = {
    Restart = "always";
    RestartForceExitStatus = "101";
  };
};
```

#### macOS (launchd)

The agent detaches the switch process using `Setsid: true` to survive the agent's own death during the `home-manager` activation (which may reload the agent's plist).

#### 3-Layer Binary Freshness Detection

To ensure the restart actually loaded a new binary, the dashboard verifies:

1. **Source Commit**: LDFlags injected at build.
2. **Store Path**: Resolved `/proc/self/exe`.
3. **Binary Hash**: SHA256 of the binary.

---

## Update Modes

NixFleet supports three update modes for flexibility:

### Mode 1: Manual Per-Step

Do each step individually with full control:

```bash
# On the host (via SSH or terminal)
cd ~/.local/state/nixfleet-agent/repo    # macOS
cd /var/lib/nixfleet-agent/repo          # NixOS

git pull
home-manager switch --flake .#hostname   # macOS
sudo nixos-rebuild switch --flake .#hostname  # NixOS
```

Or via Dashboard UI:

1. Click **Pull** → wait for completion
2. Click **Switch** → wait for completion
3. Verify status indicators turn green

### Mode 2: Per-Host Automatic

Click one button to do all steps for a single host:

**UI**: Click **⋮** → **Update Host**

This runs:

1. Pull
2. Switch
3. Verify agent restarted with new version

### Mode 3: Fleet-Wide Automatic

Update all hosts with a single action:

**UI**: Click **Update Fleet** (or **Merge & Deploy** when PR pending)

This runs:

1. (If PR pending) Merge the flake.lock update PR
2. Pull on all online hosts (parallel)
3. Switch on all hosts (sequential, for safety)
4. Verify all agents updated
5. Report any failures

---

## The Three Compartments

The dashboard shows update status via three indicators:

```
┌──────────┬──────────┬──────────┐
│   Git    │   Lock   │  System  │
└──────────┴──────────┴──────────┘
```

### Git Compartment

**Question**: Is my local repo up to date with GitHub?

| Color  | Meaning                   |
| ------ | ------------------------- |
| Green  | Local matches origin/main |
| Yellow | Behind origin (need Pull) |
| Red    | Error checking            |
| Gray   | Unknown                   |

### Lock Compartment

**Question**: How old is my flake.lock? Is agent outdated?

| Color  | Meaning                                       |
| ------ | --------------------------------------------- |
| Green  | flake.lock recent, agent current              |
| Yellow | flake.lock >7 days old                        |
| Red    | Agent version mismatch OR flake.lock very old |
| Gray   | Unknown                                       |

**Note**: The Lock compartment turns RED if the agent version doesn't match the dashboard version. This indicates the agent binary needs updating (run Switch).

### System Compartment

**Question**: Does my running system match what the flake would build?

| Color  | Meaning                        |
| ------ | ------------------------------ |
| Green  | Running system = latest config |
| Yellow | Config changed (need Switch)   |
| Red    | Error checking                 |
| Gray   | Unknown                        |

---

## Agent Version Detection

The agent reports its version in every heartbeat. The dashboard compares this to its own version.

```go
// Agent (built-in version)
const Version = "2.1.0"

// Dashboard compares:
if agent.Version != dashboard.Version {
    host.AgentOutdated = true  // Lock compartment turns red
}
```

**Why it matters**: If the agent is outdated, it may not have the latest features or bug fixes. A Switch is needed to update it.

---

## Known Issues & Workarounds

### macOS: Agent Not Updating After Switch

**Symptom**: Switch completes successfully, but agent still reports old version.

**Root Causes**:

1. **launchd doesn't reload plist automatically**: The running agent continues using the old binary even after home-manager updates the plist.

2. **Race condition**: If the agent exits too quickly after switch, launchd may restart it before home-manager finishes updating the plist.

**Workaround**:

```bash
# Force launchd to reload the agent
launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent
```

Or use **⋮** → **Restart Agent** in the dashboard.

**Fix** (planned): Add activation hook to home-manager module that runs kickstart after plist changes.

### flake.lock Not Using Latest nixfleet

**Symptom**: Switch runs, but agent version doesn't change.

**Root Cause**: The flake.lock in the isolated repo still points to an old nixfleet revision.

**Check**:

```bash
# What nixfleet version is in flake.lock?
grep -A 10 '"nixfleet"' ~/.local/state/nixfleet-agent/repo/flake.lock | grep rev
```

**Fix**: Update flake.lock to point to latest nixfleet:

```bash
cd ~/.local/state/nixfleet-agent/repo
nix flake update nixfleet
git add flake.lock
git commit -m "chore: update nixfleet"
git push
```

Then Pull + Switch on all hosts.

---

## Dashboard Updates

The dashboard (running on csb1) is separate from the agent update flow.

**Manual update**:

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker/nixfleet && git pull
cd ~/docker && docker compose build --no-cache nixfleet && docker compose up -d nixfleet
```

**Important**: After updating the dashboard, agents may show as "outdated" if the dashboard version changed. This is expected — the agents need to be updated to match.

---

## Verification Checklist

After a fleet update, verify:

- [ ] All hosts show **green** Git compartment (pulled latest)
- [ ] All hosts show **green** Lock compartment (agent version matches)
- [ ] All hosts show **green** System compartment (switch applied)
- [ ] Dashboard shows correct agent version in tooltips
- [ ] No hosts stuck in "running" state

---

## Quick Reference

| Task                        | UI Action                     | CLI Equivalent                                 |
| --------------------------- | ----------------------------- | ---------------------------------------------- |
| Update one host's code      | Click **Pull**                | `git pull` in isolated repo                    |
| Apply config to one host    | Click **Switch**              | `nixos-rebuild switch` / `home-manager switch` |
| Update one host completely  | **⋮** → **Update Host**       | Pull + Switch + Verify                         |
| Update all hosts            | **Update Fleet**              | Loop: Pull → Switch on each                    |
| Check for pending PRs       | **⋮** → **Check for Updates** | GitHub API check                               |
| Merge PR & deploy           | **Merge & Deploy**            | Merge PR + Pull All + Switch All               |
| Force agent restart (macOS) | **⋮** → **Restart Agent**     | `launchctl kickstart -k ...`                   |

---

## Related

- [BUILD-DEPLOY.md](./BUILD-DEPLOY.md) — How components are built
- [FLAKE-UPDATES.md](./FLAKE-UPDATES.md) — Understanding the three compartments
- [P4300](../+pm/done/P4300-automated-flake-updates.md) — Automated update feature (completed)
- [P1000](../+pm/backlog/P1000-reliable-agent-updates.md) — Reliable agent updates (in progress)
