// Package ops implements the NixFleet Op Engine (CORE-001).
//
// The Op Engine is the single source of truth for all executable actions.
// Every user action, scheduled task, or automated workflow ultimately
// executes one or more Ops.
package ops

import (
	"context"
	"time"
)

// OpExecutor defines where an op runs.
type OpExecutor string

const (
	// ExecutorAgent runs on the host via the agent.
	ExecutorAgent OpExecutor = "agent"
	// ExecutorDashboard runs on the dashboard server.
	ExecutorDashboard OpExecutor = "dashboard"
)

// OpStatus represents the current status of an op execution.
type OpStatus string

const (
	StatusPending    OpStatus = "PENDING"
	StatusValidating OpStatus = "VALIDATING"
	StatusBlocked    OpStatus = "BLOCKED"
	StatusExecuting  OpStatus = "EXECUTING"
	StatusSuccess    OpStatus = "SUCCESS"
	StatusError      OpStatus = "ERROR"
	StatusTimeout    OpStatus = "TIMEOUT"
	StatusSkipped    OpStatus = "SKIPPED"
)

// ValidationError contains details about why validation failed.
type ValidationError struct {
	Code    string `json:"code"`    // Machine-readable code
	Message string `json:"message"` // Human-readable explanation
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Message
}

// Host represents a managed host for op execution.
// This is the minimal interface needed by ops - the full Host struct
// lives in templates package.
type Host interface {
	GetID() string
	GetHostname() string
	GetHostType() string
	IsOnline() bool
	HasPendingCommand() bool
	GetPendingCommand() string
	GetGeneration() string
	GetAgentVersion() string
	IsAgentOutdated() bool
	GetGitStatus() string    // "ok", "outdated", "unknown"
	GetLockStatus() string   // "ok", "outdated", "unknown"
	GetSystemStatus() string // "ok", "outdated", "unknown"
}

// Op defines an atomic operation on a single host.
// See CORE-001 for the full specification.
type Op struct {
	// ID is the unique identifier: "pull", "switch", "test", etc.
	ID string

	// Description is a human-readable description of what this op does.
	Description string

	// Validate checks preconditions before execution.
	// Returns nil if op can proceed, ValidationError if not.
	Validate func(host Host) *ValidationError

	// Execute performs the action.
	// For agent ops, this sends the command and waits for completion.
	// For dashboard ops, this runs the action directly.
	Execute func(ctx context.Context, host Host) error

	// PostCheck verifies success after execution (optional).
	// Returns nil if successful, ValidationError with details if not.
	PostCheck func(host Host) *ValidationError

	// Timeout is the maximum execution time.
	Timeout time.Duration

	// WarningTimeout is when to show a warning (before hard timeout).
	WarningTimeout time.Duration

	// Retryable indicates if this op is safe to retry on failure.
	Retryable bool

	// Executor specifies where the op runs (agent or dashboard).
	Executor OpExecutor

	// RequiresTotp indicates if TOTP verification is required (e.g., reboot).
	RequiresTotp bool
}

// Command represents an op execution record.
// Persisted in the State Store (CORE-003).
type Command struct {
	ID          string    `json:"id"`           // UUID
	HostID      string    `json:"host_id"`      // FK to hosts
	OpID        string    `json:"op"`           // Op ID: "pull", "switch"
	PipelineID  string    `json:"pipeline_id"`  // FK to pipelines (empty if standalone)
	Status      OpStatus  `json:"status"`       // Current status
	CreatedAt   time.Time `json:"created_at"`   // When queued
	StartedAt   time.Time `json:"started_at"`   // When execution began
	FinishedAt  time.Time `json:"finished_at"`  // When completed
	ExitCode    *int      `json:"exit_code"`    // Process exit code (nil if not finished)
	Error       string    `json:"error"`        // Error message if failed
	OutputFile  string    `json:"output_file"`  // Path to output log file
}

// IsTerminal returns true if the status represents a completed command.
func (s OpStatus) IsTerminal() bool {
	switch s {
	case StatusSuccess, StatusError, StatusTimeout, StatusSkipped, StatusBlocked:
		return true
	}
	return false
}

// OpResult contains the outcome of an op execution.
type OpResult struct {
	Command  *Command
	Host     Host
	ExitCode int
	Error    error
	Output   string // Last N lines of output (for quick display)
}

