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
| hsb0 | `ssh mba@hsb0.lan`             | nixcfg/hosts/hsb0/configuration.nix |
| hsb1 | `ssh mba@hsb1.lan`             | nixcfg/hosts/hsb1/configuration.nix |
| hsb8 | `ssh mba@hsb8.lan`             | nixcfg/hosts/hsb8/configuration.nix |
| csb0 | `ssh mba@cs0.barta.cm -p 2222` | nixcfg/hosts/csb0/configuration.nix |
| csb1 | `ssh mba@cs1.barta.cm -p 2222` | nixcfg/hosts/csb1/configuration.nix |
| gpc0 | `ssh mba@gpc0.lan`             | nixcfg/hosts/gpc0/configuration.nix |

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
ssh mba@hsb0.lan "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb0"
ssh mba@hsb1.lan "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb1"
ssh mba@hsb8.lan "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#hsb8"
ssh mba@cs0.barta.cm -p 2222 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#csb0"
ssh mba@cs1.barta.cm -p 2222 "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#csb1"
ssh mba@gpc0.lan "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#gpc0"
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
| imac0        | `ssh markus@192.168.1.150`   | nixcfg/hosts/imac0/home.nix        |

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
ssh mba@cs1.barta.cm -p 2222

# Stop v1 dashboard
cd ~/docker/nixfleet
docker compose down

# Backup database
mkdir -p ~/backups
cp data/nixfleet.db ~/backups/nixfleet.db.v1-backup-$(date +%Y%m%d)

# Verify stopped
docker ps | grep nixfleet
# Should show nothing
```

### 1.3 Clean Up Separate NixFleet Compose

**Problem**: NixFleet currently has its own compose file (`~/docker/nixfleet/docker-compose.yml`), separate from the main compose file (`~/docker/docker-compose.yml`). This is confusing - Traefik works best with all services in one file.

**Solution**: Add NixFleet v2 to the main compose file, remove the separate one.

```bash
ssh mba@cs1.barta.cm -p 2222

# Archive the old nixfleet folder (keep for reference)
mv ~/docker/nixfleet ~/docker/!archiv/nixfleet-v1-$(date +%Y%m%d)

# The v2 NixFleet service will be added to ~/docker/docker-compose.yml
```

### 1.4 Verification Checklist - Phase 1

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
- [ ] csb1: old nixfleet folder archived

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

### 2.2 Add NixFleet v2 to Main Compose File

**On csb1**, add the NixFleet service to `~/docker/docker-compose.yml`:

```yaml
# NixFleet - Fleet Management Dashboard (v2 Go)
nixfleet:
  build:
    context: ./nixfleet
    dockerfile: Dockerfile
  restart: unless-stopped
  env_file:
    - ~/secrets/nixfleet.env
  environment:
    - NIXFLEET_DB_PATH=/data/nixfleet.db
  volumes:
    - nixfleet_data:/data
  networks:
    - traefik
  labels:
    - traefik.enable=true
    - traefik.http.routers.nixfleet.rule=Host(`fleet.barta.cm`)
    - traefik.http.routers.nixfleet.tls.certresolver=default
    - traefik.http.routers.nixfleet.tls=true
    - traefik.http.services.nixfleet.loadbalancer.server.port=8000
    - traefik.docker.network=csb1_traefik
    - traefik.http.routers.nixfleet.middlewares=cloudflarewarp@file
```

Also add volume:

```yaml
volumes:
  # ... existing volumes ...
  nixfleet_data: {} # Data for NixFleet dashboard
```

### 2.3 Create v2 Source Directory

```bash
ssh mba@cs1.barta.cm -p 2222

# Clone nixfleet repo for the v2 source/Dockerfile
cd ~/docker
git clone https://github.com/markus-barta/nixfleet.git nixfleet

# Create secrets file (follows ~/secrets/<service>.env pattern)
cat > ~/secrets/nixfleet.env << 'EOF'
NIXFLEET_PASSWORD_HASH=<bcrypt-hash>
NIXFLEET_SESSION_SECRET=<32-char-secret>
NIXFLEET_AGENT_TOKEN=<agent-token>
NIXFLEET_TOTP_SECRET=<totp-secret>
EOF
# Note: reuse values from existing v1 .env where applicable
```

### 2.4 Create v2 Dockerfile

```bash
# In ~/docker/nixfleet/Dockerfile (v2)
```

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY v2/go.mod v2/go.sum ./
RUN go mod download
COPY v2/ ./
RUN CGO_ENABLED=1 go build -o nixfleet-dashboard ./cmd/nixfleet-dashboard

FROM alpine:3.19
RUN apk add --no-cache sqlite-libs
COPY --from=builder /app/nixfleet-dashboard /usr/local/bin/
EXPOSE 8000
CMD ["nixfleet-dashboard"]
```

### 2.5 Deploy v2 Dashboard

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker

# Update main compose file with nixfleet service (as shown above)
# Then:
docker compose build nixfleet
docker compose up -d nixfleet

# Check logs
docker compose logs -f nixfleet
```

**Verify**:

```bash
# Test login page
curl -s https://fleet.barta.cm/login | head -20

# Test WebSocket endpoint
curl -s -o /dev/null -w "%{http_code}" https://fleet.barta.cm/ws
# Should return 401 (unauthorized, but endpoint exists)
```

### 2.6 Restart csb1 and Verify

```bash
ssh mba@cs1.barta.cm -p 2222

# Reboot
sudo reboot

# Wait 2 minutes, then reconnect
ssh mba@cs1.barta.cm -p 2222

# Verify dashboard auto-started
docker ps | grep nixfleet
# Should show running container

# Verify accessible
curl -s https://fleet.barta.cm/login | head -5
```

### 2.7 Deploy v2 Agent to Test Hosts

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
ssh mba@gpc0.lan "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#gpc0"
```

**Verify**:

```bash
ssh mba@gpc0.lan
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

### 2.8 Post-Restart Check on Test Hosts

#### gpc0

```bash
ssh mba@gpc0.lan
sudo reboot

# Wait, reconnect
ssh mba@gpc0.lan
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

### 2.9 Verification Checklist - Phase 2

- [ ] v2 added to main compose file
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
ssh mba@cs1.barta.cm -p 2222
cd ~/docker

# Stop v2
docker compose stop nixfleet

# Restore v1 from archive
mv ~/docker/!archiv/nixfleet-v1-* ~/docker/nixfleet
cp ~/backups/nixfleet.db.v1-backup-* ~/docker/nixfleet/data/nixfleet.db

# Restart v1 (separate compose)
cd ~/docker/nixfleet
docker compose up -d

# Comment out nixfleet from main compose file
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
