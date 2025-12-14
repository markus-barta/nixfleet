# P4349 - Fix nixpkgs Version on macOS

**Priority**: Low  
**Status**: Pending  
**Effort**: Small

## Problem

On macOS hosts, `nixpkgs_version` shows empty in dashboard. The agent should detect and report the nixpkgs version.

## Investigation

The agent runs on macOS via Home Manager. Need to check:

1. Where does the agent get nixpkgs version?
2. Is the detection code platform-specific?
3. Does Home Manager expose this differently than NixOS?

## Potential Solutions

### Option A: Read from nix-info

```bash
nix-info -m 2>/dev/null | grep "nixpkgs" | cut -d':' -f2 | tr -d ' '
```

### Option B: Read from NIX_PATH

```bash
nix eval --impure --expr 'builtins.substring 0 11 (builtins.readFile ((builtins.getFlake (builtins.toString ~/.config/home-manager)).inputs.nixpkgs.rev))'
```

### Option C: Store in Agent Config

Pass nixpkgs version as environment variable at build time:

```nix
environment.NIXFLEET_NIXPKGS_VERSION = inputs.nixpkgs.rev;
```

### Requirements

- [ ] Investigate current detection code
- [ ] Identify why it fails on macOS
- [ ] Implement cross-platform detection
- [ ] Test on macOS (mba-mbp-work)
- [ ] Verify in dashboard

## Related

- P4370 (Table Columns) - OS cell shows nixpkgs on hover
