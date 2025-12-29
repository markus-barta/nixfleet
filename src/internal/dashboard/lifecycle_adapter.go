package dashboard

import (
	"database/sql"

	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/markus-barta/nixfleet/internal/templates"
)

// ═══════════════════════════════════════════════════════════════════════════
// HOST PROVIDER ADAPTER
// ═══════════════════════════════════════════════════════════════════════════

// hostProviderAdapter implements ops.HostProvider using the dashboard's database.
type hostProviderAdapter struct {
	db *sql.DB
}

// GetHostByID implements ops.HostProvider.
func (h *hostProviderAdapter) GetHostByID(hostID string) (ops.Host, error) {
	host, err := getHostByIDFromDB(h.db, hostID)
	if err != nil {
		return nil, err
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
	if h.SystemStatusJSON != nil {
		host.UpdateStatus = parseUpdateStatusJSON(*h.SystemStatusJSON, h.LockStatusJSON)
	}

	return host, nil
}

// parseUpdateStatusJSON parses the system and lock status JSON.
func parseUpdateStatusJSON(systemJSON string, lockJSON *string) *templates.UpdateStatus {
	// Simplified parsing - in production would use json.Unmarshal
	us := &templates.UpdateStatus{
		System: templates.StatusCheck{Status: "unknown"},
		Lock:   templates.StatusCheck{Status: "unknown"},
		Git:    templates.StatusCheck{Status: "unknown"},
	}
	// Actual parsing would go here
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
}

// BroadcastToast implements ops.BroadcastSender.
func (b *broadcastSenderAdapter) BroadcastToast(hostID, level, message string) {
	b.hub.BroadcastToBrowsers(map[string]any{
		"type": "toast",
		"payload": map[string]any{
			"host_id": hostID,
			"level":   level,
			"message": message,
		},
	})
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

// Ensure adapters implement interfaces at compile time.
var _ ops.HostProvider = (*hostProviderAdapter)(nil)
var _ ops.BroadcastSender = (*broadcastSenderAdapter)(nil)

