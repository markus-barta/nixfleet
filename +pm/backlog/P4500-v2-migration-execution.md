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

### Current Setup Analysis (as of 2024-12-14)

**v1 Setup on csb1:**

- **Compose file**: `~/docker/nixfleet/docker/docker-compose.csb1.yml` (SEPARATE from main!)
- **Env file**: `~/docker/nixfleet/.env` (NOT in `~/secrets/` pattern)
- **Domain**: `fleet.barta.cm`
- **Traefik**: Properly integrated via labels, no port exposure
- **Container**: `nixfleet` running on `csb1_traefik` network

**Why NOT `ports: 8000:8000`?**

- The example compose has this, but the actual csb1 deployment does NOT
- With Traefik, you NEVER expose ports directly to host
- Traefik connects to containers via Docker network (internal port 8000)
- Direct port exposure would bypass Traefik (no TLS, no middleware)

### 2.1 Prepare v2 Dashboard

**On development machine**:

```bash
cd ~/Code/nixfleet

# Build v2 locally to verify it compiles
cd v2
go build -o ../bin/nixfleet-dashboard ./cmd/nixfleet-dashboard

# Run integration tests
go test ./tests/integration/... -v

# Verify Docker build works
docker build -t nixfleet:v2-test -f Dockerfile .
```

### 2.2 Add NixFleet v2 to Main Compose File

**On csb1**, add the NixFleet service to `~/docker/docker-compose.yml`.

Following the **exact pattern** of other services (paperless, docmost, etc.):

```yaml
# ============================================
# NixFleet - Fleet Management Dashboard (v2)
# ============================================
nixfleet:
  build:
    context: ./nixfleet/v2
    dockerfile: Dockerfile
  container_name: nixfleet
  restart: unless-stopped
  env_file:
    - ~/secrets/nixfleet.env
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
  healthcheck:
    test: ["CMD", "wget", "-q", "--spider", "http://localhost:8000/login"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 10s
```

**Also add volume** (in the `volumes:` section at the bottom):

```yaml
nixfleet_data: {} # Data for NixFleet dashboard (DB, logs)
```

**Key differences from v1:**

- Build context: `./nixfleet/v2` (v2 subdirectory)
- Dockerfile: `v2/Dockerfile` (Go-based)
- Env file: `~/secrets/nixfleet.env` (follows pattern)
- Healthcheck: Uses `wget` instead of Python

### 2.3 Create v2 Source Directory

```bash
ssh mba@cs1.barta.cm -p 2222

# Update the existing nixfleet repo (don't re-clone, it's already there)
cd ~/docker/nixfleet
git fetch origin
git checkout master
git pull

# Verify v2 directory exists
ls -la v2/
# Should show: cmd/, internal/, go.mod, go.sum, Dockerfile
```

### 2.4 Create Secrets File

**Migrate secrets from v1 `.env` to `~/secrets/nixfleet.env`:**

```bash
ssh mba@cs1.barta.cm -p 2222

# Extract values from v1 env file
cat ~/docker/nixfleet/.env

# Create new secrets file (with v2 variable names)
cat > ~/secrets/nixfleet.env << 'EOF'
# NixFleet v2 Configuration
# Migrated from v1 on YYYY-MM-DD

# Authentication (reuse from v1)
NIXFLEET_PASSWORD_HASH=$2b$12$...  # Copy from v1 .env
NIXFLEET_SESSION_SECRET=...        # Generate: openssl rand -hex 32
NIXFLEET_AGENT_TOKEN=...           # Copy NIXFLEET_API_TOKEN from v1
NIXFLEET_TOTP_SECRET=...           # Copy from v1 .env

# Optional
NIXFLEET_LOG_LEVEL=info
EOF

chmod 600 ~/secrets/nixfleet.env
```

**Variable mapping (v1 → v2):**

| v1 Variable                       | v2 Variable             | Notes         |
| --------------------------------- | ----------------------- | ------------- |
| NIXFLEET_PASSWORD_HASH            | NIXFLEET_PASSWORD_HASH  | Same          |
| NIXFLEET_SESSION_SECRETS          | NIXFLEET_SESSION_SECRET | Singular!     |
| NIXFLEET_API_TOKEN                | NIXFLEET_AGENT_TOKEN    | Renamed       |
| NIXFLEET_TOTP_SECRET              | NIXFLEET_TOTP_SECRET    | Same          |
| NIXFLEET_AGENT_TOKEN_HASH_SECRET  | (not used)              | Removed in v2 |
| NIXFLEET_ALLOW_SHARED_AGENT_TOKEN | (not used)              | Removed in v2 |

### 2.5 Deploy v2 Dashboard

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker

# Build v2 image
docker compose build nixfleet

# Start v2 dashboard
docker compose up -d nixfleet

# Watch logs for errors
docker compose logs -f nixfleet
# Look for: "starting dashboard server" and no errors
```

**Verify**:

```bash
# Test login page renders
curl -s https://fleet.barta.cm/login | grep -i "NixFleet"

# Test WebSocket endpoint exists
curl -s -o /dev/null -w "%{http_code}" https://fleet.barta.cm/ws
# Should return: 401 (unauthorized, but endpoint exists)

# Check container health
docker ps | grep nixfleet
# Should show: (healthy)
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

## Pre-Deployment: Test All Existing Services

**CRITICAL**: Before ANY changes, verify all services are working!

```bash
ssh mba@cs1.barta.cm -p 2222

# Save current state
docker ps > ~/backups/docker-ps-before-$(date +%Y%m%d-%H%M).txt

# Test each service (record results)
echo "=== Service Availability Test ===" | tee ~/backups/services-test-$(date +%Y%m%d-%H%M).txt

# Traefik dashboard
curl -s -o /dev/null -w "cs1.barta.cm: %{http_code}\n" https://cs1.barta.cm

# Docmost
curl -s -o /dev/null -w "docmost.barta.cm: %{http_code}\n" https://docmost.barta.cm

# Grafana
curl -s -o /dev/null -w "grafana.barta.cm: %{http_code}\n" https://grafana.barta.cm

# InfluxDB
curl -s -o /dev/null -w "influxdb.barta.cm: %{http_code}\n" https://influxdb.barta.cm

# Paperless
curl -s -o /dev/null -w "paperless.barta.cm: %{http_code}\n" https://paperless.barta.cm

# NixFleet v1 (current)
curl -s -o /dev/null -w "fleet.barta.cm: %{http_code}\n" https://fleet.barta.cm

# Whoami (test service)
curl -s -o /dev/null -w "whoami1.barta.cm: %{http_code}\n" https://whoami1.barta.cm
```

**Expected results**: All should return 200 or 302 (redirect to login).

---

## Post-Deployment: Verify All Services

Run the same test after deployment. **Any regression = immediate rollback!**

---

## Rollback Plan

### 🚨 EMERGENCY: If v2 Dashboard Fails

**Symptoms**: `fleet.barta.cm` not accessible, container unhealthy, login broken

**Time to rollback: < 5 minutes**

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker

# 1. IMMEDIATELY stop v2
docker compose stop nixfleet
docker compose rm -f nixfleet

# 2. Check other services are still running
docker ps | grep -E "traefik|docmost|grafana|paperless"
# If any are down, restart them:
# docker compose up -d traefik docmost grafana paperless

# 3. Restore v1 from archive
mv ~/docker/!archiv/nixfleet-v1-* ~/docker/nixfleet-v1-restored

# 4. Restore database if needed
cp ~/backups/nixfleet.db.v1-backup-* ~/docker/nixfleet-v1-restored/data/nixfleet.db

# 5. Start v1 (separate compose)
cd ~/docker/nixfleet-v1-restored/docker
docker compose -f docker-compose.csb1.yml up -d

# 6. Verify v1 is back
curl -s https://fleet.barta.cm/login | grep -i nixfleet

# 7. Comment out nixfleet from main compose to prevent confusion
cd ~/docker
# Edit docker-compose.yml, comment out the nixfleet service
```

### 🚨 EMERGENCY: If Other Services Break After v2 Deployment

**Symptoms**: docmost, grafana, paperless, etc. become inaccessible

**This should NOT happen** (we're only adding, not modifying), but if it does:

```bash
ssh mba@cs1.barta.cm -p 2222
cd ~/docker

# 1. Check what's running
docker ps -a

# 2. Check Traefik logs for routing errors
docker compose logs traefik | tail -50

# 3. If Traefik is confused, restart it
docker compose restart traefik

# 4. If still broken, stop nixfleet and restart everything
docker compose stop nixfleet
docker compose up -d

# 5. Verify all services
# Run the pre-deployment test script again
```

### 🚨 EMERGENCY: If v2 Agent Fails

**Symptoms**: Agent not connecting, errors in journal/logs

```bash
# On the affected host

# 1. Check logs
# NixOS:
journalctl -u nixfleet-agent -f
# macOS:
cat /tmp/nixfleet-agent.log

# 2. Disable agent temporarily
# In nixcfg, set: services.nixfleet-agent.enable = false;
# Deploy the change

# 3. Agent is now disabled - dashboard continues working
# Debug at leisure, re-enable when fixed
```

---

## Post-Migration Verification Script

Save this script and run it after deployment:

```bash
#!/bin/bash
# post-migration-check.sh

echo "=== NixFleet v2 Migration Verification ==="
echo "Date: $(date)"
echo ""

echo "1. Docker containers:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "NAME|nixfleet|traefik"
echo ""

echo "2. All services responding:"
for domain in cs1.barta.cm docmost.barta.cm grafana.barta.cm influxdb.barta.cm paperless.barta.cm fleet.barta.cm whoami1.barta.cm; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "https://$domain" 2>/dev/null)
  if [[ "$code" == "200" || "$code" == "302" ]]; then
    echo "  ✅ $domain: $code"
  else
    echo "  ❌ $domain: $code (PROBLEM!)"
  fi
done
echo ""

echo "3. NixFleet specifics:"
curl -s https://fleet.barta.cm/login | grep -q "NixFleet" && echo "  ✅ Login page renders" || echo "  ❌ Login page broken"
curl -s -o /dev/null -w "  WebSocket endpoint: %{http_code}\n" https://fleet.barta.cm/ws
echo ""

echo "4. Container health:"
docker inspect nixfleet --format '{{.State.Health.Status}}' 2>/dev/null || echo "  ⚠️ No health status"
echo ""

echo "=== Verification Complete ==="
```

---

## Success Criteria

- [ ] Pre-deployment: All existing services tested and working
- [ ] v1 agents disabled on all hosts
- [ ] v1 dashboard stopped and archived
- [ ] v2 dashboard added to main compose file
- [ ] v2 dashboard running and accessible at fleet.barta.cm
- [ ] v2 dashboard survives csb1 reboot
- [ ] Login with password + TOTP works
- [ ] All OTHER services still working (docmost, grafana, paperless, etc.)
- [ ] gpc0: v2 agent connected
- [ ] gpc0: survives reboot
- [ ] mba-mbp-work: v2 agent connected
- [ ] mba-mbp-work: survives reboot
- [ ] Dashboard shows both hosts online
- [ ] Can send commands to both hosts
- [ ] Commands stream output in real-time
