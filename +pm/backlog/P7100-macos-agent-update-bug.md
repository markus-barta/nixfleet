# P7100: macOS Agent Update Bug

**Created**: 2025-12-18  
**Priority**: High  
**Status**: Open

---

## Problem

On macOS hosts, after running `pull` → `switch` via the NixFleet UI, the agent does **not** automatically update to the new version. The agent continues running the OLD binary even after home-manager switch completes successfully.

### Root Causes Identified

1. **Race Condition with `os.Exit(101)`** (FIXED in ebbc837):
   - The old agent code called `os.Exit(101)` after a successful switch
   - On NixOS (systemd), this triggers `RestartForceExitStatus` correctly
   - On macOS (launchd), this races with home-manager's `launchctl bootout/bootstrap` sequence
   - launchd's `KeepAlive = true` restarts the agent with the OLD plist before home-manager updates it

2. **Isolated Repo flake.lock Stale**:
   - The agent's isolated repo (`~/.local/state/nixfleet-agent/repo`) has its own `flake.lock`
   - When `pull` is clicked, it fetches the latest nixcfg but the `flake.lock` inside may point to an old nixfleet commit
   - `home-manager switch` then builds with the old nixfleet version

3. **launchd Doesn't Reload Plist Automatically**:
   - Even after home-manager updates the plist in `~/Library/LaunchAgents/`, launchd continues using the in-memory version
   - Requires `launchctl kickstart -k` to force reload

---

## Symptoms

- Agent reports old version (e.g., 2.0.0) after switch
- Lock compartment shows red (agent outdated) even after successful switch
- Server logs show: `agent registered agent_version=2.0.0` instead of expected version

---

## Current Workaround

After `pull` → `switch` on macOS, manually run:

```bash
launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent
```

Or use the "Restart Agent" option in the ellipsis menu.

---

## Proposed Solutions

### Short-term (Manual)

1. Document the need for manual agent restart after switch on macOS
2. Update tooltip on Lock compartment to mention this

### Medium-term (Automation)

1. **Auto-restart via activation script**: Add a home-manager activation hook that runs `launchctl kickstart -k` after agent plist changes

2. **Update flake.lock during pull**: Modify the `pull` command on agents to also run `nix flake update nixfleet` in the isolated repo

### Long-term (Architecture)

1. **Use launchd's WatchPaths**: Configure launchd to watch for plist changes and auto-reload
2. **Agent self-update mechanism**: Agent detects version mismatch and triggers its own restart

---

## Affected Hosts

- mba-imac-work (macOS)
- imac0 (macOS)
- mba-mbp-work (macOS - currently offline)

NixOS hosts (csb0, csb1, hsb0, hsb1, gpc0, hsb8) are **not affected** - systemd handles the restart correctly.

---

## Related

- P7000: Unified Host State Management (introduced version checking)
- Fix commit: ebbc837 (don't auto-restart agent on macOS after switch)

---

## Acceptance Criteria

- [ ] After `pull` → `switch` on macOS, agent automatically restarts with new version
- [ ] No manual `launchctl kickstart` required
- [ ] Agent version in dashboard matches expected version within 30 seconds of switch completion
