# Build & Deployment Guide

How NixFleet gets built and deployed to your fleet.

---

## Overview

NixFleet has two components:

| Component     | What it is                      | Where it runs                 | How it's built              |
| ------------- | ------------------------------- | ----------------------------- | --------------------------- |
| **Dashboard** | Web UI + WebSocket server       | Docker container on csb1      | Docker build from Go source |
| **Agent**     | Background service on each host | NixOS systemd / macOS launchd | Nix builds Go binary        |

---

## Dashboard

### How It's Built

The dashboard is a Go application that gets built inside a Docker container.

```
┌─────────────────────────────────────────────────────────┐
│  Your Machine                                           │
│  └── git push to GitHub                                 │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  csb1 Server                                            │
│  └── git pull                                           │
│  └── docker compose build                               │
│      └── Downloads Go 1.24 Alpine image                 │
│      └── Runs: go mod download                          │
│      └── Runs: templ generate (HTML templates)          │
│      └── Runs: go build (creates binary)                │
│      └── Copies binary + assets to slim Alpine image    │
│  └── docker compose up -d                               │
│      └── Starts container with new binary               │
└─────────────────────────────────────────────────────────┘
```

### Deployment Steps

```bash
# On csb1 (via SSH)
cd ~/docker/nixfleet
git pull

cd ~/docker
docker compose build nixfleet
docker compose up -d nixfleet
```

That's it. The dashboard restarts with the new code.

### Key Files

| File                                | Purpose                                              |
| ----------------------------------- | ---------------------------------------------------- |
| `v2/Dockerfile`                     | Multi-stage build: compile Go, copy to runtime image |
| `v2/cmd/nixfleet-dashboard/main.go` | Entry point                                          |
| `v2/internal/dashboard/`            | All dashboard code                                   |
| `v2/internal/templates/`            | HTML templates (templ)                               |
| `assets/`                           | Static images                                        |

---

## Agent

### How It's Built

The agent is built by Nix as part of your system configuration.

```
┌─────────────────────────────────────────────────────────┐
│  Your nixcfg Repository                                 │
│  └── flake.lock points to nixfleet repo                 │
│      └── inputs.nixfleet = "github:user/nixfleet"       │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  When you run: nixos-rebuild switch                     │
│  (or: home-manager switch on macOS)                     │
│                                                         │
│  Nix does:                                              │
│  └── Fetches nixfleet from GitHub (cached)              │
│  └── Builds Go agent via buildGoModule                  │
│  └── Installs binary to /nix/store/...                  │
│  └── Creates systemd/launchd service                    │
│  └── Agent restarts with new binary                     │
└─────────────────────────────────────────────────────────┘
```

### Deployment Steps

**Option A: Update nixfleet version in flake.lock**

```bash
# In your nixcfg repo
nix flake update nixfleet
git add flake.lock
git commit -m "chore: update nixfleet"
git push
```

Then on each host:

```bash
# NixOS
sudo nixos-rebuild switch --flake .#hostname

# macOS
home-manager switch --flake .#hostname
```

**Option B: Via NixFleet Dashboard**

Click "Switch" button in the dashboard. The agent:

1. Pulls latest nixcfg
2. Runs nixos-rebuild/home-manager switch
3. Restarts itself with new binary

### Key Files

| File                             | Purpose                        |
| -------------------------------- | ------------------------------ |
| `v2/cmd/nixfleet-agent/main.go`  | Entry point                    |
| `v2/internal/agent/`             | All agent code                 |
| `packages/nixfleet-agent-v2.nix` | Nix package definition         |
| `modules/nixos.nix`              | NixOS module (systemd service) |
| `modules/home-manager.nix`       | Home Manager module (launchd)  |

---

## Version Flow

```
nixfleet repo                    nixcfg repo                     Host
─────────────                    ───────────                     ────
    │                                 │                            │
    │ ◄── you push code               │                            │
    │                                 │                            │
    │                    flake.lock ──┼── points to nixfleet rev   │
    │                                 │                            │
    │                                 │ ◄── you run switch         │
    │                                 │                            │
    │                                 │ ────────────────────────►  │
    │                                 │     Nix builds agent       │
    │                                 │     from nixfleet rev      │
    │                                 │                            │
```

To update agents across your fleet:

1. Push changes to nixfleet repo
2. Update `flake.lock` in nixcfg: `nix flake update nixfleet`
3. Push nixcfg
4. Run switch on each host (or click Switch in dashboard)

---

## Current Limitations

### Manual Steps Required

1. **Dashboard**: Must SSH to csb1 and run docker commands
2. **Agents**: Must update flake.lock manually, then switch each host
3. **No CI/CD**: No automated builds or deployments

---

## Potential Improvements

### 1. GitHub Actions for Dashboard

Build and push Docker image automatically on every push:

```yaml
# .github/workflows/dashboard.yml
on:
  push:
    branches: [master]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build and push
        run: |
          docker build -t ghcr.io/user/nixfleet:latest -f v2/Dockerfile .
          docker push ghcr.io/user/nixfleet:latest
```

Then csb1 just pulls the pre-built image instead of building locally.

**Benefit**: Faster deploys, no build on server, versioned images.

### 2. Automated flake.lock Updates

GitHub Action that creates a PR when nixfleet changes:

```yaml
# In nixcfg repo: .github/workflows/update-nixfleet.yml
on:
  repository_dispatch:
    types: [nixfleet-updated]
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Update flake.lock
        run: nix flake update nixfleet
      - name: Create PR
        uses: peter-evans/create-pull-request@v5
```

**Benefit**: One-click to update all hosts via PR merge.

### 3. Webhook-Triggered Deployment

Dashboard could have a `/deploy` webhook that:

1. Receives GitHub push notification
2. Pulls latest code
3. Rebuilds and restarts itself

**Benefit**: Zero-touch dashboard updates.

---

## Quick Reference

| Task                 | Command                                                                                                |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| Deploy dashboard     | `ssh csb1 "cd ~/docker && git pull && docker compose build nixfleet && docker compose up -d nixfleet"` |
| Update agent version | `nix flake update nixfleet` in nixcfg                                                                  |
| Deploy to NixOS host | `sudo nixos-rebuild switch --flake .#hostname`                                                         |
| Deploy to macOS host | `home-manager switch --flake .#hostname`                                                               |
| Check dashboard logs | `ssh csb1 "docker logs nixfleet --tail 50"`                                                            |
| Check agent logs     | `journalctl -u nixfleet-agent -f` (NixOS)                                                              |
