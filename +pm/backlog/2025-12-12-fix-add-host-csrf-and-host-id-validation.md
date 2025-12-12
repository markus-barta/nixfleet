# 2025-12-12 - Fix Add Host CSRF + host ID validation

## Status: BACKLOG

## Description

The “Add Host” API endpoint (`POST /api/hosts`) currently calls `verify_csrf(request)` but there is **no such helper** in `app/main.py`, which will crash with a `NameError` (500) when the feature is used.

In addition, manual host creation currently allows hostnames/IDs that don’t match the canonical host ID rules used elsewhere (hyphen-only). This can create hosts that can’t be addressed by other endpoints.

## Scope

Applies to: **dashboard** (backend + UI)

## Acceptance Criteria

- [ ] `POST /api/hosts` succeeds when CSRF header is correct (and user is authenticated)
- [ ] `POST /api/hosts` fails with 403 when CSRF header is missing/invalid
- [ ] Manual host IDs follow the same rules as agent host IDs (`validate_host_id` / `HOST_ID_PATTERN`)
- [ ] UI input validation matches backend rules (same hostname/ID constraints)

## Test Plan

### Manual Test

1. Login to dashboard.
2. Use “Actions → Add Host”.
3. Submit a valid hostname.
4. Confirm host appears as “Manually added, awaiting agent connection”.
5. Repeat with an invalid hostname and confirm 400 with a clear message.
6. Use DevTools to remove `X-CSRF-Token` header and confirm request is rejected (403).

### Automated Test (optional)

```bash
# Pseudocode: once an API test harness exists
# - create session
# - POST /api/hosts with and without CSRF header
```

## Notes

- This is a correctness + security fix. It’s also a blocker for making “Add Host” usable.


