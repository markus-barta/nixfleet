## External Audit Guidance (NixFleet)

### Purpose

This document is guidance for an external auditor reviewing the **NixFleet** project. It explains what the system does, its intended scope, known limitations, and where auditor feedback is most valuable.

### What is NixFleet?

NixFleet is a **personal fleet management dashboard** for managing NixOS and macOS hosts from a single web interface.

- **Target use case**: a single administrator managing ~10–15 personal/home-lab machines (not an enterprise deployment).

### Core components

| Component               | Technology              | Purpose                                               |
| ----------------------- | ----------------------- | ----------------------------------------------------- |
| **Dashboard**           | Python/FastAPI + Jinja2 | Web UI for viewing hosts and triggering commands      |
| **Agent**               | Bash script             | Runs on each host, polls dashboard for commands       |
| **NixOS module**        | Nix                     | Declarative agent configuration for NixOS hosts       |
| **Home Manager module** | Nix                     | Declarative agent configuration for macOS/Linux users |
| **Database**            | SQLite                  | Stores host state, sessions, command logs             |

### Architecture (high level)

```text
┌─────────────────────────────────────────────────────────────────┐
│                     NixFleet Dashboard                          │
│                  (Docker container on csb1)                     │
│                   https://fleet.barta.cm                        │
│                                                                 │
│  Authentication: Password (bcrypt) + Optional TOTP (2FA)        │
│  Agent Auth: Bearer token (shared secret)                       │
│  Live Updates: Server-Sent Events (SSE)                         │
└─────────────────────────────────────────────────────────────────┘
                             │
             ┌───────────────┼───────────────┐
             │               │               │
             ▼               ▼               ▼
       ┌──────────┐    ┌──────────┐    ┌──────────┐
       │  NixOS   │    │  NixOS   │    │  macOS   │
       │  Agent   │    │  Agent   │    │  Agent   │
       │          │    │          │    │          │
       │ systemd  │    │ systemd  │    │ launchd  │
       │ service  │    │ service  │    │ agent    │
       └──────────┘    └──────────┘    └──────────┘

Poll interval: 30s (configurable 1–3600s)
Commands: pull, switch, pull-switch, test
```

---

## What NixFleet CAN Do

### Host management

- Register hosts automatically on first agent poll
- Display all hosts in a web dashboard
- Show online/offline status (based on last seen time)
- Show current git hash (config version) per host
- Detect “outdated” hosts (not on latest commit)
- Display host metadata (location, device type, criticality)
- Per-host theme colors for visual identification

### Commands

- Trigger `git pull` on host’s config repository
- Trigger `nixos-rebuild switch` (NixOS) or `home-manager switch` (macOS)
- Trigger combined `pull + switch` operation
- Run host test suite (`hosts/<hostname>/tests/T*.sh`)
- Stop/cancel pending commands

### Monitoring

- Real-time dashboard updates via SSE
- Live test progress (X/Y tests, pass/fail)
- StaSysMo metrics integration (CPU/RAM/load if installed)
- Command output logging

### Security

- Password authentication (bcrypt hashed)
- Optional TOTP 2FA
- CSRF protection on all actions
- Session management (SQLite-backed, 24h expiry)
- API token authentication for agents (fail-closed)
- Rate limiting (login, registration, polling)
- Security headers (HSTS, CSP, X-Frame-Options)

### Deployment

- Docker container (non-root user)
- Traefik integration (TLS termination)
- Health check endpoint
- Nix flake for reproducible builds

---

## What NixFleet CANNOT Do

### By design (out of scope)

- Multi-user / multi-tenant (single admin only)
- Role-based access control (no user permissions system)
- Scheduled deployments (manual trigger only)
- Rollback automation (must SSH and run manually)
- Secret management (uses external tools, e.g. agenix)
- Configuration editing in-app (read-only; edit via git)
- Log aggregation (only shows last command output)
- Alerting/notifications (no email/Slack/etc.)

### Current limitations

- Push-based deployment (agents poll; no push capability)
- Concurrent commands (one command per host at a time)
- Command queuing (only one pending command stored)
- Offline command storage (missed if agent isn’t running)
- Windows support (NixOS/macOS only)

---

## Known weaknesses / risk areas

### Security

| Issue                              | Severity | Notes                                                      |
| ---------------------------------- | -------- | ---------------------------------------------------------- |
| Shared agent token                 | Medium   | All agents use the same token; compromise affects all      |
| CSP uses `'unsafe-inline'`         | Low      | Templates need nonce/hash refactoring                      |
| Session cookies not signed         | Low      | DB-backed sessions are secure; signing is defense-in-depth |
| No agent TLS pinning               | Low      | Relies on system CA store                                  |
| No rate limiting on agent commands | Medium   | Malicious agent could spam commands                        |

### Operational

| Issue                    | Severity | Notes                              |
| ------------------------ | -------- | ---------------------------------- |
| SQLite single-writer     | Low      | Acceptable for 10–15 hosts         |
| No backup automation     | Medium   | `/data` volume should be backed up |
| No metrics/observability | Low      | Logs only; no Prometheus/Grafana   |
| Manual version tracking  | Low      | Git hash embedded at build time    |

### Code quality

| Area           | Rating | Notes                                            |
| -------------- | ------ | ------------------------------------------------ |
| Test coverage  | 5/10   | No automated Python tests; agent shellcheck only |
| Documentation  | 8/10   | README good; API docs could be improved          |
| Error handling | 7/10   | Basic; could be more user-friendly               |
| Logging        | 7/10   | Adequate; structured logging would help          |

---

## Enhancement wishlist (auditor input welcome)

### Priority: high

1. Per-host agent tokens (unique token per host)
2. NixOS VM tests (automated integration tests)
3. Python unit tests (backend coverage)
4. GitHub Actions CI (automated checks on PR)

### Priority: medium

5. Nonce-based CSP (remove `'unsafe-inline'`)
6. Signed session cookies (defense in depth)
7. Agent backoff on failure (exponential retry)
8. CHANGELOG (semantic versioning)
9. Generated option docs (like nixpkgs)

### Priority: low

10. WebSocket instead of SSE (bidirectional comms)
11. Command history per host (view past operations)
12. Host groups/tags (organize by category)
13. Dry-run mode (preview what switch would do)
14. Notifications (webhook on command complete)

---

## What “good enough” means (for the intended scope)

For a personal fleet of ~10–15 machines, NixFleet is “good enough” if it:

- Can deploy config changes to all hosts from one place
- Can show which hosts are online and up-to-date
- Can run test suites remotely
- Has reasonable authentication (password + optional TOTP)
- Works reliably without constant maintenance

Not required for personal use:

- Enterprise audit logging
- Multi-user access control
- SLA/uptime guarantees
- 24/7 monitoring and alerting

---

## Audit checklist

### Suggested review areas

1. Security review
   - Review authentication flow (`app/main.py`)
   - Review agent token validation (search for `verify_api_token` or equivalent)
   - Review session management (SQLite-backed sessions)
   - Review CSRF implementation
   - Check for injection vulnerabilities (SQL, command execution)
   - Review rate limiting configuration

2. Code quality
   - Review Nix module options (`modules/nixos.nix`, `modules/home-manager.nix`)
   - Review agent script edge cases (`agent/nixfleet-agent.sh`)
   - Check error handling and logging
   - Verify input validation (Pydantic models)

3. Architecture
   - Evaluate polling vs push trade-offs
   - Review SQLite for concurrent access assumptions
   - Check Docker security posture (non-root, health check, permissions)
   - Evaluate secrets handling boundaries (env vars, deployment docs)

4. Documentation
   - Is README sufficient for a new user?
   - Are module options well-documented?
   - Is the API self-documenting / discoverable?

### Files to review (suggested order)

| File                               | Purpose                               | Priority |
| ---------------------------------- | ------------------------------------- | -------- |
| `app/main.py`                      | Backend logic (routes, auth, SSE, DB) | High     |
| `agent/nixfleet-agent.sh`          | Agent logic (poll, execute, report)   | High     |
| `modules/nixos.nix`                | NixOS module                          | Medium   |
| `modules/home-manager.nix`         | Home Manager module                   | Medium   |
| `flake.nix`                        | Nix packaging/build                   | Medium   |
| `Dockerfile`                       | Container build                       | Low      |
| `docker-compose.yml` / `docker/**` | Deployment wiring                     | Low      |

---

## Questions for the auditor

1. Security: is the shared agent token acceptable for personal use, or should per-host tokens be a blocker?
2. Architecture: any concerns with the polling model at this scale?
3. Code quality: what minimum test coverage would you recommend?
4. Nix modules: are the option definitions idiomatic? any missing features?
5. Overall: what would you prioritize to improve this from “personal project” to “sharable open source”?

---

## Contact / references

- Maintainer: Markus Barta
- Repository: [github.com/markus-barta/nixfleet](https://github.com/markus-barta/nixfleet)
- Related infrastructure repo: [github.com/markus-barta/nixcfg](https://github.com/markus-barta/nixcfg)
