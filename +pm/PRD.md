# NixFleet v3.0 - Product Requirements Document

> **Single Source of Truth** for NixFleet development.
>
> For v2.0 PRD (current implementation), see [PRD-v2.md](./PRD-v2.md).

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
| **Inconsistent Patterns** | Some actions via WebSocket, some via REST                                | Confusing, error-prone                      |

### v3.0 Solutions

| Solution              | Benefit                                            |
| --------------------- | -------------------------------------------------- |
| **Op Engine**         | Single source of truth for all actions             |
| **Pipeline Executor** | Sequential ops with `&&` semantics                 |
| **State Store**       | Unified persistence for commands, pipelines, audit |
| **Command Journal**   | Resume after dashboard restart                     |
| **Thin Frontend**     | Dispatcher only, no business logic                 |
| **Event Bus**         | Consistent state propagation                       |

---

## Core Concepts

### Op (Operation)

An **Op** is an atomic action on a single host. It is:

- **Defined once** in the Op Registry
- **Validated** before execution (pre-check)
- **Verified** after execution (post-check)
- **Testable** in isolation (no UI needed)
- **Observable** (emits progress events)

```go
type Op struct {
    ID        string                                  // "pull", "switch", "test"
    Validate  func(host *Host) *ValidationError      // Pre-check
    Execute   func(ctx context.Context, host *Host) error
    PostCheck func(host *Host) *ValidationError      // Post-check
    Timeout   time.Duration
    Retryable bool
}
```

### Pipeline

A **Pipeline** is an ordered sequence of Ops with `&&` semantics:

- Execute Op 1 on all hosts
- Only hosts that succeeded proceed to Op 2
- Continue until all ops complete or all hosts fail

```go
type Pipeline struct {
    ID   string   // "do-all", "merge-deploy"
    Ops  []string // ["pull", "switch", "test"]
}
```

### State Store

All state persists in SQLite:

```sql
-- Host registry (existing)
hosts (id, hostname, theme_color, location, device_type, ...)

-- Command journal (new)
commands (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    op TEXT NOT NULL,
    pipeline_id TEXT,
    status TEXT NOT NULL,  -- PENDING, EXECUTING, SUCCESS, ERROR, SKIPPED
    created_at TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    exit_code INTEGER,
    error TEXT,
    output_file TEXT
)

-- Pipeline tracking (new)
pipelines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,    -- "do-all"
    hosts TEXT NOT NULL,   -- JSON array of host IDs
    current_stage INTEGER,
    status TEXT NOT NULL,  -- RUNNING, PARTIAL, COMPLETE, FAILED
    created_at TIMESTAMP,
    finished_at TIMESTAMP
)

-- Audit log (new)
audit_log (
    id TEXT PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    actor TEXT,            -- "user", "system", "agent:hsb0"
    action TEXT NOT NULL,  -- "op:pull", "pipeline:do-all", "host:remove"
    target TEXT,           -- host ID or "all"
    details TEXT           -- JSON with context
)
```

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
                              WebSocket
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         DASHBOARD (Go)                                  │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                        Op Engine                                 │   │
│  │  ┌────────────────┐  ┌───────────────────┐  ┌─────────────────┐  │   │
│  │  │  Op Registry   │  │ Pipeline Executor │  │  State Machine  │  │   │
│  │  │  (all ops)     │  │ (&& semantics)    │  │  (per-host)     │  │   │
│  │  └────────────────┘  └───────────────────┘  └─────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                │                                        │
│  ┌─────────────────────────────┼────────────────────────────────────┐   │
│  │                       State Store (SQLite)                       │   │
│  │  ┌───────────────┐  ┌─────────────────┐  ┌────────────────────┐  │   │
│  │  │ Hosts         │  │ Commands        │  │ AuditLog           │  │   │
│  │  └───────────────┘  └─────────────────┘  └────────────────────┘  │   │
│  │  ┌───────────────┐  ┌─────────────────┐                          │   │
│  │  │ Pipelines     │  │ LiveState (mem) │                          │   │
│  │  └───────────────┘  └─────────────────┘                          │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                │                                        │
│  ┌─────────────────────────────┼────────────────────────────────────┐   │
│  │                       Event Bus                                  │   │
│  │  - StateChanged → broadcast to browsers                          │   │
│  │  - OpProgress → update UI                                        │   │
│  │  - AuditEvent → persist to log                                   │   │
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

## Op Registry

All operations defined in one place. Extracted from [ux-flow-cac-v2.md](../docs/ux-flow-cac-v2.md).

### Host Ops (Agent-Executed)

| Op ID            | Description                  | Pre-Check            | Post-Check               | Timeout |
| ---------------- | ---------------------------- | -------------------- | ------------------------ | ------- |
| `pull`           | Git fetch + reset            | Online, no pending   | Lock = ok                | 2 min   |
| `switch`         | nixos-rebuild/hm switch      | Lock = ok (or force) | System = ok, agent fresh | 10 min  |
| `test`           | Run host tests               | Online               | Test results             | 5 min   |
| `restart`        | Restart agent service        | Online               | Agent reconnects         | 1 min   |
| `stop`           | Stop running command         | Has pending cmd      | No pending cmd           | 30 sec  |
| `reboot`         | System reboot                | TOTP verified        | Agent reconnects         | 5 min   |
| `check-version`  | Compare running vs installed | Online               | —                        | 10 sec  |
| `refresh-git`    | Check GitHub for updates     | Online               | —                        | 30 sec  |
| `refresh-lock`   | Compare flake.lock           | Online               | —                        | 30 sec  |
| `refresh-system` | nix build --dry-run          | Online, confirmed    | —                        | 5 min   |
| `bump-flake`     | nix flake update nixfleet    | Online               | Lock changed             | 2 min   |
| `force-rebuild`  | Rebuild with cache bypass    | Online               | Agent fresh              | 15 min  |

### Dashboard Ops (Server-Side)

| Op ID       | Description         | Pre-Check    | Post-Check | Timeout |
| ----------- | ------------------- | ------------ | ---------- | ------- |
| `merge-pr`  | Merge GitHub PR     | PR mergeable | PR merged  | 30 sec  |
| `set-color` | Update theme color  | —            | —          | instant |
| `remove`    | Remove host from DB | —            | —          | instant |

---

## Pipeline Registry

| Pipeline ID    | Ops                                   | Description                             |
| -------------- | ------------------------------------- | --------------------------------------- |
| `do-all`       | `[pull, switch, test]`                | Full update cycle                       |
| `merge-deploy` | `[merge-pr, pull, switch, test]`      | Merge PR then deploy                    |
| `update-agent` | `[bump-flake, pull, switch, restart]` | Update agent to latest (P7210)          |
| `force-update` | `[force-rebuild, restart]`            | Force rebuild with cache bypass (P7220) |

---

## Design Principles

### Idempotency

Ops should be safe to retry:

| Op         | Idempotent? | Notes                             |
| ---------- | ----------- | --------------------------------- |
| `pull`     | ✅          | `git reset --hard` is idempotent  |
| `switch`   | ✅          | Nix rebuilds are deterministic    |
| `test`     | ✅          | Tests are read-only               |
| `reboot`   | ⚠️          | Safe but disruptive               |
| `merge-pr` | ✅          | Already-merged PR returns success |

### Concurrency Control

| Rule                    | Behavior                                        |
| ----------------------- | ----------------------------------------------- |
| One op per host         | Second op request returns `BLOCKED`             |
| Pipelines are exclusive | Can't start pipeline if any target host is busy |
| Dashboard restart       | Resume pending ops, don't duplicate             |

### Error Handling

| Error Type      | Response                                       |
| --------------- | ---------------------------------------------- |
| Pre-check fails | `BLOCKED` with reason, no execution            |
| Execution fails | `ERROR` with exit code, log details            |
| Timeout         | `TIMEOUT` with user options (wait/kill/ignore) |
| Network loss    | `ORPHANED` on dashboard restart, manual review |

### Graceful Shutdown

On dashboard shutdown:

1. Stop accepting new ops
2. Wait for executing ops to complete (30s max)
3. Mark incomplete ops as `INTERRUPTED` in journal
4. On restart, offer to resume or cancel interrupted ops

---

## State Machine

### Pipeline States

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│  IDLE   │────▶│ STAGE-1 │────▶│ STAGE-2 │────▶│ STAGE-N │────▶ COMPLETE
└─────────┘     │  (pull) │     │ (switch)│     │  (test) │
                └────┬────┘     └────┬────┘     └────┬────┘
                     │               │               │
                on failure      on failure      on failure
                     │               │               │
                     ▼               ▼               ▼
                PARTIAL         PARTIAL         PARTIAL
               (some hosts     (fewer hosts    (report only)
                continue)       continue)
```

### Per-Host Op States

```
┌─────────┐     ┌───────────┐     ┌─────────┐
│PENDING  │────▶│ EXECUTING │────▶│ SUCCESS │
└─────────┘     └─────┬─────┘     └─────────┘
                      │
                      │           ┌─────────┐
                      └──────────▶│  ERROR  │
                      │           └─────────┘
                      │
                      │           ┌─────────┐
                      └──────────▶│ TIMEOUT │──▶ WAIT / KILL / IGNORE
                                  └─────────┘

SKIPPED: Host failed in earlier op, excluded from subsequent ops
```

---

## Recovery & Resilience

### Dashboard Restart Recovery

On startup:

1. Load all `PENDING` and `EXECUTING` commands from journal
2. Mark orphaned `EXECUTING` commands as `ORPHANED`
3. When agents reconnect, reconcile state
4. Log recovery actions to audit log

### Command Journal

Every op execution is journaled:

```sql
INSERT INTO commands (id, host_id, op, status, created_at)
VALUES ('uuid', 'hsb0', 'switch', 'PENDING', NOW());

-- On start
UPDATE commands SET status = 'EXECUTING', started_at = NOW() WHERE id = 'uuid';

-- On complete
UPDATE commands SET status = 'SUCCESS', finished_at = NOW(), exit_code = 0 WHERE id = 'uuid';
```

---

## Observability & Logging

### Event Log (Unified System + Audit)

All UI events and audit entries persist in SQLite:

```sql
CREATE TABLE event_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    category    TEXT NOT NULL,     -- 'audit' | 'system' | 'error'
    level       TEXT NOT NULL,     -- 'info' | 'warn' | 'error' | 'success'
    actor       TEXT,              -- 'user' | 'agent:hsb0' | 'system'
    host_id     TEXT,              -- NULL for global events
    action      TEXT,              -- 'op:pull' | 'connect' | 'disconnect'
    message     TEXT NOT NULL,
    details     TEXT               -- JSON for extra context
);

CREATE INDEX idx_event_log_timestamp ON event_log(timestamp DESC);
CREATE INDEX idx_event_log_host ON event_log(host_id, timestamp DESC);
```

| Event Type        | Category | Example                           |
| ----------------- | -------- | --------------------------------- |
| Command started   | audit    | `hsb0: pull started`              |
| Command completed | audit    | `hsb0: switch completed (exit 0)` |
| Host connected    | system   | `hsb1 connected`                  |
| Host disconnected | system   | `gpc0 disconnected (timeout)`     |
| Error             | error    | `Failed to merge PR #42`          |

**Rotation**: Keep 7 days, configurable via `NIXFLEET_LOG_RETENTION_DAYS`.

### Per-Host Command Output

Command output persists in filesystem (existing approach):

```
/var/lib/nixfleet/logs/
├── hsb0.log      # Rolling log, last N commands
├── hsb1.log
├── gpc0.log
└── imac0.log
```

### Logs on Page Load

On browser connect/reconnect:

1. Server sends last 100 event_log entries
2. If host tab was previously open, send recent output
3. Continue streaming new entries via WebSocket

```go
// GET /api/event-log?limit=100
// GET /api/hosts/{id}/output?lines=500
```

### Optional: Metrics Endpoint

```
GET /metrics

nixfleet_ops_total{op="pull",status="success"} 42
nixfleet_ops_duration_seconds{op="switch",quantile="0.95"} 180
nixfleet_hosts_online 4
```

---

## Always-Live UI State

> **Goal**: The UI is NEVER stale. No manual refresh required. Ever.

### The Problem (v2)

- WebSocket messages can be missed (reconnection gaps)
- Client and server state can diverge
- No reconciliation mechanism
- User forced to CMD+R constantly

### The Solution: Version-Based State Sync

Every state change increments a global `state_version`. Clients track their version and detect drift.

```
Server                                              Browser
   │                                                    │
   │──────────── init(state, version=100) ─────────────▶│
   │                                                    │
   │──────────── delta(change, version=101) ───────────▶│
   │──────────── delta(change, version=102) ───────────▶│
   │                                                    │
   │         [connection lost, missed 103-105]          │
   │                                                    │
   │──────────── reconnect: init(state, v=106) ────────▶│
   │                                                    │
   │         [periodic sync beacon every 30s]           │
   │                                                    │
   │──────────── sync(version=110) ────────────────────▶│
   │                                                    │
   │         [client: my_version=106 != 110 → resync]   │
   │                                                    │
   │◀─────────── get_full_state() ──────────────────────│
   │──────────── full_state(state, v=110) ─────────────▶│
```

### State Sync Protocol

| Message Type | Direction       | Purpose                                  |
| ------------ | --------------- | ---------------------------------------- |
| `init`       | Server → Client | Full state on connect/resync             |
| `delta`      | Server → Client | Incremental change                       |
| `sync`       | Server → Client | Periodic version beacon (every 30s)      |
| `get_state`  | Client → Server | Request full state (on version mismatch) |

### Guarantees

| Scenario         | Behavior                                   |
| ---------------- | ------------------------------------------ |
| Normal operation | Deltas applied in order                    |
| Missed message   | Detected by version gap → auto resync      |
| Reconnection     | Always full resync                         |
| Tab inactive     | Sync beacon catches drift within 30s       |
| Server restart   | Version resets → clients detect and resync |

### Implementation

**Server:**

```go
type StateManager struct {
    mu      sync.RWMutex
    version uint64  // Increments on ANY change
}

func (sm *StateManager) ApplyChange(change Change) {
    sm.mu.Lock()
    sm.version++
    sm.mu.Unlock()

    sm.broadcast(Message{
        Type:    "delta",
        Version: sm.version,
        Payload: change,
    })
}
```

**Client:**

```javascript
let stateVersion = 0;

ws.onmessage = (msg) => {
  const { type, version, payload } = JSON.parse(msg.data);

  if (type === "delta" && version !== stateVersion + 1) {
    // Missed updates → request full state
    ws.send(JSON.stringify({ type: "get_state" }));
    return;
  }

  if (type === "sync" && version !== stateVersion) {
    // Drift detected → request full state
    ws.send(JSON.stringify({ type: "get_state" }));
    return;
  }

  // Apply update
  applyMessage(msg);
  stateVersion = version;
};
```

---

## Implementation Phases

### Phase 1: Op Engine (Foundation)

- [ ] Create `v2/internal/ops/` package
- [ ] Define Op struct and registry
- [ ] Define Pipeline struct and executor
- [ ] Migrate existing commands to Ops
- [ ] Unit tests for Op validation and execution

**Effort**: 2-3 days

### Phase 2: State Persistence & Logging

- [ ] Add `commands` table to SQLite
- [ ] Add `pipelines` table to SQLite
- [ ] Add `event_log` table (unified system + audit)
- [ ] Journal all op executions
- [ ] Implement recovery on startup
- [ ] Add log rotation (7 day default)
- [ ] Test restart scenarios

**Effort**: 3-4 days

### Phase 3: Always-Live State Sync

- [ ] Add `state_version` to StateManager
- [ ] Implement `init` message (full state on connect)
- [ ] Implement `delta` messages (incremental updates)
- [ ] Implement `sync` beacon (every 30s)
- [ ] Client: version tracking and auto-resync
- [ ] Test: reconnection, missed messages, server restart

**Effort**: 2-3 days

### Phase 4: Logs on Page Load

- [ ] API: `GET /api/event-log?limit=100`
- [ ] API: `GET /api/hosts/{id}/output?lines=500`
- [ ] Send event_log in `init` payload
- [ ] Restore host output tabs from server
- [ ] Test: refresh preserves logs

**Effort**: 1-2 days

### Phase 5: Frontend Simplification

- [ ] Remove business logic from JavaScript
- [ ] Dispatch ops via single `dispatch(op, targets)` function
- [ ] Server renders complete state (no client-side derivation)
- [ ] Test all UI flows work correctly

**Effort**: 2-3 days

### Phase 6: Polish & Testing

- [ ] E2E tests for common workflows
- [ ] Recovery test (kill dashboard mid-operation)
- [ ] State sync stress test (rapid changes)
- [ ] Documentation update
- [ ] Performance validation

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

### Phase 0: Preparation

- [ ] Create `v2/internal/ops/` package alongside existing code
- [ ] Define Op structs that wrap existing command handlers
- [ ] Run both systems in parallel (old code calls new ops internally)

### Phase 1-6: Incremental Migration

Each phase is deployable. No big-bang cutover.

| Phase | v2 Behavior             | v3 Behavior                           |
| ----- | ----------------------- | ------------------------------------- |
| 1     | Commands work as before | Ops defined, not yet used             |
| 2     | Commands work as before | Journaling added to existing commands |
| 3     | Full state on connect   | State sync protocol active            |
| 4     | Logs lost on refresh    | Logs persist                          |
| 5     | JS business logic       | Thin frontend                         |
| 6     | Complete v3             | Old code removed                      |

### Rollback Strategy

Each phase can be rolled back by:

1. Reverting the commit
2. Redeploying dashboard
3. Agents continue working (no agent changes in v3)

---

## Out of Scope (v3.0)

- Multi-user support (single admin only)
- Role-based access control
- Scheduled commands
- Host grouping/tags
- High availability (single instance)
- External audit log export (Prometheus metrics optional)
- Security hardening (deferred to v3.1)

---

## References

- [PRD-v2.md](./PRD-v2.md) — v2.0 implementation spec (current baseline)
- [UX Flow v2](../docs/ux-flow-cac-v2.md) — v2 Control-Action-Command mapping (archived)
- [Integration Test Specs](../tests/specs/) — Executable specifications
- [Backlog](./backlog/) — Implementation tasks (P-numbered)

---

## Changelog

| Date       | Version   | Changes                                       |
| ---------- | --------- | --------------------------------------------- |
| 2025-12-27 | 3.0-draft | Initial v3.0 PRD with Op Engine architecture  |
|            |           | Introduced Op, Pipeline, State Store concepts |
|            |           | Defined implementation phases                 |
|            |           | Archived v2.0 PRD as PRD-v2.md                |
