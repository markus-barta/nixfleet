# 2025-12-10 - NixFleet security improvements

**Created**: 2025-12-10  
**Priority**: High  
**Status**: Done  
**Depends On**: Current alpha

---

## Goal

Close remaining security gaps so NixFleet is safe for broader/internal exposure (beyond limited alpha).

---

## Scope

- Dashboard auth/session hardening
- Agent authentication and transport hardening
- Frontend CSP tightening
- Operational safeguards (rate limiting/backoff)

---

## Tasks

- [ ] Sign and verify session cookies (defense in depth on top of DB sessions); add rotation/expiry logic.
- [ ] Replace CSP `'unsafe-inline'` with nonces or hashes; update templates accordingly.
- [ ] Move from single shared agent token to per-host credentials or mTLS; add migration path.
- [ ] Agent TLS hardening: CA pinning/minimum TLS, and exponential backoff on auth failures.
- [ ] Add agent-side rate limiting/backoff for command execution/reporting to avoid overload when auth fails.
- [ ] Add monitoring/alerts for auth failures and rate-limit hits.
- [ ] **Proxy-aware rate limiting & logging**: behind Traefik/reverse-proxy, ensure client IP is derived from forwarded headers (otherwise rate limiting/logging keys collapse to the proxy IP).
- [ ] **Fix `/api/hosts` CSRF + validation**: `/api/hosts` currently calls `verify_csrf(request)` but there is no helper defined, which will 500 at runtime; also align manual host ID validation with the canonical host ID rules.
- [ ] **SSE robustness**: `broadcast_event()` iterates a shared set that can be mutated by connect/disconnect; make it safe (iterate over a snapshot / lock) and treat `QueueFull` as “drop event / mark slow” rather than “disconnect”.

---

## Acceptance Criteria

- Sessions use signed cookies; invalid signatures are rejected; rotation tested.
- CSP contains no `'unsafe-inline'`; pages render correctly with nonced/hashed assets.
- Agents authenticate with per-host identity (token or mTLS); shared token deprecated.
- Agent TLS pinning/min-TLS enforced; failed pin blocks communication and is logged.
- Agent loops back off on repeated failures; dashboard remains stable under auth/token mistakes.
- Basic alerts/metrics exist for auth failures and rate-limit triggers.
- Rate limiting behaves correctly behind reverse proxies (no “everyone shares one IP” effect).
- Manual “Add Host” endpoint works and is CSRF-protected (no 500s), and host IDs are consistent across UI + API.

