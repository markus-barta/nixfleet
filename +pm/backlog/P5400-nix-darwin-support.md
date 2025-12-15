# P5400 - nix-darwin Support

**Created**: 2025-12-15  
**Priority**: P5400 (Medium)  
**Status**: Backlog  
**Depends on**: P5000 (Update Status)

---

## User Story

**As a** fleet administrator with nix-darwin Macs  
**I want** NixFleet to support nix-darwin alongside Home Manager  
**So that** my system-level macOS configs are also tracked

---

## Overview

macOS can be managed two ways:

| Tool             | Scope                     | Switch Command          |
| ---------------- | ------------------------- | ----------------------- |
| **Home Manager** | User environment only     | `home-manager switch`   |
| **nix-darwin**   | System-level (like NixOS) | `darwin-rebuild switch` |

Currently NixFleet assumes Home Manager for macOS. This adds nix-darwin support.

---

## Detection

Agent detects which tool is in use:

```bash
# Check if nix-darwin is installed
if command -v darwin-rebuild &>/dev/null; then
    echo "nix-darwin"
elif [ -d ~/.config/home-manager ] || [ -f ~/.config/nixpkgs/home.nix ]; then
    echo "home-manager"
else
    echo "unknown"
fi
```

Or configured explicitly:

```nix
services.nixfleet-agent = {
  macosManager = "nix-darwin";  # or "home-manager" (default)
};
```

---

## System Check Differences

| Tool         | Current System                             | Target Build                                   |
| ------------ | ------------------------------------------ | ---------------------------------------------- |
| Home Manager | `~/.local/state/nix/profiles/home-manager` | `.#homeConfigurations.$HOST.activationPackage` |
| nix-darwin   | `/run/current-system`                      | `.#darwinConfigurations.$HOST.system`          |

---

## Switch Command Differences

| Tool         | Command                                    |
| ------------ | ------------------------------------------ |
| Home Manager | `home-manager switch --flake .#hostname`   |
| nix-darwin   | `darwin-rebuild switch --flake .#hostname` |

---

## UI Changes

System compartment icon for nix-darwin Macs:

| Manager      | Icon                           |
| ------------ | ------------------------------ |
| Home Manager | 🏠 House                       |
| nix-darwin   | 🍎 Apple (or NixOS snowflake?) |

---

## Acceptance Criteria

- [ ] Agent detects nix-darwin vs Home Manager
- [ ] System check uses correct paths for nix-darwin
- [ ] Switch command uses `darwin-rebuild` for nix-darwin
- [ ] Optional: explicit config to override detection
- [ ] UI shows appropriate icon

---

## Related

- **P5000**: Update status (System check needs correct paths)
- Current: macOS uses Home Manager only
