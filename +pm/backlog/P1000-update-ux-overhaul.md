# P1000 - Update UX Overhaul

**Created**: 2025-12-19  
**Updated**: 2025-12-19  
**Priority**: P1000 (Critical - UX)  
**Status**: Design Complete  
**Estimated Effort**: 2-3 days  
**Depends on**: None

---

## Background: What We Ruled Out

Originally we thought macOS agents weren't restarting after switch. **Manual testing proved this wrong**:

- ✅ **macOS**: `home-manager switch` correctly restarts agent via launchd
- ✅ **NixOS**: `nixos-rebuild switch` correctly restarts agent via systemd exit(101)

**The real issue**: Users click "Pull" and think they're done, but they also need to click "Switch".

---

## Executive Summary

Complete refactor of the update/action UX to make the dashboard self-explanatory. Users currently don't understand that they need to click both Pull AND Switch. The new design makes compartments clickable and adds an Action Bar that shows what will happen before clicking.

**Key Changes**:

- Remove separate action buttons (Pull, Switch, Test)
- Make compartments in Update column clickable
- Add Action Bar in header showing action preview
- Add row selection with checkboxes
- Add dependency warnings with option dialogs

---

## Design Specification

### 1. Table Layout (New)

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Host    │ Type   │ Location │ Metrics      │ Update     │ ⋮  │ ☐/☑     │
│         │        │          │              │ [G][L][S]  │    │ [select] │
├──────────────────────────────────────────────────────────────────────────┤
│ hsb1    │ 🖥️    │ home     │ CPU 12%...   │ 🟢 🟢 🟡   │ ⋮  │ ☐        │
│ gpc0    │ 🎮    │ home     │ CPU 45%...   │ 🟡 🔴 🟡   │ ⋮  │ ☐        │
└──────────────────────────────────────────────────────────────────────────┘
                                               ↑              ↑
                                          Clickable!    On hover / selected
```

**Checkbox Column Header**: Mini button to select all / select none (toggles)

### 2. Action Bar (Fixed in Header)

Position: Center of header, between logo and user menu.

#### Idle State

```
┌─────────────────────────────────────────────────┐
│          Hover a status to see actions          │
└─────────────────────────────────────────────────┘
```

#### Single Host Action Preview

```
┌─────────────────────────────────────────────────┐
│  📥 PULL                           [▶ DO NOW]   │
│  ─────────────────────────────────────────────  │
│  Fetch latest code from GitHub                  │
│  Host: hsb1                                     │
└─────────────────────────────────────────────────┘
```

#### Multi-Host Action Preview (with "Do All")

```
┌─────────────────────────────────────────────────┐
│  📥 PULL ALL                       [▶ DO ALL]   │
│  ─────────────────────────────────────────────  │
│  Fetch latest code from GitHub                  │
│  Hosts: hsb0, hsb1, gpc0 (3 selected)           │
└─────────────────────────────────────────────────┘
```

#### Action In Progress

```
┌─────────────────────────────────────────────────┐
│  ⟳ PULLING...                                   │
│  ─────────────────────────────────────────────  │
│  Fetching latest code from GitHub               │
│  Host: hsb1                                     │
└─────────────────────────────────────────────────┘
```

#### Action Complete

```
┌─────────────────────────────────────────────────┐
│  ✓ PULL COMPLETE                                │
│  ─────────────────────────────────────────────  │
│  Successfully fetched latest code               │
│  Host: hsb1                                     │
└─────────────────────────────────────────────────┘
```

### 3. Compartment Actions

| Compartment | Status            | Click Action                      |
| ----------- | ----------------- | --------------------------------- |
| **Git**     | 🟡 Behind         | `pull` command                    |
| **Git**     | 🟢 Current        | Refresh status check              |
| **Lock**    | 🟡 Old            | Info only (update via GitHub PR)  |
| **Lock**    | 🔴 Agent outdated | `switch` command (same as System) |
| **Lock**    | 🟢 Current        | Refresh status check              |
| **System**  | 🟡 Needs rebuild  | `switch` command                  |
| **System**  | 🟢 Current        | Refresh status check              |

### 4. Row Selection

**Checkbox behavior**:

- Visible on row hover
- Always visible when row is selected
- Header has select all/none toggle button

**Selection trigger**:

- Clicking checkbox
- Clicking free background area of row
- NOT clicking: text (for copy), compartment buttons, ellipsis menu

**Selected row styling**:

- Brighter background
- Checkbox checked

### 5. Option Dialog (Dependency Warnings)

When user clicks an action that has unmet dependencies:

```
┌─────────────────────────────────────────────────┐
│  ⚠️ Git is behind on hsb1                       │
│  ─────────────────────────────────────────────  │
│  Running Switch without Pull may use old code.  │
│                                                 │
│  [Cancel]  [Pull]  [Switch]  [Pull + Switch]    │
└─────────────────────────────────────────────────┘
```

**Dependency chain**:

```
Pull (Git) → Update Lock (optional) → Switch (System)
```

### 6. Ellipsis Menu (Remaining Actions)

```
┌──────────────────┐
│ 🧪 Test          │
│ 🔄 Restart Agent │
│ ⏹️ Stop Command  │
│ ─────────────────│
│ 📋 Copy Hostname │
│ 🔗 SSH Command   │
│ ─────────────────│
│ 🗑️ Remove Host   │
└──────────────────┘
```

### 7. Animation

- Action Bar content: 1s fade in, 1s fade out
- Prevents "bouncy" updates when moving mouse between hosts
- Total debounce: ~2s before content changes

---

## Implementation Sub-Items

| ID    | Item                   | Scope                              |
| ----- | ---------------------- | ---------------------------------- |
| P1010 | Action Bar Component   | Header component, fade animation   |
| P1020 | Clickable Compartments | Make Update column interactive     |
| P1030 | Row Selection          | Checkbox, multi-select, select all |
| P1040 | Option Dialog          | Dependency warning modal           |
| P1050 | Remove Action Buttons  | Delete Pull/Switch/Test buttons    |
| P1060 | Ellipsis Menu Cleanup  | Move Test, reorganize menu         |

---

## Removed from Scope

These were in the original P1000 but are NOT needed:

- ~~launchctl kickstart for macOS~~ → Agent restart works correctly
- ~~Agent self-restart code~~ → Works correctly
- ~~Activation hooks~~ → Not needed

The root cause was UX confusion, not technical bugs.

---

## Acceptance Criteria

- [ ] Action Bar shows in header (fixed position)
- [ ] Hovering compartment shows action preview in Action Bar
- [ ] Clicking compartment executes action
- [ ] Row checkboxes appear on hover
- [ ] Select all/none button in checkbox header
- [ ] Multi-select enables "Do All" in Action Bar
- [ ] Dependency warning dialog appears when needed
- [ ] Pull/Switch/Test buttons removed from table
- [ ] Test moved to ellipsis menu
- [ ] 1s fade in/out animation on Action Bar

---

## Testing

### Manual Test Cases

1. **Single host pull**: Hover Git compartment → Action Bar shows "Pull" → Click → Pull runs
2. **Multi-host switch**: Select 3 hosts → Hover "Do All" → Action Bar shows all 3 → Click → All switch
3. **Dependency warning**: Git yellow, click System → Dialog appears → Choose "Pull + Switch"
4. **Select all**: Click header checkbox → All rows selected
5. **Animation**: Move mouse quickly between hosts → Action Bar doesn't flicker

---

## Files to Modify

| File                   | Changes                                              |
| ---------------------- | ---------------------------------------------------- |
| `dashboard.templ`      | Add Action Bar, modify Update column, add checkboxes |
| `styles.css`           | Action Bar styling, selected row, animations         |
| `dashboard.templ` (JS) | Hover handlers, selection logic, action execution    |
| `hub.go`               | Handle multi-host commands                           |

---

## Related

- [P1010](./P1010-action-bar-component.md) — Action Bar implementation
- [P1020](./P1020-clickable-compartments.md) — Clickable compartments
- [P1030](./P1030-row-selection.md) — Row selection & multi-select
- [P1040](./P1040-option-dialog.md) — Dependency warning dialog
- [P1050](./P1050-remove-action-buttons.md) — Remove old buttons
- [P1060](./P1060-ellipsis-menu-cleanup.md) — Ellipsis menu reorganization
- [P6800](./P6800-mobile-card-view.md) — Mobile-specific card view
