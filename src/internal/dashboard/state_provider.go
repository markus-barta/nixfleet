package dashboard

import (
	"database/sql"

	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/markus-barta/nixfleet/internal/sync"
)

// DashboardStateProvider implements sync.StateProvider for the dashboard.
// It aggregates state from hosts, commands, pipelines, and events.
type DashboardStateProvider struct {
	db    *sql.DB
	store interface {
		GetRecentEvents(limit int) ([]any, error)
	}
}

// NewDashboardStateProvider creates a new state provider.
func NewDashboardStateProvider(db *sql.DB) *DashboardStateProvider {
	return &DashboardStateProvider{db: db}
}

// GetFullState returns the complete state for sync protocol init/full_state.
// Implements sync.StateProvider interface.
func (p *DashboardStateProvider) GetFullState() sync.FullState {
	state := sync.FullState{
		Hosts:     make([]any, 0),
		Commands:  make([]any, 0),
		Pipelines: make([]any, 0),
		Events:    make([]any, 0),
	}

	// Get all hosts
	state.Hosts = p.getHosts()

	// Get active/recent commands (last 50)
	state.Commands = p.getRecentCommands(50)

	// Get active/recent pipelines (last 20)
	state.Pipelines = p.getRecentPipelines(20)

	// Get recent events (last 100)
	state.Events = p.getRecentEvents(100)

	return state
}

func (p *DashboardStateProvider) getHosts() []any {
	rows, err := p.db.Query(`
		SELECT id, hostname, host_type, agent_version, status, 
		       last_seen, generation, pending_command, theme_color,
		       location, device_type
		FROM hosts
		ORDER BY hostname
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var hosts []any
	for rows.Next() {
		var h struct {
			ID, Hostname, HostType, Status                      string
			AgentVersion, LastSeen, Generation, PendingCommand  sql.NullString
			ThemeColor, Location, DeviceType                    sql.NullString
		}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
			&h.Status, &h.LastSeen, &h.Generation, &h.PendingCommand,
			&h.ThemeColor, &h.Location, &h.DeviceType); err != nil {
			continue
		}

		// Calculate available ops based on host state
		availableOps := p.calculateAvailableOps(&h)

		hosts = append(hosts, map[string]any{
			"id":              h.ID,
			"hostname":        h.Hostname,
			"host_type":       h.HostType,
			"agent_version":   nullStr(h.AgentVersion),
			"status":          h.Status,
			"last_seen":       nullStr(h.LastSeen),
			"generation":      nullStr(h.Generation),
			"pending_command": nullStr(h.PendingCommand),
			"theme_color":     nullStr(h.ThemeColor),
			"location":        nullStr(h.Location),
			"device_type":     nullStr(h.DeviceType),
			"available_ops":   availableOps,
		})
	}
	return hosts
}

// calculateAvailableOps determines which operations are available for a host.
// This is server-side business logic - frontend just renders what we say.
func (p *DashboardStateProvider) calculateAvailableOps(h *struct {
	ID, Hostname, HostType, Status                      string
	AgentVersion, LastSeen, Generation, PendingCommand  sql.NullString
	ThemeColor, Location, DeviceType                    sql.NullString
}) []string {
	available := make([]string, 0)

	// Host must be online (status = "online")
	isOnline := h.Status == "online"

	// Host must not have a pending command
	hasPendingCommand := h.PendingCommand.Valid && h.PendingCommand.String != ""

	// If offline or busy, no ops available
	if !isOnline || hasPendingCommand {
		return available
	}

	// Standard ops available for all online, idle hosts
	available = append(available, "pull", "switch", "test")

	// Reboot requires TOTP (always show if online + idle)
	available = append(available, "reboot")

	return available
}

func (p *DashboardStateProvider) getRecentCommands(limit int) []any {
	rows, err := p.db.Query(`
		SELECT id, host_id, op, status, created_at, started_at, finished_at, exit_code, error
		FROM commands
		WHERE status NOT IN ('SUCCESS', 'ERROR', 'TIMEOUT', 'SKIPPED', 'BLOCKED')
		   OR created_at > datetime('now', '-1 hour')
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var commands []any
	for rows.Next() {
		var c struct {
			ID, HostID, Op, Status               string
			CreatedAt, StartedAt, FinishedAt     sql.NullString
			ExitCode                             sql.NullInt64
			Error                                sql.NullString
		}
		if err := rows.Scan(&c.ID, &c.HostID, &c.Op, &c.Status,
			&c.CreatedAt, &c.StartedAt, &c.FinishedAt, &c.ExitCode, &c.Error); err != nil {
			continue
		}
		cmd := map[string]any{
			"id":          c.ID,
			"host_id":     c.HostID,
			"op":          c.Op,
			"status":      c.Status,
			"created_at":  nullStr(c.CreatedAt),
			"started_at":  nullStr(c.StartedAt),
			"finished_at": nullStr(c.FinishedAt),
		}
		if c.ExitCode.Valid {
			cmd["exit_code"] = c.ExitCode.Int64
		}
		if c.Error.Valid {
			cmd["error"] = c.Error.String
		}
		commands = append(commands, cmd)
	}
	return commands
}

func (p *DashboardStateProvider) getRecentPipelines(limit int) []any {
	rows, err := p.db.Query(`
		SELECT id, name, hosts, current_stage, status, created_at, finished_at
		FROM pipelines
		WHERE status = 'RUNNING'
		   OR created_at > datetime('now', '-1 hour')
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var pipelines []any
	for rows.Next() {
		var pl struct {
			ID, Name, Hosts, Status      string
			CurrentStage                 int
			CreatedAt, FinishedAt        sql.NullString
		}
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.Hosts, &pl.CurrentStage,
			&pl.Status, &pl.CreatedAt, &pl.FinishedAt); err != nil {
			continue
		}
		pipelines = append(pipelines, map[string]any{
			"id":            pl.ID,
			"pipeline_id":   pl.Name,
			"hosts":         pl.Hosts, // JSON string
			"current_stage": pl.CurrentStage,
			"status":        pl.Status,
			"created_at":    nullStr(pl.CreatedAt),
			"finished_at":   nullStr(pl.FinishedAt),
		})
	}
	return pipelines
}

func (p *DashboardStateProvider) getRecentEvents(limit int) []any {
	rows, err := p.db.Query(`
		SELECT id, timestamp, category, level, actor, host_id, action, message
		FROM event_log
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []any
	for rows.Next() {
		var e struct {
			ID                                   int64
			Timestamp, Category, Level, Message  string
			Actor, HostID, Action                sql.NullString
		}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Category, &e.Level,
			&e.Actor, &e.HostID, &e.Action, &e.Message); err != nil {
			continue
		}
		events = append(events, map[string]any{
			"id":        e.ID,
			"timestamp": e.Timestamp,
			"category":  e.Category,
			"level":     e.Level,
			"actor":     nullStr(e.Actor),
			"host_id":   nullStr(e.HostID),
			"action":    nullStr(e.Action),
			"message":   e.Message,
		})
	}
	return events
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// Ensure DashboardStateProvider implements sync.StateProvider at compile time.
var _ sync.StateProvider = (*DashboardStateProvider)(nil)

// Unused import guard
var _ = ops.StatusPending

