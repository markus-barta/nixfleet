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

**Compartments = INFO ONLY (safe checks allowed, modifications forbidden)**

- ✅ Show cached info (modal/popover)
- ✅ Trigger lightweight agent checks (git status, read generation, etc.)
- ❌ Modify host (git pull, nixos-rebuild, etc.)
- ❌ Heavy operations (nix build --dry-run)
- Modifying actions only available in ellipsis menu

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

### 2. Compartment Order & New "Agent" Compartment

**Order: Agent → Git → Lock → System → Tests**

Rationale: "Trust first" - agent must be trusted before believing any status it reports.

| #   | Compartment | Icon              | Status                 | Click Action (INFO ONLY)            |
| --- | ----------- | ----------------- | ---------------------- | ----------------------------------- |
| 1   | **Agent**   | `icon-package`    | agent binary freshness | Show version + build + store path   |
| 2   | Git         | `icon-git-branch` | repo freshness         | Refresh git status + show diff info |
| 3   | Lock        | `icon-lock`       | flake.lock freshness   | Show lock commit details            |
| 4   | System      | `icon-cpu`        | system derivation      | Show generation + derivation path   |
| 5   | Tests       | `icon-check`      | test results           | Show test results summary           |

**Click = INFO ONLY.** Safe lightweight checks allowed. No host modification. No heavy ops.

**Agent status logic:**

- ✅ OK: `host.AgentVersion == dashboard.Version`
- ⚠️ Warning: version mismatch
- ❌ Error: major mismatch or unknown

**Hover:** "Agent v2.2.0 (build: abc1234)"

### 3. Status Dots - Integrated into Compartments

**New visual model - dots INSIDE compartment buttons:**

```
┌────────────────────────────────────────────┐
│  [🤖]   [🔀]   [🔒]   [⚙️]   [🧪]            │
│  ●●●    ●○○    ●●●    ○○○    ●●○           │
│  Agent  Git    Lock   Sys    Tests         │
└────────────────────────────────────────────┘
```

Note: No emojis allowed. Use SVG icons instead!

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

### 4. Actions Location (Current UI Already Correct)

Current UI has NO per-host action buttons. Actions are triggered via:

1. **Ellipsis (⋮) menu per host** - Pull, Switch, Test, etc.
2. **Context bar for multi-select** - Bulk actions on selected hosts

This is the correct pattern. Compartments become pure status display.

**New action for Agent compartment:** When agent is outdated, menu shows "Force Update Agent" (P7200).

## Implementation Plan

### Phase 1: Reorder Compartments + Add Agent

1. Reorder existing compartments: Agent → Git → Lock → System
2. Add Agent compartment with `icon-package`
3. Add `AgentVersion` + `BuildHash` to HeartbeatPayload
4. Agent status logic (compare with dashboard version)
5. Hover tooltip: "Agent v2.2.0 (build: abc1234)"

### Phase 2: Decouple Compartment Clicks

1. Remove `handleCompartmentClick()` action triggering
2. Change click to show info modal / trigger status refresh
3. Actions remain in ellipsis menu only

### Phase 3: Integrate 3-Dot Status into Compartments

1. Modify compartment button HTML: icon + 3 dots below
2. CSS: position dots, define states (dim/pulse/solid/green/red)
3. Update `renderUpdateStatus()` to update all 3 dots
4. Remove or repurpose separate Status column

### Phase 4: Tests Compartment

1. Add Tests as 5th compartment (after System)
2. Same 3-dot pattern: initiated → running → complete
3. Click shows test results detail

### Phase 5: P6900 + P7200 Integration

1. Handle reboot interruption in dot states
2. Add "Force Update Agent" to menu when agent outdated
3. Wire up P7200 command chain

### Phase 6: Header Fix

1. Trace `AgentVer` value source
2. Fix to show "2.2" not "master"

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

### Compartments

- [ ] Order is: Agent → Git → Lock → System → Tests
- [ ] Agent compartment shows version/build on hover
- [ ] Agent status: ✅ when matching dashboard, ⚠️/❌ when mismatched
- [ ] Tests is now a compartment (5th position)

### Click Behavior (INFO ONLY)

- [ ] Clicking compartment does NOT modify the host
- [ ] Clicking compartment may trigger lightweight checks (git status, etc.)
- [ ] Clicking compartment does NOT trigger heavy ops (nix build --dry-run)
- [ ] Modifying actions (Pull/Switch/Test) remain in ellipsis menu only

### Status Dots

- [ ] Each compartment has 3 dots (except Lock = status only)
- [ ] Dots show: started → running → complete states
- [ ] Dot 3 = current status indicator (green/yellow/red)
- [ ] Test dots: initiated → running → complete

### Integration

- [ ] P6900: Dots handle reboot interruption gracefully
- [ ] P7200: "Force Update Agent" in menu when agent outdated
- [ ] Header shows "Agent: 2.2" not "Agent: master"

## Integration with Related Items

### P6900 - Reboot Handling

Status dots must handle reboot interruptions:

| Scenario                          | Dot Behavior                |
| --------------------------------- | --------------------------- |
| Command running → reboot detected | Dot 2 shows ⚠️ amber pulse  |
| Agent reconnects after reboot     | Dot 2 → solid, Dot 3 starts |
| Post-validation after reboot      | Dot 3 completes (green/red) |

### P7200 - Force Uncached Update

When Agent compartment shows outdated (⚠️ or ❌):

- Ellipsis menu shows "Force Update Agent"
- Triggers: `git pull && nix flake update nixfleet && nixos-rebuild switch --option narinfo-cache-negative-ttl 0`
- Agent dots show progress: started → running → complete

### P2810 - 3-Layer Binary Freshness

Agent compartment status uses P2810 data:

- `SourceCommit` - git commit agent was built from
- `StorePath` - Nix store path
- `BinaryHash` - unique build identifier (shown in tooltip)

## Dependencies

- P2810 (3-layer binary freshness) - provides `BinaryHash` ✅ implemented
- P6900 (Reboot handling) - status dots during reboot
- P7200 (Force uncached update) - Agent menu action

## Migration Notes

**Breaking change:** Users accustomed to clicking compartments to trigger Pull/Switch will now use the ellipsis menu.

Mitigation:

- Tooltip on compartment hover explains new behavior
- Context bar shows relevant action when compartment is hovered
- First-time user guidance / changelog entry
