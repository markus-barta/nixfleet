# P4500 - v2 Migration Execution Plan

**Priority**: High
**Status**: Ready to execute
**Date**: 2024-12-14

---

## Overview

Complete migration from v1 (Python/Bash) to v2 (Go) with clean slate approach.

---

## Phase 1: Decommission v1

### 1.1 Disable v1 Agents on ALL Hosts

#### NixOS Hosts (systemd)

| Host | SSH Command                    | Config Location                     |
| ---- | ------------------------------ | ----------------------------------- |
| hsb0 | `ssh hsb0`                     | nixcfg/hosts/hsb0/configuration.nix |
| hsb1 | `ssh hsb1`                     | nixcfg/hosts/hsb1/configuration.nix |
| hsb8 | `ssh hsb8`                     | nixcfg/hosts/hsb8/configuration.nix |
| csb0 | `ssh -p 2222 mba@cs0.barta.cm` | nixcfg/hosts/csb0/configuration.nix |
| csb1 | `ssh -p 2222 mba@cs1.barta.cm` | nixcfg/hosts/csb1/configuration.nix |
| gpc0 | `ssh gpc0`                     | nixcfg/hosts/gpc0/configuration.nix |

**Action**: Set `services.nixfleet-agent.enable = false;` in each host config.

```bash
# In nixcfg, for each NixOS host:
# Edit hosts/<hostname>/configuration.nix
services.nixfleet-agent.enable = false;
```

**Deploy**:

```bash
# From a NixOS machine (e.g., hsb1)
cd ~/Code/nixcfg

# Commit the changes
git add -A
git commit -m "chore: disable v1 nixfleet agents for v2 migration"
git push

# Rebuild each host (can be done in parallel)
ssh hsb0 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb0"
ssh hsb1 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb1"
ssh hsb8 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb8"
ssh -p 2222 mba@cs0.barta.cm "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#csb0"
ssh -p 2222 mba@cs1.barta.cm "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#csb1"
ssh gpc0 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#gpc0"
```

**Verify**:

```bash
# On each NixOS host
systemctl status nixfleet-agent
# Should show: inactive or not found
```

#### macOS Hosts (launchd)

| Host         | SSH Command                  | Config Location                    |
| ------------ | ---------------------------- | ---------------------------------- |
| mba-mbp-work | `ssh mba@mba-mbp-work.local` | nixcfg/hosts/mba-mbp-work/home.nix |
| imac0        | `ssh mba@imac0.local`        | nixcfg/hosts/imac0/home.nix        |

**Action**: Set `services.nixfleet-agent.enable = false;` in home.nix.

```bash
# In nixcfg, for each macOS host:
# Edit hosts/<hostname>/home.nix
services.nixfleet-agent.enable = false;
```

**Deploy**:

```bash
# On mba-mbp-work
cd ~/Code/nixcfg && git pull
home-manager switch --flake ".#mba@mba-mbp-work"

# On imac0
cd ~/Code/nixcfg && git pull
home-manager switch --flake ".#mba@imac0"
```

**Verify**:

```bash
# On each macOS host
launchctl list | grep nixfleet
# Should show nothing
```

### 1.2 Stop v1 Dashboard on csb1

```bash
ssh -p 2222 mba@cs1.barta.cm

# Stop v1 dashboard
cd ~/docker/nixfleet
docker compose down

# Backup database
cp data/nixfleet.db data/nixfleet.db.v1-backup-$(date +%Y%m%d)

# Verify stopped
docker ps | grep nixfleet
# Should show nothing
```

### 1.3 Verification Checklist - Phase 1

- [ ] hsb0: v1 agent stopped
- [ ] hsb1: v1 agent stopped
- [ ] hsb8: v1 agent stopped
- [ ] csb0: v1 agent stopped
- [ ] csb1: v1 agent stopped
- [ ] gpc0: v1 agent stopped
- [ ] mba-mbp-work: v1 agent stopped
- [ ] imac0: v1 agent stopped
- [ ] csb1: v1 dashboard stopped
- [ ] csb1: database backed up

---

## Phase 2: Deploy v2

### 2.1 Prepare v2 Dashboard

**On development machine**:

```bash
cd ~/Code/nixfleet

# Build v2 dashboard locally to verify
cd v2
go build -o ../bin/nixfleet-dashboard ./cmd/nixfleet-dashboard

# Run tests
go test ./tests/integration/...
```

### 2.2 Create v2 Docker Setup

**Update Dockerfile** (already exists, needs v2 version):

```bash
# In nixfleet/
# Create Dockerfile.v2 or update existing
```

### 2.3 Deploy v2 Dashboard to csb1

```bash
ssh -p 2222 mba@cs1.barta.cm

cd ~/docker/nixfleet

# Pull latest code
git pull

# Build v2 container
docker compose build

# Start v2 dashboard
docker compose up -d

# Check logs
docker logs nixfleet-nixfleet-1 -f
```

**Verify**:

```bash
# Test login page
curl -s https://fleet.barta.cm/login | head -20

# Test WebSocket endpoint
curl -s -o /dev/null -w "%{http_code}" https://fleet.barta.cm/ws
# Should return 401 (unauthorized, but endpoint exists)
```

### 2.4 Restart csb1 and Verify

```bash
ssh -p 2222 mba@cs1.barta.cm

# Reboot
sudo reboot

# Wait 2 minutes, then reconnect
ssh -p 2222 mba@cs1.barta.cm

# Verify dashboard auto-started
docker ps | grep nixfleet
# Should show running container

# Verify accessible
curl -s https://fleet.barta.cm/login | head -5
```

### 2.5 Deploy v2 Agent to Test Hosts

#### Prepare Nix Package

```bash
# In nixfleet/
# Update flake.nix with v2 agent package
# Update modules/nixos.nix for v2
# Update modules/home-manager.nix for v2
```

#### Deploy to gpc0 (NixOS)

```bash
# In nixcfg
# Edit hosts/gpc0/configuration.nix
services.nixfleet-agent = {
  enable = true;
  url = "wss://fleet.barta.cm/ws";  # Note: WebSocket URL
  # ... other options
};

# Deploy
ssh gpc0 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#gpc0"
```

**Verify**:

```bash
ssh gpc0
systemctl status nixfleet-agent
journalctl -u nixfleet-agent -f
# Should show: connected to dashboard, registered
```

#### Deploy to mba-mbp-work (macOS)

```bash
# In nixcfg
# Edit hosts/mba-mbp-work/home.nix
services.nixfleet-agent = {
  enable = true;
  url = "wss://fleet.barta.cm/ws";
  # ... other options
};

# On mba-mbp-work
cd ~/Code/nixcfg && git pull
home-manager switch --flake ".#mba@mba-mbp-work"
```

**Verify**:

```bash
launchctl list | grep nixfleet
tail -f /tmp/nixfleet-agent.log
# Should show: connected to dashboard, registered
```

### 2.6 Post-Restart Check on Test Hosts

#### gpc0

```bash
ssh gpc0
sudo reboot

# Wait, reconnect
ssh gpc0
systemctl status nixfleet-agent
# Should be running and connected
```

#### mba-mbp-work

```bash
# Restart the Mac
# After restart, verify:
launchctl list | grep nixfleet
# Should show agent running
```

### 2.7 Verification Checklist - Phase 2

- [ ] v2 dashboard running on csb1
- [ ] v2 dashboard survives csb1 reboot
- [ ] Login with password + TOTP works
- [ ] gpc0: v2 agent connected
- [ ] gpc0: survives reboot
- [ ] mba-mbp-work: v2 agent connected
- [ ] mba-mbp-work: survives reboot
- [ ] Dashboard shows both hosts online
- [ ] Can send commands to both hosts

---

## Phase 3: Later Expansion (Not Now)

When ready to expand v2 agents to remaining hosts:

| Host  | Priority | Notes                         |
| ----- | -------- | ----------------------------- |
| hsb1  | High     | Main home server              |
| hsb0  | Medium   | Secondary server              |
| hsb8  | Medium   | Additional server             |
| csb0  | Medium   | Cloud server                  |
| csb1  | Low      | Has dashboard, agent optional |
| imac0 | Low      | Desktop                       |

---

## Rollback Plan

### If v2 Dashboard Fails

```bash
ssh -p 2222 mba@cs1.barta.cm
cd ~/docker/nixfleet

# Stop v2
docker compose down

# Restore v1
git checkout HEAD~1  # or specific v1 commit
cp data/nixfleet.db.v1-backup-* data/nixfleet.db
docker compose build
docker compose up -d
```

### If v2 Agent Fails

```bash
# In nixcfg
# Set services.nixfleet-agent.enable = false;
# Rebuild host
```

---

## Success Criteria

- [ ] All v1 agents disabled
- [ ] v1 dashboard stopped
- [ ] v2 dashboard running and accessible
- [ ] v2 agents on gpc0 and mba-mbp-work
- [ ] Both hosts visible in dashboard
- [ ] Commands work (pull, switch)
- [ ] All survives reboot
