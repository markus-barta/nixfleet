# Switch Command Makes Agents Irresponsive

**Priority**: High  
**Created**: 2025-12-13  
**Status**: Open

## Problem

After executing a `switch` command via NixFleet dashboard:

- **Pull** works correctly ✅
- **Switch** makes agents irresponsive ❌

The agent appears to stop responding after initiating a switch. Buttons remain disabled ("Switching...") but no completion is reported back to the dashboard.

## Observed Behavior

1. User clicks "Switch" on a host
2. Dashboard shows "Switching..." status
3. Agent stops sending heartbeats
4. Host appears stuck in "Switching..." state indefinitely
5. Agent may or may not have actually completed the switch

## Affected Hosts

- Likely all hosts after migration to isolated repo mode
- Both NixOS (systemd) and macOS (launchd) hosts

## Hypothesis

With isolated repo mode, the agent runs `nixos-rebuild switch` or `home-manager switch` from `/var/lib/nixfleet-agent/repo` (or `~/.local/state/nixfleet-agent/repo` on macOS).

The switch process may:

1. Update the agent binary/script itself
2. Cause systemd/launchd to restart the agent mid-operation
3. Agent dies before reporting completion status
4. New agent starts fresh, unaware of the previous command

## Related Issues

- `2025-12-13-macos-agent-restart-after-switch.md` - macOS-specific restart problem
- This issue may be the same root cause but affecting all platforms

## Investigation Needed

1. Check agent logs during switch on NixOS vs macOS
2. Verify if switch actually completes on the host
3. Check if agent restarts during switch
4. Determine if status report is sent before agent death

## Potential Solutions

### Option A: Detached Switch Process

Run switch in a fully detached process that:

1. Agent spawns detached switch wrapper
2. Agent exits cleanly
3. Switch runs to completion
4. Wrapper restarts agent
5. New agent reports success/failure

### Option B: Status Persistence

Before starting switch:

1. Write "switch started" to a state file
2. Run switch
3. If agent restarts, check state file on startup
4. Report completion based on state file

### Option C: External Watchdog

Separate process that:

1. Monitors switch commands
2. Detects agent restart during switch
3. Reports completion status on behalf of agent

## Acceptance Criteria

- [ ] Switch command completes and reports status back to dashboard
- [ ] Agent remains responsive after switch
- [ ] Works on both NixOS and macOS
- [ ] Dashboard shows accurate command status

## Notes

Stopping work for today (2025-12-13). Will investigate further.
