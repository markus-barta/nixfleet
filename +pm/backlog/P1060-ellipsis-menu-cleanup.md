# P1060 - Ellipsis Menu Cleanup

**Created**: 2025-12-19  
**Priority**: P1060 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-reliable-agent-updates.md)  
**Estimated Effort**: 1 hour

---

## Scope

Reorganize the ellipsis (⋮) menu to include Test and other actions.

---

## New Menu Structure

```
┌──────────────────┐
│ 🧪 Test          │  ← NEW (moved from action buttons)
│ 🔄 Restart Agent │
│ ⏹️ Stop Command  │
│ ─────────────────│
│ 📋 Copy Hostname │
│ 🔗 SSH Command   │
│ ─────────────────│
│ 🗑️ Remove Host   │
└──────────────────┘
```

### Menu Groups

1. **Commands**: Test, Restart Agent, Stop Command
2. **Utilities**: Copy Hostname, SSH Command
3. **Danger**: Remove Host

---

## Implementation

### Template

```go
templ HostDropdownMenu(host Host) {
    <div class="dropdown-menu" x-show="open" @click.away="open = false">
        // Commands group
        <button class="dropdown-item" @click="sendCommand('test')" :disabled="!host.Online">
            <span class="dropdown-icon">🧪</span> Test
        </button>
        <button class="dropdown-item" @click="sendCommand('restart')">
            <span class="dropdown-icon">🔄</span> Restart Agent
        </button>
        <button class="dropdown-item" @click="stopCommand()" :disabled="!host.PendingCommand">
            <span class="dropdown-icon">⏹️</span> Stop Command
        </button>

        <div class="dropdown-divider"></div>

        // Utilities group
        <button class="dropdown-item" @click="copyHostname()">
            <span class="dropdown-icon">📋</span> Copy Hostname
        </button>
        <button class="dropdown-item" @click="copySSH()">
            <span class="dropdown-icon">🔗</span> SSH Command
        </button>

        <div class="dropdown-divider"></div>

        // Danger group
        <button class="dropdown-item dropdown-danger" @click="confirmRemove()">
            <span class="dropdown-icon">🗑️</span> Remove Host
        </button>
    </div>
}
```

---

## Acceptance Criteria

- [ ] Test command added to ellipsis menu
- [ ] Menu items grouped with dividers
- [ ] Test disabled when host offline
- [ ] Stop disabled when no command running
- [ ] All existing menu items still work

---

## Related

- **P1050**: Action buttons removed (Test moves here)
