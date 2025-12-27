# Project Management

Central hub for tracking work on the NixFleet dashboard.

---

## Product Requirements Documents

| Document                 | Version | Description                                 |
| ------------------------ | ------- | ------------------------------------------- |
| [PRD.md](./PRD.md)       | v3.0    | **Current vision** — Op Engine architecture |
| [PRD-v2.md](./PRD-v2.md) | v2.0    | Current implementation — Go rewrite         |

---

## Priority System

Tasks use a **P-number** prefix for ordering:

```
P{number}-{name}.md
```

**Lower number = Higher priority**

| Range       | Priority    | Description                             |
| ----------- | ----------- | --------------------------------------- |
| P0000-P1999 | 🔴 Critical | Blocking bugs, system broken, fix now   |
| P2000-P3999 | 🟠 High     | Important bugs/issues, fix soon         |
| P4000-P5999 | 🟡 Medium   | Features and improvements, planned work |
| P6000-P7999 | 🟢 Low      | Nice-to-have, do when time permits      |
| P8000-P9999 | ⚪ Backlog  | Ideas, future enhancements, someday     |

### Ordering Within Priority

- Start at **X000** (e.g., P4000, P5000, P6000)
- New items: add/subtract 100 (P4100, P4200...)
- Insert between: use finer granularity (P4050 between P4000 and P4100)
- **Goal**: Never need to rename existing files when priorities change

### Example

```
backlog/
  P1000-reliable-agent-updates.md      # Critical: blocking bug
  P1100-macos-agent-update-bug.md      # Critical: related analysis
  P2000-unified-host-state.md          # High: architecture work
  P4000-new-feature.md                 # Medium: planned feature
  P6000-heartbeat-visualizer.md        # Low: nice-to-have
  P8000-future-idea.md                 # Backlog: someday/maybe
```

---

## Workflow

```
┌──────────┐                      ┌──────┐
│ Backlog  │─────────────────────▶│ Done │
└──────────┘                      └──────┘
      │
      ▼
┌───────────┐
│ Cancelled │
└───────────┘
```

| State         | Folder       | Description                                               |
| ------------- | ------------ | --------------------------------------------------------- |
| **Backlog**   | `backlog/`   | All tasks: ideas, planned work, in-progress items         |
| **Done**      | `done/`      | Verified complete, kept indefinitely as historical record |
| **Cancelled** | `cancelled/` | No longer relevant/needed, kept for reference             |

### Moving Tasks

- **Backlog → Done**: Task complete, verified working
- **Backlog → Cancelled**: No longer needed, add note explaining why

---

## When to Create a Task

| Situation                        | Create +pm task?                 |
| -------------------------------- | -------------------------------- |
| Quick fix, single file, <15 min  | ❌ No, just do it                |
| Change affects multiple files    | ✅ Yes                           |
| Change takes >30 min             | ✅ Yes                           |
| New feature or capability        | ✅ Yes                           |
| Refactoring or migration         | ✅ Yes                           |
| Bug fix with root cause analysis | ✅ Yes                           |
| Documentation-only change        | ❌ No (unless major restructure) |

**Rule of thumb**: If you need to track progress or might get interrupted, create a task.

---

## File Naming Convention

```
P{number}-{short-description}.md
```

Examples:

- `P4000-agent-resilience-detached-switch.md`
- `P5100-version-generation-tracking.md`

**Note**: Date is tracked inside the file, not in the filename.

---

## Task Template

````markdown
# Task Title

**Created**: YYYY-MM-DD  
**Priority**: P{number} (Critical/Medium/Low)  
**Status**: Backlog  
**Depends on**: P{other} (optional)

---

## Problem

Brief explanation of what needs to be fixed or built.

---

## Solution

How we're going to solve it.

---

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

---

## Test Plan

### Manual Test

1. Step 1 to verify
2. Step 2 to verify

### Automated Test

```bash
# Commands to verify
```
````

---

## Related

- Depends on: P{number}
- Enables: P{number}

```

---

## Test Requirements

Every task should have tests defined:

| Test Type | Description | Required |
|-----------|-------------|----------|
| **Manual Test** | Human verification steps documented in the task | ✅ Yes |
| **Automated Test** | Script or curl commands that verify the change | Recommended |

### Testing Approaches

| Test Type | How to Test |
|-----------|-------------|
| **API** | `curl` commands, check responses |
| **Dashboard** | Browser test, verify UI elements |
| **Agent** | Deploy to test host, check registration |
| **Docker** | `docker compose build && docker compose up -d` |
| **Nix Modules** | `nix flake check` (on a NixOS machine) |

---

## Related

- [Main README](../README.md) - Project overview
- [nixcfg](https://github.com/markus-barta/nixcfg) - Parent infrastructure repository
```
