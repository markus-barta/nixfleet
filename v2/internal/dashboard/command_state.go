// Package dashboard provides the NixFleet dashboard server.
// This file implements the Command State Machine (P2800).
package dashboard

import (
	"fmt"
	"sync"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION RESULT TYPES
// ═══════════════════════════════════════════════════════════════════════════

// ValidationResult is returned by all validators.
type ValidationResult struct {
	Valid   bool   `json:"valid"`   // Can proceed?
	Code    string `json:"code"`    // Machine-readable code for UI logic
	Message string `json:"message"` // Human-readable explanation
}

// CommandState tracks the full lifecycle of a command.
type CommandState struct {
	HostID      string            `json:"host_id"`
	Command     string            `json:"command"`      // "pull", "switch", "test", "pull-switch"
	State       string            `json:"state"`        // "idle", "validating", "queued", "running", "validating-post", "success", "partial", "failed"
	StartedAt   *time.Time        `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at"`
	ExitCode    *int              `json:"exit_code"`
	PreCheck    *ValidationResult `json:"pre_check"`
	PostCheck   *ValidationResult `json:"post_check"`
	Progress    *CommandProgress  `json:"progress"`
}

// CommandProgress tracks progress during command execution.
type CommandProgress struct {
	Phase       string `json:"phase"`       // "fetching", "building", "activating", etc.
	Current     int    `json:"current"`     // e.g., 12
	Total       int    `json:"total"`       // e.g., 47
	Description string `json:"description"` // "Building derivation foo..."
}

// HostSnapshot captures host state before a command for post-validation comparison.
type HostSnapshot struct {
	Generation    string
	AgentVersion  string
	AgentOutdated bool
	UpdateStatus  *templates.UpdateStatus
}

// ═══════════════════════════════════════════════════════════════════════════
// LOG ENTRY TYPES (P2800 Verbose Logging)
// ═══════════════════════════════════════════════════════════════════════════

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelSuccess LogLevel = "success"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// LogEntry represents a state machine log entry for the System Log.
type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	HostID    string         `json:"host_id,omitempty"` // Empty for bulk/system messages
	State     string         `json:"state"`             // Current state machine state
	Message   string         `json:"message"`           // Human-readable message
	Code      string         `json:"code"`              // Machine-readable code for filtering
	Details   map[string]any `json:"details,omitempty"` // Optional structured data
}

// LogIcon returns the icon for a log level.
func (l LogLevel) Icon() string {
	switch l {
	case LogLevelDebug:
		return "·"
	case LogLevelInfo:
		return "ℹ"
	case LogLevelSuccess:
		return "✓"
	case LogLevelWarning:
		return "⚠"
	case LogLevelError:
		return "✗"
	default:
		return "?"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND STATE MACHINE
// ═══════════════════════════════════════════════════════════════════════════

// CommandStateMachine manages command state and validation.
type CommandStateMachine struct {
	log      zerolog.Logger
	hub      *Hub
	logStore []LogEntry // In-memory log store (bounded)
	mu       sync.RWMutex

	// Host snapshots for post-validation
	snapshots map[string]HostSnapshot
}

// NewCommandStateMachine creates a new state machine.
func NewCommandStateMachine(log zerolog.Logger, hub *Hub) *CommandStateMachine {
	return &CommandStateMachine{
		log:       log.With().Str("component", "command_state_machine").Logger(),
		hub:       hub,
		logStore:  make([]LogEntry, 0, 1000),
		snapshots: make(map[string]HostSnapshot),
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PRE-CONDITION VALIDATORS
// Each function checks ONE thing. Combine with AND logic for full validation.
// ═══════════════════════════════════════════════════════════════════════════

// CanExecuteCommand checks if ANY command can run on this host.
func CanExecuteCommand(host *templates.Host) ValidationResult {
	if !host.Online {
		return ValidationResult{false, "host_offline", "Host is offline"}
	}
	if host.PendingCommand != "" {
		return ValidationResult{false, "command_pending",
			fmt.Sprintf("Command '%s' already running", host.PendingCommand)}
	}
	return ValidationResult{true, "ok", "Host ready for commands"}
}

// CanPull checks if Pull is meaningful for this host.
func CanPull(host *templates.Host) ValidationResult {
	base := CanExecuteCommand(host)
	if !base.Valid {
		return base
	}

	if host.UpdateStatus == nil {
		return ValidationResult{true, "unknown_state",
			"Git status unknown - pull may help"}
	}

	git := host.UpdateStatus.Git
	if git.Status == "ok" {
		return ValidationResult{false, "already_current",
			"Git already up to date"}
	}
	if git.Status == "unknown" {
		return ValidationResult{true, "unknown_state",
			"Git status unknown - pull may help"}
	}
	// git.Status == "outdated"
	return ValidationResult{true, "outdated",
		fmt.Sprintf("Git outdated: %s", git.Message)}
}

// CanSwitch checks if Switch is meaningful for this host.
func CanSwitch(host *templates.Host) ValidationResult {
	base := CanExecuteCommand(host)
	if !base.Valid {
		return base
	}

	if host.UpdateStatus == nil {
		return ValidationResult{true, "unknown_state",
			"System status unknown - switch may help"}
	}

	// Check if git is current (prerequisite for meaningful switch)
	git := host.UpdateStatus.Git
	if git.Status == "outdated" {
		return ValidationResult{false, "git_outdated",
			"Pull required before switch (git outdated)"}
	}

	system := host.UpdateStatus.System
	if system.Status == "ok" && !host.AgentOutdated {
		return ValidationResult{false, "already_current",
			"System already up to date"}
	}
	if system.Status == "unknown" {
		return ValidationResult{true, "unknown_state",
			"System status unknown - switch may help"}
	}
	// system.Status == "outdated" or agent outdated
	msg := "System outdated"
	if system.Message != "" {
		msg = system.Message
	}
	if host.AgentOutdated {
		msg = "Agent outdated - switch to update"
	}
	return ValidationResult{true, "outdated", msg}
}

// CanTest checks if Test is meaningful for this host.
func CanTest(host *templates.Host) ValidationResult {
	base := CanExecuteCommand(host)
	if !base.Valid {
		return base
	}
	// Test can always run if host is online and not busy
	return ValidationResult{true, "ok", "Ready to test"}
}

// CanPullSwitch checks if Pull+Switch sequence is meaningful.
func CanPullSwitch(host *templates.Host) ValidationResult {
	base := CanExecuteCommand(host)
	if !base.Valid {
		return base
	}

	if host.UpdateStatus == nil {
		return ValidationResult{true, "unknown_state",
			"Status unknown - pull+switch may help"}
	}

	// At least one of git or system should need update
	git := host.UpdateStatus.Git
	system := host.UpdateStatus.System

	if git.Status == "ok" && system.Status == "ok" && !host.AgentOutdated {
		return ValidationResult{false, "already_current",
			"Both git and system already up to date"}
	}

	return ValidationResult{true, "ok", "Ready for pull + switch"}
}

// ═══════════════════════════════════════════════════════════════════════════
// POST-CONDITION VALIDATORS
// Each function checks if the command achieved its GOAL, not just exit code.
// ═══════════════════════════════════════════════════════════════════════════

// ValidatePullResult checks if Pull achieved its goal.
func ValidatePullResult(before, after HostSnapshot, exitCode int) ValidationResult {
	if exitCode != 0 {
		return ValidationResult{false, "exit_nonzero",
			fmt.Sprintf("Pull failed with exit code %d", exitCode)}
	}

	// Goal: Git compartment should now be green (or at least different)
	if after.UpdateStatus != nil && after.UpdateStatus.Git.Status == "ok" {
		return ValidationResult{true, "goal_achieved",
			"Pull successful - git now up to date"}
	}

	// Check if generation changed (partial success)
	if before.Generation != after.Generation {
		return ValidationResult{true, "partial",
			fmt.Sprintf("Pull completed - generation changed (%s → %s)",
				shortHash(before.Generation), shortHash(after.Generation))}
	}

	return ValidationResult{false, "goal_not_achieved",
		"Pull completed (exit 0) but git compartment still outdated"}
}

// ValidateSwitchResult checks if Switch achieved its goal.
func ValidateSwitchResult(before, after HostSnapshot, exitCode int) ValidationResult {
	if exitCode != 0 {
		return ValidationResult{false, "exit_nonzero",
			fmt.Sprintf("Switch failed with exit code %d", exitCode)}
	}

	// Goal: System compartment should now be green
	if after.UpdateStatus != nil && after.UpdateStatus.System.Status == "ok" {
		// Bonus check: agent version should match if agent was updated
		if before.AgentOutdated && !after.AgentOutdated {
			return ValidationResult{true, "goal_achieved_with_agent",
				"Switch successful - system current, agent updated"}
		}
		return ValidationResult{true, "goal_achieved",
			"Switch successful - system now up to date"}
	}

	// Check if we're waiting for agent restart
	if after.AgentOutdated && before.AgentVersion != after.AgentVersion {
		return ValidationResult{true, "pending_restart",
			"Switch completed - waiting for agent restart"}
	}

	return ValidationResult{false, "goal_not_achieved",
		"Switch completed (exit 0) but system compartment still outdated"}
}

// ValidateTestResult checks if Test passed.
func ValidateTestResult(exitCode int) ValidationResult {
	if exitCode != 0 {
		return ValidationResult{false, "test_failed",
			fmt.Sprintf("Test failed with exit code %d", exitCode)}
	}
	return ValidationResult{true, "test_passed", "All tests passed"}
}

// ValidatePullSwitchResult checks if Pull+Switch achieved combined goal.
func ValidatePullSwitchResult(before, after HostSnapshot, exitCode int) ValidationResult {
	if exitCode != 0 {
		return ValidationResult{false, "exit_nonzero",
			fmt.Sprintf("Pull+Switch failed with exit code %d", exitCode)}
	}

	gitOK := after.UpdateStatus != nil && after.UpdateStatus.Git.Status == "ok"
	systemOK := after.UpdateStatus != nil && after.UpdateStatus.System.Status == "ok"

	if gitOK && systemOK {
		return ValidationResult{true, "goal_achieved",
			"Pull+Switch successful - fully up to date"}
	}

	if gitOK && !systemOK {
		return ValidationResult{false, "partial_git_only",
			"Pull succeeded but switch did not update system"}
	}

	if !gitOK && systemOK {
		return ValidationResult{true, "partial_system_only",
			"System updated but git still shows outdated (may be stale)"}
	}

	return ValidationResult{false, "goal_not_achieved",
		"Pull+Switch completed but neither git nor system updated"}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE MACHINE METHODS
// ═══════════════════════════════════════════════════════════════════════════

// RunPreChecks validates a command before execution.
// Returns the validation result and logs the process.
func (sm *CommandStateMachine) RunPreChecks(host *templates.Host, command string) ValidationResult {
	sm.Log(LogEntry{
		Level:   LogLevelInfo,
		HostID:  host.ID,
		State:   "PRE-CHECK",
		Message: fmt.Sprintf("Checking CanExecuteCommand for %s...", command),
	})

	// Check base execution capability
	baseResult := CanExecuteCommand(host)
	sm.Log(LogEntry{
		Level:   levelFromValid(baseResult.Valid),
		HostID:  host.ID,
		State:   "PRE-CHECK",
		Message: fmt.Sprintf("CanExecuteCommand: %s (%s)", passOrFail(baseResult.Valid), baseResult.Message),
		Code:    baseResult.Code,
		Details: map[string]any{
			"valid":   baseResult.Valid,
			"online":  host.Online,
			"pending": host.PendingCommand,
		},
	})

	if !baseResult.Valid {
		return baseResult
	}

	// Check command-specific preconditions
	var result ValidationResult
	switch command {
	case "pull":
		result = CanPull(host)
	case "switch":
		result = CanSwitch(host)
	case "test":
		result = CanTest(host)
	case "pull-switch":
		result = CanPullSwitch(host)
	default:
		result = ValidationResult{true, "unknown_command", "Unknown command - proceeding anyway"}
	}

	sm.Log(LogEntry{
		Level:   levelFromValid(result.Valid),
		HostID:  host.ID,
		State:   "PRE-CHECK",
		Message: fmt.Sprintf("Can%s: %s (%s)", capitalize(command), passOrFail(result.Valid), result.Message),
		Code:    result.Code,
	})

	return result
}

// CaptureSnapshot saves host state before a command for post-validation.
func (sm *CommandStateMachine) CaptureSnapshot(host *templates.Host) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot := HostSnapshot{
		Generation:    host.Generation,
		AgentVersion:  host.AgentVersion,
		AgentOutdated: host.AgentOutdated,
	}
	if host.UpdateStatus != nil {
		// Deep copy update status
		us := *host.UpdateStatus
		snapshot.UpdateStatus = &us
	}
	sm.snapshots[host.ID] = snapshot

	sm.log.Debug().
		Str("host", host.ID).
		Str("generation", host.Generation).
		Msg("captured pre-command snapshot")
}

// RunPostChecks validates command results and returns the outcome.
func (sm *CommandStateMachine) RunPostChecks(hostID, command string, exitCode int, currentHost *templates.Host) ValidationResult {
	sm.mu.RLock()
	before, hasBefore := sm.snapshots[hostID]
	sm.mu.RUnlock()

	if !hasBefore {
		sm.Log(LogEntry{
			Level:   LogLevelWarning,
			HostID:  hostID,
			State:   "POST-CHECK",
			Message: "No pre-command snapshot found, skipping detailed validation",
		})
		if exitCode == 0 {
			return ValidationResult{true, "exit_zero", "Command completed successfully"}
		}
		return ValidationResult{false, "exit_nonzero", fmt.Sprintf("Command failed with exit code %d", exitCode)}
	}

	// Build after snapshot from current host
	after := HostSnapshot{
		Generation:    currentHost.Generation,
		AgentVersion:  currentHost.AgentVersion,
		AgentOutdated: currentHost.AgentOutdated,
	}
	if currentHost.UpdateStatus != nil {
		us := *currentHost.UpdateStatus
		after.UpdateStatus = &us
	}

	sm.Log(LogEntry{
		Level:   LogLevelInfo,
		HostID:  hostID,
		State:   "POST-CHECK",
		Message: fmt.Sprintf("Running Validate%sResult...", capitalize(command)),
	})

	var result ValidationResult
	switch command {
	case "pull":
		result = ValidatePullResult(before, after, exitCode)
	case "switch":
		result = ValidateSwitchResult(before, after, exitCode)
	case "test":
		result = ValidateTestResult(exitCode)
	case "pull-switch":
		result = ValidatePullSwitchResult(before, after, exitCode)
	default:
		if exitCode == 0 {
			result = ValidationResult{true, "exit_zero", "Command completed successfully"}
		} else {
			result = ValidationResult{false, "exit_nonzero", fmt.Sprintf("Command failed with exit code %d", exitCode)}
		}
	}

	sm.Log(LogEntry{
		Level:   levelFromValid(result.Valid),
		HostID:  hostID,
		State:   "POST-CHECK",
		Message: fmt.Sprintf("Validate%sResult: %s (%s)", capitalize(command), passOrFail(result.Valid), result.Message),
		Code:    result.Code,
		Details: map[string]any{
			"before_generation": before.Generation,
			"after_generation":  after.Generation,
			"exit_code":         exitCode,
		},
	})

	// Clean up snapshot
	sm.mu.Lock()
	delete(sm.snapshots, hostID)
	sm.mu.Unlock()

	return result
}

// Log adds a log entry and broadcasts to browsers.
func (sm *CommandStateMachine) Log(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Store locally (bounded)
	sm.mu.Lock()
	if len(sm.logStore) >= 1000 {
		sm.logStore = sm.logStore[100:] // Remove oldest 100 entries
	}
	sm.logStore = append(sm.logStore, entry)
	sm.mu.Unlock()

	// Log to zerolog
	event := sm.log.Info()
	if entry.Level == LogLevelError {
		event = sm.log.Error()
	} else if entry.Level == LogLevelWarning {
		event = sm.log.Warn()
	} else if entry.Level == LogLevelDebug {
		event = sm.log.Debug()
	}
	if entry.HostID != "" {
		event = event.Str("host", entry.HostID)
	}
	event.Str("state", entry.State).
		Str("code", entry.Code).
		Msg(entry.Message)

	// Broadcast to browsers
	sm.hub.BroadcastToBrowsers(map[string]any{
		"type": "state_machine_log",
		"payload": map[string]any{
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"level":     entry.Level,
			"host_id":   entry.HostID,
			"state":     entry.State,
			"message":   entry.Message,
			"code":      entry.Code,
			"icon":      entry.Level.Icon(),
		},
	})
}

// GetRecentLogs returns the most recent log entries.
func (sm *CommandStateMachine) GetRecentLogs(limit int) []LogEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if limit <= 0 || limit > len(sm.logStore) {
		limit = len(sm.logStore)
	}

	start := len(sm.logStore) - limit
	result := make([]LogEntry, limit)
	copy(result, sm.logStore[start:])
	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════

func shortHash(hash string) string {
	if len(hash) >= 7 {
		return hash[:7]
	}
	return hash
}

func levelFromValid(valid bool) LogLevel {
	if valid {
		return LogLevelSuccess
	}
	return LogLevelError
}

func passOrFail(valid bool) string {
	if valid {
		return "PASS"
	}
	return "FAIL"
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

