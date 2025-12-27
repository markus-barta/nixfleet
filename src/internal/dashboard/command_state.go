// Package dashboard provides the NixFleet dashboard server.
// This file implements the Command State Machine (P2800).
package dashboard

import (
	"fmt"
	"sync"
	"time"

	"github.com/markus-barta/nixfleet/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND STATES (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// CommandStateType represents the current state of a command.
type CommandStateType string

const (
	StateIdle              CommandStateType = "idle"
	StateValidating        CommandStateType = "validating"
	StateBlocked           CommandStateType = "blocked"
	StateQueued            CommandStateType = "queued"
	StateRunning           CommandStateType = "running"
	StateRunningWarning    CommandStateType = "running_warning"     // Warning timeout hit
	StateAwaitingReconnect CommandStateType = "awaiting_reconnect"  // Switch: waiting for agent restart
	StateTimeoutPending    CommandStateType = "timeout_pending"     // Hard timeout, user action needed
	StateKilling           CommandStateType = "killing"             // SIGTERM sent, waiting
	StateKillFailed        CommandStateType = "kill_failed"         // Process not responding
	StateValidatingPost    CommandStateType = "validating_post"     // Post-validation running
	StateSuccess           CommandStateType = "success"
	StatePartial           CommandStateType = "partial"             // Exit 0 but goal not met
	StateFailed            CommandStateType = "failed"
	StateStaleBinary       CommandStateType = "stale_binary"        // Agent running old binary
	StateSuspicious        CommandStateType = "suspicious"          // Commit changed but binary didn't
	StateTimeout           CommandStateType = "timeout"             // Reconnect timeout
	StateAbortedByReboot   CommandStateType = "aborted_by_reboot"   // P6900: Reboot triggered
	StateIgnored           CommandStateType = "ignored"             // User chose to ignore
	StateKilled            CommandStateType = "killed"              // Command was killed by user
	StateCleanup           CommandStateType = "cleanup"             // Orphaned state being cleaned up
)

// ═══════════════════════════════════════════════════════════════════════════
// TIMEOUT CONFIGURATION (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// TimeoutConfig holds timeout thresholds for commands.
type TimeoutConfig struct {
	WarningTimeout   time.Duration
	HardTimeout      time.Duration
	ReconnectTimeout time.Duration
}

// DefaultTimeouts returns the default timeout configuration.
var DefaultTimeouts = map[string]TimeoutConfig{
	"pull": {
		WarningTimeout:   2 * time.Minute,
		HardTimeout:      5 * time.Minute,
		ReconnectTimeout: 0, // N/A for pull
	},
	"switch": {
		WarningTimeout:   10 * time.Minute,
		HardTimeout:      30 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	},
	"pull-switch": {
		WarningTimeout:   12 * time.Minute,
		HardTimeout:      35 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	},
	"test": {
		WarningTimeout:   5 * time.Minute,
		HardTimeout:      10 * time.Minute,
		ReconnectTimeout: 0, // N/A for test
	},
}

// GetTimeoutConfig returns timeout config for a command type.
func GetTimeoutConfig(command string) TimeoutConfig {
	if cfg, ok := DefaultTimeouts[command]; ok {
		return cfg
	}
	// Default fallback
	return TimeoutConfig{
		WarningTimeout:   5 * time.Minute,
		HardTimeout:      15 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION RESULT TYPES
// ═══════════════════════════════════════════════════════════════════════════

// ValidationResult is returned by all validators.
type ValidationResult struct {
	Valid   bool   `json:"valid"`   // Can proceed?
	Code    string `json:"code"`    // Machine-readable code for UI logic
	Message string `json:"message"` // Human-readable explanation
}

// ═══════════════════════════════════════════════════════════════════════════
// BINARY FRESHNESS (P2810 - 3-Layer Detection)
// ═══════════════════════════════════════════════════════════════════════════

// AgentFreshness tracks 3-layer binary verification.
type AgentFreshness struct {
	SourceCommit string `json:"source_commit"` // Layer 1: Git commit (ldflags)
	StorePath    string `json:"store_path"`    // Layer 2: Nix store path
	BinaryHash   string `json:"binary_hash"`   // Layer 3: SHA256 of binary
}

// FreshnessVerdict is the result of binary freshness comparison.
type FreshnessVerdict string

const (
	FreshnessUnknown    FreshnessVerdict = "unknown"
	FreshnessFresh      FreshnessVerdict = "fresh"       // Binary changed
	FreshnessSuspicious FreshnessVerdict = "suspicious"  // Commit changed, binary didn't
	FreshnessStale      FreshnessVerdict = "stale"       // Nothing changed
)

// CompareFreshness compares before/after freshness to detect stale binaries.
func CompareFreshness(before, after AgentFreshness) (FreshnessVerdict, string) {
	commitChanged := before.SourceCommit != after.SourceCommit && before.SourceCommit != "" && after.SourceCommit != ""
	pathChanged := before.StorePath != after.StorePath && before.StorePath != "" && after.StorePath != ""
	hashChanged := before.BinaryHash != after.BinaryHash && before.BinaryHash != "" && after.BinaryHash != ""

	// If we don't have enough data, can't determine
	if before.StorePath == "" || before.BinaryHash == "" || after.StorePath == "" || after.BinaryHash == "" {
		return FreshnessUnknown, "Insufficient data for binary freshness verification"
	}

	// Any path or hash change = definitely fresh
	if pathChanged || hashChanged {
		return FreshnessFresh, "Agent binary updated successfully"
	}

	// Commit changed but binary didn't = suspicious (cache issue?)
	if commitChanged {
		return FreshnessSuspicious, "Source commit changed but binary unchanged (Nix cache issue?)"
	}

	// Nothing changed = stale
	return FreshnessStale, "Agent running old binary - run nix-collect-garbage -d and switch again"
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND STATE TRACKING (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// CommandState tracks the full lifecycle of a command.
type CommandState struct {
	HostID      string            `json:"host_id"`
	Command     string            `json:"command"`      // "pull", "switch", "test", "pull-switch"
	State       CommandStateType  `json:"state"`        // Current state
	StartedAt   *time.Time        `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at"`
	ExitCode    *int              `json:"exit_code"`
	PreCheck    *ValidationResult `json:"pre_check"`
	PostCheck   *ValidationResult `json:"post_check"`
	Progress    *CommandProgress  `json:"progress"`

	// Timeout tracking
	WarningAt        *time.Time `json:"warning_at,omitempty"`         // When warning was triggered
	TimeoutAt        *time.Time `json:"timeout_at,omitempty"`         // When hard timeout was triggered
	WarningTimeout   *time.Time `json:"warning_timeout,omitempty"`    // When warning timeout will fire
	ReconnectTimeout *time.Time `json:"reconnect_timeout,omitempty"`  // When reconnect timeout will fire
	ExtendedBy       int        `json:"extended_by,omitempty"`        // Minutes timeout extended

	// Kill tracking
	KillInitiatedAt *time.Time `json:"kill_initiated_at,omitempty"`
	KillSignal      string     `json:"kill_signal,omitempty"` // "SIGTERM" or "SIGKILL"
	KillPID         *int       `json:"kill_pid,omitempty"`

	// For reconnection verification
	PreFreshness *AgentFreshness `json:"pre_freshness,omitempty"`

	// Reboot tracking (P6900)
	AbortedCommand string `json:"aborted_command,omitempty"` // Command that was running before reboot
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
	Freshness     *AgentFreshness
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

	// Per-host command state
	hostStates map[string]*CommandState

	// Host snapshots for post-validation
	snapshots map[string]HostSnapshot

	// Pending reboot recovery (P6900)
	pendingRebootRecovery map[string]string // hostID -> aborted command
}

// NewCommandStateMachine creates a new state machine.
func NewCommandStateMachine(log zerolog.Logger, hub *Hub) *CommandStateMachine {
	return &CommandStateMachine{
		log:                   log.With().Str("component", "command_state_machine").Logger(),
		hub:                   hub,
		logStore:              make([]LogEntry, 0, 1000),
		hostStates:            make(map[string]*CommandState),
		snapshots:             make(map[string]HostSnapshot),
		pendingRebootRecovery: make(map[string]string),
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE GETTERS AND SETTERS
// ═══════════════════════════════════════════════════════════════════════════

// GetState returns the current command state for a host.
func (sm *CommandStateMachine) GetState(hostID string) *CommandState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.hostStates[hostID]
}

// SetState updates the command state for a host.
func (sm *CommandStateMachine) SetState(hostID string, state *CommandState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.hostStates[hostID] = state
}

// ClearState removes the command state for a host.
func (sm *CommandStateMachine) ClearState(hostID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.hostStates, hostID)
}

// TransitionTo transitions a host's command to a new state with logging.
func (sm *CommandStateMachine) TransitionTo(hostID string, newState CommandStateType, message string) {
	sm.mu.Lock()
	state := sm.hostStates[hostID]
	if state == nil {
		state = &CommandState{HostID: hostID}
		sm.hostStates[hostID] = state
	}
	oldState := state.State
	state.State = newState

	// Set completion time for terminal states
	if isTerminalState(newState) {
		now := time.Now()
		state.CompletedAt = &now
	}

	sm.mu.Unlock()

	// Log transition
	sm.Log(LogEntry{
		Level:   levelForState(newState),
		HostID:  hostID,
		State:   fmt.Sprintf("%s→%s", oldState, newState),
		Message: message,
		Code:    string(newState),
	})

	// Broadcast state change to browsers
	sm.broadcastStateChange(hostID, state)
}

// isTerminalState returns true for states that end the command lifecycle.
func isTerminalState(state CommandStateType) bool {
	switch state {
	case StateSuccess, StatePartial, StateFailed, StateStaleBinary,
		StateSuspicious, StateTimeout, StateAbortedByReboot, StateIgnored:
		return true
	}
	return false
}

// levelForState returns the appropriate log level for a state.
func levelForState(state CommandStateType) LogLevel {
	switch state {
	case StateSuccess:
		return LogLevelSuccess
	case StatePartial, StateSuspicious, StateRunningWarning:
		return LogLevelWarning
	case StateFailed, StateStaleBinary, StateTimeout, StateKillFailed, StateAbortedByReboot:
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// broadcastStateChange sends state update to browsers.
func (sm *CommandStateMachine) broadcastStateChange(hostID string, state *CommandState) {
	if sm.hub != nil {
		sm.hub.BroadcastToBrowsers(map[string]any{
			"type": "command_state_change",
			"payload": map[string]any{
				"host_id": hostID,
				"state":   state,
			},
		})
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

// StartCommand initializes state for a new command.
func (sm *CommandStateMachine) StartCommand(hostID, command string) {
	now := time.Now()
	sm.mu.Lock()
	sm.hostStates[hostID] = &CommandState{
		HostID:    hostID,
		Command:   command,
		State:     StateValidating,
		StartedAt: &now,
	}
	sm.mu.Unlock()

	sm.Log(LogEntry{
		Level:   LogLevelInfo,
		HostID:  hostID,
		State:   "IDLE→VALIDATING",
		Message: fmt.Sprintf("User clicked %s", command),
	})
}

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

// CaptureSnapshotWithFreshness saves host state including binary freshness.
func (sm *CommandStateMachine) CaptureSnapshotWithFreshness(host *templates.Host, freshness *AgentFreshness) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot := HostSnapshot{
		Generation:    host.Generation,
		AgentVersion:  host.AgentVersion,
		AgentOutdated: host.AgentOutdated,
		Freshness:     freshness,
	}
	if host.UpdateStatus != nil {
		us := *host.UpdateStatus
		snapshot.UpdateStatus = &us
	}
	sm.snapshots[host.ID] = snapshot

	// Also store in command state for reconnection verification
	if state := sm.hostStates[host.ID]; state != nil {
		state.PreFreshness = freshness
	}

	sm.log.Debug().
		Str("host", host.ID).
		Str("generation", host.Generation).
		Interface("freshness", freshness).
		Msg("captured pre-command snapshot with freshness")
}

// GetSnapshot returns the captured snapshot for a host.
func (sm *CommandStateMachine) GetSnapshot(hostID string) *HostSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if snapshot, ok := sm.snapshots[hostID]; ok {
		return &snapshot
	}
	return nil
}

// ClearSnapshot removes the snapshot for a host.
func (sm *CommandStateMachine) ClearSnapshot(hostID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.snapshots, hostID)
}

// RunPostChecks validates command results and returns the outcome.
func (sm *CommandStateMachine) RunPostChecks(hostID, command string, exitCode int, currentHost *templates.Host, agentMessage string) ValidationResult {
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
			// For refresh commands, check if the agent reported an unknown status in the message
			if (command == "refresh-system" || command == "refresh-lock" || command == "refresh-all") && agentMessage == "unknown" {
				return ValidationResult{false, "refresh_unknown", "Status refresh resulted in unknown state"}
			}
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
	case "refresh-system", "refresh-lock", "refresh-all":
		if exitCode == 0 {
			if agentMessage == "unknown" {
				result = ValidationResult{false, "refresh_unknown", "Status refresh resulted in unknown state"}
			} else {
				result = ValidationResult{true, "refresh_ok", "Status refreshed successfully"}
			}
		} else {
			result = ValidationResult{false, "exit_nonzero", fmt.Sprintf("Refresh failed with exit code %d", exitCode)}
		}
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

// ═══════════════════════════════════════════════════════════════════════════
// SWITCH RECONNECTION HANDLING (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// EnterAwaitingReconnect transitions to AWAITING_RECONNECT after switch exit 0.
func (sm *CommandStateMachine) EnterAwaitingReconnect(hostID string) {
	// Set reconnect timeout deadline
	sm.mu.Lock()
	state := sm.hostStates[hostID]
	var command string
	if state != nil {
		command = state.Command
		cfg := GetTimeoutConfig(command)
		timeout := time.Now().Add(cfg.ReconnectTimeout)
		state.ReconnectTimeout = &timeout
	}
	sm.mu.Unlock()

	sm.TransitionTo(hostID, StateAwaitingReconnect, "Switch completed - waiting for agent to restart with new binary")

	// Start reconnect timeout goroutine
	go sm.reconnectTimeoutWatcher(hostID)
}

// reconnectTimeoutWatcher monitors for reconnect timeout.
func (sm *CommandStateMachine) reconnectTimeoutWatcher(hostID string) {
	sm.mu.RLock()
	state := sm.hostStates[hostID]
	var command string
	if state != nil {
		command = state.Command
	}
	sm.mu.RUnlock()

	if command == "" {
		return
	}

	timeoutCfg := GetTimeoutConfig(command)
	if timeoutCfg.ReconnectTimeout == 0 {
		return // No reconnect timeout for this command
	}

	timer := time.NewTimer(timeoutCfg.ReconnectTimeout)
	defer timer.Stop()

	<-timer.C

	// Check if still in AWAITING_RECONNECT
	sm.mu.RLock()
	state = sm.hostStates[hostID]
	stillWaiting := state != nil && state.State == StateAwaitingReconnect
	sm.mu.RUnlock()

	if stillWaiting {
		sm.TransitionTo(hostID, StateTimeout, fmt.Sprintf(
			"Agent did not reconnect within %v - check host status manually",
			timeoutCfg.ReconnectTimeout))
	}
}

// HandleAgentReconnect processes agent reconnection after switch.
func (sm *CommandStateMachine) HandleAgentReconnect(hostID string, freshness AgentFreshness) {
	sm.mu.RLock()
	state := sm.hostStates[hostID]
	wasAwaitingReconnect := state != nil && state.State == StateAwaitingReconnect
	var preFreshness *AgentFreshness
	if state != nil {
		preFreshness = state.PreFreshness
	}
	sm.mu.RUnlock()

	if !wasAwaitingReconnect {
		return // Not waiting for reconnection
	}

	if preFreshness == nil {
		sm.Log(LogEntry{
			Level:   LogLevelWarning,
			HostID:  hostID,
			State:   "POST-RECONNECT",
			Message: "No pre-switch freshness data - cannot verify binary change",
		})
		sm.TransitionTo(hostID, StateSuccess, "Switch completed (verification skipped - no baseline)")
		return
	}

	// 3-layer binary freshness verification
	sm.Log(LogEntry{
		Level:  LogLevelInfo,
		HostID: hostID,
		State:  "POST-RECONNECT",
		Message: fmt.Sprintf("Binary freshness check - Before: commit=%s path=%s hash=%s",
			shortHash(preFreshness.SourceCommit),
			shortPath(preFreshness.StorePath),
			shortHash(preFreshness.BinaryHash)),
	})
	sm.Log(LogEntry{
		Level:  LogLevelInfo,
		HostID: hostID,
		State:  "POST-RECONNECT",
		Message: fmt.Sprintf("Binary freshness check - After:  commit=%s path=%s hash=%s",
			shortHash(freshness.SourceCommit),
			shortPath(freshness.StorePath),
			shortHash(freshness.BinaryHash)),
	})

	verdict, message := CompareFreshness(*preFreshness, freshness)

	switch verdict {
	case FreshnessFresh:
		sm.TransitionTo(hostID, StateSuccess, message)
	case FreshnessSuspicious:
		sm.TransitionTo(hostID, StateSuspicious, message)
	case FreshnessStale:
		sm.TransitionTo(hostID, StateStaleBinary, message)
	default:
		sm.TransitionTo(hostID, StateSuccess, "Switch completed (freshness verification inconclusive)")
	}

	// Clean up state
	sm.ClearSnapshot(hostID)
}

// ═══════════════════════════════════════════════════════════════════════════
// TIMEOUT HANDLING (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// CheckTimeouts checks all running commands for timeout conditions.
// Called periodically by the dashboard.
func (sm *CommandStateMachine) CheckTimeouts() {
	sm.mu.RLock()
	var hostsToCheck []string
	for hostID, state := range sm.hostStates {
		if state.State == StateRunning || state.State == StateRunningWarning {
			hostsToCheck = append(hostsToCheck, hostID)
		}
	}
	sm.mu.RUnlock()

	now := time.Now()
	for _, hostID := range hostsToCheck {
		sm.mu.RLock()
		state := sm.hostStates[hostID]
		if state == nil || state.StartedAt == nil {
			sm.mu.RUnlock()
			continue
		}
		elapsed := now.Sub(*state.StartedAt)
		command := state.Command
		currentState := state.State
		sm.mu.RUnlock()

		timeoutCfg := GetTimeoutConfig(command)

		// Check hard timeout first
		if elapsed >= timeoutCfg.HardTimeout && currentState != StateTimeoutPending {
			sm.TransitionTo(hostID, StateTimeoutPending,
				fmt.Sprintf("%s exceeded hard timeout (%v) - user action required", command, timeoutCfg.HardTimeout))
			continue
		}

		// Check warning timeout
		if elapsed >= timeoutCfg.WarningTimeout && currentState == StateRunning {
			sm.TransitionTo(hostID, StateRunningWarning,
				fmt.Sprintf("%s taking longer than expected (>%v)", command, timeoutCfg.WarningTimeout))
		}
	}
}

// ExtendTimeout extends the timeout for a host's command.
func (sm *CommandStateMachine) ExtendTimeout(hostID string, minutes int) {
	sm.mu.Lock()
	state := sm.hostStates[hostID]
	if state != nil {
		state.ExtendedBy += minutes
		newTimeout := time.Now().Add(time.Duration(minutes) * time.Minute)
		state.WarningTimeout = &newTimeout
	}
	sm.mu.Unlock()

	sm.Log(LogEntry{
		Level:   LogLevelInfo,
		HostID:  hostID,
		State:   "TIMEOUT",
		Message: fmt.Sprintf("Timeout extended by %d minutes", minutes),
	})

	sm.TransitionTo(hostID, StateRunning, fmt.Sprintf("Timeout extended by %d minutes - continuing", minutes))
}

// ═══════════════════════════════════════════════════════════════════════════
// KILL HANDLING (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// InitiateKill marks the start of kill process.
func (sm *CommandStateMachine) InitiateKill(hostID string, signal string, pid int) {
	now := time.Now()
	sm.mu.Lock()
	state := sm.hostStates[hostID]
	if state != nil {
		state.KillInitiatedAt = &now
		state.KillSignal = signal
		state.KillPID = &pid
	}
	sm.mu.Unlock()

	sm.TransitionTo(hostID, StateKilling,
		fmt.Sprintf("Sending %s to PID %d", signal, pid))
}

// MarkKillFailed marks that kill did not succeed.
func (sm *CommandStateMachine) MarkKillFailed(hostID string) {
	sm.TransitionTo(hostID, StateKillFailed,
		"Process not responding to kill signal - consider host reboot")
}

// MarkIgnored marks command as ignored (user chose to stop monitoring).
func (sm *CommandStateMachine) MarkIgnored(hostID string) {
	sm.TransitionTo(hostID, StateIgnored, "Command ignored by user")
	sm.ClearSnapshot(hostID)
}

// ═══════════════════════════════════════════════════════════════════════════
// REBOOT INTEGRATION (P6900)
// ═══════════════════════════════════════════════════════════════════════════

// HandleRebootTriggered processes reboot command when a command is pending.
func (sm *CommandStateMachine) HandleRebootTriggered(hostID string) {
	sm.mu.Lock()
	state := sm.hostStates[hostID]
	if state == nil || state.State == StateIdle {
		sm.mu.Unlock()
		return
	}

	abortedCommand := state.Command
	sm.pendingRebootRecovery[hostID] = abortedCommand
	sm.mu.Unlock()

	sm.Log(LogEntry{
		Level:   LogLevelWarning,
		HostID:  hostID,
		State:   fmt.Sprintf("%s→ABORTED", state.State),
		Message: fmt.Sprintf("Command '%s' aborted due to host reboot", abortedCommand),
		Code:    "aborted_by_reboot",
	})

	sm.ClearSnapshot(hostID)
	sm.TransitionTo(hostID, StateAbortedByReboot, "Host reboot initiated")
}

// GetPendingRebootRecovery returns the command that was aborted by reboot.
func (sm *CommandStateMachine) GetPendingRebootRecovery(hostID string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.pendingRebootRecovery[hostID]
}

// ClearPendingRebootRecovery clears reboot recovery marker.
func (sm *CommandStateMachine) ClearPendingRebootRecovery(hostID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.pendingRebootRecovery, hostID)
}

// HandlePostRebootReconnect processes agent reconnection after a reboot.
func (sm *CommandStateMachine) HandlePostRebootReconnect(hostID string) bool {
	abortedCommand := sm.GetPendingRebootRecovery(hostID)
	if abortedCommand == "" {
		return false
	}

	sm.Log(LogEntry{
		Level:   LogLevelWarning,
		HostID:  hostID,
		State:   "POST_REBOOT",
		Message: fmt.Sprintf("Host rebooted during '%s'. Manual verification may be needed.", abortedCommand),
		Code:    "post_reboot_recovery",
	})

	sm.ClearPendingRebootRecovery(hostID)
	sm.TransitionTo(hostID, StateIdle, "Recovered after reboot")

	// Notify UI via toast
	if sm.hub != nil {
		sm.hub.BroadcastToBrowsers(map[string]any{
			"type": "toast",
			"payload": map[string]any{
				"host_id": hostID,
				"level":   "warning",
				"message": fmt.Sprintf("%s rebooted during %s. Verify system state manually.", hostID, abortedCommand),
			},
		})
	}

	return true
}

// ═══════════════════════════════════════════════════════════════════════════
// SELF-HEALING (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// DetectOrphanedStates finds commands stuck in non-terminal states.
func (sm *CommandStateMachine) DetectOrphanedStates(threshold time.Duration) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var orphaned []string
	now := time.Now()

	for hostID, state := range sm.hostStates {
		if isTerminalState(state.State) {
			continue
		}
		if state.StartedAt != nil && now.Sub(*state.StartedAt) > threshold {
			orphaned = append(orphaned, hostID)
		}
	}

	return orphaned
}

// CleanupOrphanedState clears a stuck state with logging.
func (sm *CommandStateMachine) CleanupOrphanedState(hostID string) {
	sm.Log(LogEntry{
		Level:   LogLevelWarning,
		HostID:  hostID,
		State:   "CLEANUP",
		Message: "Orphaned command state detected and cleared",
		Code:    "orphaned_cleanup",
	})

	// Transition to CLEANUP state instead of clearing
	sm.TransitionTo(hostID, StateCleanup, "orphaned state cleaned up")
	sm.ClearSnapshot(hostID)
}

// ═══════════════════════════════════════════════════════════════════════════
// LOGGING
// ═══════════════════════════════════════════════════════════════════════════

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
	var event *zerolog.Event
	switch entry.Level {
	case LogLevelError:
		event = sm.log.Error()
	case LogLevelWarning:
		event = sm.log.Warn()
	case LogLevelDebug:
		event = sm.log.Debug()
	default:
		event = sm.log.Info()
	}
	if entry.HostID != "" {
		event = event.Str("host", entry.HostID)
	}
	event.Str("state", entry.State).
		Str("code", entry.Code).
		Msg(entry.Message)

	// Broadcast to browsers (if hub is available)
	if sm.hub != nil {
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

// GetAllHostStates returns a copy of all host states (for API).
func (sm *CommandStateMachine) GetAllHostStates() map[string]*CommandState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*CommandState, len(sm.hostStates))
	for k, v := range sm.hostStates {
		// Copy to avoid races
		stateCopy := *v
		result[k] = &stateCopy
	}
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

func shortPath(path string) string {
	if len(path) > 40 {
		return "..." + path[len(path)-37:]
	}
	return path
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
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
