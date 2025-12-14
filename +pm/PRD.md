# NixFleet v2.0 - Product Requirements Document

> **Single Source of Truth** for NixFleet development.

---

## Vision

NixFleet is a fleet management system for NixOS and macOS hosts. It enables centralized monitoring, configuration deployment, and testing across a personal infrastructure.

**v2.0 Goal**: Complete rewrite in Go with real-time communication, solving the limitations of the v1.0 prototype.

---

## Problem Statement

### v1.0 Limitations (Python/Bash)

1. **Agent becomes unresponsive during switch** - Bash agent is single-threaded; can't heartbeat while executing commands
2. **No live output** - Users wait blindly during long operations
3. **Mixed technology stack** - Python dashboard + Bash agent = inconsistent patterns
4. **HTTP polling inefficiency** - Constant polling even when idle

### v2.0 Solutions

1. **Go agent with goroutines** - Concurrent heartbeats and command execution
2. **WebSocket streaming** - Real-time output from commands to browser
3. **All-Go stack** - Consistent language, single binaries, easier deployment
4. **WebSocket communication** - Persistent connections, instant updates

---

## User Stories

### US-1: Fleet Overview

> As a fleet operator, I want to see all my hosts at a glance, so I can quickly identify which need attention.

**Acceptance Criteria:**

- Dashboard shows all registered hosts
- Each host displays: name, status (online/offline/stale), last seen, current generation
- Hosts are sorted by criticality, then name
- Online status updates in real-time (no page refresh needed)

### US-2: Deploy Configuration

> As a fleet operator, I want to deploy configuration changes to a host, so the host runs the latest config.

**Acceptance Criteria:**

- Can trigger "Pull" to update git repository
- Can trigger "Switch" to apply configuration
- Can trigger "Pull + Switch" for convenience
- Command status visible in real-time
- Output streams to dashboard as it happens
- Final result (success/failure) clearly indicated

### US-3: Run Tests

> As a fleet operator, I want to run test suites on hosts, so I can verify they're configured correctly.

**Acceptance Criteria:**

- Can trigger "Test" command
- Progress shown (e.g., "3/10 tests complete")
- Results show pass/fail counts
- Failed test details available

### US-4: Monitor Host Health

> As a fleet operator, I want to see host health metrics, so I can identify resource issues.

**Acceptance Criteria:**

- CPU, RAM, swap usage visible (when StaSysMo available)
- Metrics update with each heartbeat
- Visual indicators for high usage

### US-5: Secure Access

> As a fleet operator, I want the dashboard to be secure, so only I can control my infrastructure.

**Acceptance Criteria:**

- Password authentication required
- Optional TOTP (2FA) support
- Sessions expire after 24 hours
- CSRF protection on all mutations
- Rate limiting on login attempts

### US-6: Agent Resilience

> As a fleet operator, I want agents to stay connected reliably, so I always have visibility and control.

**Acceptance Criteria:**

- Agent reconnects automatically after network issues
- Agent continues heartbeating during long commands
- Agent survives configuration switch (doesn't die mid-operation)
- macOS agent restarts reliably after home-manager switch

---

## Functional Requirements

### FR-1: Agent

| ID      | Requirement                                                 | Priority |
| ------- | ----------------------------------------------------------- | -------- |
| FR-1.1  | Connect to dashboard via WebSocket                          | Must     |
| FR-1.2  | Send heartbeat every 30s (configurable)                     | Must     |
| FR-1.3  | Continue heartbeats during command execution                | Must     |
| FR-1.4  | Execute commands: pull, switch, test, stop, restart, update | Must     |
| FR-1.5  | Stream command output to dashboard in real-time             | Must     |
| FR-1.6  | Report OS version, nixpkgs version, generation              | Must     |
| FR-1.7  | Report StaSysMo metrics (CPU, RAM, swap, load)              | Should   |
| FR-1.8  | Auto-reconnect with exponential backoff                     | Must     |
| FR-1.9  | Support isolated repo mode (agent-managed git clone)        | Must     |
| FR-1.10 | Support SSH key for git operations                          | Should   |
| FR-1.11 | Track command PID for stop capability                       | Must     |
| FR-1.12 | Work on NixOS (systemd) and macOS (launchd)                 | Must     |

### FR-2: Dashboard Backend

| ID      | Requirement                                     | Priority |
| ------- | ----------------------------------------------- | -------- |
| FR-2.1  | Accept WebSocket connections from agents        | Must     |
| FR-2.2  | Accept WebSocket connections from browsers      | Must     |
| FR-2.3  | Authenticate agents via token                   | Must     |
| FR-2.4  | Authenticate users via password + optional TOTP | Must     |
| FR-2.5  | Manage sessions with signed cookies             | Must     |
| FR-2.6  | Store host data in SQLite                       | Must     |
| FR-2.7  | Store command logs in files                     | Must     |
| FR-2.8  | Broadcast host updates to connected browsers    | Must     |
| FR-2.9  | Forward command output from agents to browsers  | Must     |
| FR-2.10 | Rate limit login attempts                       | Must     |
| FR-2.11 | CSRF protection on mutations                    | Must     |
| FR-2.12 | Security headers (HSTS, CSP, X-Frame-Options)   | Must     |

### FR-3: Dashboard Frontend

| ID     | Requirement                                      | Priority |
| ------ | ------------------------------------------------ | -------- |
| FR-3.1 | Display host list with status                    | Must     |
| FR-3.2 | Show real-time updates via WebSocket             | Must     |
| FR-3.3 | Action buttons: Pull, Switch, Test               | Must     |
| FR-3.4 | Show command output in expandable log viewer     | Must     |
| FR-3.5 | Show command progress (building X/Y)             | Should   |
| FR-3.6 | Disable buttons while command running            | Must     |
| FR-3.7 | Show host metrics (CPU, RAM)                     | Should   |
| FR-3.8 | Responsive design (mobile-friendly)              | Should   |
| FR-3.9 | Accessible (keyboard navigation, screen readers) | Should   |

---

## Non-Functional Requirements

### NFR-1: Performance

| ID      | Requirement               | Target  |
| ------- | ------------------------- | ------- |
| NFR-1.1 | Dashboard page load time  | < 500ms |
| NFR-1.2 | WebSocket message latency | < 100ms |
| NFR-1.3 | Agent memory usage        | < 20MB  |
| NFR-1.4 | Dashboard memory usage    | < 100MB |
| NFR-1.5 | Support concurrent hosts  | 50+     |

### NFR-2: Reliability

| ID      | Requirement                | Target                  |
| ------- | -------------------------- | ----------------------- |
| NFR-2.1 | Agent uptime               | 99.9% (when host is up) |
| NFR-2.2 | Dashboard uptime           | 99.9%                   |
| NFR-2.3 | Auto-recovery from crashes | < 30 seconds            |
| NFR-2.4 | Data persistence           | Survive restarts        |

### NFR-3: Security

| ID      | Requirement      | Target                   |
| ------- | ---------------- | ------------------------ |
| NFR-3.1 | Password storage | bcrypt hashed            |
| NFR-3.2 | Session tokens   | Cryptographically random |
| NFR-3.3 | Agent tokens     | Per-host, hashed in DB   |
| NFR-3.4 | TLS              | Required in production   |
| NFR-3.5 | Rate limiting    | 5 login attempts/minute  |

### NFR-4: Deployability

| ID      | Requirement          | Target                  |
| ------- | -------------------- | ----------------------- |
| NFR-4.1 | Dashboard deployment | Single Docker container |
| NFR-4.2 | Agent deployment     | Nix flake module        |
| NFR-4.3 | Configuration        | Environment variables   |
| NFR-4.4 | Upgrade path         | Zero-downtime           |

---

## Technical Architecture

### Technology Stack

| Component          | Technology                    |
| ------------------ | ----------------------------- |
| Agent              | Go 1.25+                      |
| Dashboard Backend  | Go 1.25+ with Chi router      |
| Dashboard Frontend | Templ + HTMX + Alpine.js      |
| Database           | SQLite                        |
| Communication      | WebSocket (gorilla/websocket) |
| Authentication     | bcrypt + TOTP (pquerna/otp)   |

### Component Diagram

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                          NixFleet Dashboard (Go)                         │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │   Router    │  │   Auth      │  │   WebSocket │  │   Templ      │   │
│  │   (Chi)     │  │   Middleware│  │   Hub       │  │   Templates  │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────────────┘   │
│                              │                                          │
│  ┌───────────────────────────┴───────────────────────────────────────┐ │
│  │                     SQLite + File Store                            │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │ WebSocket     │ WebSocket     │ WebSocket
              ▼               ▼               ▼
         Go Agent        Go Agent         Browser
         (NixOS)         (macOS)          (HTMX)
```

### Message Protocol

```json
// Agent → Dashboard
{"type": "register", "payload": {"hostname": "hsb0", ...}}
{"type": "heartbeat", "payload": {"metrics": {...}}}
{"type": "output", "payload": {"line": "building...", "command": "switch"}}
{"type": "status", "payload": {"status": "ok", "generation": "abc123"}}

// Dashboard → Agent
{"type": "command", "payload": {"command": "switch"}}
{"type": "ping"}

// Dashboard → Browser
{"type": "host_update", "payload": {"host_id": "hsb0", ...}}
{"type": "command_output", "payload": {"host_id": "hsb0", "line": "..."}}
```

---

## Success Metrics

| Metric                        | Target     | How to Measure        |
| ----------------------------- | ---------- | --------------------- |
| All integration tests pass    | 100%       | CI pipeline           |
| Agent heartbeat during switch | Continuous | Test with long switch |
| Output streaming latency      | < 1 second | Manual test           |
| Successful deployments        | 100%       | Deployment logs       |

---

## Out of Scope (v2.0)

- Multi-user support (single admin only)
- Role-based access control
- Scheduled commands
- Host grouping/tags
- Audit logging to external system
- High availability (single instance)

---

## References

- [Integration Test Specs](../tests/specs/) - Executable specifications
- [Backlog](./backlog/) - Implementation tasks (P-numbered)
- [v1.0 Codebase](../app/, ../agent/) - Reference implementation

---

## Changelog

| Date       | Version | Changes                      |
| ---------- | ------- | ---------------------------- |
| 2025-12-14 | 1.0     | Initial PRD for v2.0 rewrite |
