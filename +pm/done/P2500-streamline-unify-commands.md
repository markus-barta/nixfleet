# P2500: Streamline and Unify Commands

**Priority**: High  
**Complexity**: Medium  
**Depends On**: P1000 (Update UX Overhaul) - completed  
**Status**: ✅ DONE  
**Updated**: 2025-12-20  
**Completed**: 2025-12-20

---

## Completion Notes

Implemented in commit on 2025-12-20:

### Phase 1: Header "More" Dropdown ✅

- Removed "Update All" (was duplicate of Pull All)
- Added "Do All" (Pull → Switch → Test on all hosts)
- Added "Restart All Agents" bulk action
- Reordered: Do All first, then individual commands

### Phase 3: Per-Host Dropdown ✅

- Added Pull and Switch to dropdown (for completeness)
- Reorganized groups: Deployment / Management / Utilities / Diagnostics / Danger
- Better separation with dividers

### Phase 5: Enhanced Context Bar ✅

- Added PR section with "Merge & Deploy" button
- Added full description text on hover (from data-description attributes)
- Added Go functions: gitContextDescription, lockContextDescription, systemContextDescription
- Added icon-git-pull-request SVG
- Removed tooltips from compartment buttons (info now in context bar)

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

### 3. Context Bar (Unified Info + Actions)

**Current content:**

| Section        | Content                           | Availability                 |
| -------------- | --------------------------------- | ---------------------------- |
| Hover info     | "→ click to {command} {hostname}" | On compartment hover         |
| Selection info | "{N} hosts selected ({M} online)" | When hosts selected          |
| Pull button    | `bulkCommand('pull')`             | When hosts selected + online |
| Switch button  | `bulkCommand('switch')`           | When hosts selected + online |
| Test button    | `bulkCommand('test')`             | When hosts selected + online |
| Do All button  | `doAll()`                         | When hosts selected + online |
| Clear button   | `clearSelection()`                | When hosts selected          |

**Issues:**

- ✅ Well-organized for batch operations
- ✅ Appears on hover (not just selection) — discoverable
- ❌ Hover shows minimal info ("click to pull") — needs full detail
- ❌ PR merge not integrated — hidden in header "More" dropdown
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
- ❌ No visual indication of what clicking will do → **FIXED: Context bar shows on hover**
- ❌ Lock "outdated" shows toast instead of action - inconsistent
- ❌ "refresh" is undocumented - users don't understand it
- ❌ Context bar shows minimal info ("click to pull") not full detail
- ❌ PR merge hidden in "More" dropdown, not contextual

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

NOTE: Do NOT use emojis in the real UI, use SVG-icons instead. Propsed emoji-icons below are placeholders for understanding.

#### Tier 1: Primary Actions (Always Visible)

| Command | Icon       | Description          | Single Host       | Bulk (Header)       | Bulk (Selection) |
| ------- | ---------- | -------------------- | ----------------- | ------------------- | ---------------- |
| Pull    | ↓ download | Git pull nixcfg      | Compartment click | "More" → Pull All   | Context Bar      |
| Switch  | ↻ refresh  | NixOS rebuild switch | Compartment click | "More" → Switch All | Context Bar      |
| Test    | 🧪 flask   | NixOS rebuild test   | Per-host dropdown | "More" → Test All   | Context Bar      |
| Do All  | ▶ play    | Pull → Switch → Test | -                 | "More" → Do All     | Context Bar      |

#### Tier 2: Host Management (Per-Host Dropdown)

| Command       | Icon         | Description            | Category                 |
| ------------- | ------------ | ---------------------- | ------------------------ | --------------- | ----------- |
| Stop          | ■ stop       | Cancel running command | Management (conditional) |
| Pull          | ↓ download   | Git pull nixcfg        | Deployment               |
| Switch        | ↻ refresh    | NixOS rebuild switch   | Deployment               |
| Test          | 🧪 flask     | NixOS rebuild test     | Deployment               |
| Do All        | ▶ play      | Pull → Switch → Test   | -                        | "More" → Do All | Context Bar |
| Restart Agent | ↻ refresh-cw | Restart NixFleet agent | Management               |
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
3. ~~Should Context Bar appear on single-host hover (not just selection)?~~ → YES, already implemented
4. How to handle mixed online/offline selection in bulk actions?

---

## Phase 5: Enhanced Context Bar (NEW)

### Design Decisions

| Question        | Answer                                         |
| --------------- | ---------------------------------------------- |
| PR visibility   | Show as first item in context bar info section |
| Detail level    | Full human-readable text                       |
| Tooltips        | Remove where context bar shows the info        |
| Multi-host + PR | Show both (PR row + selection row)             |

### Context Bar Layout

```
┌────────────────────────────────────────────────────────────────────────────────┐
│ [PR info if pending]  [Hover info]  [Selection info]           [Action buttons]│
└────────────────────────────────────────────────────────────────────────────────┘
```

**Three info sections (left to right, all optional):**

1. **PR Section** (if `pendingPR` exists):
   - "PR #42 ready to merge — update dependencies"
   - Button: [Merge & Deploy]

2. **Hover Section** (if hovering compartment):
   - Full description from tooltip, e.g.:
   - "hsb1: Agent 2.0.0 needs update to 2.1.0 — click to switch"
   - "hsb1: Code is 3 commits behind main — click to pull"
   - "hsb1: Dependencies outdated — merge PR #42 to update"

3. **Selection Section** (if hosts selected):
   - "3 hosts selected (2 online)"
   - Buttons: [Pull] [Switch] [Test] [Do All] [×]

### Hover Info Templates (Full Detail)

| Compartment | Status           | Context Bar Text                                             |
| ----------- | ---------------- | ------------------------------------------------------------ |
| Git         | ok               | "hsb1: Code is up to date"                                   |
| Git         | outdated         | "hsb1: Code is {N} commits behind main — click to pull"      |
| Git         | error            | "hsb1: Git check failed — {error message}"                   |
| Lock        | ok               | "hsb1: Dependencies up to date, agent v{version}"            |
| Lock        | outdated (deps)  | "hsb1: Dependencies outdated — merge PR #{N} to update"      |
| Lock        | outdated (agent) | "hsb1: Agent {old} → {new} needed — click to switch"         |
| Lock        | error            | "hsb1: Lock check failed — {error message}"                  |
| System      | ok               | "hsb1: Configuration applied (gen {N})"                      |
| System      | outdated         | "hsb1: Config changed (gen {old} → {new}) — click to switch" |
| System      | error            | "hsb1: System check failed — {error message}"                |

### Implementation Changes

1. **Add data attributes to compartment buttons:**

   ```html
   data-description="Code is 3 commits behind main" data-detail="Last commit:
   abc1234 (2 days ago)"
   ```

2. **Update `handleCompartmentHover()` to pass description:**

   ```javascript
   window.dispatchEvent(
     new CustomEvent("context-preview", {
       detail: {
         hostId,
         hostname,
         command,
         description: btn.dataset.description, // NEW
         detail: btn.dataset.detail, // NEW (optional)
       },
     }),
   );
   ```

3. **Update Context Bar template:**

   ```html
   <template x-if="hoverAction">
     <span class="context-hover">
       <span class="context-host" x-text="hoverAction.hostname + ':'"></span>
       <span
         class="context-description"
         x-text="hoverAction.description"
       ></span>
       <span
         class="context-action"
         x-text="'— click to ' + hoverAction.command"
       ></span>
     </span>
   </template>
   ```

4. **Add PR section to Context Bar:**

   ```html
   <template x-if="pendingPR">
     <span class="context-pr">
       <span
         >PR #<span x-text="pendingPR.number"></span> ready to merge — update
         dependencies</span
       >
       <button class="btn btn-sm" @click="mergeAndDeploy(pendingPR.number)">
         Merge & Deploy
       </button>
     </span>
   </template>
   ```

5. **Remove tooltips from compartment buttons** (or simplify to just status name)

6. **Add deploy progress display** (moved from P2100):
   - Currently `handleFlakeUpdateJob()` just logs to console
   - Should show deploy progress in context bar or toast
   - States: "Merging PR #42...", "Deploying to 3 hosts...", "Complete"

### Tasks

- [ ] Add `data-description` and `data-detail` to compartment buttons
- [ ] Update Go tooltip functions to return structured data for JS
- [ ] Update `handleCompartmentHover()` to pass description
- [ ] Update Context Bar to show full description
- [ ] Add PR section to Context Bar (when `pendingPR` exists)
- [ ] Remove/simplify tooltips from compartments
- [ ] Ensure both PR + selection show together when applicable
- [ ] Test all compartment states with new descriptions
- [ ] Implement deploy progress display (replace `handleFlakeUpdateJob()` console.log)

---

## Related Items

- P1000: Update UX Overhaul (completed)
- P1020: Clickable Compartments (completed)
- P1015: Selection Bar → Context Bar (completed)
- P1060: Ellipsis Menu Cleanup (completed)
