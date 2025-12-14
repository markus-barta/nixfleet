# Pickup TODOs - 2025-12-13

## Priority Tasks

### 1. Update Agent from UI ✅

- Completed! Moved to `+pm/done/2025-12-12-update-agent-from-ui.md`
- Update Agent button working in dashboard

### 2. UI Polish Issues ✅ (Partial)

#### Font Size Inconsistency ✅

- Fixed: Header stats now match th font size (0.65rem)

#### Status Column ✅

- Fixed: Increased collapsed height from 3em to 4em

#### Row Background for Online Hosts ✅

- Fixed: Added 5% green tint for online rows, 10% on hover

#### Config/Fleet SOT Icons ✅

- Fixed: Added 📄 Config and 🚀 Fleet icons in headers

#### Actions Buttons Padding

- Skipped: Needs visual inspection to confirm issue

### 3. Fix All Host Tests

- csb0: 0/8 tests failing
- Other hosts also have test failures
- Review and fix test scripts

### 4. Replace NixFleet Logo [DEFERRED - DO NOT DO THIS YET]

- Deferred until after new logo is defined
- Top-left logo needs replacement
- Get new logo asset

---

## Completed Today (2025-12-12)

### Status Column Papertrail ✅

- Dual in-memory history (truncated UI + full logs)
- Download Logs feature working
- Expand/collapse with tiny buttons
- SSE live updates
- Fixed old code overwriting papertrail

### UI Fixes ✅

- Font size 50% smaller in status column
- Test column now shows just x/y (not "x/y pass, y fail")
- Fixed test progress going to wrong column (Fleet → Tests)

### Critical Bug Fix ✅

- **sudo-rs wrapper path**: Agent couldn't run switch because PATH
  didn't include `/run/wrappers/bin` for setuid sudo wrapper
- Fixed in `modules/nixos.nix`
- csb0 switch now working from dashboard!

---

## Notes

- Other NixOS hosts need `nix flake update nixfleet` + rebuild to get the sudo fix
- Dashboard deployed at `28354b2`, agent module at `4cf1eac`
