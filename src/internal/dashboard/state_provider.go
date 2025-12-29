package dashboard

import (
	"database/sql"
	"encoding/json"

	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/markus-barta/nixfleet/internal/sync"
)

// DashboardStateProvider implements sync.StateProvider for the dashboard.
// It aggregates state from hosts, commands, pipelines, and events.
type DashboardStateProvider struct {
	db *sql.DB
	vf *VersionFetcher // Optional: compute Git status for init/full_state hosts
	store interface {
		GetRecentEvents(limit int) ([]any, error)
	}
}

// NewDashboardStateProvider creates a new state provider.
func NewDashboardStateProvider(db *sql.DB, vf *VersionFetcher) *DashboardStateProvider {
	return &DashboardStateProvider{db: db, vf: vf}
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
		       location, device_type, metrics_json,
		       lock_status_json, system_status_json, tests_status_json, tests_generation,
		       repo_url, repo_dir
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
			ID, Hostname, HostType, Status                                 string
			AgentVersion, LastSeen, Generation, PendingCommand             sql.NullString
			ThemeColor, Location, DeviceType                               sql.NullString
			MetricsJSON, LockStatusJSON, SystemStatusJSON, TestsStatusJSON  sql.NullString
			TestsGeneration, RepoURL, RepoDir                               sql.NullString
		}
		if err := rows.Scan(
			&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
			&h.Status, &h.LastSeen, &h.Generation, &h.PendingCommand,
			&h.ThemeColor, &h.Location, &h.DeviceType, &h.MetricsJSON,
			&h.LockStatusJSON, &h.SystemStatusJSON, &h.TestsStatusJSON, &h.TestsGeneration,
			&h.RepoURL, &h.RepoDir,
		); err != nil {
			continue
		}

		// Calculate available ops based on host state
		availableOps := p.calculateAvailableOps(&struct {
			ID, Hostname, HostType, Status                      string
			AgentVersion, LastSeen, Generation, PendingCommand  sql.NullString
			ThemeColor, Location, DeviceType                    sql.NullString
		}{
			ID:             h.ID,
			Hostname:       h.Hostname,
			HostType:       h.HostType,
			Status:         h.Status,
			AgentVersion:   h.AgentVersion,
			LastSeen:       h.LastSeen,
			Generation:     h.Generation,
			PendingCommand: h.PendingCommand,
			ThemeColor:     h.ThemeColor,
			Location:       h.Location,
			DeviceType:     h.DeviceType,
		})

		// Compute agent_outdated (dashboard-side)
		agentVersion := nullStr(h.AgentVersion)
		agentOutdated := agentVersion != "" && agentVersion != Version

		// Parse metrics JSON
		var metrics any
		if h.MetricsJSON.Valid && h.MetricsJSON.String != "" {
			var m any
			if err := json.Unmarshal([]byte(h.MetricsJSON.String), &m); err == nil {
				metrics = m
			}
		}

		// Build update_status (git from VersionFetcher, lock/system from DB)
		updateStatus := p.buildUpdateStatus(nullStr(h.Generation), h.LockStatusJSON, h.SystemStatusJSON, h.TestsStatusJSON, h.TestsGeneration, h.RepoURL, h.RepoDir)

		hosts = append(hosts, map[string]any{
			"id":              h.ID,
			"hostname":        h.Hostname,
			"host_type":       h.HostType,
			"agent_version":   agentVersion,
			"agent_outdated":  agentOutdated,
			"status":          h.Status,
			"last_seen":       nullStr(h.LastSeen),
			"generation":      nullStr(h.Generation),
			"pending_command": nullStr(h.PendingCommand),
			"theme_color":     nullStr(h.ThemeColor),
			"location":        nullStr(h.Location),
			"device_type":     nullStr(h.DeviceType),
			"available_ops":   availableOps,
			"metrics":         metrics,
			"update_status":   updateStatus,
		})
	}
	return hosts
}

func (p *DashboardStateProvider) buildUpdateStatus(generation string, lockJSON, systemJSON, testsJSON, testsGen, repoURL, repoDir sql.NullString) map[string]any {
	var gitStatus map[string]any
	if p.vf != nil {
		status, msg, checked := p.vf.GetGitStatus(generation)
		gitStatus = map[string]any{"status": status, "message": msg, "checked_at": checked}
	} else {
		gitStatus = map[string]any{
			"status":     "error",
			"message":    "Version tracking not configured (remote desired state unavailable)",
			"checked_at": "",
		}
	}

	var lockStatus any
	if lockJSON.Valid && lockJSON.String != "" {
		var v any
		if err := json.Unmarshal([]byte(lockJSON.String), &v); err == nil {
			lockStatus = v
		}
	}
	var systemStatus any
	if systemJSON.Valid && systemJSON.String != "" {
		var v any
		if err := json.Unmarshal([]byte(systemJSON.String), &v); err == nil {
			systemStatus = v
		}
	}
	var testsStatus any
	if testsJSON.Valid && testsJSON.String != "" {
		var v any
		if err := json.Unmarshal([]byte(testsJSON.String), &v); err == nil {
			testsStatus = v
		}
	}

	us := map[string]any{
		"git":      gitStatus,
		"lock":     lockStatus,
		"system":   systemStatus,
		"tests":    testsStatus,
		"repo_url": nullStr(repoURL),
		"repo_dir": nullStr(repoDir),
	}

	// CORE-006: Remote-gate System (and keep semantics consistent on init/full_state)
	getStatus := func(v any) string {
		m, ok := v.(map[string]any)
		if !ok || m == nil {
			return ""
		}
		s, _ := m["status"].(string)
		return s
	}
	gs := getStatus(gitStatus)
	ls := getStatus(lockStatus)
	ss := getStatus(systemStatus)

	gitOK := gs == "ok"
	lockOK := ls == "ok"
	gitOutdated := gs == "outdated"
	lockOutdated := ls == "outdated"
	gitError := gs == "error"
	lockError := ls == "error"
	gitUnknown := gs == "" || gs == "unknown"
	lockUnknown := ls == "" || ls == "unknown"

	// If System is missing but Git/Lock clearly indicate not-current, avoid gray.
	if ss == "" {
		switch {
		case gitOutdated || lockOutdated:
			us["system"] = map[string]any{"status": "outdated", "message": "System not current vs remote (Git/Lock behind)", "checked_at": ""}
		case gitError || lockError:
			us["system"] = map[string]any{"status": "outdated", "message": "Remote verification degraded (Git/Lock error)", "checked_at": ""}
		}
	}
	// If System claims ok but prerequisites aren't ok, degrade (unknown vs outdated).
	if ss == "ok" && (!gitOK || !lockOK) {
		switch {
		case gitOutdated || lockOutdated:
			us["system"] = map[string]any{"status": "outdated", "message": "System not current vs remote (Git/Lock behind)", "checked_at": ""}
		case gitError || lockError:
			us["system"] = map[string]any{"status": "outdated", "message": "Remote verification degraded (Git/Lock error)", "checked_at": ""}
		case gitUnknown || lockUnknown:
			us["system"] = map[string]any{"status": "unknown", "message": "Cannot verify System vs remote (insufficient signal)", "checked_at": ""}
		}
	}

	// CORE-006: Tests are generation-scoped (old-generation pass => 🟡 on new deployment)
	tgs := ""
	if testsGen.Valid {
		tgs = testsGen.String
	}
	if testsStatus != nil && tgs != "" && generation != "" && tgs != generation {
		us["tests"] = map[string]any{
			"status":     "outdated",
			"message":    "Tests outdated for current deployed state",
			"checked_at": "",
		}
	}

	return us
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

