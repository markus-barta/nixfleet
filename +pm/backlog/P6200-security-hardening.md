# Security Hardening

**Created**: 2025-12-14
**Priority**: P6200 (Low)
**Status**: Backlog
**Depends on**: P4000-P4400 (Core rewrite)

---

## Overview

Additional security hardening beyond baseline.

---

## Items

### Agent TLS Hardening

- [ ] CA pinning option
- [ ] Minimum TLS version enforcement
- [ ] Certificate validation logging

### Rate Limiting Enhancements

- [ ] Per-host rate limits
- [ ] Circuit breaker for dashboard unreachable
- [ ] Exponential backoff on auth failures

### Monitoring

- [ ] Prometheus metrics endpoint
- [ ] Alert on repeated auth failures
- [ ] Webhook notifications for anomalies

---

## Related

- Post-MVP hardening
- Current security is already solid for private use
