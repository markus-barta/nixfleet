# NixFleet - SSE Live Updates

**Created**: 2025-12-10
**Completed**: 2025-12-11
**Priority**: Medium
**Status**: ✅ Complete

---

## Goal

Replace polling-based UI updates with Server-Sent Events (SSE) for real-time dashboard updates, especially during test runs.

---

## What Was Done

Implemented SSE (Server-Sent Events) instead of WebSocket for simplicity:

- [x] Dashboard connects via SSE to receive real-time updates
- [x] Host status changes appear immediately (online/offline)
- [x] Test progress updates live
- [x] Git hash changes highlight immediately after pull
- [x] No need for manual page refresh
- [x] Works behind Traefik/Cloudflare proxy

---

## Implementation Details

### Server Side

- Added `/api/events` SSE endpoint
- Events broadcast on:
  - Host registration/status change
  - Command completion
  - Test progress updates
- Used FastAPI's `EventSourceResponse`

### Client Side

- JavaScript EventSource connection in dashboard template
- Reconnect logic on disconnect
- DOM updates on message receipt
- Visual indicator for connection status (green dot in header)

### Agent Side

- Publishes test progress after each test file
- Uses existing status reporting endpoint

---

## Why SSE Instead of WebSocket?

1. **Simpler** - One-way communication is all we need
2. **HTTP-based** - Works through all proxies without special config
3. **Auto-reconnect** - Built into EventSource API
4. **Sufficient** - Dashboard only needs to receive updates, not send them

---

## Acceptance Criteria

- [x] SSE connection established on dashboard load
- [x] Host status updates appear within 1s
- [x] Test progress shows live during execution
- [x] Connection auto-reconnects on disconnect
- [x] Works behind Traefik/Cloudflare proxy

---

## References

- FastAPI EventSourceResponse docs
- [app/main.py](../../app/main.py) - SSE endpoint implementation
- [app/templates/dashboard.html](../../app/templates/dashboard.html) - Client-side SSE handling

