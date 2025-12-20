# P2000: Streamline and Unify Commands

**Priority**: High  
**Complexity**: Medium  
**Depends On**: P1000 (Update UX Overhaul) - completed  
**Status**: Planning

---

## Problem Statement

The NixFleet dashboard has accumulated commands across multiple UI elements without a coherent organizational strategy. Users face confusion about:

1. **Where to find actions** - Commands are scattered across 4 different UI locations
2. **What commands do** - Similar-sounding commands (Update vs Pull) have unclear differences
3. **When to use what** - Compartment buttons vs dropdown vs context bar overlap
4. **Bulk vs single** - Inconsistent availability of actions for single hosts vs multiple

This creates cognitive overhead for both sysops (who need efficiency) and beginners (who need clarity).

---

## Current State: Complete Command Inventory

### 1. Header "More" Dropdown (Global Bulk Actions)

| UI Label             | Backend Command                  | Description                                   | Scope            |
| -------------------- | -------------------------------- | --------------------------------------------- | ---------------- |
| Merge & Deploy PR #N | `mergeAndDeploy(prNum)`          | Merge GitHub PR, then run update on all hosts | All online hosts |
| Update All           | `bulkCommand('update')` → `pull` | Alias for Pull All                            | All online hosts |
| Pull All             | `bulkCommand('pull')`            | Git pull nixcfg repo                          | All online hosts |
| Switch All           | `bulkCommand('switch')`          | NixOS rebuild switch                          | All online hosts |

**Issues:**

- ❌ "Update All" and "Pull All" are confusingly similar
- ❌ No "Test All"
- ❌ No "Restart Agent All"
- ❌ Naming inconsistent with Context Bar

---

### 2. Per-Host Ellipsis Dropdown (Single Host Actions)

| UI Label      | Backend Command                  | Description                       | Availability         |
| ------------- | -------------------------------- | --------------------------------- | -------------------- |
| Test          | `sendCommand(hostId, 'test')`    | Run nixos-rebuild test            | Online only          |
| Stop          | `sendCommand(hostId, 'stop')`    | Cancel running command            | When command running |
| Restart Agent | `sendCommand(hostId, 'restart')` | Restart NixFleet agent            | Online only          |
| Copy Hostname | `copyToClipboard(hostname)`      | Copy hostname to clipboard        | Always               |
| SSH Command   | `copyToClipboard(sshCmd)`        | Copy `ssh user@host` to clipboard | Always               |
| Download Logs | `downloadLogs(hostId)`           | Download agent logs               | Always               |
| Remove Host   | `confirmRemoveHost(hostId)`      | Remove host from dashboard        | Always (dangerous)   |

**Issues:**

- ❌ No "Pull" or "Switch" - must use compartment buttons
- ❌ "Restart Agent" only here, not in bulk actions
- ❌ Grouped by arbitrary "Actions/Utilities/Admin" that don't match user mental model

---

### 3. Context Bar (Selection Bulk Actions)

| UI Label | Backend Command         | Description                      | Availability                 |
| -------- | ----------------------- | -------------------------------- | ---------------------------- |
| Pull     | `bulkCommand('pull')`   | Git pull on selected             | When hosts selected + online |
| Switch   | `bulkCommand('switch')` | NixOS rebuild switch on selected | When hosts selected + online |
| Test     | `bulkCommand('test')`   | NixOS rebuild test on selected   | When hosts selected + online |
| Do All   | `doAll()`               | Pull → Switch → Test sequence    | When hosts selected + online |
| (Clear)  | `clearSelection()`      | Deselect all                     | When hosts selected          |

**Issues:**

- ✅ Well-organized for batch operations
- ❌ Only appears when hosts are selected - not discoverable
- ❌ No "Restart Agent" batch

---

### 4. Compartment Buttons (Status-Driven Single Host)

**Git Compartment** (leftmost):

| Status            | Click Action          | Description           |
| ----------------- | --------------------- | --------------------- |
| ok (green)        | `refreshHost()`       | Re-check git status   |
| outdated (yellow) | `sendCommand('pull')` | Pull latest changes   |
| error (red)       | Show error toast      | Display error message |

**Lock Compartment** (middle):

| Status            | Click Action            | Description                                              |
| ----------------- | ----------------------- | -------------------------------------------------------- |
| ok (green)        | `refreshHost()`         | Re-check lock status                                     |
| outdated (yellow) | Show info toast         | "Dependencies outdated. Update flake.lock via GitHub PR" |
| agent outdated    | `sendCommand('switch')` | Switch to update agent                                   |
| error (red)       | Show error toast        | Display error message                                    |

**System Compartment** (rightmost):

| Status            | Click Action            | Description             |
| ----------------- | ----------------------- | ----------------------- |
| ok (green)        | `refreshHost()`         | Re-check system status  |
| outdated (yellow) | `sendCommand('switch')` | Switch to apply changes |
| error (red)       | Show error toast        | Display error message   |

**Issues:**

- ❌ Actions are context-dependent → unpredictable
- ❌ No visual indication of what clicking will do
- ❌ Lock "outdated" shows toast instead of action - inconsistent
- ❌ "refresh" is undocumented - users don't understand it
- ❌ No hover state explaining the action before clicking

---

## Refined Proposal: Unified Command Structure

### Design Principles

1. **Predictability**: Same action should be in the same place
2. **Discoverability**: User can find any action within 2 clicks
3. **Consistency**: Naming matches across all UI locations
4. **Progressive disclosure**: Common actions visible, advanced actions in menus
5. **Context-awareness**: Show relevant actions based on selection state

---

### Proposed Command Organization

#### Tier 1: Primary Actions (Always Visible)

| Command | Icon       | Description          | Single Host       | Bulk (Header)       | Bulk (Selection) |
| ------- | ---------- | -------------------- | ----------------- | ------------------- | ---------------- |
| Pull    | ↓ download | Git pull nixcfg      | Compartment click | "More" → Pull All   | Context Bar      |
| Switch  | ↻ refresh  | NixOS rebuild switch | Compartment click | "More" → Switch All | Context Bar      |
| Test    | 🧪 flask   | NixOS rebuild test   | Per-host dropdown | "More" → Test All   | Context Bar      |
| Do All  | ▶ play    | Pull → Switch → Test | -                 | "More" → Do All     | Context Bar      |

#### Tier 2: Host Management (Per-Host Dropdown)

| Command       | Icon         | Description            | Category                 |
| ------------- | ------------ | ---------------------- | ------------------------ |
| Test          | 🧪 flask     | NixOS rebuild test     | Deployment               |
| Restart Agent | ↻ refresh-cw | Restart NixFleet agent | Management               |
| Stop          | ■ stop       | Cancel running command | Management (conditional) |
| Copy Hostname | 📋 copy      | Copy hostname          | Utilities                |
| Copy SSH      | 💻 terminal  | Copy ssh command       | Utilities                |
| Download Logs | 📄 file      | Download agent logs    | Diagnostics              |
| Remove Host   | 🗑 trash     | Remove from dashboard  | Danger                   |

#### Tier 3: Global Operations (Header "More")

| Command            | Icon         | Description                       | Notes               |
| ------------------ | ------------ | --------------------------------- | ------------------- |
| Merge & Deploy PR  | ✓ check      | Merge PR + Pull All + Switch All  | Only when PR exists |
| Do All             | ▶ play      | Pull → Switch → Test on ALL hosts | NEW                 |
| Pull All           | ↓ download   | Pull on all online hosts          |                     |
| Switch All         | ↻ refresh    | Switch on all online hosts        |                     |
| Test All           | 🧪 flask     | Test on all online hosts          | NEW                 |
| Restart All Agents | ↻ refresh-cw | Restart agents on all hosts       | NEW                 |

#### Tier 4: Compartment Buttons (Status Indicators)

**Simplified behavior:**

| Compartment | Green (ok)  | Yellow (outdated) | Red (error) |
| ----------- | ----------- | ----------------- | ----------- |
| Git         | No action\* | Pull              | Show error  |
| Lock        | No action\* | Info toast        | Show error  |
| System      | No action\* | Switch            | Show error  |

\*Green = no action needed, clicking refreshes status

**Context bar preview on hover:**

- Hovering shows "→ click to pull on hostname" in context bar
- User knows what will happen before clicking

---

### Key Changes Summary

1. **Remove "Update All"** - redundant with "Pull All"
2. **Add "Test All" to header "More"** - consistency with Context Bar
3. **Add "Do All" to header "More"** - full deployment in one click
4. **Add "Restart All Agents" to header "More"** - missing bulk action
5. **Keep compartment buttons simple** - status indicators with contextual action
6. **Improve hover preview** - show exactly what action will happen
7. **Rename categories in per-host dropdown**:
   - "Actions" → "Deployment"
   - "Utilities" → "Utilities" (keep)
   - "Admin" → "Diagnostics" + "Danger"

---

### User Journeys

#### Sysop: "I need to update all hosts after a config change"

**Current experience:**

1. Click "More" → "Update All" (wait, or "Pull All"? What's the difference?)
2. Then click... where? System compartments one by one?

**Proposed experience:**

1. Click "More" → "Do All" (Pull → Switch → Test on all hosts)
2. Done.

#### Beginner: "How do I deploy my changes to one host?"

**Current experience:**

1. Click hostname → nothing happens
2. Click ellipsis → "Test"? "Restart Agent"? Where's "Deploy"?
3. Click compartment... what does yellow mean?
4. Eventually clicks yellow System → "It worked!"

**Proposed experience:**

1. Hover over yellow System compartment → Context bar shows "→ click to switch on hsb1"
2. Click → Action executes
3. OR: Select host checkbox → Context bar shows Pull/Switch/Test/Do All buttons

#### Sysop: "I need to restart agents on all hosts"

**Current experience:**

1. Click ellipsis per host → Restart Agent
2. Repeat 8 times

**Proposed experience:**

1. Click "More" → "Restart All Agents"
2. Done.

---

## Implementation Tasks

### Phase 1: Clean Up Header "More" Dropdown

- [ ] Remove "Update All" (duplicate of "Pull All")
- [ ] Add "Test All"
- [ ] Add "Do All" (Pull → Switch → Test on ALL online hosts)
- [ ] Add "Restart All Agents"
- [ ] Reorder: Do All > Pull All > Switch All > Test All > Restart All

### Phase 2: Improve Compartment UX

- [ ] Ensure hover preview always shows expected action
- [ ] Green compartments: clicking shows "No action needed, status refreshed" toast
- [ ] Yellow compartments: consistent action (Pull for Git, Switch for System)
- [ ] Lock yellow: show clearer message about GitHub PR requirement

### Phase 3: Reorganize Per-Host Dropdown

- [ ] Rename groups: Deployment / Utilities / Diagnostics / Danger
- [ ] Keep order: Test, Stop (if applicable), Restart Agent | Copy Hostname, SSH | Logs | Remove

### Phase 4: Documentation

- [ ] Update RUNBOOK with command reference
- [ ] Add tooltips explaining each action
- [ ] Context bar always hints at available actions

---

## Success Criteria

1. **User can find any action within 2 clicks**
2. **No duplicate/confusing commands** (Update vs Pull)
3. **Consistent naming** across all UI locations
4. **Hover always previews** what will happen
5. **Bulk operations available** for all common single-host actions

---

## Open Questions

1. Should "Do All" be the primary/default action (most prominent button)?
2. Should per-host dropdown include Pull/Switch for completeness?
3. Should Context Bar appear on single-host hover (not just selection)?
4. How to handle mixed online/offline selection in bulk actions?

---

## Related Items

- P1000: Update UX Overhaul (completed)
- P1020: Clickable Compartments (completed)
- P1015: Selection Bar → Context Bar (completed)
- P1060: Ellipsis Menu Cleanup (completed)
