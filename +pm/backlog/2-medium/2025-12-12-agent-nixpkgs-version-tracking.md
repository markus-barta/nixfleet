# Agent & NixOS Version Tracking

**Created**: 2025-12-12  
**Priority**: Medium  
**Status**: Backlog

## Overview

Enhance version tracking in the NixFleet dashboard to show:

1. Agent version (injected from flake at build time)
2. NixOS/nixpkgs version the host is running
3. Latest available nixpkgs version (for upgrade awareness)

## Requirements

### 1. Agent Version from Flake (Build-Time Injection)

**Current state**: Agent has hardcoded `AGENT_VERSION="1.0.0"` requiring manual updates.

**Goal**: Automatically inject the nixfleet flake version at build time.

**Implementation approach**:

```nix
# In modules/nixos.nix
let
  # Get version from flake (e.g., "v0.2.1-15" or commit hash)
  flakeVersion = inputs.nixfleet.shortRev or inputs.nixfleet.rev or "unknown";
in
agentScriptSrc = pkgs.replaceVars ../agent/nixfleet-agent.sh {
  agentVersion = flakeVersion;
};
```

Or use `pkgs.writeShellApplication` with version prepended.

**Display**: Show agent version LEFT of git hash in Version column:

```
v0.2.1 • 9b18fd8 ↓
```

### 2. NixOS/Nixpkgs Version Tracking

**Goal**: Track and display the nixpkgs version each host is running.

**Data to capture** (sent during agent registration):

- `nixpkgs_version`: The nixpkgs commit hash the system was built with
- `nixpkgs_channel`: Channel name if applicable (e.g., "nixos-24.11", "nixos-unstable")
- `nixos_version`: Full NixOS version string (e.g., "24.11.20241210.abc1234")

**How to detect on host**:

```bash
# Get nixpkgs version from system
nixos-version --json | jq -r '.nixpkgsRevision'

# Or from /etc/os-release
grep VERSION_ID /etc/os-release
```

### 3. Latest Nixpkgs Version Awareness

**Goal**: Show if host's nixpkgs is outdated compared to latest stable/unstable.

**Questions to resolve**:

1. **What "latest" should we compare against?**
   - [ ] Latest stable release (24.11)?
   - [ ] Latest commit on nixos-unstable?
   - [ ] The nixpkgs version in the main nixcfg flake.lock?
   - [ ] User-configurable per host?

2. **How to fetch "latest" version?**
   - Dashboard polls GitHub API for nixpkgs releases?
   - Dashboard reads version.json from nixcfg GitHub Pages?
   - Manual configuration in dashboard settings?

3. **Display format?**
   - New column "NixOS" showing version with outdated indicator?
   - Tooltip on existing version column?
   - Separate "System" section in host details?

**Proposed display** (in new column or tooltip):

```
24.11.1234 ↓  (hover: "Latest: 24.11.5678 - 15 days behind")
```

## Decisions (2025-12-12)

### 1. Reference for "latest"

**Decision**: Both (a) AND (b) with hover details

- Primary comparison: nixcfg flake.lock (your intended baseline)
- On hover: Show actual latest channel version for awareness

### 2. Display format

**Decision**: (c) Combined in Version column with detailed hovers

Display format in Version column:

```
v0.2.1 • 24.11 • 9b18fd8 ↓
```

Where:

- `v0.2.1` = Agent version (from nixfleet flake)
- `24.11` = NixOS/nixpkgs version
- `9b18fd8` = Config git hash
- `↓` = Outdated indicator (config or nixpkgs)

**Hover tooltips** (each element has its own tooltip):

- Agent version hover: "NixFleet Agent v0.2.1 (latest: v0.2.2)"
- NixOS version hover: "NixOS 24.11.20241210 | Your flake: 24.11.20241215 | Channel latest: 24.11.20241220"
- Git hash hover: "Config: 9b18fd8 | Latest: abc1234 (3 commits behind)"

### 3. macOS handling

**Decision**: (a) + (b) - Track both

- nixpkgs version from the flake
- macOS version (e.g., "Sonoma 14.7.1")

Display for macOS:

```
v0.2.1 • 14.7 • 9b18fd8 ↓
```

Hover on "14.7": "macOS Sonoma 14.7.1 | nixpkgs: 24.11.20241210"

## Implementation Tasks

### Phase 1: Agent Version from Flake ✅ COMPLETE (2025-12-12)

- [x] Modify NixOS module to inject flake version into agent script
- [x] Modify Home Manager module similarly
- [x] Remove hardcoded `AGENT_VERSION` from agent script (now uses `@agentVersion@` placeholder)
- [x] Update UI: Agent version now in combined version column

### Phase 2: OS Version Tracking ✅ COMPLETE (2025-12-12)

- [x] Add nixpkgs/NixOS version detection to agent (nixos-version --json)
- [x] Add macOS version detection for Darwin hosts (sw_vers)
- [x] Add database columns: `nixpkgs_version`, `os_version`, `os_name`
- [x] Send OS version info during agent registration
- [x] Update UI: Combined version display `v0.3.0 • 26.05 • abc1234`

### Phase 3: Latest Version Awareness (PARTIALLY COMPLETE)

- [x] Display nixpkgs version in UI with hover details
- [ ] Dashboard fetches nixpkgs version from nixcfg flake.lock
- [ ] Compare host nixpkgs vs target nixpkgs
- [ ] Add "outdated nixpkgs" indicator (similar to config outdated)
- [ ] Optionally fetch latest channel versions from GitHub API

### Phase 4: Polish

- [x] Basic tooltips for each version element
- [x] Test with NixOS hosts (csb0 verified working)
- [ ] Enhanced hover tooltips with comparison details
- [ ] Handle edge cases (missing data, offline hosts)
- [ ] Test with macOS hosts

## Related

- Agent version display was added in v0.2.1-12 (hardcoded)
- Config outdated tracking already exists (compares git hash to latest)

## Notes

This feature would enable:

- Quick visibility of which hosts need `nix flake update`
- Tracking infrastructure consistency (all hosts on same nixpkgs?)
- Planning upgrade windows for major nixpkgs updates
