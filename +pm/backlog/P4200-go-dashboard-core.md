# Go Dashboard: Core

**Created**: 2025-12-14
**Priority**: P4200 (Critical)
**Status**: ✅ Code Complete (T04-T06 passing)
**Depends on**: P4000 (Go Agent Core)

---

## Tests to Pass

| Test                                                | Description                      |
| --------------------------------------------------- | -------------------------------- |
| [T04](../../tests/specs/T04-dashboard-auth.md)      | Dashboard Authentication         |
| [T05](../../tests/specs/T05-dashboard-websocket.md) | Dashboard WebSocket              |
| [T06](../../tests/specs/T06-dashboard-commands.md)  | Dashboard Commands               |
| [T07](../../tests/specs/T07-e2e-deploy-flow.md)     | E2E Deploy Flow (dashboard part) |
| [T08](../../tests/specs/T08-e2e-test-flow.md)       | E2E Test Flow (dashboard part)   |

---

## Overview

Rewrite the NixFleet dashboard in Go with Templ templates, HTMX interactivity, and unified WebSocket.

---

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                        NixFleet Dashboard (Go)                           │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │   Router    │  │   Auth      │  │   WebSocket │  │   Templ      │   │
│  │   (Chi)     │  │   Middleware│  │   Hub       │  │   Templates  │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────────────┘   │
│         │               │               │                  │            │
│  ┌──────┴───────────────┴───────────────┴──────────────────┴─────────┐ │
│  │                        Handlers                                    │ │
│  │  • Auth (login, logout, sessions)                                 │ │
│  │  • Hosts (CRUD, commands)                                         │ │
│  │  • WebSocket (agents + browsers)                                  │ │
│  │  • API (health, metrics)                                          │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                              │                                          │
│  ┌───────────────────────────┴───────────────────────────────────────┐ │
│  │                     Storage Layer                                  │ │
│  │  • SQLite (hosts, sessions, command_log)                          │ │
│  │  • File Store (command output logs)                               │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. Router (Chi)

```go
r := chi.NewRouter()

// Middleware
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Use(securityHeaders)
r.Use(rateLimiter)

// Public routes
r.Get("/login", h.LoginPage)
r.Post("/login", h.Login)
r.Get("/health", h.Health)

// Protected routes
r.Group(func(r chi.Router) {
    r.Use(requireAuth)
    r.Get("/", h.Dashboard)
    r.Get("/ws", h.WebSocket)
    r.Route("/api", func(r chi.Router) {
        r.Get("/hosts", h.ListHosts)
        r.Post("/hosts/{id}/command", h.QueueCommand)
        // ...
    })
})
```

### 2. Authentication

- **Password**: bcrypt hashed
- **TOTP**: Optional 2FA via `pquerna/otp`
- **Sessions**: SQLite-backed, signed cookies
- **CSRF**: Token per session, validated on mutations

```go
type Session struct {
    Token     string
    CSRFToken string
    ExpiresAt time.Time
    CreatedAt time.Time
}
```

### 3. WebSocket Hub

Single endpoint for agents AND browsers:

```go
type Hub struct {
    agents   map[string]*AgentConn    // host_id → connection
    browsers map[string]*BrowserConn  // session_id → connection
    mu       sync.RWMutex
}

func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Upgrade to WebSocket
    conn, _ := upgrader.Upgrade(w, r, nil)

    // Determine if agent or browser based on auth
    if agentToken := r.Header.Get("Authorization"); agentToken != "" {
        h.registerAgent(conn, hostID)
    } else if session := getSession(r); session != nil {
        h.registerBrowser(conn, session.Token)
    }
}
```

### 4. Message Protocol

```go
// Agent → Dashboard
type AgentMessage struct {
    Type    string `json:"type"`
    HostID  string `json:"host_id,omitempty"`
    Payload any    `json:"payload"`
}

// Types: register, heartbeat, output, status, test_progress

// Dashboard → Agent
type DashboardMessage struct {
    Type    string `json:"type"`
    Command string `json:"command,omitempty"`
}

// Types: command, ping

// Dashboard → Browser
type BrowserMessage struct {
    Type    string `json:"type"`
    HostID  string `json:"host_id,omitempty"`
    Payload any    `json:"payload"`
}

// Types: host_update, command_output, test_progress
```

### 5. Templ Templates

```go
// templates/dashboard.templ
package templates

templ Dashboard(hosts []Host, stats Stats) {
    <!DOCTYPE html>
    <html>
    <head>
        <title>NixFleet</title>
        <script src="https://unpkg.com/htmx.org@1.9.10"></script>
        <script src="https://unpkg.com/alpinejs@3.13.3"></script>
    </head>
    <body>
        @Header(stats)
        @HostTable(hosts)
        @Footer()
    </body>
    </html>
}

templ HostRow(host Host) {
    <tr id={ "host-" + host.ID } hx-swap-oob="true">
        <td>{ host.Hostname }</td>
        <td>@StatusBadge(host.Status, host.Online)</td>
        // ...
    </tr>
}
```

### 6. HTMX Interactivity

```html
<!-- Command button with HTMX -->
<button
  hx-post="/api/hosts/{id}/command"
  hx-vals='{"command": "switch"}'
  hx-swap="none"
  hx-indicator="#spinner-{id}"
>
  Switch
</button>
```

Updates via WebSocket push new `<tr>` elements with `hx-swap-oob="true"`.

---

## Database Schema

Same as current Python version (SQLite):

```sql
CREATE TABLE hosts (
    id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    host_type TEXT NOT NULL,
    -- ... (same columns)
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE command_log (
    id INTEGER PRIMARY KEY,
    host_id TEXT NOT NULL,
    command TEXT NOT NULL,
    status TEXT NOT NULL,
    output TEXT,
    created_at TEXT NOT NULL,
    completed_at TEXT
);
```

---

## Security

- **HSTS**: Strict-Transport-Security header
- **CSP**: Content-Security-Policy with nonces
- **X-Frame-Options**: DENY
- **Rate Limiting**: Per-IP using `go-chi/httprate`
- **Input Validation**: Struct tags + custom validators

---

## Configuration

Same env vars as Python version for easy migration:

| Variable                   | Required | Description                  |
| -------------------------- | -------- | ---------------------------- |
| `NIXFLEET_PASSWORD_HASH`   | Yes      | bcrypt hash                  |
| `NIXFLEET_SESSION_SECRETS` | Yes      | Comma-separated signing keys |
| `NIXFLEET_TOTP_SECRET`     | No       | Base32 TOTP secret           |
| `NIXFLEET_DATA_DIR`        | No       | Database directory           |
| `NIXFLEET_DEV_MODE`        | No       | Relaxed security for dev     |

---

## File Structure

```text
dashboard/
├── cmd/
│   └── nixfleet-dashboard/
│       └── main.go
├── internal/
│   ├── handlers/
│   │   ├── auth.go
│   │   ├── dashboard.go
│   │   ├── hosts.go
│   │   └── websocket.go
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── csrf.go
│   │   └── security.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── files.go
│   └── hub/
│       └── hub.go
├── templates/
│   ├── base.templ
│   ├── dashboard.templ
│   ├── login.templ
│   └── components/
│       ├── host_row.templ
│       └── status.templ
├── static/
│   ├── logo.png
│   └── styles.css
├── go.mod
└── go.sum
```

---

## Dependencies

```go
require (
    github.com/go-chi/chi/v5 v5.0.11
    github.com/go-chi/httprate v0.8.0
    github.com/gorilla/websocket v1.5.0
    github.com/a-h/templ v0.2.543
    github.com/mattn/go-sqlite3 v1.14.19
    github.com/pquerna/otp v1.4.0
    golang.org/x/crypto v0.17.0  // bcrypt
)
```

---

## Acceptance Criteria

- [ ] Login page with password + optional TOTP
- [ ] Session management (signed cookies)
- [ ] CSRF protection on all mutations
- [ ] Dashboard shows all hosts
- [ ] Host CRUD (add, delete, update)
- [ ] Command dispatch via WebSocket
- [ ] Live host updates to browsers
- [ ] Rate limiting on login, API
- [ ] Security headers (HSTS, CSP, etc.)
- [ ] Health endpoint (/health)
- [ ] Metrics endpoint (/api/metrics)
- [ ] Static file serving (logo, CSS)
- [ ] Responsive design (mobile-friendly)

---

## Related

- Depends on: P4000 (Go Agent for WebSocket protocol)
- Enables: P4300 (Live Log Streaming)
- Replaces: `app/main.py` (Python dashboard)
