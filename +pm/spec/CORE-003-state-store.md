# CORE-003: State Store

> **Spec Type**: Core Building Block  
> **Status**: Stable  
> **Last Updated**: 2025-12-27

---

## Purpose

The **State Store** provides unified persistence for all NixFleet state. It ensures:

- Commands survive dashboard restarts
- Audit trail for all mutations
- Recovery from crashes/interruptions
- Single source of truth (no split brain)

---

## Storage Backend

SQLite database at `$NIXFLEET_DATA_DIR/nixfleet.db` (default: `/var/lib/nixfleet/nixfleet.db`).

Why SQLite:

- Single file, easy backup
- ACID transactions
- No external dependencies
- Fast enough for single-user fleet management

---

## Schema

### Hosts Table (Existing)

```sql
CREATE TABLE hosts (
    id           TEXT PRIMARY KEY,
    hostname     TEXT NOT NULL UNIQUE,
    theme_color  TEXT,
    location     TEXT,
    device_type  TEXT,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Commands Table (New)

Journals all op executions for recovery and audit.

```sql
CREATE TABLE commands (
    id          TEXT PRIMARY KEY,           -- UUID
    host_id     TEXT NOT NULL,              -- FK to hosts
    op          TEXT NOT NULL,              -- Op ID: "pull", "switch"
    pipeline_id TEXT,                       -- FK to pipelines (NULL if standalone)
    status      TEXT NOT NULL,              -- PENDING, EXECUTING, SUCCESS, ERROR, SKIPPED, TIMEOUT
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at  TIMESTAMP,
    finished_at TIMESTAMP,
    exit_code   INTEGER,
    error       TEXT,
    output_file TEXT,                       -- Path to output log file

    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (pipeline_id) REFERENCES pipelines(id)
);

CREATE INDEX idx_commands_host ON commands(host_id, created_at DESC);
CREATE INDEX idx_commands_status ON commands(status);
CREATE INDEX idx_commands_pipeline ON commands(pipeline_id);
```

### Pipelines Table (New)

Tracks pipeline executions.

```sql
CREATE TABLE pipelines (
    id            TEXT PRIMARY KEY,         -- UUID
    name          TEXT NOT NULL,            -- Pipeline ID: "do-all"
    hosts         TEXT NOT NULL,            -- JSON array of host IDs
    current_stage INTEGER DEFAULT 0,
    status        TEXT NOT NULL,            -- RUNNING, PARTIAL, COMPLETE, FAILED, CANCELLED
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at   TIMESTAMP
);

CREATE INDEX idx_pipelines_status ON pipelines(status);
```

### Event Log Table (New)

Unified system events and audit trail.

```sql
CREATE TABLE event_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    category    TEXT NOT NULL,              -- 'audit' | 'system' | 'error'
    level       TEXT NOT NULL,              -- 'info' | 'warn' | 'error' | 'success'
    actor       TEXT,                       -- 'user' | 'agent:hsb0' | 'system'
    host_id     TEXT,                       -- NULL for global events
    action      TEXT,                       -- 'op:pull' | 'connect' | 'disconnect'
    message     TEXT NOT NULL,
    details     TEXT                        -- JSON for extra context
);

CREATE INDEX idx_event_log_timestamp ON event_log(timestamp DESC);
CREATE INDEX idx_event_log_host ON event_log(host_id, timestamp DESC);
CREATE INDEX idx_event_log_category ON event_log(category, timestamp DESC);
```

---

## Event Categories

| Category | Description            | Examples                          |
| -------- | ---------------------- | --------------------------------- |
| `audit`  | User-initiated actions | Op started, Op completed          |
| `system` | System events          | Host connected, Host disconnected |
| `error`  | Errors and warnings    | Connection timeout, Build failed  |

## Event Levels

| Level     | Description               | UI Treatment     |
| --------- | ------------------------- | ---------------- |
| `info`    | Informational             | Default styling  |
| `warn`    | Warning, attention needed | Yellow highlight |
| `error`   | Error occurred            | Red highlight    |
| `success` | Successful completion     | Green highlight  |

---

## Retention Policy

| Table       | Retention | Notes                                       |
| ----------- | --------- | ------------------------------------------- |
| `hosts`     | Permanent | User-managed                                |
| `commands`  | 30 days   | Configurable: `NIXFLEET_CMD_RETENTION_DAYS` |
| `pipelines` | 30 days   | Follows commands retention                  |
| `event_log` | 7 days    | Configurable: `NIXFLEET_LOG_RETENTION_DAYS` |

Cleanup runs daily via background goroutine.

---

## Command Output Storage

Command output is stored in the filesystem (not SQLite) for efficiency:

```
$NIXFLEET_DATA_DIR/logs/
├── hsb0.log          # Rolling log, last N commands
├── hsb1.log
├── gpc0.log
└── imac0.log
```

The `commands.output_file` column references these files.

---

## Recovery on Startup

When dashboard starts:

```go
func (s *StateStore) RecoverOrphanedCommands() error {
    // 1. Find commands stuck in EXECUTING state
    orphaned, _ := s.db.Query(`
        SELECT id, host_id, op FROM commands
        WHERE status = 'EXECUTING'
    `)

    for orphaned.Next() {
        var cmd Command
        orphaned.Scan(&cmd.ID, &cmd.HostID, &cmd.Op)

        // 2. Mark as ORPHANED
        s.db.Exec(`
            UPDATE commands SET status = 'ORPHANED', error = 'Dashboard restarted'
            WHERE id = ?
        `, cmd.ID)

        // 3. Log to event_log
        s.LogEvent("system", "warn", "system", nil, "recovery",
            fmt.Sprintf("Command %s on %s orphaned due to restart", cmd.Op, cmd.HostID), nil)
    }

    return nil
}
```

---

## Transactions

Critical operations use transactions:

```go
func (s *StateStore) StartCommand(hostID, op string) (*Command, error) {
    tx, _ := s.db.Begin()
    defer tx.Rollback()

    // 1. Check no pending command on host
    var count int
    tx.QueryRow(`
        SELECT COUNT(*) FROM commands
        WHERE host_id = ? AND status IN ('PENDING', 'EXECUTING')
    `, hostID).Scan(&count)

    if count > 0 {
        return nil, ErrHostBusy
    }

    // 2. Insert new command
    cmd := &Command{
        ID:     uuid.New().String(),
        HostID: hostID,
        Op:     op,
        Status: "PENDING",
    }
    tx.Exec(`INSERT INTO commands (...) VALUES (...)`, ...)

    tx.Commit()
    return cmd, nil
}
```

---

## Implementation Location

```
src/internal/store/
├── store.go        # StateStore struct, initialization, all operations
├── recovery.go     # Orphaned command recovery, retention cleanup
└── (migrations inline in store.go)
```

---

## Implementing Backlog Items

> Updated as backlog items are created/completed.

| Backlog Item | Description                   | Status |
| ------------ | ----------------------------- | ------ |
| (pending)    | Add commands table migration  | —      |
| (pending)    | Add pipelines table migration | —      |
| (pending)    | Add event_log table migration | —      |
| (pending)    | Implement command journaling  | —      |
| (pending)    | Implement startup recovery    | —      |
| (pending)    | Implement log rotation        | —      |

---

## Changelog

| Date       | Change                          |
| ---------- | ------------------------------- |
| 2025-12-27 | Initial spec extracted from PRD |
