# P4100 - Go Agent Packaging & Deployment

**Priority**: High (blocks E2E testing)
**Depends on**: T01-T03 (agent tests) ✅ Complete
**Status**: ✅ Done

---

## Goal

Package the v2 Go agent as a Nix flake and deploy to test hosts.

---

## Test Hosts

| Host         | OS    | Type    | Current Agent | Notes                   |
| ------------ | ----- | ------- | ------------- | ----------------------- |
| gpc0         | NixOS | Desktop | v1 (bash)     | NixOS module deployment |
| mba-mbp-work | macOS | Laptop  | v1 (bash)     | Home Manager deployment |

---

## Tasks

### 1. Build Go Agent Binary

```bash
# In v2/
go build -o bin/nixfleet-agent ./cmd/nixfleet-agent
```

**Output**: `nixfleet-agent` binary (~10MB)

### 2. Create Nix Package

```nix
# packages/nixfleet-agent.nix
{ buildGoModule, lib }:
buildGoModule rec {
  pname = "nixfleet-agent";
  version = "2.0.0";
  src = ../v2;
  vendorHash = "sha256-...";
  subPackages = [ "cmd/nixfleet-agent" ];
}
```

### 3. Update NixOS Module

```nix
# modules/nixos.nix - changes needed:
# - Replace bash agent with Go agent
# - Update environment variables for v2 protocol
# - WebSocket URL instead of HTTP
```

**Key changes**:
| v1 (bash) | v2 (Go) |
| ---------------------------- | ------------------------------ |
| `NIXFLEET_URL` (HTTP) | `NIXFLEET_URL` (WebSocket) |
| `NIXFLEET_INTERVAL` polling | `NIXFLEET_INTERVAL` heartbeat |
| Bash script | Go binary |

### 4. Update Home Manager Module

```nix
# modules/home-manager.nix - changes needed:
# - Replace bash agent with Go agent
# - LaunchAgent plist updates for Go binary
```

### 5. Deploy to Test Hosts

#### gpc0 (NixOS)

```bash
# On gpc0 or via nixcfg
cd ~/Code/nixcfg
nix flake update nixfleet
sudo nixos-rebuild switch --flake .#gpc0
```

**Verification**:

```bash
systemctl status nixfleet-agent
journalctl -u nixfleet-agent -f
```

#### mba-mbp-work (macOS)

```bash
# On mba-mbp-work
cd ~/Code/nixcfg
nix flake update nixfleet
home-manager switch --flake ".#mba@mba-mbp-work"
```

**Verification**:

```bash
launchctl list | grep nixfleet
tail -f /tmp/nixfleet-agent.log
```

---

## Rollback Plan

If v2 agent fails:

1. Revert nixcfg to previous commit
2. Rebuild: `nixos-rebuild switch` or `home-manager switch`
3. v1 agent resumes

---

## Acceptance Criteria

- [x] Go agent builds via Nix
- [x] NixOS module updated for v2
- [x] Home Manager module updated for v2
- [x] gpc0 running v2 agent, visible in dashboard (2025-12-14)
- [x] mba-mbp-work running v2 agent, visible in dashboard (2025-12-14)
- [ ] Both can receive commands via WebSocket (manual test pending)

---

## Decisions

1. **No parallel operation** - Direct swap, rollback via Nix if needed
2. **Same URL** - Keep `fleet.barta.cm`
3. **Reuse tokens** - Same agent tokens from agenix
