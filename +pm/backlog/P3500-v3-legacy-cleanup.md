# P3500: v3 Legacy Code Cleanup

**Priority**: P3500 (Medium - Post v3)  
**Status**: Backlog  
**Effort**: Large (3-5 days)  
**Depends on**: v3 architecture stable + tested

---

## Purpose

Now that v3 Op Engine is in place, we have duplicate code paths:

- **Legacy**: `handleCommand`, `CommandStateMachine`, scattered validation
- **v3**: Op Engine, Pipeline Executor, `/api/dispatch`

This item consolidates and removes legacy code, migrating the frontend to v3 endpoints.

---

## Inventory of Legacy Code

### Dashboard Backend (~3000 lines)

| File                          | Lines | Purpose                              | v3 Replacement                         |
| ----------------------------- | ----- | ------------------------------------ | -------------------------------------- |
| `command_state.go`            | 1353  | Command lifecycle, pre/post checks   | `ops/executor.go`, `ops/validation.go` |
| `handlers.go` (command parts) | ~500  | `handleCommand`, validation, routing | `handlers_ops.go`                      |

### Frontend (~30 calls)

| Pattern                    | Count | v3 Replacement           |
| -------------------------- | ----- | ------------------------ |
| `sendCommand(hostId, cmd)` | 30    | `stateSync.dispatchOp()` |
| `/api/hosts/{id}/command`  | 30    | `/api/dispatch`          |

### Tests

| File                                | Status               |
| ----------------------------------- | -------------------- |
| `t06_dashboard_commands_test.go`    | Uses legacy endpoint |
| `t13_command_state_machine_test.go` | Tests legacy code    |

---

## Migration Tasks

### Phase 1: Frontend Migration

- [ ] Replace all `sendCommand()` calls with `stateSync.dispatchOp()`
- [ ] Update button handlers to use new API
- [ ] Remove legacy `sendCommand` function
- [ ] Update CSRF handling (already done in state-sync.js)

### Phase 2: Backend Consolidation

- [ ] Route `/api/hosts/{id}/command` to Op Engine (adapter)
- [ ] Move validation logic from `command_state.go` → `ops/validation.go`
- [ ] Move timeout tracking to Op Engine
- [ ] Consolidate `CommandState` with `ops.Command`

### Phase 3: Code Removal

- [ ] Delete most of `command_state.go` (keep what's still needed)
- [ ] Remove duplicate handlers from `handlers.go`
- [ ] Update tests to use v3 endpoints

### Phase 4: Test Migration

- [ ] Update `t06_dashboard_commands_test.go` to use `/api/dispatch`
- [ ] Move relevant tests to Op Engine unit tests
- [ ] Delete `t13_command_state_machine_test.go` (replaced)

---

## Technical Decisions

### What to Keep from CommandStateMachine

- **Timeout tracking**: Useful, migrate to Op Engine
- **Pre/post validation functions**: Already migrated to `ops/validation.go`
- **Reconnect handling for switch**: Keep until Op Engine handles it

### What to Delete

- State machine itself (Ops have cleaner lifecycle)
- Duplicate validation logic
- Legacy snapshot capture
- Browser-specific state tracking (server is source of truth)

---

## Acceptance Criteria

- [ ] All UI actions use `/api/dispatch`
- [ ] `command_state.go` < 200 lines (or deleted)
- [ ] `handlers.go` command handling removed
- [ ] All tests pass with v3 endpoints
- [ ] No `sendCommand()` in frontend
- [ ] Dashboard version stays at 3.x

---

## Risk Assessment

| Risk                              | Mitigation                           |
| --------------------------------- | ------------------------------------ |
| Breaking changes during migration | Feature flag for v3 endpoints        |
| Lost functionality                | Audit each function before deletion  |
| Test coverage gaps                | Run full integration before deletion |

---

## Related

- **CORE-001**: Op Engine spec
- **CORE-002**: Pipeline Executor spec
- **P3400**: Frontend Simplification (overlaps)
