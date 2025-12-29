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

## Implementation Scope

v3 is implemented as a **single cohesive change** — all components built and validated together.

### Core Components (All Required)

| Component             | Spec                                             | Description                            |
| --------------------- | ------------------------------------------------ | -------------------------------------- |
| **Op Engine**         | [CORE-001](./spec/CORE-001-op-engine.md)         | Op struct, registry, executor          |
| **Pipeline Executor** | [CORE-002](./spec/CORE-002-pipeline-executor.md) | Multi-op sequences with `&&` semantics |
| **State Store**       | [CORE-003](./spec/CORE-003-state-store.md)       | SQLite persistence, recovery, audit    |
| **State Sync**        | [CORE-004](./spec/CORE-004-state-sync.md)        | Version-based WebSocket protocol       |
| **Thin Frontend**     | —                                                | Dispatcher only, no business logic     |

### Estimated Effort

| Area                     | Effort      |
| ------------------------ | ----------- |
| Op Engine + Pipeline     | 2-3 days    |
| State Store + Migrations | 2-3 days    |
| State Sync Protocol      | 2-3 days    |
| Frontend Simplification  | 2-3 days    |
| Integration + Testing    | 2-3 days    |
| **Total**                | ~10-15 days |

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

## Migration Strategy: Hard Cut

v3 is a **complete architectural replacement** — no parallel code, no gradual migration.

### Approach

| Principle                   | Description                                                       |
| --------------------------- | ----------------------------------------------------------------- |
| **Big Bang**                | All v3 components implemented and validated together before merge |
| **Delete, Don't Deprecate** | Old patterns removed as new ones land — no `*_legacy` files       |
| **Clean Slate**             | Each file either fully v3 or untouched — no hybrid code           |
| **Agents Unchanged**        | Dashboard-only refactor; agent protocol stays compatible          |

### What Gets Replaced

| v2 Code                              | Replaced By             | Notes                      |
| ------------------------------------ | ----------------------- | -------------------------- |
| `handleCommand()` scattered handlers | Op Registry + Executor  | Single source of truth     |
| `CommandStateMachine`                | Op Engine state machine | Formalized, spec-driven    |
| Ad-hoc SQLite queries                | State Store layer       | Unified persistence        |
| Current WebSocket messages           | State Sync Protocol     | Version-based, always-live |
| Frontend business logic              | Thin dispatcher         | Server-authoritative       |

### Rollback Strategy

Git revert of the v3 merge commit. Agents continue working (no agent changes in v3).

---

## Out of Scope (v3.0)

- Multi-user support (single admin only)
- Role-based access control
- Scheduled commands
- Host grouping/tags
- High availability
- Security hardening (deferred to v3.1)

---

## Release & Deployment Flow

NixFleet uses **fully automated releases** via GitHub Actions.

### Flow

```
1. Push code to nixfleet       →  Docker image builds automatically
2. Docker build succeeds       →  Triggers nixcfg to update flake.lock
3. Hosts show "Git outdated"   →  Pull + Switch updates agent
```

### Key Points

- **No manual tagging required** — every push triggers a build
- **No manual flake.lock updates** — automation handles it
- **Dashboard deploys instantly** — `just deploy` pulls latest image
- **Agents update via Pull + Switch** — standard fleet operation

### Compartment Status Dots (CORE-006)

NixFleet’s primary UX is the **row of compartments** (Agent/Git/Lock/System/Tests). The core requirement:

- **🟢 Green** means **current vs remote desired state** (not just “local looks fine”).
- **🟡 Yellow** means **action needed** (or verification degraded; e.g., System/Tests must not be green if remote desired state cannot be fetched).
- **🔴 Red** means a **real problem** (including inability to fetch remote desired state for Git/Lock).

Canonical spec: `+pm/spec/CORE-006-compartments.md`.

### Documentation

- [Release Guide](../docs/RELEASE.md) — Simple step-by-step guide
- [nixcfg Workflow Template](../docs/nixcfg-workflow-template.yml) — Copy to nixcfg

---

## References

- [Specifications](./spec/) — Core building block contracts
- [PRD-v2.md](./archive/PRD-v2.md) — v2.0 implementation spec
- [Backlog](./backlog/) — Implementation tasks
- [Release Guide](../docs/RELEASE.md) — How to release NixFleet

---

## Changelog

| Date       | Version | Changes                                       |
| ---------- | ------- | --------------------------------------------- |
| 2025-12-27 | 3.0.1   | Added Release & Deployment Flow section       |
|            |         | Added automated flake.lock update workflow    |
| 2025-12-27 | 3.0.0   | Initial v3.0 implementation                   |
|            |         | Op Engine, State Store, State Sync            |
|            |         | LifecycleManager replaces CommandStateMachine |
