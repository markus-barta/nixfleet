# P5000 - Host Update Status (Three-Compartment Indicator)

**Created**: 2025-12-14  
**Updated**: 2025-12-15  
**Priority**: P5000 (Medium)  
**Status**: Backlog  
**Depends on**: P4370 (Table Columns)  
**Unblocks**: P4370 Config column

---

## User Story

**As a** fleet administrator  
**I want** a three-compartment status indicator in the hosts table  
**So that** I can quickly see which hosts need updates and why

---

## Overview

Display three compartments with icons for each host:

1. **Git** — Local repo matches remote branch
2. **Lock** — flake.lock is current with allowed inputs
3. **System** — Running system matches what current flake would build

```text
┌───────────────────────────────────────────────────────────────────────────┐
│ Host     │ Update Status       │ ...                                      │
├──────────┼─────────────────────┼──────────────────────────────────────────┤
│ hsb1     │ [branch][lock][❄]   │ All calm = fully up to date (NixOS)      │
│ hsb0     │ [branch][lock][❄]   │ Lock glowing = can update deps           │
│ gpc0     │ [branch][lock][❄]   │ Git glowing = behind remote              │
│ imac0    │ [branch][lock][🏠]  │ System glowing = needs switch (macOS)    │
└───────────────────────────────────────────────────────────────────────────┘
```

### Visual Design

**Compartment Box**: Three adjacent cells forming one unit

| Position    | Icon       | Source                             |
| ----------- | ---------- | ---------------------------------- |
| 1st         | Git branch | Heroicons `code-branch` or similar |
| 2nd         | Padlock    | Heroicons `lock-closed`            |
| 3rd (NixOS) | Snowflake  | NixOS logo SVG                     |
| 3rd (macOS) | House      | Heroicons `home`                   |

| State       | Background            | Icon       | Effect                                 |
| ----------- | --------------------- | ---------- | -------------------------------------- |
| ✓ Current   | Gray fill (`#374151`) | Dark icon  | Static, calm                           |
| ⚡ Outdated | Gray fill             | Light icon | **Soft glow pulse** (30% → 100% → 30%) |
| ✗ Error     | Red tint              | Red icon   | Static warning                         |
| ? Unknown   | Transparent           | Dim icon   | Static, waiting                        |

**Glow Animation**: Subtle white pulse, not harsh blinking:

```css
@keyframes needs-attention {
  0%,
  100% {
    opacity: 0.3;
  }
  50% {
    opacity: 1;
  }
}
.needs-update {
  animation: needs-attention 2s ease-in-out infinite;
}
```

**Why this works:**

- Distinct from online status ripple (different shape, different animation)
- At-a-glance: calm = good, glowing = action needed
- Icons are self-explanatory with tooltips for detail

---

## Acceptance Criteria

### UI

- [ ] New "Update" column with three-compartment indicator
- [ ] Each compartment has an **SVG icon** (no emojis per NFR-2):
  - **Git**: Branch icon (git-branch)
  - **Lock**: Padlock icon (lock-closed)
  - **System**: OS-specific icon:
    - NixOS → Snowflake (NixOS logo)
    - macOS → House (Home Manager concept)
- [ ] Each compartment has tooltip explaining its meaning:
  - Git: "Up to date with remote" / "Behind remote by N commits"
  - Lock: "Dependencies current" / "Updates available"
  - System: "System current" / "Needs rebuild"
- [ ] States:
  - Current: Gray background (`#374151`), dark icon (`#1f2937`), static
  - Outdated: Gray background, icon glows white (pulse 30%→100%→30%, 2s cycle)
  - Error: Red tint (`#7f1d1d`), static
  - Unknown: Transparent background, dim icon (`opacity: 0.4`)
- [ ] Optional: Click compartment to trigger action (Pull, Update, Switch)

### Agent Detection

- [ ] Agent detects OS type (NixOS vs macOS/Home Manager)
- [ ] Agent runs safe, read-only commands only
- [ ] Agent reports all three statuses in heartbeat
- [ ] Results cached with reasonable TTL (e.g., 5 minutes)

### Backend

- [ ] Protocol includes `UpdateStatus` in heartbeat
- [ ] Dashboard stores and displays status
- [ ] Status ages gracefully (show "checking..." during refresh)

---

## Technical Design

### Protocol Addition

```go
// In protocol/messages.go
type UpdateStatus struct {
    Git    StatusCheck `json:"git"`
    Lock   StatusCheck `json:"lock"`
    System StatusCheck `json:"system"`
}

type StatusCheck struct {
    Status  string `json:"status"`  // "ok", "outdated", "error", "unknown"
    Message string `json:"message"` // Human-readable detail
    CheckedAt string `json:"checked_at"` // ISO timestamp
}

type HeartbeatPayload struct {
    // ... existing fields ...
    UpdateStatus *UpdateStatus `json:"update_status,omitempty"`
}
```

### Agent Commands

All commands are **read-only** or **dry-run**. Agent needs the flake path configured.

#### 1. Git Status Check ✅ (Already Solved)

**No agent-side git commands needed!**

The dashboard fetches the latest commit from GitHub Pages:

```text
https://<user>.github.io/<repo>/version.json
→ { "gitCommit": "abc123...", "message": "..." }
```

The agent already reports its deployed `generation` (git hash) in heartbeat.

**Comparison happens dashboard-side:**

```go
latestHash := fetchFromGitHubPages()  // cached, 5s TTL
agentHash := host.Generation          // from heartbeat

if latestHash == agentHash {
    status = "ok"
} else {
    status = "outdated"
    message = fmt.Sprintf("Behind by commits (deployed: %s, latest: %s)",
        agentHash[:7], latestHash[:7])
}
```

**Setup required**: nixcfg repo must have GitHub Pages workflow publishing `version.json`. See README.md "Enable Version Tracking" section.

#### 2. Flake Lock Status Check (Display Only)

**P5000 scope**: Show informational status only. Full automation in P5300.

**What we show:**

- Days since last flake.lock update
- Whether an update PR is pending (if GitHub integration enabled)

**How to get "days since update":**

```bash
# Get last commit that modified flake.lock
git log -1 --format="%ci" -- flake.lock
# → 2025-12-10 14:30:00 +0100

# Agent calculates days since this date
```

**PR detection** (optional, requires GitHub token):

```go
// Dashboard checks GitHub API
GET /repos/markus-barta/nixcfg/pulls?state=open&labels=automated
// If any results → "Update PR pending"
```

**On-click behavior:**

- Opens tooltip with details: "Lock updated 5 days ago"
- If PR pending: "Update available — see P5300 for merge actions"

**Note**: Expensive `nix flake lock --dry-run` is NOT run automatically. Reserved for P5300's on-demand check.

#### 3. System Status Check

**NixOS:**

```bash
# Dry-activate shows what would change
sudo nixos-rebuild dry-activate --flake $FLAKE_PATH#$HOSTNAME 2>&1

# Check output for changes
# "would restart" / "would start" / "would stop" = changes pending
# Empty or "No systemd units to start" = up to date
```

**Alternative (faster, no sudo):**

```bash
# Compare current system derivation with what would be built
CURRENT=$(readlink /run/current-system)
TARGET=$(nix build --dry-run --json "$FLAKE_PATH#nixosConfigurations.$HOSTNAME.config.system.build.toplevel" 2>/dev/null | jq -r '.[0].outputs.out')

[ "$CURRENT" = "$TARGET" ] && echo "ok" || echo "outdated"
```

**macOS (Home Manager):**

```bash
# Dry-run shows what would change
home-manager build --flake $FLAKE_PATH#$HOSTNAME --dry-run 2>&1

# Or compare generations
CURRENT=$(home-manager generations | head -1 | awk '{print $5}')
TARGET=$(nix build --dry-run --json "$FLAKE_PATH#homeConfigurations.$HOSTNAME.activationPackage" 2>/dev/null | jq -r '.[0].outputs.out')

[ -z "$TARGET" ] && echo "unknown" || ([ "$CURRENT" = "$TARGET" ] && echo "ok" || echo "outdated")
```

### Agent Configuration

```nix
# In nixfleet module
services.nixfleet-agent = {
  enable = true;
  flakePath = "/home/mba/Code/nixcfg";  # Required for status checks
  # ... existing options ...
};
```

### Caching Strategy

- Git check: Every heartbeat (fast, ~100ms) — dashboard-side comparison
- Lock check: On dashboard load (days-since from git log)
- System check: Every 5 minutes (configurable in settings)
- Force refresh on command completion (after pull/switch)

### Check Interval Setting

System check interval should be configurable (needs Settings page — see P6400).

Default: 5 minutes. Range: 1-30 minutes.

### Display Behavior

| State           | Primary Text       | Hover Detail                            |
| --------------- | ------------------ | --------------------------------------- |
| Git current     | (calm)             | "Up to date with remote"                |
| Git outdated    | (glow)             | "Behind remote by 3 commits"            |
| Lock fresh      | "2d" badge         | "Lock updated 2 days ago"               |
| Lock stale      | "12d" badge + glow | "Lock updated 12 days ago • PR pending" |
| System current  | (calm)             | "System current • Checked 2m ago"       |
| System outdated | (glow)             | "Needs switch • Checked 1m ago"         |

### Bulk Actions (from v1)

Header dropdown with:

- **Update All** — Pull + Switch on all online hosts (NEW, combines both)
- **Pull All** — Pull on all online hosts
- **Switch All** — Switch on all online hosts
- **Test All** — Run tests on all online hosts

Per-host actions remain in the row's action dropdown.

---

## Visual States

| State   | Status     | Appearance                 | Meaning         |
| ------- | ---------- | -------------------------- | --------------- |
| Calm    | `ok`       | Gray bg, dark icon, static | Up to date      |
| Glowing | `outdated` | Gray bg, icon pulses white | Needs attention |
| Warning | `error`    | Red tint, static           | Check failed    |
| Waiting | `unknown`  | Transparent, dim icon      | Not yet checked |

### CSS Implementation

```css
.update-compartment {
  display: inline-flex;
  background: #374151;
  border-radius: 4px;
  padding: 4px 6px;
  margin-right: 2px;
}

.update-compartment svg {
  width: 14px;
  height: 14px;
  fill: #1f2937;
}

.update-compartment.needs-update svg {
  fill: #f9fafb;
  animation: pulse-glow 2s ease-in-out infinite;
}

.update-compartment.error {
  background: #7f1d1d;
}

.update-compartment.unknown {
  background: transparent;
  opacity: 0.4;
}

@keyframes pulse-glow {
  0%,
  100% {
    opacity: 0.3;
  }
  50% {
    opacity: 1;
  }
}
```

### HTML Structure

```html
<!-- NixOS host -->
<div class="update-status">
  <span class="update-compartment" title="Git: Up to date">
    <svg class="icon-git-branch">...</svg>
  </span>
  <span
    class="update-compartment needs-update"
    title="Lock: 3 updates available"
  >
    <svg class="icon-lock">...</svg>
  </span>
  <span class="update-compartment" title="System: Current">
    <svg class="icon-nixos">...</svg>
  </span>
</div>

<!-- macOS host -->
<div class="update-status">
  <span class="update-compartment" title="Git: Up to date">
    <svg class="icon-git-branch">...</svg>
  </span>
  <span class="update-compartment" title="Lock: Current">
    <svg class="icon-lock">...</svg>
  </span>
  <span
    class="update-compartment needs-update"
    title="Home Manager: Needs switch"
  >
    <svg class="icon-home">...</svg>
  </span>
</div>
```

### Icon Sources

Use inline SVGs for colorability. Suggested sources:

- **Git branch**: [Heroicons](https://heroicons.com/) or [Lucide](https://lucide.dev/)
- **Lock**: Heroicons `lock-closed`
- **House**: Heroicons `home`
- **NixOS snowflake**: [NixOS Branding](https://nixos.org/branding/) — simplified for small sizes

---

## Implementation Order

1. ✅ **nixcfg**: GitHub Pages workflow already publishes `version.json`
2. **Dashboard**: Fetch GitHub Pages version + compare with agent's `generation`
3. **Dashboard**: Display three-compartment UI with CSS
4. ✅ **Git compartment**: Working (no agent changes needed!)
5. **Agent**: Add `flakePath` config option to nixfleet module
6. **Agent**: Report flake.lock last-modified date in heartbeat
7. **Dashboard**: Lock compartment shows "X days ago" badge
8. **Agent**: Implement system status check (compare derivations)
9. **Protocol**: Add `UpdateStatus` to heartbeat (for System status)

**Note**: Lock automation (merge PR, deploy) is in P5300, not here.

---

## Related

- **P4370**: Blocked "Config column" depends on this
- **P5300**: Automated flake updates (merge PR, deploy all) — extends Lock compartment
- **P5400**: nix-darwin support for macOS system-level configs
- **P6400**: Settings page for configurable intervals
- **docs/VERSIONS-EXPLAINED.md**: User-friendly explanation of the version concepts
- Supersedes original P5000 scope (version tracking subsumed into this)

---

## Notes

### Shared Flake Consideration

All hosts share the same `nixcfg` flake. Git and Lock status will be identical across hosts on the same machine. Only System status is truly per-host.

For remote hosts (csb0, csb1), each has its own repo clone, so Git/Lock can differ.

**Tooltip note**: When hosts share a flake, show "Shares config with gpc0, imac0" in hover.

### Error Handling

- If git fetch fails (network): Show unknown state with "Network error"
- If nix commands fail: Show error state with message
- If flakePath not configured: Show unknown state with "Not configured" — graceful, not startup error
- **Timeout**: If check takes >30s, kill it and show "Check timeout"

### Initial Load State

Before first check completes, show:

- Compartment in "unknown" state (transparent, dim icon)
- Subtle spinner or "..." indicator
- Tooltip: "Checking..."

### Refresh Behavior

- **Click compartment**: Triggers refresh of that specific check
- **After any command**: Auto-refresh all three checks (cascade)
- **Dashboard load**: Refresh all in background, show cached immediately

### Offline Hosts

For hosts that are offline:

- Show **last known status** from when they were online
- Tooltip includes: "Offline • Last seen 2h ago"
- Compartments slightly dimmed (like offline row overlay)
- Status is stale but still useful for planning

### macOS Support

**v1**: Home Manager only

**Future (P5400)**: nix-darwin support for system-level macOS configs

### flakePath Configuration

Agent needs to know the repo path for System check:

```nix
services.nixfleet-agent = {
  flakePath = "/home/mba/Code/nixcfg";  # Required for status checks
};
```

If not set, System compartment shows "Not configured" (unknown state, not error).

### Security

All commands are read-only:

- System check: `nix build --dry-run` — no root needed
- Lock check: `git log` — read-only
- Git check: Dashboard-side only, agent doesn't fetch

### Settings Page Needed

Check interval should be configurable. See **P6400** (to be created) for Settings page.

Settings to include:

- System check interval (default: 5 min)
- Lock "stale" threshold (default: 7 days for glow)
- Auto-refresh on dashboard load (default: on)
