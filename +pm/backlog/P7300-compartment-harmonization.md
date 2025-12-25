# P7300: Compartment & Status Harmonization

**Priority:** High  
**Status:** Refined  
**Epic:** UI/UX Polish  
**Created:** 2025-12-25

## Problem Statement

Current dashboard has inconsistencies between compartments and status visualization:

1. **Header agent target** shows "Agent: master" instead of semantic version like "2.2"
2. **Compartments** only cover Git/Lock/System, missing Agent Version
3. **Status dots** use a complex multi-phase model that doesn't align with compartments
4. **Compartment clicks trigger actions** - conflates status display with action triggering

## Design Decisions (Refined)

### Core Principle: Separation of Concerns

**Compartments = STATUS ONLY (read-only)**

- Clicking a compartment triggers info retrieval / status check
- NO changing actions (no Pull, no Switch, no Force Update)
- Actions move to dedicated action buttons / menu

### Decision Summary

| Question           | Decision                                                             |
| ------------------ | -------------------------------------------------------------------- |
| Compartment clicks | Info/check only, NO actions                                          |
| Dots location      | Inside compartment buttons (3 dots under icon)                       |
| Lock compartment   | Status indicator only (no dots, current behavior)                    |
| Dot semantics      | started → running → complete = pre-validate → action → post-validate |
| Test dots          | Same 3-dot pattern: initiated → running → complete                   |

## Proposed Solution

### 1. Header Agent Target Fix

**Current:** `Agent: master`  
**New:** `Agent: 2.2`

Trace issue: `FleetTarget.AgentVer` likely gets wrong value from version fetcher.

### 2. New "Agent" Compartment (4th button)

| Compartment | Icon              | Status                 | Click Action          |
| ----------- | ----------------- | ---------------------- | --------------------- |
| Git         | `icon-git-branch` | repo freshness         | Refresh git status    |
| Lock        | `icon-lock`       | flake.lock freshness   | Show lock info        |
| System      | `icon-cpu`        | system derivation      | Refresh system status |
| **Agent**   | `icon-package`    | agent binary freshness | Show version details  |

**Agent status logic:**

- ✅ OK: `host.AgentVersion == dashboard.Version`
- ⚠️ Warning: version mismatch
- ❌ Error: major mismatch or unknown

**Hover:** "Agent v2.2.0 (build: abc1234)"

### 3. Status Dots - Integrated into Compartments

**New visual model - dots INSIDE compartment buttons:**

```
┌─────────────────────────────────────────────────┐
│  [🔀]   [🔒]   [⚙️]   [📦]   │   [🧪]          │
│  ●○○    ●●●    ○○○    ●●●   │   ●●○           │
│  Git    Lock   Sys    Agent │   Tests         │
└─────────────────────────────────────────────────┘
```

**Dot meanings (3 dots per compartment):**
| Dot | State | Visual |
|-----|-------|--------|
| 1 - Started | Pre-validation running/complete | ○ dim / ● solid |
| 2 - Running | Main action running/complete | ○ dim / ◐ pulse / ● solid |
| 3 - Complete | Post-validation done | ○ dim / ● green (ok) / ● red (fail) |

**The 3rd dot = current status indicator** (what we have now as single dot)

**Lock compartment:** No dots (info-only, no associated action)

**Test dots:** Same pattern
| Dot | Meaning |
|-----|---------|
| 1 - Initiated | Test command started |
| 2 - Running | Tests executing |
| 3 - Complete | All tests done (green=pass, red=fail) |

### 4. Actions Move to Dedicated UI

Since compartments no longer trigger actions, we need:

**Option A: Action buttons in row**

```
[Pull] [Switch] [Test] [Force Update]  (existing cmd-buttons)
```

**Option B: Actions in dropdown menu only**

- Pull → Menu item
- Switch → Menu item
- Force Update → Menu item
- Test → Menu item

**Option C: Context bar actions**

- Hovering compartment shows action in context bar
- Context bar has "Run" button

**Recommendation:** Keep existing action buttons, compartments become pure status display.

## Implementation Plan

### Phase 1: Decouple Compartment Clicks from Actions

1. Remove `handleCompartmentClick()` action triggering
2. Change click to show info / trigger status refresh
3. Keep existing action buttons functional

### Phase 2: Integrate Dots into Compartments

1. Modify compartment button HTML to include 3 dots
2. CSS: position dots below/beside icon
3. Update `renderUpdateStatus()` to update dots
4. Remove separate Status column dots (or repurpose for Tests only)

### Phase 3: Add Agent Compartment

1. Add `BuildHash` to protocol/heartbeat
2. Add 4th compartment button
3. Status logic for version comparison
4. Hover tooltip with version + build

### Phase 4: Header Fix

1. Trace `AgentVer` value source
2. Fix to show semantic version

## Files to Modify

### Protocol

- `v2/internal/protocol/messages.go` - add `AgentVersion`, `BuildHash` to HeartbeatPayload

### Agent

- `v2/internal/agent/heartbeat.go` - populate new fields

### Templates

- `v2/internal/templates/dashboard.templ`:
  - `UpdateStatusCell` - add 4th compartment, integrate dots
  - `handleCompartmentClick()` - change to info-only
  - `renderUpdateStatus()` - update all 3 dots per compartment
- `v2/internal/templates/base.templ` - CSS for 3-dot layout

### Dashboard

- `v2/internal/dashboard/hub.go` - handle new protocol fields
- `v2/internal/dashboard/handlers.go` - fix `AgentVer` in FleetTarget

## Acceptance Criteria

- [ ] Header shows "Agent: 2.2" not "Agent: master"
- [ ] Clicking compartment does NOT trigger Pull/Switch/etc
- [ ] Clicking compartment triggers status refresh or shows info
- [ ] Each compartment has 3 dots (except Lock)
- [ ] Dots show: started → running → complete states
- [ ] 4th "Agent" compartment visible
- [ ] Agent compartment shows version/build on hover
- [ ] Test dots follow same 3-dot pattern
- [ ] Existing action buttons still work

## Dependencies

- P2810 (3-layer binary freshness) - provides `BinaryHash`
- P7200 (Force uncached update) - action for outdated agent

## Migration Notes

**Breaking change:** Users accustomed to clicking compartments to trigger actions will need to use action buttons instead. Consider:

- Tooltip on compartment hover: "Status only - use action buttons to update"
- Transition period with both behaviors?
