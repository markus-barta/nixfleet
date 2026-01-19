package dashboard

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/markus-barta/nixfleet/internal/ops"
	syncproto "github.com/markus-barta/nixfleet/internal/sync"
	"github.com/markus-barta/nixfleet/internal/templates"
)

// ═══════════════════════════════════════════════════════════════════════════
// HOST PROVIDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════

// hostProviderAdapter implements ops.HostProvider using the dashboard's database.
type hostProviderAdapter struct {
	db *sql.DB
	vf *VersionFetcher
}

// GetHostByID implements ops.HostProvider.
func (h *hostProviderAdapter) GetHostByID(hostID string) (ops.Host, error) {
	host, err := getHostByIDFromDB(h.db, hostID)
	if err != nil {
		return nil, err
	}
	// Fill git status (dashboard-side) for ops validation / post-checks
	if h.vf != nil {
		status, msg, checked := h.vf.GetGitStatus(host.Generation)
		if host.UpdateStatus == nil {
			host.UpdateStatus = &templates.UpdateStatus{}
		}
		host.UpdateStatus.Git = templates.StatusCheck{Status: status, Message: msg, CheckedAt: checked}
	}
	return ops.NewHostAdapter(host), nil
}

// getHostByIDFromDB fetches a host from the database.
func getHostByIDFromDB(db *sql.DB, hostID string) (*templates.Host, error) {
	var h struct {
		ID, Hostname, HostType                              string
		AgentVersion, OSVersion, NixpkgsVersion, Generation *string
		LastSeen                                            *string
		Status                                              string
		PendingCommand, ThemeColor                          *string
		Location, DeviceType                                *string
		LockStatusJSON, SystemStatusJSON                    *string
		RepoURL, RepoDir                                    *string
	}

	err := db.QueryRow(`
		SELECT id, hostname, host_type, agent_version, os_version,
		       nixpkgs_version, generation, last_seen, status, pending_command,
		       theme_color, location, device_type, lock_status_json, system_status_json,
		       repo_url, repo_dir
		FROM hosts WHERE id = ?
	`, hostID).Scan(&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
		&h.OSVersion, &h.NixpkgsVersion, &h.Generation, &h.LastSeen,
		&h.Status, &h.PendingCommand, &h.ThemeColor, &h.Location, &h.DeviceType,
		&h.LockStatusJSON, &h.SystemStatusJSON, &h.RepoURL, &h.RepoDir)

	if err != nil {
		return nil, err
	}

	host := &templates.Host{
		ID:       h.ID,
		Hostname: h.Hostname,
		HostType: h.HostType,
		Status:   h.Status,
		Online:   h.Status == "online",
	}

	if h.AgentVersion != nil {
		host.AgentVersion = *h.AgentVersion
	}
	if h.OSVersion != nil {
		host.OSVersion = *h.OSVersion
	}
	if h.Generation != nil {
		host.Generation = *h.Generation
	}
	if h.PendingCommand != nil {
		host.PendingCommand = *h.PendingCommand
	}
	if h.ThemeColor != nil {
		host.ThemeColor = *h.ThemeColor
	}
	if h.Location != nil {
		host.Location = *h.Location
	}
	if h.DeviceType != nil {
		host.DeviceType = *h.DeviceType
	}
	if h.RepoURL != nil {
		host.RepoURL = *h.RepoURL
	}
	if h.RepoDir != nil {
		host.RepoDir = *h.RepoDir
	}

	// Parse update status from JSON
	host.UpdateStatus = parseUpdateStatusJSON(h.SystemStatusJSON, h.LockStatusJSON)

	return host, nil
}

// parseUpdateStatusJSON parses the system and lock status JSON.
func parseUpdateStatusJSON(systemJSON *string, lockJSON *string) *templates.UpdateStatus {
	us := &templates.UpdateStatus{
		System: templates.StatusCheck{Status: "unknown"},
		Lock:   templates.StatusCheck{Status: "unknown"},
		Git:    templates.StatusCheck{Status: "unknown"},
	}

	if lockJSON != nil && *lockJSON != "" {
		var ls templates.StatusCheck
		if err := json.Unmarshal([]byte(*lockJSON), &ls); err == nil && ls.Status != "" {
			us.Lock = ls
		}
	}

	if systemJSON != nil && *systemJSON != "" {
		var ss templates.StatusCheck
		if err := json.Unmarshal([]byte(*systemJSON), &ss); err == nil && ss.Status != "" {
			us.System = ss
		}
	}

	return us
}

// ═══════════════════════════════════════════════════════════════════════════
// BROADCAST SENDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════

// broadcastSenderAdapter implements ops.BroadcastSender using Hub.
type broadcastSenderAdapter struct {
	hub *Hub
}

// BroadcastCommandState implements ops.BroadcastSender.
func (b *broadcastSenderAdapter) BroadcastCommandState(hostID string, cmd *ops.ActiveCommand) {
	b.hub.BroadcastToBrowsers(map[string]any{
		"type": "command_state",
		"payload": map[string]any{
			"host_id": hostID,
			"command": cmd,
		},
	})

	// CORE-004: Emit command deltas for drift-safe UI (even if current UI doesn't consume yet).
	if b.hub.stateManager != nil && cmd != nil {
		// NOTE: state-sync.js expects:
		// - command_started: change.payload = full command
		// - command_progress/finished: change.fields = partial or full command fields
		if cmd.Status.IsTerminal() {
			b.hub.stateManager.ApplyChange(syncproto.Change{
				Type:   syncproto.ChangeCommandFinished,
				ID:     cmd.ID,
				Fields: cmd,
			})
			return
		}
		if !cmd.StartedAt.IsZero() {
			b.hub.stateManager.ApplyChange(syncproto.Change{
				Type:    syncproto.ChangeCommandStarted,
				ID:      cmd.ID,
				Payload: cmd,
			})
			return
		}
		b.hub.stateManager.ApplyChange(syncproto.Change{
			Type:   syncproto.ChangeCommandProgress,
			ID:     cmd.ID,
			Fields: cmd,
		})
	}
}

// BroadcastToast implements ops.BroadcastSender.
func (b *broadcastSenderAdapter) BroadcastToast(hostID, level, message string) {
	// CORE-004: Also record as an event delta (optional consumer)
	if b.hub.stateManager != nil {
		b.hub.stateManager.ApplyChange(syncproto.Change{
			Type: syncproto.ChangeEvent,
			Payload: map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"category":  "ops",
				"level":     level,
				"host_id":   hostID,
				"message":   message,
			},
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// LIFECYCLE MANAGER WRAPPER
// ═══════════════════════════════════════════════════════════════════════════

// lifecycleManagerWrapper wraps ops.LifecycleManager to implement the Hub interface.
type lifecycleManagerWrapper struct {
	lm *ops.LifecycleManager
}

// HandleCommandComplete implements lifecycleManagerInterface.
func (w *lifecycleManagerWrapper) HandleCommandComplete(hostID, opID string, exitCode int, message string) (interface{}, error) {
	return w.lm.HandleCommandComplete(hostID, opID, exitCode, message)
}

func (w *lifecycleManagerWrapper) HandleCommandRejected(hostID, reason, currentCommand string, currentPID int) {
	w.lm.HandleCommandRejected(hostID, reason, currentCommand, currentPID)
}

// HandleHeartbeat implements lifecycleManagerInterface.
func (w *lifecycleManagerWrapper) HandleHeartbeat(hostID string, freshness interface{}) {
	// Convert interface{} to *ops.AgentFreshness
	// Freshness comes from Hub's agentFreshness map which stores the dashboard's AgentFreshness type
	var opsFreshness *ops.AgentFreshness
	switch f := freshness.(type) {
	case *ops.AgentFreshness:
		opsFreshness = f
	case ops.AgentFreshness:
		opsFreshness = &f
	default:
		// Try to extract fields via reflection or type assertion
		// For now, pass nil which is handled gracefully
		opsFreshness = nil
	}
	w.lm.HandleHeartbeat(hostID, opsFreshness)
}

// HandleAgentReconnect implements lifecycleManagerInterface.
func (w *lifecycleManagerWrapper) HandleAgentReconnect(hostID string, freshness interface{}) {
	// Convert interface{} to ops.AgentFreshness
	var opsFreshness ops.AgentFreshness
	switch f := freshness.(type) {
	case ops.AgentFreshness:
		opsFreshness = f
	case *ops.AgentFreshness:
		if f != nil {
			opsFreshness = *f
		}
	default:
		// For unknown types, try a zero value which is handled gracefully
		opsFreshness = ops.AgentFreshness{}
	}
	w.lm.HandleAgentReconnect(hostID, opsFreshness)
}

// HasActiveCommand implements lifecycleManagerInterface.
// P1100: Used by stale cleanup to avoid clearing pending_command for tracked commands.
func (w *lifecycleManagerWrapper) HasActiveCommand(hostID string) bool {
	return w.lm.HasActiveCommand(hostID)
}

// GetActiveCommand implements lifecycleManagerInterface.
// P1920: Used to detect agent disconnect during switch.
func (w *lifecycleManagerWrapper) GetActiveCommand(hostID string) *ops.ActiveCommand {
	return w.lm.GetActiveCommand(hostID)
}

// EnterAwaitingReconnectOnDisconnect implements lifecycleManagerInterface.
// P1920: Called when agent disconnects during switch execution.
func (w *lifecycleManagerWrapper) EnterAwaitingReconnectOnDisconnect(hostID string) {
	w.lm.EnterAwaitingReconnectOnDisconnect(hostID)
}

// Ensure adapters implement interfaces at compile time.
var _ ops.HostProvider = (*hostProviderAdapter)(nil)
var _ ops.BroadcastSender = (*broadcastSenderAdapter)(nil)
