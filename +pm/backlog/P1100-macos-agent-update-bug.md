# P1100: macOS Agent Update Bug

**Created**: 2025-12-18  
**Updated**: 2025-12-19  
**Priority**: High (see P1000 for implementation)  
**Status**: Analysis Complete → Superseded by P1000

---

## Problem

On macOS hosts, after running `pull` → `switch` via the NixFleet UI (and even via SSH), the agent does **not** update to the new version. The agent continues running the OLD binary even after home-manager switch completes successfully.

This has been observed multiple times:

- Switch run from UI
- Switch run from SSH directly
- Multiple switches run on same host
- **Result**: Agent still reports old version (e.g., 2.0.0 instead of 2.1.0)

---

## Root Cause Analysis

### The Update Chain

For the agent to update, this entire chain must succeed:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. nixfleet repo has new agent code (Version = "2.1.0")                    │
│                         ▼                                                   │
│  2. nixcfg flake.lock points to that nixfleet commit                        │
│                         ▼                                                   │
│  3. Host's isolated repo has latest flake.lock (via pull)                   │
│                         ▼                                                   │
│  4. home-manager switch builds NEW agent from that flake.lock               │
│                         ▼                                                   │
│  5. Switch creates a NEW home-manager generation                            │
│                         ▼                                                   │
│  6. launchd plist is updated to point to NEW store path                     │
│                         ▼                                                   │
│  7. Running agent is restarted to use NEW binary                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Any break in this chain = agent stays on old version.**

### Observed Failure Points

#### Failure Point A: flake.lock Not Updated

**Evidence on imac0 (2025-12-19)**:

- Isolated repo commit: `af2e7a18` (Dec 18 14:07) — contains nixfleet 2.1.0
- Last home-manager generation: Dec 17 19:24 — **before** the 2.1.0 commit
- LaunchAgent plist: points to `nixfleet-agent-2.0.0`

**Conclusion**: The pull happened, but no switch was run AFTER the pull that brought in 2.1.0.

#### Failure Point B: launchd Doesn't Reload Plist

Even when switch creates a new generation:

- home-manager updates `~/Library/LaunchAgents/com.nixfleet.agent.plist`
- But launchd has the OLD plist **in memory**
- The running agent continues with the OLD binary
- Requires explicit `launchctl kickstart -k` to reload

#### Failure Point C: Switch Doesn't Create New Generation

If the flake.lock hasn't changed since the last switch, Nix may:

- Determine "nothing changed"
- Skip creating a new generation
- Agent stays on current version

This is actually correct behavior — but confusing when you expect an update.

#### Failure Point D: Race Condition with Agent Exit

Previously (fixed in ebbc837):

- Agent called `os.Exit(101)` after switch
- On macOS, this raced with home-manager's `launchctl bootout/bootstrap`
- launchd's `KeepAlive = true` sometimes restarted the OLD agent

**Current behavior** (ebbc837+): Agent does NOT auto-exit on macOS. This avoids the race but means the agent must be manually restarted.

---

## Diagnostic Commands

Check the current state on a macOS host:

```bash
# What agent version is running?
pgrep -a nixfleet-agent

# What does the plist point to?
grep nixfleet ~/Library/LaunchAgents/com.nixfleet.agent.plist | grep store

# When was the last home-manager switch?
home-manager generations | head -5

# What nixfleet version is in the isolated repo?
grep -A 10 '"nixfleet"' ~/.local/state/nixfleet-agent/repo/flake.lock | grep rev

# What commit is the isolated repo on?
git -C ~/.local/state/nixfleet-agent/repo log -1 --oneline
```

---

## Current Workaround

After `pull` → `switch` on macOS, manually restart the agent:

```bash
launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent
```

Or use the **⋮** → **Restart Agent** option in the dashboard.

---

## Proposed Solutions

### Short-term: Documentation & UI

1. ✅ Document the issue (this file + UPDATE-ARCHITECTURE.md)
2. Add tooltip on Lock compartment when agent is outdated: "Run Switch, then Restart Agent"
3. Make "Restart Agent" more prominent when agent version mismatches

### Medium-term: Activation Script

Add a home-manager activation hook that automatically restarts the agent when the plist changes:

```nix
# In modules/home-manager.nix
home.activation.restartNixfleetAgent = lib.hm.dag.entryAfter ["reloadSystemd"] ''
  # Only run if plist changed
  if [[ -f ~/Library/LaunchAgents/com.nixfleet.agent.plist ]]; then
    $DRY_RUN_CMD /bin/launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent || true
  fi
'';
```

**Pros**: Automatic, no user action needed
**Cons**: Requires home-manager module change, needs testing

### Medium-term: Agent Self-Restart on macOS

After switch completes successfully on macOS:

1. Agent detects the plist was updated (compare file hash)
2. Agent calls `launchctl kickstart -k` on itself
3. New agent binary starts

```go
// In commands.go, after successful switch on macOS
if runtime.GOOS == "darwin" && exitCode == 0 {
    // Trigger self-restart via launchctl
    exec.Command("launchctl", "kickstart", "-k",
        fmt.Sprintf("gui/%d/com.nixfleet.agent", os.Getuid())).Run()
}
```

**Pros**: Works within current architecture
**Cons**: Agent kills itself, may look like a crash

### Long-term: Unified Update Command

Implement a single "Update Host" command that:

1. Pulls latest code
2. Runs switch
3. Waits for completion
4. Verifies new generation created
5. On macOS, triggers `launchctl kickstart`
6. Verifies agent reports new version

This consolidates all the steps and handles platform differences automatically.

---

## Affected Hosts

- imac0 (macOS) — confirmed affected
- mba-imac-work (macOS) — likely affected
- mba-mbp-work (macOS) — likely affected (currently offline)

NixOS hosts (csb0, csb1, hsb0, hsb1, gpc0, hsb8) use systemd's `RestartForceExitStatus=101` and are **not affected** — the agent correctly restarts with the new binary after switch.

---

## Related

- **[P1000](./P1000-reliable-agent-updates.md)** — Implementation plan (this issue is now tracked there)
- [UPDATE-ARCHITECTURE.md](../../docs/UPDATE-ARCHITECTURE.md) — Complete update flow documentation
- [P2000](./P2000-unified-host-state-management.md) — Introduced version checking
- [P4300](./P4300-automated-flake-updates.md) — Automated fleet updates
- Fix commit: ebbc837 (don't auto-restart agent on macOS after switch)

---

## Acceptance Criteria

- [ ] After `pull` → `switch` on macOS, agent automatically restarts with new version
- [ ] No manual `launchctl kickstart` required
- [ ] Agent version in dashboard matches expected version within 60 seconds of switch completion
- [ ] Works reliably for all macOS hosts (imac0, mba-imac-work, mba-mbp-work)

---

## Implementation Notes

### Why NixOS Works But macOS Doesn't

| Platform | Service Manager | Restart Mechanism            | Works?     |
| -------- | --------------- | ---------------------------- | ---------- |
| NixOS    | systemd         | `RestartForceExitStatus=101` | ✅ Yes     |
| macOS    | launchd         | `KeepAlive=true`             | ⚠️ Partial |

**NixOS**: Agent exits with code 101 → systemd restarts it → new binary from updated symlink

**macOS**:

- Agent can't safely exit during switch (race condition)
- Even if switch completes, launchd has old plist in memory
- Need explicit kickstart to reload

### The Fundamental Tension

The agent is updating itself via the switch command. This is inherently tricky:

- The running agent binary is in `/nix/store/xxx-nixfleet-agent-OLD/`
- Switch creates a new binary in `/nix/store/yyy-nixfleet-agent-NEW/`
- The OLD binary is still running
- Something must restart the agent to pick up the NEW binary

On NixOS, systemd's symlink-based activation handles this.
On macOS, launchd doesn't have this mechanism — the plist points to a specific store path.
