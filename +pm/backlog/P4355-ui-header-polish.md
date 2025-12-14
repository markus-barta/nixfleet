# P4355 - UI: Header Polish

**Priority**: High  
**Status**: Pending  
**Effort**: Medium  
**References**: `+pm/legacy/v1.0/dashboard.html`, `+pm/legacy/v1.0/screenshot.png`

## Problem

v2 header is bare-bones:

- Missing logo
- Missing subtitle
- Missing "Source of Truth" panel (nixcfg hash)
- Connection indicator exists but needs polish

## Solution

### 1. Logo + Branding

```html
<div class="brand">
  <h1>
    <img src="/static/nixfleet_fade_1k.png" alt="NixFleet" class="brand-logo" />
    NixFleet
  </h1>
  <p class="subtitle">Simple, unified fleet management for NixOS and macOS</p>
</div>
```

### 2. Source of Truth Panel

Show the target nixcfg commit hash with:

- GitHub link to commit
- Refresh button to fetch latest
- Commit message preview

```html
<div class="source-of-truth">
  <span class="sot-header">Target</span>
  <div class="sot-target">
    <button class="sot-refresh" title="Refresh config hash">
      <svg><use href="#icon-refresh" /></svg>
    </button>
    <span class="sot-label">Config</span>
    <a href="https://github.com/.../commit/{hash}" class="sot-hash">
      <svg><use href="#icon-github" /></svg>
      {hash}
    </a>
  </div>
</div>
```

### 3. Host Count Badge

Show online/total in header:

```html
<th>Host <span class="online">3</span>/<span class="total">5</span></th>
```

### 4. Actions Dropdown

Add bulk actions menu:

- Add Host
- Pull All
- Switch All
- Test All
- Refresh Page

### Requirements

- [ ] Add logo image to static files
- [ ] Implement brand section with logo + subtitle
- [ ] Create Source of Truth component
- [ ] Add `/api/nixcfg/refresh` endpoint
- [ ] Add host count to header
- [ ] Create Actions dropdown with bulk commands
- [ ] Polish connection indicator styling

## Related

- P4350 (Icons) - Needs icon system first
- P4360 (Footer) - Consistent branding
