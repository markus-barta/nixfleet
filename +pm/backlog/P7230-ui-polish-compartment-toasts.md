# P7230: UI Polish + Compartment Click Toasts

**Priority**: Medium  
**Effort**: Medium  
**Status**: In Progress

## UI Polish

### 1. Compartment Icon Sizing

- Icon: 20% bigger (11px → 13px), centered X and Y
- Indicator dot: 15% smaller (6px → 5px)

### 2. Column Reordering

Current order:

```
Hosts | Type | Metrics | Agent | Status | Last Seen | Menu | Select
```

New order:

```
Select | Hosts (with device icon prefix) | Type | Metrics | Agent | Status | Last Seen | Menu
```

Changes:

- Checkbox column moves to first position
- Device type icon (server/desktop/laptop) becomes prefix before hostname
- Type column stays (still has location + OS icons)
- Last seen stays after metrics (no change needed)

### 3. Checkbox Visibility

- Unchecked, not hovered: visible at 10% opacity
- Checked or hovered: full opacity

### 4. Device Icon Before Hostname

- Show device type icon (server/desktop/laptop/gaming) inline before hostname
- Sized like the subscript/superscript icons in Type column
- Small gap between icon and hostname text

## Functionality

### 1. Agent `check-version` Command

New command that runs `nixfleet-agent --version` as subprocess and reports:

- **Running version**: from `agent.Version` constant (current process)
- **Installed version**: from subprocess output (disk binary)

Detects discrepancy when binary updated but service not restarted.

### 2. Compartment Click → Toast

When clicking ANY compartment, show toast with TL;DR:

| Compartment | Toast Example                                    |
| ----------- | ------------------------------------------------ |
| Agent       | "Agent OK: 2.3.2 running, 2.3.2 installed"       |
| Agent       | "Agent mismatch: running 2.3.1, installed 2.3.2" |
| Git         | "Git: 3 commits behind origin/main"              |
| Lock        | "Lock: flake.lock is current"                    |
| System      | "System: needs rebuild (outdated)"               |

### 3. Compartment Click → Log

In addition to toast, append detailed log line to host's output panel:

- Toast: short TL;DR
- Log: full details (timestamps, command output, etc.)

## Acceptance Criteria

- [ ] Compartment icon 20% bigger, centered
- [ ] Indicator dot 15% smaller
- [ ] Checkbox column first, visible at 10% opacity when unchecked
- [ ] Device icon prefix before hostname
- [ ] Agent compartment click runs `check-version` command
- [ ] All compartment clicks show toast with TL;DR
- [ ] All compartment clicks append log line with details

## Implementation Notes

### CSS Changes (base.templ)

```css
.update-compartment .update-icon {
  width: 13px; /* was 11px */
  height: 13px;
  /* remove position offsets, use flexbox centering */
}

.compartment-indicator {
  width: 5px; /* was 6px */
  height: 5px;
}

.row-select-toggle {
  opacity: 0.1; /* when unchecked */
}
.row-select-toggle:hover,
.row-select-toggle.selected {
  opacity: 1;
}
```

### Agent Command (commands.go)

```go
case "check-version":
    a.handleCheckVersion()
    return
```

### JavaScript (dashboard.templ)

Update `handleCompartmentClick` to:

1. Show toast for all compartments
2. Append log line for all compartments
3. For agent: send `check-version` command instead of just showing local info
