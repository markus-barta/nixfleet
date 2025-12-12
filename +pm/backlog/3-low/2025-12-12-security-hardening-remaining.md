# Security Hardening - Remaining Items

**Created**: 2025-12-12  
**Priority**: Low  
**Status**: Backlog

---

## Overview

Most security hardening from the original security-improvements task is **DONE**. This tracks the remaining low-priority items.

---

## ✅ Completed Items (for reference)

| Item | Status | Notes |
|------|--------|-------|
| Session cookies with CSRF | ✅ Done | Sessions in DB, CSRF tokens verified |
| CSP without `unsafe-inline` | ✅ Done | Uses nonces for all scripts/styles |
| Per-host agent credentials | ✅ Done | `agent_token_hash` column, HMAC verification |
| Proxy-aware rate limiting | ✅ Done | Uses `X-Forwarded-For`, `X-Real-IP` |
| Fix `/api/hosts` CSRF | ✅ Done | `verify_csrf()` implemented and called |
| SSE broadcast robustness | ✅ Done | Uses lock + snapshot, QueueFull drops gracefully |

---

## Remaining Items

### 1. Agent TLS Hardening (Low Priority)

- [ ] CA pinning for agent → dashboard communication
- [ ] Minimum TLS version enforcement
- [ ] Agent logs/alerts on TLS handshake failures

**Notes**: Currently uses standard HTTPS. Extra hardening is nice-to-have for high-security environments.

### 2. Agent-Side Rate Limiting (Low Priority)

- [ ] Exponential backoff on repeated auth failures (already has some backoff)
- [ ] Rate limit command execution to avoid overload
- [ ] Circuit breaker pattern for dashboard unreachable

**Notes**: Basic backoff exists. Full implementation is low priority.

### 3. Monitoring/Alerts for Auth Failures (Low Priority)

- [ ] Set up actual alerting (currently just metrics)
- [ ] Dashboard widget showing auth failure trends
- [ ] Slack/webhook notifications for unusual activity

**Notes**: Metrics exist (`auth_failures`, `csrf_failures`, etc.). Need alerting integration.

---

## Acceptance Criteria

- [ ] Document current security posture in README
- [ ] Consider agent TLS hardening for future
- [ ] Consider alerting integration

---

## Consolidated From

- `2025-12-10-nixfleet-security-improvements.md` (original, most items done)
- `2025-12-12-fix-add-host-csrf-and-host-id-validation.md` (DONE)
- `2025-12-12-sse-broadcast-robustness.md` (DONE)

