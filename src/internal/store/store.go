// Package store implements the NixFleet State Store (CORE-003).
//
// The State Store provides unified persistence for all NixFleet state:
// - Commands survive dashboard restarts
// - Audit trail for all mutations
// - Recovery from crashes/interruptions
// - Single source of truth (no split brain)
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// StateStore provides unified persistence for all NixFleet state.
type StateStore struct {
	log     zerolog.Logger
	db      *sql.DB
	version atomic.Uint64 // State version for sync protocol (CORE-004)
}

// New creates a new StateStore with the given database.
func New(log zerolog.Logger, db *sql.DB) *StateStore {
	return &StateStore{
		log: log.With().Str("component", "store").Logger(),
		db:  db,
	}
}

// Open opens a SQLite database and runs migrations.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// runMigrations creates or updates the schema.
func runMigrations(db *sql.DB) error {
	// v3 schema - CORE-003
	schema := `
	-- Hosts table (existing, may already exist)
	CREATE TABLE IF NOT EXISTS hosts (
		id           TEXT PRIMARY KEY,
		hostname     TEXT NOT NULL UNIQUE,
		host_type    TEXT NOT NULL,
		agent_version TEXT,
		os_version   TEXT,
		nixpkgs_version TEXT,
		generation   TEXT,
		last_seen    DATETIME,
		status       TEXT DEFAULT 'unknown',
		pending_command TEXT,
		comment      TEXT,
		theme_color  TEXT DEFAULT '#7aa2f7',
		metrics_json TEXT,
		location     TEXT DEFAULT 'home',
		device_type  TEXT DEFAULT 'desktop',
		test_progress TEXT,
		repo_url     TEXT,
		repo_dir     TEXT,
		lock_status_json TEXT,
		system_status_json TEXT,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Sessions table (existing)
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		csrf_token TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

	-- Commands table (NEW - CORE-003)
	-- Journals all op executions for recovery and audit
	CREATE TABLE IF NOT EXISTS commands (
		id          TEXT PRIMARY KEY,
		host_id     TEXT NOT NULL,
		op          TEXT NOT NULL,
		pipeline_id TEXT,
		status      TEXT NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at  DATETIME,
		finished_at DATETIME,
		exit_code   INTEGER,
		error       TEXT,
		output_file TEXT,
		FOREIGN KEY (host_id) REFERENCES hosts(id),
		FOREIGN KEY (pipeline_id) REFERENCES pipelines(id)
	);
	CREATE INDEX IF NOT EXISTS idx_commands_host ON commands(host_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_commands_status ON commands(status);
	CREATE INDEX IF NOT EXISTS idx_commands_pipeline ON commands(pipeline_id);

	-- Pipelines table (NEW - CORE-003)
	CREATE TABLE IF NOT EXISTS pipelines (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		hosts         TEXT NOT NULL,
		current_stage INTEGER DEFAULT 0,
		status        TEXT NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at   DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_pipelines_status ON pipelines(status);

	-- Event log table (NEW - CORE-003)
	-- Unified system events and audit trail
	CREATE TABLE IF NOT EXISTS event_log (
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
	CREATE INDEX IF NOT EXISTS idx_event_log_timestamp ON event_log(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_event_log_host ON event_log(host_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_event_log_category ON event_log(category, timestamp DESC);

	-- State version table (NEW - CORE-004)
	-- Persists state version across restarts
	CREATE TABLE IF NOT EXISTS state_version (
		id      INTEGER PRIMARY KEY CHECK (id = 1),
		version INTEGER NOT NULL DEFAULT 0
	);
	INSERT OR IGNORE INTO state_version (id, version) VALUES (1, 0);

	-- Reboot attempts (existing)
	CREATE TABLE IF NOT EXISTS reboot_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id TEXT NOT NULL,
		attempted_at INTEGER NOT NULL,
		success INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (host_id) REFERENCES hosts(id)
	);
	CREATE INDEX IF NOT EXISTS idx_reboot_attempts_host_time ON reboot_attempts(host_id, attempted_at);

	-- Audit log (existing)
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		host_id TEXT,
		user_session TEXT,
		timestamp INTEGER NOT NULL,
		success INTEGER NOT NULL DEFAULT 0,
		details TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_log_action_time ON audit_log(action, timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_log_host_time ON audit_log(host_id, timestamp);
	`

	_, err := db.Exec(schema)
	return err
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE VERSION (CORE-004)
// ═══════════════════════════════════════════════════════════════════════════

// GetVersion returns the current state version.
func (s *StateStore) GetVersion() uint64 {
	return s.version.Load()
}

// IncrementVersion atomically increments and returns the new version.
// Also persists to database.
func (s *StateStore) IncrementVersion() uint64 {
	newVersion := s.version.Add(1)
	_, err := s.db.Exec(`UPDATE state_version SET version = ? WHERE id = 1`, newVersion)
	if err != nil {
		s.log.Error().Err(err).Uint64("version", newVersion).Msg("failed to persist version")
	}
	return newVersion
}

// LoadVersion loads the persisted version from database on startup.
func (s *StateStore) LoadVersion() error {
	var version uint64
	err := s.db.QueryRow(`SELECT version FROM state_version WHERE id = 1`).Scan(&version)
	if err != nil {
		return fmt.Errorf("load version: %w", err)
	}
	s.version.Store(version)
	s.log.Info().Uint64("version", version).Msg("loaded state version")
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND OPERATIONS (CORE-003)
// ═══════════════════════════════════════════════════════════════════════════

// CreateCommand persists a new command record.
func (s *StateStore) CreateCommand(cmd *ops.Command) error {
	_, err := s.db.Exec(`
		INSERT INTO commands (id, host_id, op, pipeline_id, status, created_at, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, cmd.ID, cmd.HostID, cmd.OpID, nullString(cmd.PipelineID), string(cmd.Status), cmd.CreatedAt, nullTime(cmd.StartedAt))
	if err != nil {
		return fmt.Errorf("create command: %w", err)
	}
	s.IncrementVersion()
	return nil
}

// UpdateCommandStatus updates a command's status.
func (s *StateStore) UpdateCommandStatus(cmdID string, status ops.OpStatus, exitCode *int, errMsg string) error {
	finishedAt := sql.NullTime{}
	if status.IsTerminal() {
		finishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	_, err := s.db.Exec(`
		UPDATE commands
		SET status = ?, finished_at = ?, exit_code = ?, error = ?
		WHERE id = ?
	`, string(status), finishedAt, exitCode, nullString(errMsg), cmdID)
	if err != nil {
		return fmt.Errorf("update command status: %w", err)
	}
	s.IncrementVersion()
	return nil
}

// GetCommand retrieves a command by ID.
func (s *StateStore) GetCommand(cmdID string) (*ops.Command, error) {
	var cmd ops.Command
	var pipelineID, errStr, outputFile sql.NullString
	var startedAt, finishedAt sql.NullTime
	var exitCode sql.NullInt64
	var status string

	err := s.db.QueryRow(`
		SELECT id, host_id, op, pipeline_id, status, created_at, started_at, finished_at, exit_code, error, output_file
		FROM commands WHERE id = ?
	`, cmdID).Scan(&cmd.ID, &cmd.HostID, &cmd.OpID, &pipelineID, &status, &cmd.CreatedAt, &startedAt, &finishedAt, &exitCode, &errStr, &outputFile)
	if err != nil {
		return nil, fmt.Errorf("get command: %w", err)
	}

	cmd.Status = ops.OpStatus(status)
	if pipelineID.Valid {
		cmd.PipelineID = pipelineID.String
	}
	if startedAt.Valid {
		cmd.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		cmd.FinishedAt = finishedAt.Time
	}
	if exitCode.Valid {
		code := int(exitCode.Int64)
		cmd.ExitCode = &code
	}
	if errStr.Valid {
		cmd.Error = errStr.String
	}
	if outputFile.Valid {
		cmd.OutputFile = outputFile.String
	}

	return &cmd, nil
}

// GetPendingCommands returns all non-terminal commands for a host.
func (s *StateStore) GetPendingCommands(hostID string) ([]*ops.Command, error) {
	rows, err := s.db.Query(`
		SELECT id, host_id, op, pipeline_id, status, created_at, started_at
		FROM commands
		WHERE host_id = ? AND status NOT IN ('SUCCESS', 'ERROR', 'TIMEOUT', 'SKIPPED', 'BLOCKED')
		ORDER BY created_at
	`, hostID)
	if err != nil {
		return nil, fmt.Errorf("get pending commands: %w", err)
	}
	defer rows.Close()

	var commands []*ops.Command
	for rows.Next() {
		var cmd ops.Command
		var pipelineID sql.NullString
		var startedAt sql.NullTime
		var status string

		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.OpID, &pipelineID, &status, &cmd.CreatedAt, &startedAt); err != nil {
			continue
		}
		cmd.Status = ops.OpStatus(status)
		if pipelineID.Valid {
			cmd.PipelineID = pipelineID.String
		}
		if startedAt.Valid {
			cmd.StartedAt = startedAt.Time
		}
		commands = append(commands, &cmd)
	}

	return commands, nil
}

// GetOrphanedCommands returns commands stuck in EXECUTING state.
// Used for recovery after dashboard restart.
func (s *StateStore) GetOrphanedCommands() ([]*ops.Command, error) {
	rows, err := s.db.Query(`
		SELECT id, host_id, op, pipeline_id, status, created_at, started_at
		FROM commands
		WHERE status = 'EXECUTING'
	`)
	if err != nil {
		return nil, fmt.Errorf("get orphaned commands: %w", err)
	}
	defer rows.Close()

	var commands []*ops.Command
	for rows.Next() {
		var cmd ops.Command
		var pipelineID sql.NullString
		var startedAt sql.NullTime
		var status string

		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.OpID, &pipelineID, &status, &cmd.CreatedAt, &startedAt); err != nil {
			continue
		}
		cmd.Status = ops.OpStatus(status)
		if pipelineID.Valid {
			cmd.PipelineID = pipelineID.String
		}
		if startedAt.Valid {
			cmd.StartedAt = startedAt.Time
		}
		commands = append(commands, &cmd)
	}

	return commands, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// PIPELINE OPERATIONS (CORE-003)
// ═══════════════════════════════════════════════════════════════════════════

// CreatePipeline persists a new pipeline record.
func (s *StateStore) CreatePipeline(p *ops.PipelineRecord) error {
	hostsJSON, _ := json.Marshal(p.Hosts)
	_, err := s.db.Exec(`
		INSERT INTO pipelines (id, name, hosts, current_stage, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.PipelineID, string(hostsJSON), p.CurrentStage, string(p.Status), p.CreatedAt)
	if err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	s.IncrementVersion()
	return nil
}

// UpdatePipelineStage updates a pipeline's current stage.
func (s *StateStore) UpdatePipelineStage(pipelineID string, stage int) error {
	_, err := s.db.Exec(`UPDATE pipelines SET current_stage = ? WHERE id = ?`, stage, pipelineID)
	if err != nil {
		return fmt.Errorf("update pipeline stage: %w", err)
	}
	s.IncrementVersion()
	return nil
}

// FinishPipeline marks a pipeline as finished.
func (s *StateStore) FinishPipeline(pipelineID string, status ops.PipelineStatus) error {
	_, err := s.db.Exec(`
		UPDATE pipelines SET status = ?, finished_at = ? WHERE id = ?
	`, string(status), time.Now(), pipelineID)
	if err != nil {
		return fmt.Errorf("finish pipeline: %w", err)
	}
	s.IncrementVersion()
	return nil
}

// GetPipeline retrieves a pipeline by ID.
func (s *StateStore) GetPipeline(pipelineID string) (*ops.PipelineRecord, error) {
	var p ops.PipelineRecord
	var hostsJSON string
	var finishedAt sql.NullTime
	var status string

	err := s.db.QueryRow(`
		SELECT id, name, hosts, current_stage, status, created_at, finished_at
		FROM pipelines WHERE id = ?
	`, pipelineID).Scan(&p.ID, &p.PipelineID, &hostsJSON, &p.CurrentStage, &status, &p.CreatedAt, &finishedAt)
	if err != nil {
		return nil, fmt.Errorf("get pipeline: %w", err)
	}

	p.Status = ops.PipelineStatus(status)
	if finishedAt.Valid {
		p.FinishedAt = finishedAt.Time
	}
	_ = json.Unmarshal([]byte(hostsJSON), &p.Hosts)

	return &p, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// EVENT LOG OPERATIONS (CORE-003)
// ═══════════════════════════════════════════════════════════════════════════

// LogEvent logs an event to the event_log table.
func (s *StateStore) LogEvent(category, level, actor, hostID, action, message string, details map[string]any) {
	var detailsJSON sql.NullString
	if details != nil {
		if data, err := json.Marshal(details); err == nil {
			detailsJSON = sql.NullString{String: string(data), Valid: true}
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO event_log (timestamp, category, level, actor, host_id, action, message, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, time.Now(), category, level, nullString(actor), nullString(hostID), nullString(action), message, detailsJSON)
	if err != nil {
		s.log.Error().Err(err).Str("category", category).Str("action", action).Msg("failed to log event")
	}
	s.IncrementVersion()
}

// Event represents an event log entry.
type Event struct {
	ID        int64          `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Category  string         `json:"category"`
	Level     string         `json:"level"`
	Actor     string         `json:"actor,omitempty"`
	HostID    string         `json:"host_id,omitempty"`
	Action    string         `json:"action,omitempty"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

// GetRecentEvents returns the most recent events.
func (s *StateStore) GetRecentEvents(limit int) ([]Event, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, category, level, actor, host_id, action, message, details
		FROM event_log
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var actor, hostID, action, detailsJSON sql.NullString

		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Category, &e.Level, &actor, &hostID, &action, &e.Message, &detailsJSON); err != nil {
			continue
		}
		if actor.Valid {
			e.Actor = actor.String
		}
		if hostID.Valid {
			e.HostID = hostID.String
		}
		if action.Valid {
			e.Action = action.String
		}
		if detailsJSON.Valid {
			_ = json.Unmarshal([]byte(detailsJSON.String), &e.Details)
		}
		events = append(events, e)
	}

	return events, nil
}

// GetHostEvents returns events for a specific host.
func (s *StateStore) GetHostEvents(hostID string, limit int) ([]Event, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, category, level, actor, host_id, action, message, details
		FROM event_log
		WHERE host_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, hostID, limit)
	if err != nil {
		return nil, fmt.Errorf("get host events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var actor, hostIDCol, action, detailsJSON sql.NullString

		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Category, &e.Level, &actor, &hostIDCol, &action, &e.Message, &detailsJSON); err != nil {
			continue
		}
		if actor.Valid {
			e.Actor = actor.String
		}
		if hostIDCol.Valid {
			e.HostID = hostIDCol.String
		}
		if action.Valid {
			e.Action = action.String
		}
		if detailsJSON.Valid {
			_ = json.Unmarshal([]byte(detailsJSON.String), &e.Details)
		}
		events = append(events, e)
	}

	return events, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// RETENTION / CLEANUP (CORE-003)
// ═══════════════════════════════════════════════════════════════════════════

// CleanupOldCommands removes commands older than the given duration.
func (s *StateStore) CleanupOldCommands(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	result, err := s.db.Exec(`
		DELETE FROM commands
		WHERE created_at < ? AND status IN ('SUCCESS', 'ERROR', 'TIMEOUT', 'SKIPPED', 'BLOCKED')
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup commands: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		s.log.Info().Int64("deleted", rows).Msg("cleaned up old commands")
	}
	return rows, nil
}

// CleanupOldPipelines removes pipelines older than the given duration.
func (s *StateStore) CleanupOldPipelines(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	result, err := s.db.Exec(`
		DELETE FROM pipelines
		WHERE created_at < ? AND status IN ('COMPLETE', 'PARTIAL', 'FAILED', 'CANCELLED')
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup pipelines: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		s.log.Info().Int64("deleted", rows).Msg("cleaned up old pipelines")
	}
	return rows, nil
}

// CleanupOldEvents removes events older than the given duration.
func (s *StateStore) CleanupOldEvents(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	result, err := s.db.Exec(`DELETE FROM event_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup events: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		s.log.Info().Int64("deleted", rows).Msg("cleaned up old events")
	}
	return rows, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

