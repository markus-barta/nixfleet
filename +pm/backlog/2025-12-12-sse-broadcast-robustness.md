# 2025-12-12 - SSE broadcast robustness

## Status: BACKLOG

## Description

The backend keeps a global `sse_clients` set and broadcasts events by iterating it.

Two pragmatic reliability issues:

1. `broadcast_event()` iterates `sse_clients` directly while other coroutines may add/remove clients (connect/disconnect), which can raise `RuntimeError: Set changed size during iteration`.
2. `QueueFull` currently causes the server to drop the client entirely, even though it might simply be a temporarily slow browser tab.

For a hobby project, the goal is: **no crashes, bounded memory, and graceful degradation**.

## Scope

Applies to: **dashboard backend** (`app/main.py`) + indirectly the dashboard UI (SSE).

## Acceptance Criteria

- [ ] Broadcasting never crashes due to concurrent connect/disconnect
- [ ] Slow clients do not crash the server; events can be dropped without disconnecting everything
- [ ] SSE memory remains bounded per client (already uses bounded queue)

## Test Plan

### Manual Test

1. Open dashboard in multiple tabs.
2. Trigger frequent SSE events (queue commands, run tests).
3. Rapidly open/close tabs while actions are happening.
4. Confirm no server errors and remaining tabs still update.

### Automated Test (optional)

```bash
# Future: unit-test broadcast_event with concurrent add/remove using asyncio tasks
```


