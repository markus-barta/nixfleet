# T02: Switch Command

Test the NixOS rebuild / home-manager switch command via dashboard.

## Prerequisites

- NixFleet dashboard running
- Agent registered and online
- Valid Nix configuration in nixcfg repository
- Sufficient privileges (sudo for NixOS)

## What This Test Verifies

| Component       | Verification                                    |
| --------------- | ----------------------------------------------- |
| OS Detection    | Agent correctly identifies NixOS vs macOS       |
| NixOS Switch    | Uses `sudo nixos-rebuild switch --flake`        |
| macOS Switch    | Uses `home-manager switch --flake`              |
| Flake Path      | Correct flake reference with hostname           |
| Status Report   | Reports success/failure with details            |
| Generation      | New system generation activated                 |

## OS-Specific Behavior

### NixOS

```bash
sudo nixos-rebuild switch --flake /path/to/nixcfg#hostname
```

- Requires sudo privileges
- Activates new system generation
- May require reboot for kernel updates

### macOS (Home Manager)

```bash
home-manager switch --flake /path/to/nixcfg#hostname
```

- Runs as regular user
- Updates user profile
- Reloads launchd agents if needed

## Manual Test Procedures

### Test 1: UI Switch Button

**Steps:**

1. Open dashboard: <https://fleet.barta.cm>
2. Find a host row with agent online
3. Click "Switch" button
4. Observe status column

**Expected Results:**

- Button shows loading state
- Status shows "⧖ Switching..."
- After completion: "✓ Switch successful" or "✗ Switch failed: reason"
- May take 30-120 seconds for full rebuild

**Status:** ⏳ Pending

### Test 2: Agent Log During Switch

**Steps:**

1. SSH to target host
2. Tail agent log:

   ```bash
   tail -f /tmp/nixfleet-agent.err
   ```

3. Trigger switch from dashboard

**Expected Results:**

- Log shows: `[INFO] Received command: switch`
- Log shows: `[INFO] Executing: switch (nixos|macos)`
- Log shows build output or errors
- Log shows: `[INFO] Switch completed`

**Status:** ⏳ Pending

### Test 3: NixOS Generation

**Steps:**

1. On NixOS host, check current generation:

   ```bash
   sudo nix-env --list-generations -p /nix/var/nix/profiles/system | tail -3
   ```

2. Trigger switch from dashboard
3. Check generations again

**Expected Results:**

- New generation created with current date/time
- New generation is now current

**Status:** ⏳ Pending

### Test 4: Home Manager Generation (macOS)

**Steps:**

1. On macOS host, check current generation:

   ```bash
   home-manager generations | head -3
   ```

2. Trigger switch from dashboard
3. Check generations again

**Expected Results:**

- New generation created
- Profile updated

**Status:** ⏳ Pending

### Test 5: Agent Survives Switch

**Steps:**

1. Note agent PID before switch:

   ```bash
   pgrep -f nixfleet-agent
   ```

2. Trigger switch from dashboard
3. Wait for completion
4. Check agent status

**Expected Results:**

- Agent continues running after switch
- OR agent restarts automatically (macOS launchd)
- Dashboard shows host still online

**Status:** ⏳ Pending

### Test 6: Error Handling

**Steps:**

1. Create a broken Nix configuration (syntax error)
2. Commit and push
3. Pull on host
4. Trigger switch

**Expected Results:**

- Switch fails with clear error message
- Status shows "✗ Switch failed: build error"
- System remains on previous working generation
- Agent continues running

**Status:** ⏳ Pending

## Agent Function Reference

The agent's `do_switch()` function:

```bash
do_switch() {
    log_info "Executing: switch ($OS_NAME)"
    cd "$NIXCFG_PATH" || return 1
    
    if [[ "$OS_NAME" == "macOS" ]]; then
        home-manager switch --flake ".#${HOSTNAME}"
    else
        sudo nixos-rebuild switch --flake ".#${HOSTNAME}"
    fi
    
    if [[ $? -eq 0 ]]; then
        report_status "success" "Switch completed"
    else
        report_status "error" "Switch failed"
    fi
}
```

## Summary

- Total Tests: 6
- Passed: 0
- Pending: 6

## Related

- Automated: [T02-command-switch.sh](./T02-command-switch.sh)
- Agent: [nixfleet-agent.sh](../agent/nixfleet-agent.sh) - `do_switch()` function
- NixOS Module: [nixos.nix](../modules/nixos.nix)
- Home Manager Module: [home-manager.nix](../modules/home-manager.nix)
