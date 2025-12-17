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

## Why NixFleet? 🚀

Managing multiple Nix machines can be a hassle. You've got servers in the cloud, desktops at home, laptops on the go — and they all need to stay in sync with your configuration. NixFleet gives you a single place to see everything, push updates, and know exactly which hosts need attention.

No more SSH-ing into each machine one by one. Just open your dashboard, see who's online, and hit "Switch" to deploy your latest config. It's that simple!

## Features

### Dashboard 🖥️

Your command center for the entire fleet! Everything updates in real-time via WebSockets, so you'll always see the current state without refreshing.

- **Real-time Updates**: Hosts appear the moment they connect, status changes instantly
- **Fleet Target**: See which Git commit everyone should be on at a glance
- **Smart Status Indicators**: Know immediately if a host is behind, needs a rebuild, or has a stale lock file
- **One-Click Actions**: Pull, Switch, or Test any host with a single click
- **Bulk Operations**: Update your entire fleet at once when you're feeling confident

### Agent 🤖

A tiny Go binary that runs on each host. It connects to the dashboard, reports its status, and executes commands when you click buttons.

- **Isolated Repository Mode**: Each agent keeps its own clone of your config — no conflicts with your working directory!
- **Clean-Slate Pull**: When you click "Pull", it does a proper `git fetch` + `git reset --hard` so you always get exactly what's on origin
- **Works Everywhere**: Same agent for NixOS (uses `nixos-rebuild`) and macOS (uses `home-manager`)
- **Survives Restarts**: On macOS, the agent cleverly detaches before running `home-manager switch` so it doesn't kill itself

### Security 🔒

We take security seriously. Your fleet is your infrastructure, after all!

- **Password + TOTP**: Add two-factor authentication for extra peace of mind
- **Signed Cookies**: Sessions are cryptographically signed and support key rotation
- **Per-Host Tokens**: Each machine can have its own unique token (hashed in the database)
- **CSRF Protection**: All forms are protected against cross-site request forgery
- **Rate Limiting**: Brute-force protection on login and registration endpoints

## How It Works

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                         NIXFLEET DASHBOARD                              │
│                       (Go + templ + htmx)                               │
│                     https://fleet.example.com                           │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Fleet Target: abc1234 (main) - Agent v2.0.0                     │   │
│  ├──────────────────────────────────────────────────────────────────┤   │
│  │  HOST           │ STATUS │ UPDATE      │ ACTIONS                 │   │
│  │  ──────────────────────────────────────────────────────────────  │   │
│  │  web-server     │ Online │ [G][L][S]   │ [Pull] [Switch] [Test]  │   │
│  │  home-nas       │ Online │ [G][L][S]   │ [Pull] [Switch] [Test]  │   │
│  │  macbook-pro    │ Online │ [G][L][S][A]│ [Pull] [Switch] [Test]  │   │
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

### Understanding the Status Indicators

The dashboard shows a compact status for each host using letter codes. Here's what they mean:

| Icon  | What It Checks | Green Means                      | Yellow/Red Means                   |
| ----- | -------------- | -------------------------------- | ---------------------------------- |
| **G** | Git status     | Host is on the target commit     | Host is behind, needs a pull       |
| **L** | Lock freshness | flake.lock was updated recently  | Lock file is getting stale         |
| **S** | System state   | Running config matches the flake | A rebuild would change something   |
| **A** | Agent version  | (not shown when current)         | Agent is outdated, update the host |

When everything is green, you're golden! ✨

## Quick Start

Ready to get going? Here's how to set up NixFleet in just a few steps.

### Step 1: Add NixFleet to Your Flake

First, add NixFleet as an input to your `flake.nix`. This gives you access to the agent modules.

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixfleet.url = "github:markus-barta/nixfleet";
    nixfleet.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { nixpkgs, nixfleet, ... }: {
    # For NixOS machines
    nixosConfigurations.my-server = nixpkgs.lib.nixosSystem {
      modules = [
        nixfleet.nixosModules.nixfleet-agent
        ./hosts/my-server/configuration.nix
      ];
    };

    # For macOS machines (using Home Manager standalone)
    homeConfigurations.my-macbook = home-manager.lib.homeManagerConfiguration {
      modules = [
        nixfleet.homeManagerModules.nixfleet-agent
        ./hosts/my-macbook/home.nix
      ];
    };
  };
}
```

### Step 2: Configure the Agent

Now tell each host how to connect to your dashboard.

**For NixOS hosts** — add this to your `configuration.nix`:

```nix
{ config, ... }:
{
  # Store your token securely (we recommend agenix or sops-nix)
  age.secrets.nixfleet-token.file = ./secrets/nixfleet-token.age;

  services.nixfleet-agent = {
    enable = true;
    url = "wss://fleet.example.com";      # Your dashboard URL (note: wss:// not https://)
    tokenFile = config.age.secrets.nixfleet-token.path;
    isolatedRepoMode = true;              # Highly recommended!
    location = "cloud";                   # Where is this machine? (cloud, home, work, other)
    deviceType = "server";                # What kind of machine? (server, desktop, laptop, gaming, other)
  };
}
```

**For macOS hosts** — add this to your `home.nix`:

```nix
{
  services.nixfleet-agent = {
    enable = true;
    url = "wss://fleet.example.com";
    tokenFile = "/Users/yourname/.config/nixfleet/token";  # Store your token here
    isolatedRepoMode = true;
    location = "home";
    deviceType = "laptop";
  };
}
```

**Pro tip:** The `isolatedRepoMode` option is a game-changer! It means the agent keeps its own separate clone of your config repo. No more conflicts with the repo you're actively editing. 🎉

### Step 3: Enable Version Tracking (Recommended)

Want the dashboard to show you which hosts are behind? Set up a simple GitHub Action that publishes your current commit to GitHub Pages.

Create `.github/workflows/version-pages.yml` in your config repo:

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

Then enable GitHub Pages in your repo settings (Settings → Pages → Source: GitHub Actions).

Now every time you push to main, your dashboard will know the latest commit and can tell you which hosts need updating! 📡

### Step 4: Deploy the Dashboard

The dashboard runs as a Docker container. Here's the quick setup:

```bash
# Clone the repo
git clone https://github.com/markus-barta/nixfleet.git
cd nixfleet

# Generate your credentials
# 1. Create a password hash
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-secure-password', bcrypt.gensalt()).decode())"

# 2. Generate random secrets
openssl rand -hex 32  # For NIXFLEET_SESSION_SECRET
openssl rand -hex 32  # For NIXFLEET_AGENT_TOKEN

# Create your .env file with the generated values
cat > .env << EOF
NIXFLEET_PASSWORD_HASH=\$2b\$12\$...paste-your-hash-here...
NIXFLEET_SESSION_SECRET=your-generated-hex-secret
NIXFLEET_AGENT_TOKEN=your-generated-agent-token
EOF

# Fire it up!
docker compose up -d
```

That's it! Open your browser, log in, and watch your hosts appear as they connect. 🎊

## Configuration Reference

### Agent Options

These options are available for both NixOS and Home Manager modules:

| Option             | Type   | Required | Description                                       |
| ------------------ | ------ | -------- | ------------------------------------------------- |
| `enable`           | bool   | -        | Enable the agent                                  |
| `url`              | string | Yes      | Dashboard WebSocket URL (use `wss://`)            |
| `tokenFile`        | path   | Yes      | Path to your API token file                       |
| `isolatedRepoMode` | bool   | No       | Agent manages its own repo clone (default: true)  |
| `repoUrl`          | string | No       | Git URL for isolated mode (usually auto-detected) |
| `interval`         | int    | No       | Heartbeat interval in seconds (default: 30)       |
| `location`         | enum   | No       | Location category (default: "other")              |
| `deviceType`       | enum   | No       | Device type (default: "server")                   |
| `themeColor`       | string | No       | Hex color for dashboard row (default: "#769ff0")  |

### Dashboard Commands

These are the actions you can trigger from the UI:

| Command       | What It Does                                           |
| ------------- | ------------------------------------------------------ |
| `pull`        | Updates the repo (fetch + reset in isolated mode)      |
| `switch`      | Runs `nixos-rebuild switch` or `home-manager switch`   |
| `pull-switch` | Does both in sequence — the "update everything" button |
| `test`        | Runs your test scripts from `hosts/<host>/tests/T*.sh` |
| `stop`        | Cancels a currently running command                    |

### Environment Variables

Configure these when running the dashboard container:

| Variable                  | Required | What It's For                                  |
| ------------------------- | -------- | ---------------------------------------------- |
| `NIXFLEET_PASSWORD_HASH`  | Yes      | bcrypt hash of your admin password             |
| `NIXFLEET_SESSION_SECRET` | Yes      | Secret for signing session cookies             |
| `NIXFLEET_AGENT_TOKEN`    | Yes      | Shared token that agents use to auth           |
| `NIXFLEET_TOTP_SECRET`    | No       | Base32 secret if you want 2FA                  |
| `NIXFLEET_LOG_LEVEL`      | No       | How verbose? (debug, info, warn, error)        |
| `NIXFLEET_VERSION_URL`    | No       | URL to your version.json for Git status        |
| `NIXFLEET_DATA_DIR`       | No       | Where to store the database (default: `/data`) |

## Day-to-Day Operations

### Updating Your Fleet

The typical workflow looks like this:

1. **Make changes** to your Nix config and push to GitHub
2. **Open the dashboard** — you'll see hosts showing "behind" status (yellow G indicator)
3. **Click "Pull"** on a host to fetch the latest changes
4. **Click "Switch"** to apply the new configuration
5. Watch the status turn green! ✅

For brave souls, there's also **"Bulk Actions"** to update all hosts at once. Use with confidence (after testing on one host first 😉).

### When Things Go Wrong

If a host shows a red "A" indicator, it means the agent itself needs updating. This happens when you've updated NixFleet but the host's `flake.lock` still points to an old version.

To fix it:

```bash
# SSH to the host and update the lock file
cd /path/to/your/nixcfg
nix flake update nixfleet
git add flake.lock && git commit -m "Update nixfleet"

# Then rebuild
sudo nixos-rebuild switch --flake .#hostname  # NixOS
# or
home-manager switch --flake .#hostname        # macOS
```

### Restarting Agents

Usually you won't need to do this, but if an agent gets stuck:

```bash
# NixOS
sudo systemctl restart nixfleet-agent

# macOS
launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent
```

## Development

Want to hack on NixFleet? Welcome aboard! 🛠️

```bash
# Enter the dev environment
cd v2
nix develop

# Run the dashboard locally
go run ./cmd/nixfleet-dashboard

# Run the agent locally
go run ./cmd/nixfleet-agent

# Run the test suite
go test ./tests/...
```

The codebase uses Go with `templ` for type-safe HTML templates and `htmx` for interactive updates. It's a joy to work with!

## License

GNU AGPL v3.0 — See [LICENSE](LICENSE) for details.

---

<p align="center">
  Made with ❤️ for the Nix community
</p>
