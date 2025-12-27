# P3600: Op Engine Lifecycle Completion

**Priority**: P3600 (Medium)  
**Status**: Backlog  
**Effort**: Medium (2-3 days)  
**Depends on**: P3500 (v3 Legacy Cleanup)

---

## Purpose

Currently we have **dual command tracking**:

- **Op Engine**: Dispatch, pre-validation, execution
- **CommandStateMachine**: Timeouts, post-checks, reconnect, kill

This creates confusion, maintenance burden, and risk of drift. The Op Engine should handle the **complete lifecycle**, then CommandStateMachine can be deleted.

---

## Current State (Bad)

```
User clicks "Pull"
       │
       ▼
┌─────────────────┐     ┌──────────────────────┐
│   Op Engine     │     │ CommandStateMachine  │
├─────────────────┤     ├──────────────────────┤
│ • Dispatch      │     │ • Timeout tracking   │
│ • Pre-validate  │     │ • Post-check defer   │
│ • Send to agent │     │ • Kill lifecycle     │
│ • Track status  │     │ • Reconnect handling │
└────────┬────────┘     └──────────┬───────────┘
         │                         │
         └─────── BOTH ────────────┘
                   │
         (Two sources of truth!)
```

---

## Target State (Good)

```
User clicks "Pull"
       │
       ▼
┌─────────────────────────────────────────────┐
│              Op Engine (Single)             │
├─────────────────────────────────────────────┤
│ • Dispatch                                  │
│ • Pre-validate                              │
│ • Send to agent                             │
│ • Timeout tracking (warning → hard → user)  │
│ • Post-check (with fresh heartbeat data)    │
│ • Kill/cancel as first-class ops            │
│ • Reconnect handling for switch             │
│ • Persisted in State Store                  │
└─────────────────────────────────────────────┘

CommandStateMachine: DELETED
```

---

## Tasks

### 1. Timeout Tracking in Op Engine

Move from `command_state.go`:

- [ ] Add `WarningAt`, `TimeoutAt` fields to `ops.Command`
- [ ] Add `TimeoutStatus` enum: `NORMAL`, `WARNING`, `TIMED_OUT`
- [ ] Create `TimeoutChecker` that runs periodically
- [ ] Emit timeout events to State Sync
- [ ] Frontend shows timeout UI from Op Engine state

**Files to change**:

- `src/internal/ops/op.go` - Add timeout fields
- `src/internal/ops/executor.go` - Start timeout tracking
- `src/internal/ops/timeout.go` - New file for timeout logic

### 2. Post-Check with Fresh Data

Move from `command_state.go`:

- [ ] Op Engine stores snapshot before execution
- [ ] Hub calls `opExecutor.HandleCommandComplete(hostID, opID, exitCode, host)`
- [ ] Post-check receives current host state (not stale)
- [ ] For agent ops, defer post-check until heartbeat arrives

**Key insight**: The fix for "pull completed but git still outdated" should live in Op Engine, not scattered across Hub and CommandStateMachine.

### 3. Kill/Cancel as First-Class Ops

- [ ] Register `kill` as an Op with SIGTERM → wait → SIGKILL logic
- [ ] `cancel` op for dashboard-side cancellation
- [ ] Kill targets a running command, not a host

**API**:

```json
POST /api/dispatch
{
  "op": "kill",
  "hosts": ["gpc0"],
  "options": { "signal": "SIGTERM" }
}
```

### 4. Reconnect Handling for Switch

Move from `command_state.go`:

- [ ] After `switch` success (exit 0), Op Engine enters `AWAITING_RECONNECT`
- [ ] Hub notifies Op Engine when agent reconnects
- [ ] Op Engine compares binary freshness (before vs after)
- [ ] Emit appropriate status: `SUCCESS`, `STALE_BINARY`, `SUSPICIOUS`

### 5. Delete CommandStateMachine

Once all functionality is migrated:

- [ ] Delete `src/internal/dashboard/command_state.go` (1353 lines)
- [ ] Remove `cmdStateMachine` from Server and Hub
- [ ] Remove `SetCommandStateMachine()` calls
- [ ] Update any remaining callers

---

## Acceptance Criteria

- [ ] `command_state.go` deleted
- [ ] All timeout UI works via Op Engine state
- [ ] Post-checks run with fresh heartbeat data
- [ ] Kill command works via `/api/dispatch`
- [ ] Switch reconnect verified via Op Engine
- [ ] Single source of truth: `ops.Command`
- [ ] All tests pass

---

## Migration Notes

### Gradual vs Big-Bang

Recommend **gradual migration**:

1. Add feature to Op Engine
2. Disable in CommandStateMachine
3. Verify works
4. Repeat for next feature
5. Delete CommandStateMachine when empty

### What to Keep from CommandStateMachine

The **logic** is good, just needs to move:

- Timeout thresholds per command type
- 3-layer binary freshness comparison
- Deferred post-check pattern
- Kill signal escalation

---

## Related

- **CORE-001**: Op Engine spec (needs update for lifecycle)
- **P3500**: v3 Legacy Cleanup (completed, but left dual tracking)
- **P2800**: Original CommandStateMachine implementation
