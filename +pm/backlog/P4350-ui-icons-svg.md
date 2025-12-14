# P4350 - UI: SVG Icon System

**Priority**: High  
**Status**: Pending  
**Effort**: Medium  
**References**: `+pm/legacy/v1.0/dashboard.html`

## Problem

v2 currently uses:

- Text buttons ("Pull", "Switch", "Test") without icons
- Unicode characters that render inconsistently (e.g., "Te t" instead of "Test")
- No visual differentiation for OS types, locations, device types

## Solution

Port the complete SVG icon system from v1:

### Icon Set Required

```html
<svg class="svg-defs" aria-hidden="true">
  <!-- OS Types -->
  <symbol id="icon-nixos">...</symbol>
  <symbol id="icon-apple">...</symbol>

  <!-- Locations -->
  <symbol id="icon-cloud">...</symbol>
  <symbol id="icon-home">...</symbol>
  <symbol id="icon-office">...</symbol>

  <!-- Device Types -->
  <symbol id="icon-server">...</symbol>
  <symbol id="icon-desktop">...</symbol>
  <symbol id="icon-laptop">...</symbol>
  <symbol id="icon-game">...</symbol>
  <symbol id="icon-mobile">...</symbol>

  <!-- Actions -->
  <symbol id="icon-download">...</symbol>
  <!-- Pull -->
  <symbol id="icon-refresh">...</symbol>
  <!-- Switch -->
  <symbol id="icon-flask">...</symbol>
  <!-- Test -->
  <symbol id="icon-stop">...</symbol>
  <!-- Stop -->
  <symbol id="icon-plus">...</symbol>
  <!-- Add -->
  <symbol id="icon-trash">...</symbol>
  <!-- Remove -->
  <symbol id="icon-more">...</symbol>
  <!-- Ellipsis menu -->
  <symbol id="icon-check">...</symbol>
  <!-- Success -->
  <symbol id="icon-chevron">...</symbol>
  <!-- Expand/Collapse -->
  <symbol id="icon-file">...</symbol>
  <!-- Config/Logs -->

  <!-- Metrics -->
  <symbol id="icon-cpu">...</symbol>
  <symbol id="icon-ram">...</symbol>

  <!-- Branding -->
  <symbol id="icon-github">...</symbol>
  <symbol id="icon-heart">...</symbol>
  <symbol id="icon-license">...</symbol>
  <symbol id="icon-rocket">...</symbol>
</svg>
```

### Usage Pattern

```html
<button class="btn btn-pull">
  <svg><use href="#icon-download" /></svg>
  Pull
</button>
```

### Requirements

- [ ] Create SVG defs block in base template
- [ ] Port all 25+ icons from v1
- [ ] Update all buttons to use icons
- [ ] Add icons to OS, Location, Device columns
- [ ] Add icons to metrics (CPU, RAM)
- [ ] Ensure icons inherit color from parent (currentColor)
- [ ] No emojis or Unicode symbols

## Related

- P4300 (Dashboard Live Logs) - Buttons need icons
- P4360 (Footer) - Footer links need icons
