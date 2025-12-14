# P4350 - UI: SVG Icon System

**Priority**: High  
**Status**: Done  
**Effort**: Medium  
**References**: `+pm/legacy/v1.0/dashboard.html`

## Problem

v2 currently uses:

- Text buttons ("Pull", "Switch", "Test") without icons
- Unicode characters that render inconsistently (e.g., "Te t" instead of "Test")
- No visual differentiation for OS types, locations, device types

## Solution

Port the complete SVG icon system from v1.

### Requirements

- [x] Create SVG defs block in base template
- [x] Port all 25+ icons from v1 (nixos, apple, cloud, home, office, server, desktop, laptop, game, download, refresh, flask, stop, plus, trash, more, check, chevron, file, cpu, ram, github, heart, license)
- [x] Update all buttons to use icons via `commandIcon` component
- [ ] Add icons to OS, Location, Device columns (deferred to P4370)
- [x] Add icons to metrics (CPU, RAM)
- [x] Ensure icons inherit color from parent (currentColor)
- [x] No emojis or Unicode symbols

## Implementation Notes

- Added `commandIcon` templ component that renders appropriate SVG based on command type
- CommandButton now accepts optional label (empty = icon-only for table rows)
- CSS classes: `.icon` (14px), `.btn .icon` (12px), `.metric-icon` (10px)

## Related

- P4300 (Dashboard Live Logs) - Buttons need icons
- P4360 (Footer) - Footer links need icons
- P4370 (Table Columns) - Will add OS/Location/Device icons
