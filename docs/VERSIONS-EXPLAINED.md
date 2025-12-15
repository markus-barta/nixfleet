# Understanding NixOS Versions & Updates

A simple guide to the different "versions" in a NixOS flake setup.

---

## The 4 Things That Have Versions

| #   | What                  | Example             | Think of it as...     |
| --- | --------------------- | ------------------- | --------------------- |
| 1   | **Your config (git)** | `abc1234`           | "My recipe"           |
| 2   | **flake.lock (deps)** | nixpkgs @ `def5678` | "Ingredient versions" |
| 3   | **NixOS version**     | `25.05`             | "Just a label"        |
| 4   | **Deployed system**   | `/nix/store/xxx...` | "The meal I cooked"   |

---

## In Plain English

```text
┌─────────────────────────────────────────────────────────────┐
│  YOUR CONFIG (git)         │  DEPENDENCIES (flake.lock)     │
│  ────────────────          │  ─────────────────────────     │
│  • configuration.nix       │  • nixpkgs pinned to abc123    │
│  • home.nix                │  • home-manager pinned to xyz  │
│  • modules/*.nix           │  • agenix pinned to 789def     │
│                            │                                │
│  Changed by: git pull      │  Changed by: nix flake update  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  nix build /    │
                    │  nixos-rebuild  │
                    └─────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  DEPLOYED SYSTEM                                            │
│  ───────────────                                            │
│  /run/current-system → /nix/store/xxx-nixos-system-...      │
│                                                             │
│  This is what's actually RUNNING on the machine.            │
│  Changed by: nixos-rebuild switch / home-manager switch     │
└─────────────────────────────────────────────────────────────┘
```

---

## The Update Workflow

```bash
# Step 1: Get latest config from GitHub
git pull                    # Updates YOUR CONFIG

# Step 2: Update dependency versions
just update                 # Updates FLAKE.LOCK
# (same as: nix flake update)

# Step 3: Build and apply
just switch                 # Builds and activates DEPLOYED SYSTEM
# (same as: sudo nixos-rebuild switch --flake .#hostname)
```

Or all at once:

```bash
just upgrade                # = update + build + switch
```

---

## Status Indicators (What Each Means)

| Indicator     | Status        | Problem                          | Fix           |
| ------------- | ------------- | -------------------------------- | ------------- |
| **Git** ⚡    | Behind remote | New commits on GitHub not pulled | `git pull`    |
| **Lock** ⚡   | Deps outdated | Newer nixpkgs/inputs available   | `just update` |
| **System** ⚡ | Not switched  | Config changed but not applied   | `just switch` |

When all three are calm (no glow) = fully up to date! ✓

---

## Common Scenarios

### "I made changes but forgot to switch"

```text
Git: ✓ (up to date)
Lock: ✓ (up to date)
System: ⚡ GLOWING (needs switch!)
```

→ Run `just switch`

### "Team pushed new config"

```text
Git: ⚡ GLOWING (behind remote!)
Lock: ✓ (up to date)
System: ✓ (matches local)
```

→ Run `git pull`, then `just switch`

### "Haven't updated in a while"

```text
Git: ⚡ (behind)
Lock: ⚡ (outdated)
System: ⚡ (stale)
```

→ Run `git pull && just upgrade`

---

## Quick Reference

| Command                            | What it updates              |
| ---------------------------------- | ---------------------------- |
| `git pull`                         | Your config files            |
| `just update` / `nix flake update` | flake.lock (dependency pins) |
| `just switch`                      | The running system           |
| `just upgrade`                     | All of the above             |

---

## See Also

- [P5000 - Host Update Status](../+pm/backlog/P5000-version-generation-tracking.md) - Dashboard feature spec
