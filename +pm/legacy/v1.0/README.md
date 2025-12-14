# NixFleet v1.0 Legacy Reference

This folder contains the v1.0 prototype implementation preserved for reference during the v2.0 Go rewrite.

## Purpose

These files serve as visual and code references to ensure the v2.0 rewrite maintains feature parity and design consistency with the working prototype.

## Contents

### Visual Reference

- `screenshot.png` - Dashboard screenshot at v0.4.0-46

### Frontend Templates (Jinja2/Python)

- `base.html` - Base template with Tokyo Night theme CSS
- `dashboard.html` - Full dashboard with ~1,200 lines CSS + ~1,000 lines JS
- `login.html` - Login page with TOTP support

### Agent (Bash)

- `nixfleet-agent.sh` - Original Bash polling agent

## Key Design Elements

### Color Palette (Tokyo Night)

```css
--bg: #1a1b26; /* Main background */
--bg-dark: #16161e; /* Darker panels */
--bg-highlight: #292e42; /* Selection/hover */
--fg: #c0caf5; /* Primary text */
--blue: #7aa2f7; /* Primary accent */
--cyan: #7dcfff; /* Info, highlights */
--green: #9ece6a; /* Success, online */
--yellow: #e0af68; /* Warning, pending */
--red: #f7768e; /* Error, critical */
--purple: #bb9af7; /* Secondary accent */
```

### Font Stack

```css
font-family: "JetBrains Mono", "Fira Code", "SF Mono", "Consolas", monospace;
```

### Key UI Components

1. **Host Table** - Sortable columns with per-host theme colors
2. **Status Papertrail** - Expandable status history per host
3. **Heartbeat Ripple** - Animated connection indicator
4. **Action Buttons** - Pull, Switch, Test with disabled states
5. **Dropdown Menus** - Bulk actions, host-specific actions
6. **Modals** - Add Host, Remove Host confirmation
7. **SSE Connection** - Real-time updates with reconnect logic

### Data Bindings (for Go templates)

The Jinja2 templates show what data the backend must provide:

```python
# Dashboard context
hosts: List[Host]           # All registered hosts
stats: {online, total}      # Summary counts
nixcfg_hash: str            # Target config hash
version: str                # NixFleet version
server_hostname: str        # Where dashboard runs
csrf_token: str             # CSRF protection
csp_nonce: str              # CSP nonce for inline scripts

# Per-host data
host.id, host.hostname, host.online
host.os_version, host.host_type, host.location, host.device_type
host.current_generation, host.outdated
host.pending_command, host.test_running
host.metrics: {cpu, ram, swap, load}
host.status_history: List[{timestamp, icon, message}]
host.theme_color
```

## Notes

- This is **read-only reference material**
- The actual v1.0 code remains in the main codebase until v2.0 is complete
- Git tag `v1.0.0` marks the prototype completion point
