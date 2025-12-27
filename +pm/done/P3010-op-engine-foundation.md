# P3010: Op Engine Foundation

**Priority**: P3010 (Critical - v3 Phase 1)  
**Status**: ✅ Done  
**Effort**: Medium (2-3 days)  
**Implements**: [CORE-001](../spec/CORE-001-op-engine.md)

---

## User Story

**As a** developer  
**I want** a unified Op Engine with a registry of all operations  
**So that** every action is defined once, testable, and consistently executed

---

## Scope

### Create Package Structure

```
src/internal/ops/
├── op.go           # Op struct and interfaces
├── registry.go     # Op registration and lookup
├── executor.go     # Execution engine
├── validation.go   # Pre/post check helpers
├── pipeline.go     # Pipeline struct and executor
└── host_adapter.go # Adapter for templates.Host
```

### Define Core Types

```go
type Op struct {
    ID        string
    Validate  func(host *Host) *ValidationError
    Execute   func(ctx context.Context, host *Host) error
    PostCheck func(host *Host) *ValidationError
    Timeout   time.Duration
    Retryable bool
    Executor  OpExecutor  // "agent" or "dashboard"
}

type OpRegistry struct {
    ops map[string]*Op
}

func (r *OpRegistry) Register(op *Op) error
func (r *OpRegistry) Get(id string) (*Op, error)
func (r *OpRegistry) List() []*Op
```

### Migrate Existing Commands

Wrap existing command handlers as Ops:

| Current Handler      | → Op ID         |
| -------------------- | --------------- |
| `handlePull`         | `pull`          |
| `handleSwitch`       | `switch`        |
| `handleTest`         | `test`          |
| `handleRestart`      | `restart`       |
| `handleStop`         | `stop`          |
| `handleReboot`       | `reboot`        |
| `handleCheckVersion` | `check-version` |

### Unit Tests

- Registry registration/lookup
- Validation error handling
- Timeout enforcement
- Op execution flow

---

## Acceptance Criteria

- [x] `src/internal/ops/` package exists
- [x] Op struct matches CORE-001 spec
- [x] All existing commands wrapped as Ops (16 ops registered)
- [x] Registry registers all ops at startup
- [x] New `/api/dispatch` endpoint uses Op Engine
- [x] New `/api/dispatch/pipeline` endpoint uses Pipeline Executor

---

## Notes

Phase 1 runs both systems in parallel. Old code calls new ops internally. No breaking changes.

---

## Related

- **CORE-001**: Op Engine spec
- **P3020**: Pipeline Executor (Phase 1 continuation)
