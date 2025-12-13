# macOS Agent Not Restarting After home-manager Switch

**Priority**: High  
**Created**: 2025-12-13  
**Status**: Open

## Problem

On macOS hosts, when `home-manager switch` is executed (either manually or via NixFleet agent), the agent process gets killed but doesn't reliably restart. This leaves the host appearing offline in the dashboard until someone manually restarts the agent.

### Observed Behavior

1. Agent receives `switch` command
2. Agent starts `home-manager switch`
3. Switch process updates `~/Library/LaunchAgents/com.nixfleet.agent.plist`
4. launchd kills the running agent (because the plist changed)
5. **Agent does NOT restart** → Host goes offline

### Evidence

**mba-mbp-work (2025-12-13 ~16:07)**:

```
[2025-12-13 16:07:09] [INFO] Received command: switch
[2025-12-13 16:07:09] [INFO] Executing: switch (macos)
# Agent died here - no further logs for 1+ hour
```

**imac0 (2025-12-13 earlier)**:

- Same pattern: switch command started, agent died, no restart

### Previous Attempt

We tried adding a custom `home.activation.reloadNixfleetAgent` hook that would restart the agent after switch, but it caused issues:

- Double-reloads
- Race conditions
- Sometimes left the agent dead anyway

The hook was removed in favor of relying on launchd's built-in lifecycle management, but launchd doesn't always restart the agent.

## Root Cause Analysis

The issue is the order of operations:

1. `home-manager switch` runs as a child process of the agent
2. Switch updates the launchd plist
3. launchd detects plist change and kills the agent
4. The parent process (agent) is dead, so no post-switch hook can run
5. launchd should restart via `KeepAlive = true`, but doesn't always

### Why KeepAlive Might Fail

- The plist itself is being replaced during switch
- launchd may not properly track the new plist
- Race condition between plist replacement and launchd's monitoring

## Proposed Solutions

### Option A: Pre-Switch Agent Detach

Before running switch, the agent spawns the switch process as a fully detached daemon, then exits cleanly. The detached switch process:

1. Waits for agent to exit
2. Runs home-manager switch
3. Reloads the agent via launchctl

**Pros**: Clean separation, agent isn't killed mid-operation  
**Cons**: Complex, requires new script/binary

### Option B: launchd Watchdog Script

Create a separate launchd job that monitors the agent and restarts it if missing:

```xml
<key>StartInterval</key>
<integer>30</integer>
<key>ProgramArguments</key>
<array>
  <string>/bin/sh</string>
  <string>-c</string>
  <string>launchctl list | grep -q com.nixfleet.agent || launchctl load ~/Library/LaunchAgents/com.nixfleet.agent.plist</string>
</array>
```

**Pros**: Simple, works regardless of how agent dies  
**Cons**: Additional launchd job, 30-second recovery delay

### Option C: Switch in Background with Explicit Reload

Modify the agent's `do_switch()` to:

1. Run switch in background: `nohup home-manager switch ... &`
2. Immediately report "Switch started (async)"
3. Add a final step in home-manager activation that always runs `launchctl kickstart`

**Pros**: Agent exits gracefully before being killed  
**Cons**: Switch completion status not reported back to dashboard

### Option D: External Switch Wrapper

Create a wrapper script that's installed separately (not managed by home-manager) that:

1. Agent calls wrapper instead of direct switch
2. Wrapper runs switch
3. Wrapper reloads agent after switch completes

**Pros**: Reliable, wrapper survives the switch  
**Cons**: Extra complexity, wrapper must be installed separately

## Recommended Approach

**Option B (Watchdog)** is the most reliable and least invasive:

- Works for any crash/kill scenario, not just switch
- Simple to implement
- Can be part of the home-manager module
- 30-second worst-case recovery is acceptable

## Acceptance Criteria

- [ ] macOS agent reliably restarts after `home-manager switch`
- [ ] Maximum offline time after switch is < 60 seconds
- [ ] Works for both user-initiated and NixFleet-initiated switches
- [ ] No manual intervention required
- [ ] Solution works on both Intel and Apple Silicon Macs

## Affected Hosts

- imac0
- mba-mbp-work
- mba-imac-work
- (Any future macOS host with home-manager)

## Related

- NixOS doesn't have this problem (systemd handles restarts properly)
- Agent isolated repo mode (completed) - not directly related but context
