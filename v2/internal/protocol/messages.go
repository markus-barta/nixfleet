// Package protocol defines the WebSocket message types shared between agent and dashboard.
package protocol

import "encoding/json"

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewMessage creates a message with the given type and payload.
func NewMessage(msgType string, payload any) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    msgType,
		Payload: data,
	}, nil
}

// ParsePayload unmarshals the payload into the given target.
func (m *Message) ParsePayload(target any) error {
	return json.Unmarshal(m.Payload, target)
}

// Message types (agent → dashboard)
const (
	TypeRegister          = "register"
	TypeHeartbeat         = "heartbeat"
	TypeOutput            = "output"
	TypeStatus            = "status"
	TypeRejected          = "command_rejected"
	TypeTestProgress      = "test_progress"
	TypeOperationProgress = "operation_progress" // P2800: phase-by-phase progress
)

// Message types (dashboard → agent)
const (
	TypeRegistered = "registered"
	TypeCommand    = "command"
)

// RegisterPayload is sent by the agent when connecting.
type RegisterPayload struct {
	Hostname          string `json:"hostname"`
	HostType          string `json:"host_type"` // "nixos" or "macos"
	AgentVersion      string `json:"agent_version"`
	OSVersion         string `json:"os_version"`
	NixpkgsVersion    string `json:"nixpkgs_version"`
	Generation        string `json:"generation"`   // git commit hash
	ThemeColor        string `json:"theme_color"`  // hex color like #7aa2f7
	HeartbeatInterval int    `json:"heartbeat_interval"`
	Location          string `json:"location"`    // home, work, cloud
	DeviceType        string `json:"device_type"` // server, desktop, laptop, gaming
	RepoURL           string `json:"repo_url"`    // git repo URL (isolated mode)
	RepoDir           string `json:"repo_dir"`    // local repo path
}

// RegisteredPayload is sent by the dashboard to confirm registration.
type RegisteredPayload struct {
	HostID string `json:"host_id"`
}

// HeartbeatPayload is sent periodically by the agent.
type HeartbeatPayload struct {
	Generation      string        `json:"generation"`
	NixpkgsVersion  string        `json:"nixpkgs_version"`
	PendingCommand  *string       `json:"pending_command"`  // nil if no command running
	CommandPID      *int          `json:"command_pid"`      // nil if no command running
	Metrics         *Metrics      `json:"metrics"`          // nil if StaSysMo not available
	UpdateStatus    *UpdateStatus `json:"update_status"`    // Lock and System status from agent
}

// Metrics contains system metrics from StaSysMo.
type Metrics struct {
	CPU  float64 `json:"cpu"`  // percentage 0-100
	RAM  float64 `json:"ram"`  // percentage 0-100
	Swap float64 `json:"swap"` // percentage 0-100
	Load float64 `json:"load"` // 1-minute load average
}

// CommandPayload is sent by the dashboard to request command execution.
type CommandPayload struct {
	Command string `json:"command"` // "pull", "switch", "test", etc.
}

// OutputPayload is sent by the agent to stream command output.
type OutputPayload struct {
	Line    string `json:"line"`
	Stream  string `json:"stream"`   // "stdout" or "stderr"
	Command string `json:"command"`  // command that produced this output
	IsError bool   `json:"is_error"` // true if this is from stderr
}

// StatusPayload is sent by the agent when a command completes.
type StatusPayload struct {
	Status   string `json:"status"` // "ok" or "error"
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message,omitempty"`
}

// CommandRejectedPayload is sent when a command cannot be executed.
type CommandRejectedPayload struct {
	Reason         string `json:"reason"`
	CurrentCommand string `json:"current_command,omitempty"`
	CurrentPID     int    `json:"current_pid,omitempty"`
}

// TestProgressPayload is sent during test execution.
type TestProgressPayload struct {
	Current int    `json:"current"` // current test number
	Total   int    `json:"total"`   // total tests
	Passed  int    `json:"passed"`  // passed so far
	Running bool   `json:"running"` // still running
	Result  string `json:"result"`  // summary result when done
}

// OperationProgressPayload is sent during command execution (P2800).
// This drives the status column progress dots in the dashboard.
type OperationProgressPayload struct {
	Progress OperationProgress `json:"progress"` // full progress state
}

// CommandProgressPayload is sent during command execution (P2700/P2800).
type CommandProgressPayload struct {
	Command     string `json:"command"`     // "pull", "switch", etc.
	Phase       string `json:"phase"`       // "fetch", "merge", "build", "activate"
	Current     int    `json:"current"`     // current step within phase
	Total       int    `json:"total"`       // total steps in phase
	Description string `json:"description"` // e.g., "Building derivation 12/47"
}

// OperationProgress tracks detailed progress for STATUS column (P2700).
type OperationProgress struct {
	Pull   *PhaseProgress `json:"pull,omitempty"`
	Lock   *PhaseProgress `json:"lock,omitempty"`
	System *PhaseProgress `json:"system,omitempty"`
	Tests  *TestsProgress `json:"tests,omitempty"`
}

// PhaseProgress tracks progress within a single phase.
type PhaseProgress struct {
	Current int    `json:"current"` // current step (0-based)
	Total   int    `json:"total"`   // total steps
	Status  string `json:"status"`  // "pending", "in_progress", "complete", "error"
}

// TestsProgress tracks individual test results.
type TestsProgress struct {
	Current int      `json:"current"` // current test number (0-based)
	Total   int      `json:"total"`   // total tests (capped at 8 for display)
	Results []string `json:"results"` // "pending", "pass", "fail" per test
	Status  string   `json:"status"`  // "pending", "in_progress", "complete"
}

// UpdateStatus contains the three-compartment update status.
type UpdateStatus struct {
	Git    StatusCheck `json:"git"`
	Lock   StatusCheck `json:"lock"`
	System StatusCheck `json:"system"`
}

// StatusCheck represents a single status check result.
type StatusCheck struct {
	Status    string `json:"status"`     // "ok", "outdated", "error", "unknown"
	Message   string `json:"message"`    // Human-readable detail
	CheckedAt string `json:"checked_at"` // ISO timestamp
}

