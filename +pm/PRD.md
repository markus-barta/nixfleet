# NixFleet v3.0 - Product Requirements Document

> **Single Source of Truth** for NixFleet development.
>
> For v2.0 PRD (current implementation), see [PRD-v2.md](./archive/PRD-v2.md).

---

## Vision

NixFleet is a fleet management system for NixOS and macOS hosts. It enables centralized monitoring, configuration deployment, and testing across a personal infrastructure.

**v3.0 Goal**: Architectural refactor introducing the **Op Engine** — a unified, testable, and resilient action system that replaces scattered command handling with well-defined operations and pipelines.

---

## Problem Statement

### v2.0 Limitations

| Area                      | Problem                                                                  | Impact                                      |
| ------------------------- | ------------------------------------------------------------------------ | ------------------------------------------- |
| **Action Duplication**    | Same logic repeated in row buttons, menu, context bar, bulk menu         | Hard to maintain, inconsistent behavior     |
| **State Fragmentation**   | State split between SQLite, Hub (memory), CommandState (memory), browser | Dashboard restart = lose in-flight commands |
| **No Command History**    | Commands are ephemeral, no audit trail                                   | Can't trace what happened                   |
| **No Recovery**           | Dashboard restart orphans running commands                               | Manual intervention needed                  |
| **Frontend Logic**        | Business logic in JavaScript                                             | Hard to test, duplicates backend            |
| **UI Staleness**          | WebSocket gaps cause stale UI                                            | User forced to CMD+R constantly             |
| **Inconsistent Patterns** | Some actions via WebSocket, some via REST                                | Confusing, error-prone                      |

### v3.0 Solutions

| Solution              | Benefit                                 | Spec                                             |
| --------------------- | --------------------------------------- | ------------------------------------------------ |
| **Op Engine**         | Single source of truth for all actions  | [CORE-001](./spec/CORE-001-op-engine.md)         |
| **Pipeline Executor** | Sequential ops with `&&` semantics      | [CORE-002](./spec/CORE-002-pipeline-executor.md) |
| **State Store**       | Unified persistence for commands, audit | [CORE-003](./spec/CORE-003-state-store.md)       |
| **State Sync**        | Always-live UI, no manual refresh       | [CORE-004](./spec/CORE-004-state-sync.md)        |
| **Thin Frontend**     | Dispatcher only, no business logic      | —                                                |
| **Event Bus**         | Consistent state propagation            | —                                                |

---

## Architecture

### Target State

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              BROWSER                                    │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │  Thin UI Layer                                                    │  │
│  │  - Renders server state                                           │  │
│  │  - Dispatches Ops/Pipelines                                       │  │
│  │  - No business logic                                              │  │
│  └───────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                              WebSocket (State Sync Protocol)
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         DASHBOARD (Go)                                  │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                        Op Engine                                 │   │
│  │  ┌────────────────┐  ┌───────────────────┐  ┌─────────────────┐  │   │
│  │  │  Op Registry   │  │ Pipeline Executor │  │  State Machine  │  │   │
│  │  │  [CORE-001]    │  │    [CORE-002]     │  │  (per-host)     │  │   │
│  │  └────────────────┘  └───────────────────┘  └─────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                │                                        │
│  ┌─────────────────────────────┼────────────────────────────────────┐   │
│  │                 State Store (SQLite) [CORE-003]                  │   │
│  │  ┌───────────────┐  ┌─────────────────┐  ┌────────────────────┐  │   │
│  │  │ Hosts         │  │ Commands        │  │ Event Log          │  │   │
│  │  └───────────────┘  └─────────────────┘  └────────────────────┘  │   │
│  │  ┌───────────────┐  ┌─────────────────┐                          │   │
│  │  │ Pipelines     │  │ LiveState (mem) │                          │   │
│  │  └───────────────┘  └─────────────────┘                          │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                │                                        │
│  ┌─────────────────────────────┼────────────────────────────────────┐   │
│  │              State Sync Protocol [CORE-004]                      │   │
│  │  - init → full state on connect                                  │   │
│  │  - delta → incremental updates                                   │   │
│  │  - sync → periodic version beacon                                │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                              WebSocket
                                    │
                                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│    Agent      │  │    Agent      │  │    Agent      │  │    Agent      │
│   (hsb0)      │  │   (hsb1)      │  │   (gpc0)      │  │   (imac0)     │
│               │  │               │  │               │  │               │
│ - Execute Ops │  │ - Execute Ops │  │ - Execute Ops │  │ - Execute Ops │
│ - Report back │  │ - Report back │  │ - Report back │  │ - Report back │
│ - Resilient   │  │ - Resilient   │  │ - Resilient   │  │ - Resilient   │
└───────────────┘  └───────────────┘  └───────────────┘  └───────────────┘
```

---

## Core Concepts (Summary)

> Detailed specifications in [+pm/spec/](./spec/).

### Op (Operation)

An **Op** is an atomic action on a single host. Defined once in the Op Registry, validated before execution, verified after. See [CORE-001](./spec/CORE-001-op-engine.md).

### Pipeline

A **Pipeline** is an ordered sequence of Ops with `&&` semantics. Hosts that fail are excluded from subsequent ops. See [CORE-002](./spec/CORE-002-pipeline-executor.md).

### State Store

All state persists in SQLite: hosts, commands, pipelines, event log. Enables recovery after dashboard restart. See [CORE-003](./spec/CORE-003-state-store.md).

### State Sync

Version-based WebSocket protocol ensures UI is always live. No manual refresh required. See [CORE-004](./spec/CORE-004-state-sync.md).

---

## Design Principles

| Principle                | Description                                       |
| ------------------------ | ------------------------------------------------- |
| **Single Definition**    | Each op defined once, used everywhere             |
| **Testable Ops**         | Ops can be tested without UI                      |
| **Persistent State**     | Commands survive dashboard restarts               |
| **Always-Live UI**       | No CMD+R required, ever                           |
| **Thin Frontend**        | UI dispatches ops, doesn't contain business logic |
| **Graceful Degradation** | Partial failures don't break the whole system     |
| **Audit Trail**          | All mutations logged                              |

---

## Implementation Phases

### Phase 1: Op Engine Foundation

- Create `v2/internal/ops/` package
- Define Op struct and registry
- Define Pipeline struct and executor
- Migrate existing commands to Ops
- Unit tests for validation/execution

**Effort**: 2-3 days

### Phase 2: State Persistence

- Add `commands`, `pipelines`, `event_log` tables
- Journal all op executions
- Implement startup recovery
- Add log rotation

**Effort**: 3-4 days

### Phase 3: Always-Live State Sync

- Add `state_version` to StateManager
- Implement init/delta/sync messages
- Client-side version tracking
- Auto-resync on drift

**Effort**: 2-3 days

### Phase 4: Logs on Page Load

- API: `GET /api/event-log`
- API: `GET /api/hosts/{id}/output`
- Event log in init payload
- Host output restoration

**Effort**: 1-2 days

### Phase 5: Frontend Simplification

- Remove business logic from JavaScript
- Single `dispatch(op, targets)` function
- Server renders complete state

**Effort**: 2-3 days

### Phase 6: Polish & Testing

- E2E tests for workflows
- Recovery stress tests
- Documentation update

**Effort**: 1-2 days

**Total**: ~12-18 days

---

## Success Criteria

| Metric                             | Target                    |
| ---------------------------------- | ------------------------- |
| All existing tests pass            | 100%                      |
| Ops defined in registry            | 100% (no ad-hoc commands) |
| Commands survive dashboard restart | Yes                       |
| Event log captures all mutations   | Yes                       |
| Logs persist across page refresh   | Yes                       |
| UI auto-resyncs after reconnect    | Yes (within 1s)           |
| UI catches drift via sync beacon   | Yes (within 30s)          |
| No manual CMD+R needed             | **Ever**                  |
| Frontend has no business logic     | Yes                       |

---

## Migration Path (v2 → v3)

v3 is a refactor, not a rewrite. Agents are unchanged.

| Phase | v2 Behavior             | v3 Behavior                           |
| ----- | ----------------------- | ------------------------------------- |
| 1     | Commands work as before | Ops defined, not yet used             |
| 2     | Commands work as before | Journaling added to existing commands |
| 3     | Full state on connect   | State sync protocol active            |
| 4     | Logs lost on refresh    | Logs persist                          |
| 5     | JS business logic       | Thin frontend                         |
| 6     | Complete v3             | Old code removed                      |

### Rollback Strategy

Each phase can be rolled back by reverting the commit. Agents continue working (no agent changes in v3).

---

## Out of Scope (v3.0)

- Multi-user support (single admin only)
- Role-based access control
- Scheduled commands
- Host grouping/tags
- High availability
- Security hardening (deferred to v3.1)

---

## References

- [Specifications](./spec/) — Core building block contracts
- [PRD-v2.md](./archive/PRD-v2.md) — v2.0 implementation spec
- [UX Flow v2](../docs/ux-flow-cac-v2.md) — v2 Control-Action-Command mapping (archived)
- [Backlog](./backlog/) — Implementation tasks

---

## Changelog

| Date       | Version   | Changes                      |
| ---------- | --------- | ---------------------------- |
| 2025-12-27 | 3.0-draft | Initial v3.0 PRD             |
|            |           | Extracted specs to +pm/spec/ |
|            |           | Added spec references        |
