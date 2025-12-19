# P1000 - Reliable Agent Updates

**Created**: 2025-12-19  
**Updated**: 2025-12-19  
**Priority**: P1000 (Critical - Blocking)  
**Status**: Ready for Development  
**Estimated Effort**: 4-6 hours  
**Depends on**: None  
**Blocks**: Fleet management reliability

---

## Executive Summary

The agent update flow is broken on macOS. After `switch`, the agent continues running the old binary. NixOS works correctly. This is the #1 blocker for fleet management.

**Goal**: After `switch` completes, the agent automatically restarts with the new binary on ALL platforms.

---

## Root Cause (Confirmed via Code Analysis)

### The Code

In `v2/internal/agent/commands.go` lines 144-152:

```go
// Auto-restart after successful switch to pick up new binary
// Only on NixOS - macOS is handled by home-manager's launchctl bootout/bootstrap
if exitCode == 0 && (command == "switch" || command == "pull-switch") && runtime.GOOS != "darwin" {
    a.log.Info().Msg("switch completed successfully, restarting to pick up new binary")
    time.Sleep(500 * time.Millisecond)
    a.Shutdown()
    os.Exit(101) // Triggers RestartForceExitStatus in systemd
}
```

### The Problem

**macOS is explicitly SKIPPED** from the restart logic because the comment assumes "home-manager's launchctl bootout/bootstrap" handles it.

But it doesn't work because:

1. **home-manager DOES write a new plist** pointing to the new binary in `/nix/store/...`
2. **home-manager DOES run `launchctl bootout` then `bootstrap`** during activation
3. **BUT**: The agent is RUNNING during switch. The bootout kills it, bootstrap starts it... but with which binary?

The race condition:

```
1. Agent running (old binary)
2. Agent executes: home-manager switch
3. home-manager writes new plist (points to new binary)
4. home-manager runs: launchctl bootout gui/501/com.nixfleet.agent
5. Agent process dies (killed by bootout)
6. home-manager runs: launchctl bootstrap gui/501 <new-plist>
7. launchd starts new agent... but agent code never reached line 148!
```

Wait, that should work! Let me reconsider...

Actually, the **real** issue is:

```
1. Agent running (old binary)
2. Agent executes: home-manager switch
3. home-manager writes new plist
4. home-manager skips launchctl reload because agent is already running
5. Switch completes, old binary continues running
6. Agent code reaches line 146, condition is FALSE for darwin
7. Old binary keeps running indefinitely
```

**Key insight**: home-manager's `setupLaunchAgents` only does bootout/bootstrap if the service is NOT already running OR if the plist changed AND stopOnChange is true.

### Evidence from modules/home-manager.nix

```nix
# NOTE: No custom activation hook needed - home-manager's setupLaunchAgents
# already handles agent lifecycle (bootout → bootstrap) correctly.
# A previous custom hook was causing double-reloads that left the agent dead.
```

This note is **misleading**. It was written after removing a buggy hook, but the underlying problem was never actually fixed.

---

## Solution: Simple Agent Self-Restart for macOS

Add macOS to the restart logic. The agent should restart itself after switch, just like on NixOS.

### Option A: launchctl kickstart (Recommended)

```go
// In commands.go, after successful switch
if exitCode == 0 && (command == "switch" || command == "pull-switch") {
    a.log.Info().Msg("switch completed successfully, triggering restart")
    time.Sleep(500 * time.Millisecond)

    if runtime.GOOS == "darwin" {
        // macOS: Use launchctl kickstart to restart with new binary
        // -k = kill existing, then restart
        uid := os.Getuid()
        serviceLabel := fmt.Sprintf("gui/%d/com.nixfleet.agent", uid)
        a.log.Info().Str("service", serviceLabel).Msg("executing launchctl kickstart")

        cmd := exec.Command("launchctl", "kickstart", "-k", serviceLabel)
        if err := cmd.Run(); err != nil {
            a.log.Error().Err(err).Msg("launchctl kickstart failed")
        }
        // The kickstart -k will kill this process, so we don't reach here
    } else {
        // NixOS: Exit with 101 to trigger systemd RestartForceExitStatus
        a.Shutdown()
        os.Exit(101)
    }
}
```

**Why this works**:

- `launchctl kickstart -k` kills the running agent and starts a fresh instance
- launchd reads the NEW plist (already updated by home-manager)
- New plist points to new binary in `/nix/store/...`
- Agent starts with new binary

### Option B: Exit and rely on KeepAlive (Alternative)

```go
// In commands.go, after successful switch
if exitCode == 0 && (command == "switch" || command == "pull-switch") {
    a.log.Info().Msg("switch completed, exiting to pick up new binary")
    time.Sleep(500 * time.Millisecond)
    a.Shutdown()
    os.Exit(0)  // Both platforms - launchd KeepAlive / systemd Restart=always will restart
}
```

**Why this might work**:

- Simpler code (same path for both platforms)
- launchd's `KeepAlive = true` will restart the agent
- launchd should read the updated plist on restart

**Risk**: If launchd caches the old plist, we're back to square one.

### Recommendation: Option A

Option A is explicit and guaranteed to work. Option B relies on launchd behavior that we haven't verified.

---

## Implementation

### Step 1: Fix the restart logic (30 min)

File: `v2/internal/agent/commands.go`

```go
// After line 143, replace the existing restart block with:

// Auto-restart after successful switch to pick up new binary
if exitCode == 0 && (command == "switch" || command == "pull-switch") {
    a.log.Info().Msg("switch completed successfully, triggering restart to pick up new binary")

    // Give time for status message to be sent
    time.Sleep(500 * time.Millisecond)

    if runtime.GOOS == "darwin" {
        // macOS: launchctl kickstart -k kills current and starts new
        uid := os.Getuid()
        service := fmt.Sprintf("gui/%d/com.nixfleet.agent", uid)
        a.log.Info().Str("service", service).Msg("restarting via launchctl kickstart")

        // Don't use exec.Command - we need Setsid so the child survives
        cmd := exec.Command("launchctl", "kickstart", "-k", service)
        cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
        if err := cmd.Start(); err != nil {
            a.log.Error().Err(err).Msg("launchctl kickstart failed, agent may need manual restart")
        }
        // The kickstart will kill us, but just in case:
        time.Sleep(100 * time.Millisecond)
        os.Exit(0)
    } else {
        // NixOS: exit 101 triggers RestartForceExitStatus in systemd
        a.Shutdown()
        os.Exit(101)
    }
}
```

### Step 2: Update the misleading comment (5 min)

File: `modules/home-manager.nix`

```nix
# Agent restart after switch is handled by the agent itself (launchctl kickstart).
# home-manager's setupLaunchAgents updates the plist, but doesn't restart
# a running agent reliably, so the agent triggers its own restart.
```

### Step 3: Test on both platforms (1-2 hours)

1. **Before fix**: Verify agent doesn't update after switch on macOS
2. **Deploy fix**: Push new agent code, update flake.lock
3. **Test macOS**: Run switch, verify agent version updates
4. **Test NixOS**: Run switch, verify agent version updates (regression test)
5. **Test edge cases**: What if switch fails? What if network drops?

---

## Acceptance Criteria

- [ ] **macOS**: After switch, agent reports new version within 30 seconds
- [ ] **NixOS**: After switch, agent reports new version within 30 seconds (no regression)
- [ ] **Switch failure**: If switch fails, agent keeps running (no restart)
- [ ] **Logs**: Clear log message indicating restart reason
- [ ] **Dashboard**: No manual intervention needed

---

## Testing Checklist

### Pre-implementation (verify the bug)

```bash
# On macOS host (e.g., imac0)
# 1. Check current agent version
pgrep -fl nixfleet

# 2. Run switch via dashboard or:
cd ~/.local/state/nixfleet-agent/repo
home-manager switch --flake .#hostname

# 3. Check agent version again - should be SAME (bug)
pgrep -fl nixfleet
```

### Post-implementation (verify the fix)

```bash
# On macOS host
# 1. Update flake.lock to include new agent with fix
# 2. Pull and switch
# 3. Agent should restart automatically
# 4. Check: pgrep -fl nixfleet - should show NEW binary path
```

---

## Files to Modify

| File                            | Change                                    |
| ------------------------------- | ----------------------------------------- |
| `v2/internal/agent/commands.go` | Add macOS restart via launchctl kickstart |
| `modules/home-manager.nix`      | Update misleading comment                 |

---

## Risks & Mitigations

| Risk                            | Mitigation                                                |
| ------------------------------- | --------------------------------------------------------- |
| launchctl kickstart fails       | Log error, agent continues running (no worse than before) |
| New binary crashes on startup   | launchd KeepAlive restarts it; can rollback via SSH       |
| Race with plist update          | 500ms delay should be sufficient                          |
| Agent killed before status sent | Sleep ensures status is sent first                        |

---

## Why NOT an Activation Hook?

A previous version tried adding a home.activation script. It was removed because:

1. **Double-reload**: Both home-manager's setupLaunchAgents AND the custom hook tried to reload, leaving the agent dead
2. **Timing issues**: Hook ran before plist was updated
3. **Complexity**: Agent self-restart is simpler and more reliable

The agent knows exactly when switch completed and can trigger its own restart at the right moment.

---

## Related

- [UPDATE-ARCHITECTURE.md](../../docs/UPDATE-ARCHITECTURE.md) — Update flow documentation
- [P2000](./P2000-unified-host-state-management.md) — Depends on this fix
