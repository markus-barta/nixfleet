# P3500: v3 Legacy Code Cleanup

**Priority**: P3500 (Medium - Post v3)  
**Status**: ✅ Completed  
**Effort**: Large (3-5 days)  
**Depends on**: v3 architecture stable + tested

---

## Purpose

Now that v3 Op Engine is in place, we have duplicate code paths:

- **Legacy**: `handleCommand`, `CommandStateMachine`, scattered validation
- **v3**: Op Engine, Pipeline Executor, `/api/dispatch`

This item consolidates and removes legacy code, migrating the frontend to v3 endpoints.

---

## Completed Work

### Phase 1: Frontend Migration ✅

- [x] `sendCommand()` function now uses `/api/dispatch` internally
- [x] All 20 frontend calls use the new Op Engine
- [x] CSRF handling added to state-sync.js

### Phase 2: Backend Consolidation ✅

- [x] `/api/hosts/{id}/command` delegates to Op Engine (adapter pattern)
- [x] Validation logic exists in both places (Op Engine is primary)
- [x] Hub notifies Op Engine on command completion
- [x] `opExecutorWrapper` bridges Hub → Op Engine

### Phase 3: Test Migration ✅

- [x] `t06_dashboard_commands_test.go` updated to use `/api/dispatch`
- [x] `t04_dashboard_auth_test.go` updated to use `/api/dispatch`
- [x] `t07_e2e_deploy_test.go` updated to use `/api/dispatch`
- [x] `log_streaming.go` updated to use `/api/dispatch`

---

## Remaining Work (Future Cleanup)

These are **optional** cleanup tasks for when the v3 architecture is proven stable:

### CommandStateMachine Slimdown (Deferred)

The `CommandStateMachine` (1353 lines) still handles:

- Timeout tracking
- Post-check deferral for fresh heartbeat data
- Agent reconnect handling after switch
- Kill command lifecycle

**Decision**: Keep for now. The Op Engine handles dispatch but CommandStateMachine handles lifecycle management. Full consolidation can happen when Op Engine gains these features.

### Code That Could Eventually Be Removed

| File                            | Lines | Status                          |
| ------------------------------- | ----- | ------------------------------- |
| `command_state.go` RunPreChecks | ~50   | Used by tests only              |
| `handlers.go` handleCommand     | ~50   | Adapter, delegates to Op Engine |
| Legacy validation funcs         | ~200  | Duplicated in ops/validation.go |

---

## What Was Achieved

1. **Unified Entry Point**: All commands now flow through Op Engine
2. **Frontend Consistency**: Single `sendCommand()` uses `/api/dispatch`
3. **Test Consistency**: All tests use v3 endpoints
4. **Clean Adapter Pattern**: Legacy endpoint preserved for compatibility
5. **Dual Tracking**: Both Op Engine and CommandStateMachine get notified

---

## Technical Notes

### Why Keep CommandStateMachine?

1. **Timeout Tracking**: Handles warning/hard timeouts with user actions
2. **Deferred Post-Checks**: Waits for fresh heartbeat before validation
3. **Switch Reconnect**: Handles agent restart after `nixos-rebuild switch`
4. **Kill Command**: Manages SIGTERM/SIGKILL lifecycle

### Migration Path Forward

If full consolidation is needed later:

1. Add timeout tracking to Op Engine
2. Move post-check deferral to Op Engine
3. Handle reconnect in Op Engine
4. Then delete CommandStateMachine

---

## Related

- **CORE-001**: Op Engine spec
- **CORE-002**: Pipeline Executor spec
- **P3400**: Frontend Simplification (overlaps)
