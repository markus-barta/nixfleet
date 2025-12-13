# Project Management

Central hub for tracking work on the NixFleet dashboard.

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

Files are date-prefixed: `YYYY-MM-DD-short-description.md`

Example: `2025-12-10-add-host-grouping.md`

---

## Task Template

````markdown
# YYYY-MM-DD - Task Title

## Status: BACKLOG | DONE | CANCELLED

## Description

Brief explanation of what needs to be done.

## Scope

Applies to: [dashboard / agent / modules / docker]

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Test Plan

### Manual Test

1. Step 1 to verify
2. Step 2 to verify
3. Expected result

### Automated Test

```bash
# Reference to test script or inline commands
docker compose up -d
curl http://localhost:8000/health
```
````

## Notes

- Relevant context, links, or references

```

---

## Test Requirements

Every task should have tests defined:

| Test Type          | Description                                     | Required    |
| ------------------ | ----------------------------------------------- | ----------- |
| **Manual Test**    | Human verification steps documented in the task | ✅ Yes      |
| **Automated Test** | Script or curl commands that verify the change  | Recommended |

### Testing Approaches

| Test Type      | How to Test                                             |
| -------------- | ------------------------------------------------------- |
| **API**        | `curl` commands, check responses                        |
| **Dashboard**  | Browser test, verify UI elements                        |
| **Agent**      | Deploy to test host, check registration                 |
| **Docker**     | `docker compose build && docker compose up -d`          |
| **Nix Modules** | `nix flake check` (on a NixOS machine)                 |

---

## Related

- [Main README](../README.md) - Project overview
- [nixcfg](https://github.com/markus-barta/nixcfg) - Parent infrastructure repository

```
