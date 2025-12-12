# Pickup TODOs - 2025-12-13

## Priority Tasks

### 1. Update Agent from UI
- Backlog item already exists: `2025-12-12-update-agent-from-ui.md`
- Allow updating nixfleet agent version from dashboard

### 2. UI Polish Issues

#### Font Size Inconsistency
- HOST column vs Tests column (6/8) have inconsistent font sizes
- Standardize across table

#### Status Column Height
- Scrollable area still cut off at ~55% height
- Needs proper max-height calculation

#### Row Background for Online Hosts
- Add subtle greenish tint (10%) for online hosts
- Makes it easier to spot online vs offline at a glance

#### Config/Fleet SOT Icons
- Add icons for Config and Fleet "Source of Truth" indicators
- SOT checkmark should NOT be green
- Use white/gray instead for neutral appearance

#### Actions Buttons Padding
- Large unused right padding on action buttons
- Tighten up the layout

### 3. Fix All Host Tests
- csb0: 0/8 tests failing
- Other hosts also have test failures
- Review and fix test scripts

### 4. Replace NixFleet Logo
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

