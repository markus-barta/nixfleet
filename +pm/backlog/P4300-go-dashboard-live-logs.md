# Go Dashboard: Live Log Streaming

**Created**: 2025-12-14
**Priority**: P4300 (Critical)
**Status**: Partial (streaming works, UI/storage pending)
**Depends on**: P4200 (Go Dashboard Core)

---

## Tests to Pass

| Test                                            | Description                |
| ----------------------------------------------- | -------------------------- |
| [T03](../../tests/specs/T03-agent-commands.md)  | Output streaming scenarios |
| [T07](../../tests/specs/T07-e2e-deploy-flow.md) | Real-time output in deploy |

---

## Overview

Real-time command output streaming from agents to browser, with log viewer UI.

---

## Data Flow

```text
┌─────────┐    WebSocket     ┌───────────┐    WebSocket     ┌─────────┐
│  Agent  │ ───────────────▶ │ Dashboard │ ───────────────▶ │ Browser │
│         │  output lines    │           │  output lines    │         │
└─────────┘                  └───────────┘                  └─────────┘
     │                            │                              │
     │                            ▼                              │
     │                    ┌───────────────┐                      │
     │                    │  File Store   │                      │
     │                    │  /data/logs/  │                      │
     │                    └───────────────┘                      │
```

---

## Agent → Dashboard

```go
// Agent sends output lines
type OutputMessage struct {
    Type      string `json:"type"`      // "output"
    HostID    string `json:"host_id"`
    Command   string `json:"command"`
    Line      string `json:"line"`
    Timestamp string `json:"timestamp"`
    IsError   bool   `json:"is_error"`  // stderr vs stdout
}
```

---

## Dashboard → Browser

```go
// Dashboard forwards to subscribed browsers
type BrowserOutputMessage struct {
    Type      string `json:"type"`      // "command_output"
    HostID    string `json:"host_id"`
    Command   string `json:"command"`
    Line      string `json:"line"`
    Timestamp string `json:"timestamp"`
    IsError   bool   `json:"is_error"`
}
```

---

## Log Storage

```text
/data/logs/
├── hsb0/
│   ├── 2025-12-14T10-30-00-switch.log
│   └── 2025-12-14T09-15-00-test.log
├── csb1/
│   └── 2025-12-14T11-00-00-pull.log
```

Format:

```text
# Command: switch
# Started: 2025-12-14T10:30:00Z
# Host: hsb0

[10:30:01] evaluating derivation '/nix/store/...'
[10:30:05] building '/nix/store/abc123...'
[10:30:10] building '/nix/store/def456...'
[10:31:00] activating the configuration...
[10:31:05] setting up /etc...
[10:31:10] switching to configuration /nix/store/...

# Completed: 2025-12-14T10:31:15Z
# Exit code: 0
```

---

## UI Components

### 1. Log Panel (Collapsible)

```html
<div x-data="{ open: false }" class="log-panel">
  <button @click="open = !open">
    <span x-show="!open">▶ Show Log</span>
    <span x-show="open">▼ Hide Log</span>
  </button>

  <div x-show="open" class="log-content" x-ref="logContent">
    <!-- Log lines injected here -->
  </div>
</div>
```

### 2. Auto-Scroll with Pause

```javascript
// Alpine.js component
Alpine.data("logViewer", (hostId) => ({
  lines: [],
  autoScroll: true,

  init() {
    this.subscribe(hostId);
  },

  addLine(line) {
    this.lines.push(line);
    if (this.autoScroll) {
      this.$refs.logContent.scrollTop = this.$refs.logContent.scrollHeight;
    }
  },

  pauseScroll() {
    this.autoScroll = false;
  },

  resumeScroll() {
    this.autoScroll = true;
  },
}));
```

### 3. Progress Indicator

Parse nixos-rebuild output for phases:

| Pattern                        | Phase      | Icon |
| ------------------------------ | ---------- | ---- |
| `evaluating derivation`        | Evaluation | 🔄   |
| `building '/nix/store/`        | Building   | 🔨   |
| `activating the configuration` | Activation | ⚡   |
| `switching to configuration`   | Switch     | 🔄   |

Display: `🔨 Building (15/42)`

---

## API Endpoints

### Download Log

```http
GET /api/hosts/{id}/logs/download?format=txt
GET /api/hosts/{id}/logs/download?format=json
```

---

## Acceptance Criteria

- [ ] Output lines stream from agent to dashboard to browser
- [ ] Logs stored in file system
- [ ] Log viewer panel in UI (collapsible)
- [ ] Auto-scroll with pause-on-hover
- [ ] Progress indicator (phase + count)
- [ ] Download log as text/JSON
- [ ] Rate limiting (batch lines, max 10/sec to browser)
- [ ] Works for all commands: switch, pull, test, update

---

## Related

- Depends on: P4200 (Dashboard Core with WebSocket hub)
- Part of: Core rewrite deliverables
