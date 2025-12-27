# P3010: Op Engine Foundation

**Priority**: P3010 (Critical - v3 Phase 1)  
**Status**: Backlog  
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
v2/internal/ops/
├── op.go           # Op struct and interfaces
├── registry.go     # Op registration and lookup
├── executor.go     # Execution engine
├── validation.go   # Pre/post check helpers
└── ops/
    ├── pull.go
    ├── switch.go
    ├── test.go
    ├── restart.go
    ├── stop.go
    ├── reboot.go
    ├── check_version.go
    └── ...
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

- [ ] `v2/internal/ops/` package exists
- [ ] Op struct matches CORE-001 spec
- [ ] All existing commands wrapped as Ops
- [ ] Registry registers all ops at startup
- [ ] Unit tests for Op validation and execution
- [ ] Old handlers delegate to new Op engine (parallel running)

---

## Notes

Phase 1 runs both systems in parallel. Old code calls new ops internally. No breaking changes.

---

## Related

- **CORE-001**: Op Engine spec
- **P3020**: Pipeline Executor (Phase 1 continuation)
