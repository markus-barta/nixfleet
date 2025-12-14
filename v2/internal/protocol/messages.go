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
	TypeRegister     = "register"
	TypeHeartbeat    = "heartbeat"
	TypeOutput       = "output"
	TypeStatus       = "status"
	TypeRejected     = "command_rejected"
	TypeTestProgress = "test_progress"
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
	Generation        string `json:"generation"` // git commit hash
	HeartbeatInterval int    `json:"heartbeat_interval"`
}

// RegisteredPayload is sent by the dashboard to confirm registration.
type RegisteredPayload struct {
	HostID string `json:"host_id"`
}

// HeartbeatPayload is sent periodically by the agent.
type HeartbeatPayload struct {
	Generation      string   `json:"generation"`
	NixpkgsVersion  string   `json:"nixpkgs_version"`
	PendingCommand  *string  `json:"pending_command"` // nil if no command running
	CommandPID      *int     `json:"command_pid"`     // nil if no command running
	Metrics         *Metrics `json:"metrics"`         // nil if StaSysMo not available
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

