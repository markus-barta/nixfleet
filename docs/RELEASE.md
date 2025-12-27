# NixFleet Release Guide

## The Simple Version

**When you push to nixfleet, everything updates automatically.**

```
You push code → Dashboard updates → Hosts update on next "Pull + Switch"
```

That's it. Keep reading only if something breaks.

---

## What Happens Automatically

```
┌─────────────────────────────────────────────────────────────────┐
│  1. You push to nixfleet                                        │
│     git push                                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. GitHub builds Docker image (2 min)                          │
│     → ghcr.io/markus-barta/nixfleet:master                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. GitHub triggers nixcfg update (automatic)                   │
│     → nix flake update nixfleet                                 │
│     → commits new flake.lock                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  4. Dashboard shows "Git outdated" on hosts                     │
│     → You click "Do All" or "Pull + Switch"                     │
│     → Hosts rebuild with new agent                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Your Daily Workflow

### Making Changes

```bash
cd ~/Code/nixfleet

# Make your changes
# ...

# Commit and push
git add -A
git commit -m "feat: your change"
git push

# Deploy dashboard (waits for Docker build)
just deploy
```

### Updating Hosts

In the NixFleet dashboard:

1. Look for hosts with **yellow Git compartment**
2. Click **"Do All"** (or select hosts → Pull → Switch)
3. Wait for completion
4. Verify new agent version shows up

---

## Version Bumping

When you want a "real" version number:

```bash
cd ~/Code/nixfleet

# Bump version
echo "3.0.2" > VERSION
git add VERSION
git commit -m "chore: bump version to 3.0.2"
git push

# Deploy
just deploy
```

---

## If Automation Fails

### Manual flake.lock update

If nixcfg doesn't auto-update, do it manually:

```bash
cd ~/Code/nixcfg
nix flake update nixfleet
git add flake.lock
git commit -m "bump nixfleet"
git push
```

Then do "Pull + Switch" on hosts.

### Manual host update

If the UI isn't working, SSH to the host:

```bash
# NixOS
cd ~/Code/nixcfg
git pull
sudo nixos-rebuild switch --flake .#$(hostname)

# macOS
cd ~/Code/nixcfg
git pull
home-manager switch --flake .#$(hostname -s)
```

---

## One-Time Setup

### 1. Create GitHub Token

1. Go to https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Name: `nixcfg-dispatch`
4. Scopes: `repo` (full control)
5. Copy the token

### 2. Add Token to nixfleet Secrets

1. Go to https://github.com/markus-barta/nixfleet/settings/secrets/actions
2. Click "New repository secret"
3. Name: `NIXCFG_DISPATCH_TOKEN`
4. Value: paste your token

### 3. Add Workflow to nixcfg

Copy `docs/nixcfg-workflow-template.yml` to your nixcfg repo:

```bash
cp docs/nixcfg-workflow-template.yml ~/Code/nixcfg/.github/workflows/update-nixfleet.yml
cd ~/Code/nixcfg
git add .github/workflows/update-nixfleet.yml
git commit -m "feat: auto-update nixfleet on new releases"
git push
```

---

## Troubleshooting

| Problem                    | Solution                                             |
| -------------------------- | ---------------------------------------------------- |
| Agent version not updating | Run `nix flake update nixfleet` in nixcfg manually   |
| Docker build failing       | Check GitHub Actions tab in nixfleet repo            |
| Hosts not pulling          | Check Git compartment - should be yellow if outdated |
| Switch failing             | Check agent logs: `journalctl -u nixfleet-agent -f`  |
