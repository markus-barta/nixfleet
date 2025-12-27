# NixFleet Specifications

> **Contractual obligations — the source of truth for NixFleet.**

Specs define **what we must build**. They are stable, citable contracts.

**When referencing NixFleet internals** (in docs, README, code comments), **cite specs** — not backlog items. Backlog items are transient work; specs are permanent contracts.

---

## Spec Index

| ID       | Name                                                 | Description                                        |
| -------- | ---------------------------------------------------- | -------------------------------------------------- |
| CORE-001 | [Op Engine](./CORE-001-op-engine.md)                 | Atomic operations: definition, registry, execution |
| CORE-002 | [Pipeline Executor](./CORE-002-pipeline-executor.md) | Multi-op sequences with && semantics               |
| CORE-003 | [State Store](./CORE-003-state-store.md)             | Unified persistence (SQLite schemas)               |
| CORE-004 | [State Sync](./CORE-004-state-sync.md)               | Always-live UI via version-based sync              |

---

## Hierarchy: PRD → Specs → Backlog

```
PRD + Specs = Source of Truth (cite these)
         ↓
    Backlog = Work Items (do these, don't cite)
```

| Layer       | Purpose                        | Cite in docs/code? | Changes           |
| ----------- | ------------------------------ | ------------------ | ----------------- |
| **PRD**     | Vision, goals, architecture    | ✅ Yes             | Per major version |
| **Specs**   | Contracts, interfaces, schemas | ✅ Yes (preferred) | Rarely            |
| **Backlog** | Tasks to implement specs       | ❌ No              | Frequently        |

---

## Naming Convention

```
CORE-NNN-short-name.md
```

- `CORE` = Core building block (stable, foundational)
- `NNN` = Three-digit number for ordering
- `short-name` = Descriptive, hyphenated

Future categories (if needed):

- `EXT-NNN` = Extensions (optional features)
- `INT-NNN` = Integrations (external systems)

---

## Spec Structure

Each spec contains:

1. **Purpose** — What problem it solves
2. **Definition** — Go structs, interfaces, schemas
3. **Behavior** — State machines, flows, rules
4. **Implementation Location** — Where code lives
5. **Implementing Backlog Items** — Tracking table
6. **Changelog** — History of changes

---

## How to Use

1. **Before implementing**: Read the relevant spec
2. **During implementation**: Follow the spec's interfaces
3. **When documenting**: Cite specs (e.g., "See CORE-001 for Op Engine")
4. **After implementing**: Update spec's backlog table (internal tracking)
5. **To change a spec**: Discuss first — specs are contracts

## When to Create a Spec

- **Yes**: Core building blocks, shared abstractions, stable interfaces
- **Maybe**: New features (decide case-by-case when touching specless code)
- **No**: One-off tasks, bug fixes, polish items
