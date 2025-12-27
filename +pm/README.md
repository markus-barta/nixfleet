# NixFleet Project Management

> Project documentation and task management.

---

## Hierarchy

```
PRD + Specs = Source of Truth (cite these in docs/code)
         ↓
    Backlog = Work Items (implement these, don't cite)
```

| Layer       | Purpose                        | Cite in docs/code? |
| ----------- | ------------------------------ | ------------------ |
| **PRD**     | Vision, goals, architecture    | ✅ Yes             |
| **Specs**   | Contracts, interfaces, schemas | ✅ Yes (preferred) |
| **Backlog** | Tasks to implement specs       | ❌ No              |

---

## Quick Links

| Document                   | Purpose                                |
| -------------------------- | -------------------------------------- |
| [PRD.md](./PRD.md)         | Product Requirements Document (v3.0)   |
| [spec/](./spec/)           | Core specifications (stable contracts) |
| [backlog/](./backlog/)     | Implementation tasks                   |
| [done/](./done/)           | Completed tasks                        |
| [cancelled/](./cancelled/) | Cancelled/deferred tasks               |
| [archive/](./archive/)     | Archived documents                     |

---

## v3.0 Roadmap

NixFleet v3.0 introduces the **Op Engine** architecture. See [PRD.md](./PRD.md) for details.

### Core Specs

| Spec                                             | Purpose                                |
| ------------------------------------------------ | -------------------------------------- |
| [CORE-001](./spec/CORE-001-op-engine.md)         | Op Engine (atomic operations)          |
| [CORE-002](./spec/CORE-002-pipeline-executor.md) | Pipeline Executor (multi-op sequences) |
| [CORE-003](./spec/CORE-003-state-store.md)       | State Store (persistence)              |
| [CORE-004](./spec/CORE-004-state-sync.md)        | State Sync (always-live UI)            |

### Implementation Phases

| Phase | Backlog Item                                        | Description             |
| ----- | --------------------------------------------------- | ----------------------- |
| 1     | [P3010](./backlog/P3010-op-engine-foundation.md)    | Op Engine Foundation    |
| 1     | [P3020](./backlog/P3020-pipeline-executor.md)       | Pipeline Executor       |
| 2     | [P3100](./backlog/P3100-state-persistence.md)       | State Persistence       |
| 3     | [P3200](./backlog/P3200-state-sync-protocol.md)     | State Sync Protocol     |
| 4     | [P3300](./backlog/P3300-logs-on-page-load.md)       | Logs on Page Load       |
| 5     | [P3400](./backlog/P3400-frontend-simplification.md) | Frontend Simplification |

---

## Backlog Categories

### v3 Core (P3xxx)

Critical architectural work for v3.0. Do these first.

### Features (P4xxx-P6xxx)

New capabilities. Most depend on v3 core.

### Polish (P7xxx-P8xxx)

UI improvements, accessibility, quality-of-life.

---

## Priority Scheme

| Range       | Category | Description                 |
| ----------- | -------- | --------------------------- |
| P1xxx       | Critical | Blocking bugs, urgent fixes |
| P2xxx       | High     | Important features          |
| P3xxx       | v3 Core  | v3.0 architectural work     |
| P4xxx-P5xxx | Medium   | Normal features             |
| P6xxx-P7xxx | Low      | Nice-to-have, polish        |
| P8xxx       | Future   | Deferred, long-term         |

---

## Archived Documents

| Document                                       | Description                       |
| ---------------------------------------------- | --------------------------------- |
| [PRD-v2.md](./archive/PRD-v2.md)               | v2.0 requirements (current impl)  |
| [ux-flow-cac-v2.md](../docs/ux-flow-cac-v2.md) | v2 Control-Action-Command mapping |

---

## Workflow

1. **Check spec** before implementing a feature
2. **Create/update backlog item** with clear acceptance criteria
3. **Implement** following spec interfaces
4. **Test** according to acceptance criteria
5. **Move to done/** when complete
6. **Update spec's backlog table** with completion status
