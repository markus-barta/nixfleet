# 2025-12-11 - Action button locking during commands

**Created**: 2025-12-11
**Priority**: Medium
**Status**: Backlog

---

## Goal

Disable action buttons (Pull, Switch, Test) while a command is in progress, re-enable when complete.

---

## Problem

The UI should prevent accidental command spam while a host is busy.

Current behavior (as of today):

- Buttons are disabled on click and re-enabled either:
  - when SSE later reports completion, or
  - **after a 30s fallback timeout** if SSE doesn’t update (too short for long rebuilds/tests).
- On **page refresh**, hosts with `pending_command` don’t render with disabled buttons, so you can click again.

## Solution

Track command execution state per-host and disable buttons appropriately:

1. When command queued → disable all buttons for that host
2. When SSE receives `command_queued` event → disable buttons
3. When SSE receives `host_update` with `pending_command: null` → re-enable buttons
4. Visual feedback: change button text to "Pulling...", "Switching...", "Testing..."
5. Safety fallback: if no completion is observed, auto-unlock after a **configurable max duration** (e.g. 24h)
6. Manual override: add “Unlock actions” entry in the per-host ellipsis menu (local-only, does not cancel host execution)

---

## Tasks

- [x] Disable action buttons in initial render when host has `pending_command` or `test_running`
- [x] In JS, treat "busy" as: `pending_command != null` OR `test_running == true`
- [ ] Replace the 30s fallback timeout with:
  - a long **max lock duration** (default 24h, configurable), and
  - unlock only on server completion (`host_update` / status), unless max duration expires.
- [ ] Persist per-host locks across reloads (localStorage) so refresh can't be used to spam.
- [x] Add "Unlock actions" in the per-host ellipsis dropdown (confirm dialog; local unlock only).
- [ ] Add visual feedback (button label changes, optional spinner)
- [ ] Edge cases:
  - page refresh while command running
  - SSE disconnect (should not unlock too early)
  - offline host (still disabled)

---

## References

- [app/templates/dashboard.html](../app/templates/dashboard.html)
