package ops

import "github.com/markus-barta/nixfleet/internal/templates"

// HostAdapter adapts templates.Host to the ops.Host interface.
type HostAdapter struct {
	host *templates.Host
}

// NewHostAdapter creates a new HostAdapter from a templates.Host.
func NewHostAdapter(host *templates.Host) *HostAdapter {
	return &HostAdapter{host: host}
}

// GetID implements Host.GetID.
func (h *HostAdapter) GetID() string {
	return h.host.ID
}

// GetHostname implements Host.GetHostname.
func (h *HostAdapter) GetHostname() string {
	return h.host.Hostname
}

// GetHostType implements Host.GetHostType.
func (h *HostAdapter) GetHostType() string {
	return h.host.HostType
}

// IsOnline implements Host.IsOnline.
func (h *HostAdapter) IsOnline() bool {
	return h.host.Online
}

// HasPendingCommand implements Host.HasPendingCommand.
func (h *HostAdapter) HasPendingCommand() bool {
	return h.host.PendingCommand != ""
}

// GetPendingCommand implements Host.GetPendingCommand.
func (h *HostAdapter) GetPendingCommand() string {
	return h.host.PendingCommand
}

// GetGeneration implements Host.GetGeneration.
func (h *HostAdapter) GetGeneration() string {
	return h.host.Generation
}

// GetAgentVersion implements Host.GetAgentVersion.
func (h *HostAdapter) GetAgentVersion() string {
	return h.host.AgentVersion
}

// IsAgentOutdated implements Host.IsAgentOutdated.
func (h *HostAdapter) IsAgentOutdated() bool {
	return h.host.AgentOutdated
}

// GetGitStatus implements Host.GetGitStatus.
func (h *HostAdapter) GetGitStatus() string {
	if h.host.UpdateStatus == nil {
		return "unknown"
	}
	return h.host.UpdateStatus.Git.Status
}

// GetLockStatus implements Host.GetLockStatus.
func (h *HostAdapter) GetLockStatus() string {
	if h.host.UpdateStatus == nil {
		return "unknown"
	}
	return h.host.UpdateStatus.Lock.Status
}

// GetSystemStatus implements Host.GetSystemStatus.
func (h *HostAdapter) GetSystemStatus() string {
	if h.host.UpdateStatus == nil {
		return "unknown"
	}
	return h.host.UpdateStatus.System.Status
}

// Underlying returns the underlying templates.Host.
func (h *HostAdapter) Underlying() *templates.Host {
	return h.host
}

// Ensure HostAdapter implements Host interface at compile time.
var _ Host = (*HostAdapter)(nil)

