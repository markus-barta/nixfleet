# External Audit Guidance

**Created**: 2025-12-12
**Priority**: Low
**Status**: Open (Awaiting Auditor)

---

## Purpose

This document provides guidance for an external auditor reviewing the NixFleet project. It explains what the system does, its intended scope, known limitations, and areas where feedback is requested.

---

## What is NixFleet?

NixFleet is a **personal fleet management dashboard** for managing NixOS and macOS hosts from a single web interface.

**Target use case**: A single administrator managing 10-15 personal/home-lab machines, not an enterprise deployment.

### Core Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Dashboard** | Python/FastAPI + Jinja2 | Web UI for viewing hosts and triggering commands |
| **Agent** | Bash script | Runs on each host, polls dashboard for commands |
| **NixOS Module** | Nix | Declarative agent configuration for NixOS hosts |
| **Home Manager Module** | Nix | Declarative agent configuration for macOS/Linux users |
| **Database** | SQLite | Stores host state, sessions, command logs |

### Architecture

```
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

        Poll interval: 30s (configurable 1-3600s)
        Commands: pull, switch, pull-switch, test
```

---

## What NixFleet CAN Do ✅

### Host Management

- [x] Register hosts automatically on first agent poll
- [x] Display all hosts in a web dashboard
- [x] Show online/offline status (based on last seen time)
- [x] Show current git hash (config version) per host
- [x] Detect "outdated" hosts (not on latest commit)
- [x] Display host metadata (location, device type, criticality)
- [x] Per-host theme colors for visual identification

### Commands

- [x] Trigger `git pull` on host's config repository
- [x] Trigger `nixos-rebuild switch` (NixOS) or `home-manager switch` (macOS)
- [x] Trigger combined `pull + switch` operation
- [x] Run host test suite (`hosts/<hostname>/tests/T*.sh`)
- [x] Stop/cancel pending commands

### Monitoring

- [x] Real-time dashboard updates via SSE
- [x] Live test progress (X/Y tests, pass/fail)
- [x] StaSysMo metrics integration (CPU/RAM/load if installed)
- [x] Command output logging

### Security

- [x] Password authentication (bcrypt hashed)
- [x] Optional TOTP 2FA
- [x] CSRF protection on all actions
- [x] Session management (SQLite-backed, 24h expiry)
- [x] API token authentication for agents (fail-closed)
- [x] Rate limiting (login, registration, polling)
- [x] Security headers (HSTS, CSP, X-Frame-Options)

### Deployment

- [x] Docker container (non-root user)
- [x] Traefik integration (TLS termination)
- [x] Health check endpoint
- [x] Nix flake for reproducible builds

---

## What NixFleet CANNOT Do ❌

### By Design (Out of Scope)

- ❌ **Multi-user/multi-tenant**: Single admin only
- ❌ **Role-based access control**: No user permissions system
- ❌ **Scheduled deployments**: Manual trigger only
- ❌ **Rollback automation**: Must SSH and run manually
- ❌ **Secret management**: Uses external tools (agenix)
- ❌ **Configuration editing**: Read-only; edit via git
- ❌ **Log aggregation**: Only shows last command output
- ❌ **Alerting/notifications**: No email/Slack/etc.

### Limitations

- ❌ **Push-based deployment**: Agents poll; no push capability
- ❌ **Concurrent commands**: One command per host at a time
- ❌ **Command queuing**: Only one pending command stored
- ❌ **Offline command storage**: Missed if agent not running
- ❌ **Windows support**: NixOS/macOS only

---

## Known Weaknesses ⚠️

### Security

| Issue | Severity | Status | Notes |
|-------|----------|--------|-------|
| Shared agent token | Medium | Open | All agents use same token; compromise affects all |
| CSP uses `'unsafe-inline'` | Low | Open | Templates need nonce/hash refactoring |
| Session cookies not signed | Low | Open | DB-backed sessions are secure, signing is defense-in-depth |
| No agent TLS pinning | Low | Open | Relies on system CA store |
| No rate limiting on agent commands | Medium | Open | Malicious agent could spam commands |

### Operational

| Issue | Severity | Notes |
|-------|----------|-------|
| SQLite single-writer | Low | Acceptable for 10-15 hosts |
| No backup automation | Medium | `/data` volume should be backed up |
| No metrics/observability | Low | Logs only, no Prometheus/Grafana |
| Manual version tracking | Low | Git hash embedded at build time |

### Code Quality

| Area | Rating | Notes |
|------|--------|-------|
| Test coverage | 5/10 | No automated Python tests; agent shellcheck only |
| Documentation | 8/10 | README good; needs API docs |
| Error handling | 7/10 | Basic; could be more user-friendly |
| Logging | 7/10 | Adequate; structured logging would help |

---

## Enhancement Wishlist 🚀

### Priority: High (Would Accept PRs)

1. **Per-host agent tokens** - Each host gets unique token
2. **NixOS VM tests** - Automated integration tests
3. **Python unit tests** - Backend coverage
4. **GitHub Actions CI** - Automated checks on PR

### Priority: Medium (Nice to Have)

5. **Nonce-based CSP** - Remove `'unsafe-inline'`
6. **Signed session cookies** - Defense in depth
7. **Agent backoff on failure** - Exponential retry
8. **CHANGELOG.md** - Semantic versioning
9. **Generated option docs** - Like nixpkgs

### Priority: Low (Future Consideration)

10. **WebSocket instead of SSE** - Bidirectional comms
11. **Command history per host** - View past operations
12. **Host groups/tags** - Organize by category
13. **Dry-run mode** - Preview what switch would do
14. **Notifications** - Webhook on command complete

---

## What "Good Enough" Means

For a **personal fleet of 10-15 machines**, NixFleet is considered adequate if:

- ✅ Can deploy config changes to all hosts from one place
- ✅ Can see which hosts are online and up-to-date
- ✅ Can run test suites remotely
- ✅ Authentication is reasonably secure (password + TOTP)
- ✅ Works reliably without constant maintenance

**Not required for personal use:**

- Enterprise audit logging
- Multi-user access control
- SLA/uptime guarantees
- 24/7 monitoring and alerting

---

## Audit Checklist

### Suggested Review Areas

1. **Security Review**
   - [ ] Review authentication flow (`app/main.py` lines 360-430)
   - [ ] Review agent token validation (`verify_api_token()`)
   - [ ] Review session management (SQLite-backed sessions)
   - [ ] Review CSRF implementation
   - [ ] Check for injection vulnerabilities (SQL, command)
   - [ ] Review rate limiting configuration

2. **Code Quality**
   - [ ] Review Nix module options (`modules/nixos.nix`, `modules/home-manager.nix`)
   - [ ] Review agent script for edge cases (`agent/nixfleet-agent.sh`)
   - [ ] Check error handling and logging
   - [ ] Verify input validation (Pydantic models)

3. **Architecture**
   - [ ] Evaluate polling vs push trade-offs
   - [ ] Review SQLite for concurrent access
   - [ ] Check Docker security (non-root, health check)
   - [ ] Evaluate secrets handling

4. **Documentation**
   - [ ] Is README sufficient for new users?
   - [ ] Are module options well-documented?
   - [ ] Is the API self-documenting?

### Files to Review

| File | Purpose | Priority |
|------|---------|----------|
| `app/main.py` | All backend logic | High |
| `agent/nixfleet-agent.sh` | Agent logic | High |
| `modules/nixos.nix` | NixOS module | Medium |
| `modules/home-manager.nix` | HM module | Medium |
| `flake.nix` | Nix packaging | Medium |
| `Dockerfile` | Container build | Low |

---

## Questions for the Auditor

1. **Security**: Is the shared agent token acceptable for personal use, or should per-host tokens be a blocker?

2. **Architecture**: Any concerns with the polling model for this scale?

3. **Code Quality**: What's the minimum test coverage you'd recommend?

4. **Nix Modules**: Are the option definitions idiomatic? Missing features?

5. **Overall**: What would you prioritize to improve this from "personal project" to "sharable open source"?

---

## Contact

- **Maintainer**: Markus Barta
- **Repository**: <https://github.com/markus-barta/nixfleet>
- **Related**: <https://github.com/markus-barta/nixcfg> (infrastructure using NixFleet)
