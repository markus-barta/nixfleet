# NixFleet

<p align="center">
  <img src="app/static/nixfleet_fade.png" alt="NixFleet" width="60%">
</p>

<p align="center">
  <strong>Unified fleet management for your NixOS and macOS infrastructure.</strong><br>
  Deploy, monitor, and control all your hosts from a single dashboard.
</p>

---

**Part of the [nixcfg](https://github.com/markus-barta/nixcfg) ecosystem** — manage your Nix fleet with confidence.

## Features

### Dashboard

- **Real-time WebSocket Updates**: Live host status, no page refresh needed
- **Fleet Target Display**: Shows the Git commit all hosts should be on
- **Three-Compartment Status**: Git (behind/current), Lock (freshness), System (rebuild needed)
- **Agent Version Tracking**: Alerts when agents are outdated vs dashboard
- **Per-Host Actions**: Pull, Switch, Pull+Switch, Test, Stop, Remove
- **Bulk Actions**: Apply commands to all online hosts at once
- **Add/Remove Hosts**: Manage your fleet directly from the UI

### Agent

- **Isolated Repository Mode**: Agent maintains its own clone, no shared repo conflicts
- **Clean-Slate Pull**: `git fetch` + `git reset --hard` ensures repo matches origin
- **Unified Pattern**: Same Go agent for NixOS (`nixos-rebuild`) and macOS (`home-manager`)
- **Survives Restarts**: Uses detached process on macOS to survive `home-manager switch`
- **Automatic Registration**: Agents register on first connect, show up in dashboard

### Security

- **Password + TOTP**: Optional 2FA via authenticator apps
- **Signed Session Cookies**: Rotation-ready secret management
- **Per-Host Agent Tokens**: Each host can have its own token (hashed in DB)
- **CSRF Protection**: All forms protected against cross-site attacks
- **Rate Limiting**: Login, registration, and poll endpoints

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                         NIXFLEET DASHBOARD                              │
│                       (Go + templ + htmx)                               │
│                     https://fleet.example.com                           │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Fleet Target: abc1234 (main) • Agent v2.0.0                     │   │
│  ├──────────────────────────────────────────────────────────────────┤   │
│  │  HOST       │ STATUS │ UPDATE      │ ACTIONS                     │   │
│  │  ─────────────────────────────────────────────────────────────── │   │
│  │  hsb1       │ ● 🟢   │ [G][L][S]   │ [Pull] [Switch] [Test] [⋮]  │   │
│  │  csb0       │ ● 🟢   │ [G][L][S]   │ [Pull] [Switch] [Test] [⋮]  │   │
│  │  imac0      │ ● 🟢   │ [G][L][S][A]│ [Pull] [Switch] [Test] [⋮]  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                              │ WebSocket
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │  NixOS   │    │  NixOS   │    │  macOS   │
        │  Server  │    │  Desktop │    │  Laptop  │
        │          │    │          │    │          │
        │ Agent    │    │ Agent    │    │ Agent    │
        │ (Go)     │    │ (Go)     │    │ (Go)     │
        │          │    │          │    │          │
        │ nixos-   │    │ nixos-   │    │ home-    │
        │ rebuild  │    │ rebuild  │    │ manager  │
        └──────────┘    └──────────┘    └──────────┘
```

**Update Status Compartments:**

| Icon  | Meaning | Status                                                  |
| ----- | ------- | ------------------------------------------------------- |
| **G** | Git     | Green = current, Yellow = behind target                 |
| **L** | Lock    | Green = fresh, Yellow = stale flake.lock                |
| **S** | System  | Green = running matches config, Yellow = rebuild needed |
| **A** | Agent   | Red = agent version outdated vs dashboard               |

## Quick Start

### 1. Add to Your Flake

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixfleet.url = "github:markus-barta/nixfleet";
    nixfleet.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { nixpkgs, nixfleet, ... }: {
    # NixOS
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      modules = [
        nixfleet.nixosModules.nixfleet-agent
        ./configuration.nix
      ];
    };

    # Home Manager (standalone)
    homeConfigurations.myuser = home-manager.lib.homeManagerConfiguration {
      modules = [
        nixfleet.homeManagerModules.nixfleet-agent
        ./home.nix
      ];
    };
  };
}
```

### 2. Configure the Agent

**NixOS** (`configuration.nix`):

```nix
{ config, ... }:
{
  # Load the secret (using agenix, sops-nix, etc.)
  age.secrets.nixfleet-token.file = ./secrets/nixfleet-token.age;

  services.nixfleet-agent = {
    enable = true;
    url = "wss://fleet.example.com";  # Note: wss:// for WebSocket
    tokenFile = config.age.secrets.nixfleet-token.path;
    isolatedRepoMode = true;  # Recommended: agent manages its own repo clone
    location = "cloud";       # cloud | home | work | other
    deviceType = "server";    # server | desktop | laptop | gaming | other
  };
}
```

**Home Manager / macOS** (`home.nix`):

```nix
{
  services.nixfleet-agent = {
    enable = true;
    url = "wss://fleet.example.com";
    tokenFile = "/Users/myuser/.config/nixfleet/token";
    isolatedRepoMode = true;
    location = "home";
    deviceType = "desktop";
  };
}
```

### 3. Enable Version Tracking (Recommended)

For the dashboard to detect outdated hosts, your nixcfg repo needs to publish its version to GitHub Pages.

**Add this workflow to your nixcfg repo** (`.github/workflows/version-pages.yml`):

```yaml
name: Publish Version to GitHub Pages

on:
  push:
    branches: [main]

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Generate version.json
        run: |
          mkdir -p _site
          cat > _site/version.json << EOF
          {
            "repo": "${{ github.server_url }}/${{ github.repository }}",
            "gitCommit": "${{ github.sha }}",
            "message": $(git log -1 --format='%s' | jq -Rs .),
            "branch": "${{ github.ref_name }}",
            "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
          }
          EOF
      - uses: actions/configure-pages@v4
      - uses: actions/upload-pages-artifact@v3
        with:
          path: "_site"

  deploy:
    environment:
      name: github-pages
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/deploy-pages@v4
```

**Enable GitHub Pages** in your nixcfg repo:

1. Go to **Settings** → **Pages**
2. Set Source to **GitHub Actions**

The dashboard fetches from `https://<user>.github.io/<repo>/version.json` to compare against each host's deployed generation.

### 4. Deploy the Dashboard

```bash
# Clone the repo
git clone https://github.com/markus-barta/nixfleet.git
cd nixfleet

# Generate credentials
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt()).decode())"
openssl rand -hex 32  # NIXFLEET_SESSION_SECRET
openssl rand -hex 32  # NIXFLEET_AGENT_TOKEN

# Create .env file
cat > .env << EOF
NIXFLEET_PASSWORD_HASH=\$2b\$12\$...your-hash...
NIXFLEET_SESSION_SECRET=hexsecret
NIXFLEET_AGENT_TOKEN=your-agent-token
EOF

# Start the container
docker compose up -d
```

## Module Options

### NixOS Module (`services.nixfleet-agent`)

| Option             | Type   | Required | Description                                      |
| ------------------ | ------ | -------- | ------------------------------------------------ |
| `enable`           | bool   | -        | Enable the agent                                 |
| `url`              | string | **Yes**  | Dashboard WebSocket URL (`wss://...`)            |
| `tokenFile`        | path   | **Yes**  | Path to API token file                           |
| `isolatedRepoMode` | bool   | No       | Agent manages its own repo clone (default: true) |
| `repoUrl`          | string | No       | Git URL for isolated mode (auto-detected)        |
| `interval`         | int    | No       | Heartbeat interval in seconds (default: 30)      |
| `location`         | enum   | No       | Location category (default: "other")             |
| `deviceType`       | enum   | No       | Device type (default: "server")                  |
| `themeColor`       | string | No       | Hex color for dashboard (default: "#769ff0")     |

### Home Manager Module

Same options as NixOS module.

## Dashboard Commands

| Command       | Description                                         |
| ------------- | --------------------------------------------------- |
| `pull`        | Git pull (isolated mode: fetch + reset --hard)      |
| `switch`      | Run `nixos-rebuild switch` or `home-manager switch` |
| `pull-switch` | Run both in sequence                                |
| `test`        | Run host test suite (`hosts/<host>/tests/T*.sh`)    |
| `stop`        | Stop currently running command                      |

## API Endpoints

| Endpoint                  | Method   | Auth    | Description                |
| ------------------------- | -------- | ------- | -------------------------- |
| `/`                       | GET      | Session | Dashboard UI               |
| `/login`                  | GET/POST | -       | Login page                 |
| `/health`                 | GET      | -       | Health check               |
| `/ws`                     | GET      | Token   | Agent WebSocket connection |
| `/api/hosts`              | GET      | Session | List all hosts             |
| `/api/hosts`              | POST     | Session | Add a new host             |
| `/api/hosts/{id}`         | DELETE   | Session | Remove a host              |
| `/api/hosts/{id}/command` | POST     | Session | Queue a command            |

## Environment Variables

| Variable                  | Required | Description                           |
| ------------------------- | -------- | ------------------------------------- |
| `NIXFLEET_PASSWORD_HASH`  | Yes      | bcrypt hash of admin password         |
| `NIXFLEET_SESSION_SECRET` | Yes      | Secret for signed session cookies     |
| `NIXFLEET_AGENT_TOKEN`    | Yes      | Shared agent authentication token     |
| `NIXFLEET_TOTP_SECRET`    | No       | Base32 TOTP secret for 2FA            |
| `NIXFLEET_LOG_LEVEL`      | No       | Log level (debug, info, warn, error)  |
| `NIXFLEET_VERSION_URL`    | No       | URL to version.json for Git status    |
| `NIXFLEET_DATA_DIR`       | No       | Database directory (default: `/data`) |

## Operations

### Deploying Dashboard Updates

The v2 dashboard runs on **csb1** as part of the main docker-compose stack.

```bash
# 1. SSH to csb1
ssh mba@cs1.barta.cm -p 2222

# 2. Pull latest nixfleet code
cd ~/docker/nixfleet && git pull

# 3. Rebuild and restart
cd ~/docker
docker compose build --no-cache nixfleet
docker compose up -d nixfleet

# 4. Verify
docker logs nixfleet --tail 20
curl -s https://fleet.barta.cm/health
```

### Updating Agents on Hosts

After pushing nixfleet changes, update the flake lock and deploy:

```bash
# From dashboard UI (recommended):
# 1. Click "Pull" on host → updates repo
# 2. Click "Switch" on host → rebuilds system

# Or manually:
cd ~/Code/nixcfg
nix flake update nixfleet
git add flake.lock && git commit -m "chore: Update nixfleet" && git push

# Then deploy via dashboard or SSH
```

### Restarting Agents

Use the dashboard UI: **⋮ menu → actions**

Or manually:

```bash
# NixOS
sudo systemctl restart nixfleet-agent

# macOS
launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent
```

## Development

```bash
# Enter dev environment
cd v2
nix develop

# Run dashboard locally
go run ./cmd/nixfleet-dashboard

# Run agent locally
go run ./cmd/nixfleet-agent

# Run tests
go test ./tests/...
```

## License

GNU AGPL v3.0 — See [LICENSE](LICENSE) for details.
