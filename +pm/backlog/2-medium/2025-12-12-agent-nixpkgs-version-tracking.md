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
agentScript = pkgs.substituteAll {
  src = ../agent/nixfleet-agent.sh;
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

## Open Questions

Please clarify:

1. **Reference for "latest"**: Should "latest" be:
   - (a) The nixpkgs version in YOUR nixcfg flake.lock (so all hosts should match)?
   - (b) The actual latest nixos-unstable or stable channel?
   - (c) Something else?

2. **Display preference**: 
   - (a) New "NixOS" column next to Version?
   - (b) Combined in Version column with hover details?
   - (c) Separate section/row for system info?

3. **macOS handling**: For macOS hosts (nix-darwin), should we track:
   - (a) Just the nixpkgs version from the flake?
   - (b) Also the macOS version (Sonoma 14.x, etc.)?

## Implementation Tasks

Once questions are resolved:

- [ ] Modify NixOS module to inject flake version into agent script
- [ ] Modify Home Manager module similarly
- [ ] Add nixpkgs version detection to agent registration
- [ ] Add database columns: `agent_version`, `nixpkgs_version`, `nixpkgs_channel`
- [ ] Update dashboard to fetch/display latest nixpkgs version
- [ ] Update UI to show agent version left of git hash
- [ ] Add "outdated nixpkgs" indicator similar to "outdated config" indicator

## Related

- Agent version display was added in v0.2.1-12 (hardcoded)
- Config outdated tracking already exists (compares git hash to latest)

## Notes

This feature would enable:
- Quick visibility of which hosts need `nix flake update`
- Tracking infrastructure consistency (all hosts on same nixpkgs?)
- Planning upgrade windows for major nixpkgs updates

