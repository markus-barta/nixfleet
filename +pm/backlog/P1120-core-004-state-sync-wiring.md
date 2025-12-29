# P1120: CORE-004 State Sync Wiring (Browser ↔ StateManager)

**Priority**: P1120 (High - Architecture Gap)  
**Type**: Refactor + Correctness  
**Status**: ✅ COMPLETE  
**Created**: 2025-12-29  
**Related**: `CORE-004`, `CORE-003`, `P1110`

---

## Summary

CORE-004 is now wired end-to-end for the dashboard UI: browser receives `init(full_state)` on connect, can request `get_state`, and the server emits `delta` updates for meaningful changes. The dashboard’s host row state is now drift-safe and no longer depends on legacy `host_heartbeat` / `host_status_update` messages.

This item wires the **actual CORE-004 protocol** end-to-end:

- Browser connects → receives `init(full_state)`
- Browser can send `get_state` → receives `full_state`
- Server can broadcast `delta` changes for hosts/commands/events
- Browser drift detection (`sync` beacon) works as designed

---

## Current Reality (2025-12-29)

- Dashboard creates `stateManager` and starts the beacon: `src/internal/dashboard/server.go`
- Browser WS registers with `StateManager`, routes `get_state`: `src/internal/dashboard/hub.go`
- `full_state` host objects include compartment data via `update_status`: `src/internal/dashboard/state_provider.go`
- Server emits `delta` for host state changes (online/offline, pending_command, update_status refresh, theme_color, add/remove): `src/internal/dashboard/hub.go`, `src/internal/dashboard/handlers.go`

---

## Acceptance Criteria

- ✅ Browser receives `init` with complete host objects (including `update_status` for the dots).
- ✅ Browser can request `get_state` and get `full_state`.
- ✅ Server emits `delta` on meaningful changes (host online/offline, heartbeat refreshes, pending_command, update_status refresh, add/remove, theme_color).
- ✅ UI uses CORE-004 for host state; legacy host message types are ignored in the browser.

---

## Notes

`P1110` depends on correct and complete compartment data; CORE-004 wiring is the cleanest long-term transport to keep the UI always-live and spec-aligned.

### Follow-ups (nice-to-have cleanup)

- Remove dead/legacy host WS message code paths once we’re confident in production (keep legacy for command_output/log streaming for now).
