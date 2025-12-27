# P3100: State Persistence

**Priority**: P3100 (Critical - v3 Phase 2)  
**Status**: ✅ Done  
**Effort**: Medium-Large (3-4 days)  
**Implements**: [CORE-003](../spec/CORE-003-state-store.md)  
**Depends on**: P3010, P3020 (Op Engine)

---

## User Story

**As a** fleet administrator  
**I want** commands and logs to survive dashboard restarts  
**So that** I never lose track of what happened or have orphaned commands

---

## Scope

### Database Migrations

Add new tables per CORE-003 spec:

```sql
-- Migration 002: commands
CREATE TABLE commands (
    id          TEXT PRIMARY KEY,
    host_id     TEXT NOT NULL,
    op          TEXT NOT NULL,
    pipeline_id TEXT,
    status      TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at  TIMESTAMP,
    finished_at TIMESTAMP,
    exit_code   INTEGER,
    error       TEXT,
    output_file TEXT,
    FOREIGN KEY (host_id) REFERENCES hosts(id),
    FOREIGN KEY (pipeline_id) REFERENCES pipelines(id)
);

-- Migration 003: pipelines
CREATE TABLE pipelines (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    hosts         TEXT NOT NULL,
    current_stage INTEGER DEFAULT 0,
    status        TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at   TIMESTAMP
);

-- Migration 004: event_log
CREATE TABLE event_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    category    TEXT NOT NULL,
    level       TEXT NOT NULL,
    actor       TEXT,
    host_id     TEXT,
    action      TEXT,
    message     TEXT NOT NULL,
    details     TEXT
);
```

### Command Journaling

Every op execution is journaled:

```go
func (e *OpExecutor) Execute(ctx context.Context, op *Op, host *Host) error {
    // 1. Create command record (PENDING)
    cmdID := e.store.CreateCommand(host.ID, op.ID, "PENDING")

    // 2. Update to EXECUTING
    e.store.UpdateCommandStatus(cmdID, "EXECUTING")

    // 3. Execute op
    err := op.Execute(ctx, host)

    // 4. Update final status
    if err != nil {
        e.store.FinishCommand(cmdID, "ERROR", 1, err.Error())
    } else {
        e.store.FinishCommand(cmdID, "SUCCESS", 0, "")
    }

    return err
}
```

### Event Logging

Replace browser-only system log with persistent event_log:

```go
func (s *StateStore) LogEvent(category, level, actor, hostID, action, message string, details map[string]any) {
    s.db.Exec(`
        INSERT INTO event_log (category, level, actor, host_id, action, message, details)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `, category, level, actor, hostID, action, message, jsonEncode(details))
}
```

### Startup Recovery

On dashboard start:

```go
func (s *StateStore) RecoverOrphanedCommands() error {
    // Mark EXECUTING commands as ORPHANED
    s.db.Exec(`
        UPDATE commands SET status = 'ORPHANED', error = 'Dashboard restarted'
        WHERE status = 'EXECUTING'
    `)

    // Log recovery
    s.LogEvent("system", "warn", "system", nil, "recovery",
        "Dashboard restarted, orphaned commands marked", nil)

    return nil
}
```

### Log Rotation

Background goroutine cleans old records:

```go
func (s *StateStore) StartRetentionWorker() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        s.db.Exec(`DELETE FROM event_log WHERE timestamp < datetime('now', '-7 days')`)
        s.db.Exec(`DELETE FROM commands WHERE created_at < datetime('now', '-30 days')`)
    }
}
```

---

## Acceptance Criteria

- [x] `commands` table created via migration
- [x] `pipelines` table created via migration
- [x] `event_log` table created via migration
- [x] `state_version` table for CORE-004 sync protocol
- [x] All op executions journaled
- [x] All system events logged to event_log
- [x] `GetOrphanedCommands()` for startup recovery
- [x] Cleanup runs hourly with 7-day retention

---

## Related

- **CORE-003**: State Store spec
- **P3010**: Op Engine Foundation
- **P3020**: Pipeline Executor
