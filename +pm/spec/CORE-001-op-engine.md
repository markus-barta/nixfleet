# CORE-001: Op Engine

> **Spec Type**: Core Building Block  
> **Status**: Stable  
> **Last Updated**: 2025-12-27

---

## Purpose

The **Op Engine** is the single source of truth for all executable actions in NixFleet. Every user action, scheduled task, or automated workflow ultimately executes one or more Ops.

---

## Op Definition

An **Op** (Operation) is an atomic action on a single host.

```go
type Op struct {
    ID        string                                  // Unique identifier: "pull", "switch", "test"
    Validate  func(host *Host) *ValidationError      // Pre-check before execution
    Execute   func(ctx context.Context, host *Host) error
    PostCheck func(host *Host) *ValidationError      // Post-check after execution
    Timeout   time.Duration                          // Max execution time
    Retryable bool                                   // Safe to retry on failure
    Executor  OpExecutor                             // "agent" or "dashboard"
}

type OpExecutor string

const (
    ExecutorAgent     OpExecutor = "agent"     // Runs on the host via agent
    ExecutorDashboard OpExecutor = "dashboard" // Runs on dashboard server
)
```

### Properties

| Property    | Description                                             |
| ----------- | ------------------------------------------------------- |
| `ID`        | Unique string identifier, lowercase, hyphen-separated   |
| `Validate`  | Returns nil if op can proceed, error with reason if not |
| `Execute`   | Performs the action, returns error on failure           |
| `PostCheck` | Verifies success after execution (optional)             |
| `Timeout`   | Context deadline; exceeded = `TIMEOUT` status           |
| `Retryable` | If true, UI may offer "Retry" on failure                |
| `Executor`  | Where the op runs (agent vs dashboard)                  |

---

## Op Registry

All Ops are registered at dashboard startup. No ad-hoc commands.

### Host Ops (Agent-Executed)

| Op ID            | Description                  | Pre-Check            | Post-Check               | Timeout | Retryable |
| ---------------- | ---------------------------- | -------------------- | ------------------------ | ------- | --------- |
| `pull`           | Git fetch + reset            | Online, no pending   | Lock = ok                | 2 min   | ✅        |
| `switch`         | nixos-rebuild/hm switch      | Lock = ok (or force) | System = ok, agent fresh | 10 min  | ✅        |
| `test`           | Run host tests               | Online               | Test results             | 5 min   | ✅        |
| `restart`        | Restart agent service        | Online               | Agent reconnects         | 1 min   | ⚠️        |
| `stop`           | Stop running command         | Has pending cmd      | No pending cmd           | 30 sec  | ❌        |
| `reboot`         | System reboot                | TOTP verified        | Agent reconnects         | 5 min   | ⚠️        |
| `check-version`  | Compare running vs installed | Online               | —                        | 10 sec  | ✅        |
| `refresh-git`    | Check GitHub for updates     | Online               | —                        | 30 sec  | ✅        |
| `refresh-lock`   | Compare flake.lock           | Online               | —                        | 30 sec  | ✅        |
| `refresh-system` | nix build --dry-run          | Online, confirmed    | —                        | 5 min   | ✅        |
| `bump-flake`     | nix flake update nixfleet    | Online               | Lock changed             | 2 min   | ✅        |
| `force-rebuild`  | Rebuild with cache bypass    | Online               | Agent fresh              | 15 min  | ✅        |

### Dashboard Ops (Server-Side)

| Op ID       | Description         | Pre-Check    | Post-Check | Timeout | Retryable |
| ----------- | ------------------- | ------------ | ---------- | ------- | --------- |
| `merge-pr`  | Merge GitHub PR     | PR mergeable | PR merged  | 30 sec  | ✅        |
| `set-color` | Update theme color  | —            | —          | instant | ✅        |
| `remove`    | Remove host from DB | —            | —          | instant | ❌        |

---

## Op Lifecycle

```
┌────────────┐     ┌────────────┐     ┌────────────┐     ┌────────────┐
│  PENDING   │────▶│ VALIDATING │────▶│ EXECUTING  │────▶│  SUCCESS   │
└────────────┘     └─────┬──────┘     └─────┬──────┘     └────────────┘
                         │                  │
                   validation               │            ┌────────────┐
                    failed                  └───────────▶│   ERROR    │
                         │                  │            └────────────┘
                         ▼                  │
                   ┌──────────┐             │            ┌────────────┐
                   │ BLOCKED  │             └───────────▶│  TIMEOUT   │
                   └──────────┘                          └────────────┘
```

### Status Definitions

| Status       | Description                                |
| ------------ | ------------------------------------------ |
| `PENDING`    | Op queued, waiting to execute              |
| `VALIDATING` | Running pre-check validation               |
| `BLOCKED`    | Validation failed, execution skipped       |
| `EXECUTING`  | Currently running                          |
| `SUCCESS`    | Completed successfully                     |
| `ERROR`      | Execution failed (exit code != 0)          |
| `TIMEOUT`    | Exceeded timeout, user intervention needed |
| `SKIPPED`    | Skipped due to earlier pipeline failure    |

---

## Concurrency Rules

| Rule                    | Behavior                                        |
| ----------------------- | ----------------------------------------------- |
| One op per host         | Second op request returns `BLOCKED`             |
| Pipelines are exclusive | Can't start pipeline if any target host is busy |
| Dashboard restart       | Resume pending ops, don't duplicate             |

---

## Idempotency

Ops should be safe to retry when marked `Retryable`:

| Op         | Idempotent? | Notes                             |
| ---------- | ----------- | --------------------------------- |
| `pull`     | ✅          | `git reset --hard` is idempotent  |
| `switch`   | ✅          | Nix rebuilds are deterministic    |
| `test`     | ✅          | Tests are read-only               |
| `reboot`   | ⚠️          | Safe but disruptive               |
| `merge-pr` | ✅          | Already-merged PR returns success |

---

## Error Handling

| Error Type      | Response                                       |
| --------------- | ---------------------------------------------- |
| Pre-check fails | `BLOCKED` with reason, no execution            |
| Execution fails | `ERROR` with exit code, log details            |
| Timeout         | `TIMEOUT` with user options (wait/kill/ignore) |
| Network loss    | `ORPHANED` on dashboard restart, manual review |

---

## Implementation Location

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
    └── ...
```

---

## Implementing Backlog Items

> Updated as backlog items are created/completed.

| Backlog Item | Description               | Status |
| ------------ | ------------------------- | ------ |
| (pending)    | Create Op Engine package  | —      |
| (pending)    | Define core ops           | —      |
| (pending)    | Migrate existing commands | —      |

---

## Changelog

| Date       | Change                          |
| ---------- | ------------------------------- |
| 2025-12-27 | Initial spec extracted from PRD |
