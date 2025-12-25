# P7300: Compartment & Status Harmonization

**Priority:** High  
**Status:** Implemented  
**Epic:** UI/UX Polish  
**Created:** 2025-12-25  
**Updated:** 2025-12-25

## Problem Statement

Current dashboard has inconsistencies between compartments and status visualization:

1. **Header agent target** shows "Agent: master" instead of semantic version like "2.3.0"
2. **Compartments** only cover Git/Lock/System, missing Agent Version
3. **Status dots** had complex 3-dot layout that was inconsistent and confusing
4. **Compartment clicks trigger actions** - conflates status display with action triggering

## Design Decisions (Final)

### Core Principle: Separation of Concerns

**Compartments = INFO ONLY (safe checks allowed, modifications forbidden)**

- ✅ Show cached info (toast/modal)
- ✅ Trigger lightweight agent checks (git status, read generation, etc.)
- ❌ Modify host (git pull, nixos-rebuild, etc.)
- ❌ Heavy operations (nix build --dry-run)
- Modifying actions only available in ellipsis menu

### Single Dot Status Indicator

**Simplified from 3 dots to 1 dot per compartment with 5 color states:**

| Color      | State           | Meaning                                 |
| ---------- | --------------- | --------------------------------------- |
| **Gray**   | Not checked     | No data, offline, never fetched         |
| **Blue**   | Working (pulse) | Command/check in progress               |
| **Green**  | OK              | Status matches expected, up-to-date     |
| **Yellow** | Warning         | Outdated but not critical               |
| **Red**    | Error           | Failed, critical mismatch, needs action |

### State Update Timing

States are updated:

1. **On command completion** - Agent sends status with new generation/state
2. **On explicit refresh** - Compartment click triggers lightweight check
3. **NOT on heartbeat** - Too expensive to re-check statuses every 5s

## Implemented Solution

### 1. Header Agent Target Fix

**Before:** `Agent: master`  
**After:** `Agent: 2.3.0 • Config: abc1234 (main, 2h ago)`

Agent version shown first (always), then config commit info if version tracking is configured.

### 2. Compartment Order & Agent Compartment

**Order: Agent → Git → Lock → System**

Rationale: "Trust first" - agent must be trusted before believing any status it reports.

| #   | Compartment | Icon              | Status Logic                                            |
| --- | ----------- | ----------------- | ------------------------------------------------------- |
| 1   | **Agent**   | `icon-robot`      | Green: version matches dashboard, Red: mismatch         |
| 2   | Git         | `icon-git-branch` | Green: up-to-date, Yellow: behind remote                |
| 3   | Lock        | `icon-lock`       | Green: matches remote, Yellow: behind, Red: agent issue |
| 4   | System      | `icon-nixos`      | Green: current gen, Yellow: rebuild needed              |

### 3. Agent Version Column

New explicit column showing agent version per host:

- **Green text**: Version matches dashboard (current)
- **Red text with background**: Version mismatch (outdated)
- **Gray text**: Version unknown

### 4. Compartment Click Behavior (INFO ONLY)

| Compartment | Click Action                                           |
| ----------- | ------------------------------------------------------ |
| Agent       | Toast with version info                                |
| Git         | Trigger lightweight git status refresh                 |
| Lock        | Trigger lightweight lock status refresh                |
| System      | Trigger lightweight system status refresh (NO dry-run) |

**All clicks are safe. No host modifications. No heavy operations.**

### 5. Larger Compartments

Compartments increased from 29px to 38px for better visibility.

## Implementation Summary

### Files Modified

- `v2/internal/templates/dashboard.templ`:
  - `FleetTargetLine` - Agent version shown first
  - `UpdateStatusCell` - 4 compartments with single indicator dots
  - `AgentVersionCell` - new explicit version column
  - `handleCompartmentClick()` - info-only behavior
  - `renderUpdateStatus()` - single dot color updates
  - Helper functions: `agentIndicatorClass`, `compartmentIndicatorClass`, etc.

- `v2/internal/templates/base.templ`:
  - CSS for 5 indicator colors (gray/blue/green/yellow/red)
  - Larger compartment buttons (38px)
  - Robot icon SVG for Agent compartment

- `v2/internal/dashboard/handlers.go`:
  - `FleetTarget.AgentVer` correctly set from dashboard Version

## Acceptance Criteria

### Compartments ✅

- [x] Order is: Agent → Git → Lock → System
- [x] Agent compartment has robot icon
- [x] Agent status: green when matching dashboard, red when mismatched
- [x] Explicit Agent Version column added

### Click Behavior (INFO ONLY) ✅

- [x] Clicking compartment does NOT modify the host
- [x] Clicking compartment triggers lightweight checks only
- [x] Clicking compartment does NOT trigger heavy ops (nix build --dry-run)
- [x] Modifying actions (Pull/Switch/Test) remain in ellipsis menu only

### Status Dots ✅

- [x] Single dot per compartment (simplified from 3-dot layout)
- [x] 5 color states: gray, blue-pulse, green, yellow, red
- [x] Consistent sizing (6px) and positioning

### Header ✅

- [x] Shows "Agent: 2.3.0" not "Agent: master"
- [x] Agent version always shown first

## Integration with Related Items

### P6900 - Reboot Handling

Status dots handle reboot gracefully:

- Gray during reconnection
- Updates to correct color after agent sends fresh status

### P7200 - Force Uncached Update

When Agent compartment shows red (outdated):

- Ellipsis menu shows "Force Update Agent"
- Triggers force uncached rebuild

## Version

This feature is part of NixFleet v2.3.0.
