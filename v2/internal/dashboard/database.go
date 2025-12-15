package dashboard

import (
	"database/sql"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver (no CGO)
)

// InitDatabase creates the database and tables.
func InitDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency
	_, err = db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		csrf_token TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

	CREATE TABLE IF NOT EXISTS hosts (
		id TEXT PRIMARY KEY,
		hostname TEXT NOT NULL UNIQUE,
		host_type TEXT NOT NULL,
		agent_version TEXT,
		os_version TEXT,
		nixpkgs_version TEXT,
		generation TEXT,
		last_seen DATETIME,
		status TEXT DEFAULT 'unknown',
		pending_command TEXT,
		comment TEXT,
		theme_color TEXT DEFAULT '#7aa2f7',
		metrics_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS command_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id TEXT NOT NULL,
		command TEXT NOT NULL,
		status TEXT NOT NULL,
		exit_code INTEGER,
		message TEXT,
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		FOREIGN KEY (host_id) REFERENCES hosts(id)
	);

	CREATE INDEX IF NOT EXISTS idx_command_logs_host ON command_logs(host_id);
	CREATE INDEX IF NOT EXISTS idx_command_logs_started ON command_logs(started_at);

	CREATE TABLE IF NOT EXISTS metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id TEXT NOT NULL,
		cpu REAL,
		ram REAL,
		swap REAL,
		load REAL,
		recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (host_id) REFERENCES hosts(id)
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_host_time ON metrics(host_id, recorded_at);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations for existing databases
	migrations := []string{
		// Add metrics_json column if not exists (v2.1.0)
		`ALTER TABLE hosts ADD COLUMN metrics_json TEXT`,
		// Add location column (v2.2.0 - P4370)
		`ALTER TABLE hosts ADD COLUMN location TEXT DEFAULT 'home'`,
		// Add device_type column (v2.2.0 - P4370)
		`ALTER TABLE hosts ADD COLUMN device_type TEXT DEFAULT 'desktop'`,
		// Add test_progress column for test results (v2.2.0 - P4370)
		`ALTER TABLE hosts ADD COLUMN test_progress TEXT`,
	}

	for _, m := range migrations {
		// Ignore errors - column may already exist
		_, _ = db.Exec(m)
	}

	return nil
}

