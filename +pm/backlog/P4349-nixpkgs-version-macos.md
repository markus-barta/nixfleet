# P4349 - Fix nixpkgs Version on macOS

**Priority**: Low  
**Status**: Done  
**Effort**: Small

## Problem

On macOS hosts, `nixpkgs_version` shows empty in dashboard. The agent should detect and report the nixpkgs version.

## Solution

Implemented Option C: Pass nixpkgs version as environment variable from Nix config.

### Changes Made

1. **shared.nix**: Added `nixpkgsVersion` option
2. **config.go**: Added `NixpkgsVersion` field and `NIXFLEET_NIXPKGS_VERSION` env var
3. **heartbeat.go**: Use env var if set, otherwise detect from `/run/current-system`

### nixcfg Usage

In each host's home.nix that uses Home Manager:

```nix
services.nixfleet-agent = {
  enable = true;
  # ... other options ...
  nixpkgsVersion = inputs.nixpkgs.shortRev; # e.g., "abc1234"
};
```

Note: The nixcfg flake needs to be updated to:

1. Update flake.lock to get the new nixfleet version
2. Add `nixpkgsVersion` to macOS home configurations

### Requirements

- [x] Investigate current detection code
- [x] Identify why it fails on macOS (reads /run/current-system which doesn't exist)
- [x] Implement cross-platform detection (via NIXFLEET_NIXPKGS_VERSION env var)
- [ ] Update nixcfg to pass nixpkgsVersion (separate PR)
- [ ] Test on macOS (mba-mbp-work)
- [ ] Verify in dashboard

## Related

- P4370 (Table Columns) - OS cell shows nixpkgs on hover
