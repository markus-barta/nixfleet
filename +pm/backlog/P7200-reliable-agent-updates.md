# P7200 - Reliable Agent Updates

**Created**: 2025-12-19  
**Priority**: P7200 (Critical - Blocking)  
**Status**: Ready for Development  
**Estimated Effort**: 1-2 days  
**Depends on**: None  
**Blocks**: All fleet update operations

---

## Executive Summary

The agent update flow is broken on macOS and unreliable across the fleet. This is the #1 blocker for fleet management. Without reliable agent updates, NixFleet cannot fulfill its core purpose.

**Goal**: After clicking "Switch" in the dashboard, the agent should automatically restart with the new binary within 60 seconds on ALL platforms.

---

## Problem Statement

Users have run `switch` multiple times via the UI AND via SSH, yet agents still report old versions:

| Host  | Expected Agent | Actual Agent | Switches Run | Status    |
| ----- | -------------- | ------------ | ------------ | --------- |
| imac0 | 2.1.0          | 2.0.0        | Many         | ❌ Broken |
| gpc0  | 2.1.0          | 2.0.0        | Multiple     | ❌ Broken |

This breaks the fundamental value proposition of NixFleet.

---

## Root Causes (from P7100 analysis)

### 1. flake.lock Version Mismatch

The isolated repo may have pulled the latest `nixcfg` commits, but the `flake.lock` inside still points to an old `nixfleet` revision.

**Check**: Does `flake.lock` point to the commit with `Version = "2.1.0"`?

### 2. Switch Not Creating New Generation

If flake.lock hasn't changed since last switch, Nix may skip rebuilding:

- "Nothing to do" = no new generation
- Agent stays on current version

**Check**: Did a new home-manager generation appear after switch?

### 3. macOS launchd Not Reloading (P7100)

Even when switch creates a new generation:

- home-manager updates the plist file
- launchd has old plist in memory
- Old binary continues running

**Check**: Does plist point to new store path? Is launchd running old binary?

### 4. NixOS systemd Exit Code 101

On NixOS, the agent exits with code 101 after switch, expecting systemd to restart it. This usually works but can fail if:

- Switch fails silently
- Agent crashes before exit(101)

---

## Implementation Plan

### Phase 1: Diagnostic Tooling (Day 1 morning)

Add dashboard diagnostic endpoints to debug update issues:

```go
// GET /api/hosts/{id}/update-debug
type UpdateDebugInfo struct {
    // Isolated repo state
    RepoCommit      string `json:"repo_commit"`
    RepoNixfleetRev string `json:"repo_nixfleet_rev"` // from flake.lock

    // Agent state
    AgentVersion    string `json:"agent_version"`
    AgentStorePath  string `json:"agent_store_path"` // running binary path
    AgentPID        int    `json:"agent_pid"`

    // Expected state
    ExpectedVersion string `json:"expected_version"` // dashboard version

    // Platform-specific
    PlistStorePath  string `json:"plist_store_path,omitempty"` // macOS: what plist says
    ServiceStatus   string `json:"service_status"` // systemd/launchd state

    // Generation info
    CurrentGen      string `json:"current_gen"` // home-manager/nixos generation
    LastSwitchTime  string `json:"last_switch_time"`
}
```

### Phase 2: macOS Auto-Restart (Day 1 afternoon)

Implement automatic agent restart after switch on macOS.

**Option A: Activation Script (preferred)**

```nix
# In modules/home-manager.nix
home.activation.restartNixfleetAgent = lib.hm.dag.entryAfter ["setupLaunchAgents"] ''
  if [[ "$(uname)" == "Darwin" ]]; then
    # Only restart if plist exists and agent is running
    if launchctl list | grep -q com.nixfleet.agent; then
      $DRY_RUN_CMD /bin/launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent || true
    fi
  fi
'';
```

**Option B: Agent Self-Restart**

```go
// In commands.go, after successful switch on macOS
if runtime.GOOS == "darwin" && exitCode == 0 && command == "switch" {
    // Check if plist points to a different binary
    newPath := getPlistAgentPath()
    currentPath := getCurrentBinaryPath()
    if newPath != currentPath {
        log.Info().Msg("plist updated, triggering restart via launchctl")
        exec.Command("launchctl", "kickstart", "-k",
            fmt.Sprintf("gui/%d/com.nixfleet.agent", os.Getuid())).Run()
    }
}
```

### Phase 3: Version Verification (Day 2 morning)

After switch completes, verify the agent actually updated:

```go
// In flake_updates.go runDeploy()
// After switch completes...

// Wait for agent to restart and reconnect
time.Sleep(10 * time.Second)

// Verify agent version
for _, hostID := range hosts {
    agent := s.hub.GetAgent(hostID)
    if agent == nil {
        s.log.Warn().Str("host", hostID).Msg("agent not reconnected after switch")
        continue
    }

    // Get reported version from last heartbeat
    host := s.getHostFromDB(hostID)
    if host.AgentVersion != Version {
        s.log.Error().
            Str("host", hostID).
            Str("expected", Version).
            Str("actual", host.AgentVersion).
            Msg("agent version mismatch after switch")

        // macOS: Try force restart
        if host.HostType == "macos" {
            s.hub.SendCommand(hostID, "restart")
        }
    }
}
```

### Phase 4: UI Feedback (Day 2 afternoon)

Show clear feedback when agent version doesn't match:

1. Lock compartment tooltip shows version mismatch clearly
2. After switch, if agent version still wrong, show warning
3. Add "Force Restart" button for macOS hosts

---

## Acceptance Criteria

- [ ] **NixOS**: After switch, agent reports new version within 30 seconds
- [ ] **macOS**: After switch, agent automatically restarts with new binary
- [ ] **Verification**: Dashboard confirms agent version matches after deploy
- [ ] **Diagnostics**: `/api/hosts/{id}/update-debug` endpoint works
- [ ] **Fallback**: "Restart Agent" button works when auto-restart fails

---

## Testing Checklist

### Pre-implementation (verify current state)

- [ ] Document current agent versions on all hosts
- [ ] Document current flake.lock nixfleet revisions
- [ ] Run switch on one macOS host, observe result

### Post-implementation

- [ ] NixOS: Switch via UI → agent version updates within 30s
- [ ] macOS: Switch via UI → agent version updates within 60s
- [ ] Fleet deploy: All hosts update correctly
- [ ] Rollback: Old generation still works if switch fails

---

## Files to Modify

| File                                     | Changes                                 |
| ---------------------------------------- | --------------------------------------- |
| `modules/home-manager.nix`               | Add activation script for agent restart |
| `v2/internal/agent/commands.go`          | Optional: self-restart logic for macOS  |
| `v2/internal/dashboard/handlers.go`      | Add `/api/hosts/{id}/update-debug`      |
| `v2/internal/dashboard/flake_updates.go` | Add version verification after deploy   |
| `v2/internal/templates/dashboard.templ`  | Better version mismatch UI feedback     |

---

## Related

- [P7100](./P7100-macos-agent-update-bug.md) — Root cause analysis (macOS specific)
- [P7000](./P7000-unified-host-state-management.md) — State architecture (depends on this fix)
- [UPDATE-ARCHITECTURE.md](../../docs/UPDATE-ARCHITECTURE.md) — Update flow documentation
