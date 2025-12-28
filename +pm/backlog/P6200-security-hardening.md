# P6200 - Security Hardening

**Created**: 2025-12-14  
**Updated**: 2025-12-28  
**Priority**: P6200 (🟢 Medium Priority - Sprint 3)  
**Status**: Backlog  
**Depends on**: None (post-MVP)

**Note**: Priority raised - important for production deployment

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
