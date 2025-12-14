# Version & Generation Tracking

**Created**: 2025-12-14
**Priority**: P5000 (Medium)
**Status**: Backlog
**Depends on**: P4000-P4400 (Core rewrite)

---

## Overview

Complete version and generation tracking in the new Go dashboard.

---

## Features

### 1. Agent Version

- Injected at build time via ldflags
- Displayed in dashboard: `v2.0.0`
- Tooltip: "NixFleet Agent v2.0.0"

### 2. NixOS Generation

Agent sends in heartbeat:

```go
type Heartbeat struct {
    Hostname   string `json:"hostname"`
    Generation int    `json:"generation"`      // e.g., 42
    GenTime    string `json:"generation_time"` // ISO timestamp
    // ...
}
```

Display: `Gen 42 (2h ago)`

### 3. Nixpkgs Version

Agent detects via `nixos-version --json`:

```go
type Heartbeat struct {
    NixpkgsVersion string `json:"nixpkgs_version"` // commit hash
    OSVersion      string `json:"os_version"`      // e.g., "24.11.20241210"
    OSName         string `json:"os_name"`         // e.g., "NixOS"
    // ...
}
```

Display: `24.11 • abc1234`

### 4. Outdated Indicator

Compare host nixpkgs vs dashboard target (from nixcfg GitHub Pages):

```text
v2.0.0 • 24.11 • 9b18fd8 ↓
                        └── Outdated indicator
```

Hover: "Config: 9b18fd8 | Latest: abc1234 (3 commits behind)"

---

## Acceptance Criteria

- [ ] Agent version in heartbeat
- [ ] Generation number in heartbeat
- [ ] Generation age displayed
- [ ] Nixpkgs version detected and sent
- [ ] Outdated indicator when behind target
- [ ] Works for NixOS and macOS

---

## Related

- Part of dashboard features
- Carries forward from prototype work
