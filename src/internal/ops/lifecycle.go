// Package ops - lifecycle.go implements complete command lifecycle management.
//
// This replaces the CommandStateMachine with a cleaner architecture:
// - Single source of truth: ActiveCommand
// - All lifecycle in one place
// - Goroutine-based timeout/reconnect watchers
// - Clean interfaces for Hub integration
package ops

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// EXTENDED STATUS VALUES
// ═══════════════════════════════════════════════════════════════════════════

// Additional status values for lifecycle management
const (
	StatusRunningWarning    OpStatus = "RUNNING_WARNING"    // Warning timeout hit
	StatusTimeoutPending    OpStatus = "TIMEOUT_PENDING"    // Hard timeout, user action needed
	StatusAwaitingReconnect OpStatus = "AWAITING_RECONNECT" // Switch: waiting for agent restart
	StatusKilling           OpStatus = "KILLING"            // SIGTERM sent, waiting
	StatusKilled            OpStatus = "KILLED"             // Command was killed by user
	StatusPartial           OpStatus = "PARTIAL"            // Exit 0 but goal not met
	StatusStaleBinary       OpStatus = "STALE_BINARY"       // Agent running old binary after switch
	StatusSuspicious        OpStatus = "SUSPICIOUS"         // Commit changed but binary didn't
)

// ═══════════════════════════════════════════════════════════════════════════
// ACTIVE COMMAND (Complete lifecycle state)
// ═══════════════════════════════════════════════════════════════════════════

// ActiveCommand extends Command with full lifecycle state.
// This is the single source of truth for command status.
type ActiveCommand struct {
	Command // Embedded base command

	// Timeout tracking
	WarningAt      *time.Time `json:"warning_at,omitempty"`       // When warning was triggered
	HardTimeoutAt  *time.Time `json:"hard_timeout_at,omitempty"`  // When hard timeout was triggered
	TimeoutConfig  TimeoutConfig `json:"-"`                       // Timeout thresholds
	TimeoutExtended time.Duration `json:"timeout_extended,omitempty"` // User-extended duration

	// Kill tracking
	KillInitiatedAt *time.Time `json:"kill_initiated_at,omitempty"`
	KillSignal      string     `json:"kill_signal,omitempty"` // "SIGTERM" or "SIGKILL"
	KillPID         int        `json:"kill_pid,omitempty"`

	// Reconnect tracking (for switch commands)
	ReconnectDeadline *time.Time      `json:"reconnect_deadline,omitempty"`
	PreFreshness      *AgentFreshness `json:"pre_freshness,omitempty"`

	// Pre-command snapshot for post-validation
	PreSnapshot *HostSnapshot `json:"-"`

	// Deferred post-check (wait for fresh heartbeat)
	PostCheckDeferred bool   `json:"post_check_deferred,omitempty"`
	DeferredExitCode  *int   `json:"deferred_exit_code,omitempty"`
	DeferredMessage   string `json:"deferred_message,omitempty"`

	// Lifecycle control
	cancelTimeout   chan struct{} // Signal to stop timeout watcher
	cancelReconnect chan struct{} // Signal to stop reconnect watcher
}

// ═══════════════════════════════════════════════════════════════════════════
// LIFECYCLE MANAGER
// ═══════════════════════════════════════════════════════════════════════════

// HostProvider provides current host state for post-checks.
// Implemented by dashboard to fetch fresh data from DB.
type HostProvider interface {
	GetHostByID(hostID string) (Host, error)
}

// BroadcastSender sends state changes to connected browsers.
type BroadcastSender interface {
	BroadcastCommandState(hostID string, cmd *ActiveCommand)
	BroadcastToast(hostID, level, message string)
}

// PendingCommandStore manages the pending_command column in hosts table.
// This is the SINGLE SOURCE OF TRUTH for command state in the database.
// Hub should NOT update pending_command directly - only LifecycleManager does.
type PendingCommandStore interface {
	// SetPendingCommand sets the pending_command for a host (nil to clear)
	SetPendingCommand(hostID string, command *string) error
	// ClearPendingCommand clears the pending_command for a host
	ClearPendingCommand(hostID string) error
}

// LifecycleManager handles the complete command lifecycle.
// Replaces CommandStateMachine.
// IMPORTANT: This is the SINGLE SOURCE OF TRUTH for pending_command state.
type LifecycleManager struct {
	log       zerolog.Logger
	registry  *Registry
	sender    CommandSender
	store     StateStore
	events    EventLogger
	hosts     HostProvider
	broadcast BroadcastSender
	pending   PendingCommandStore // P1100: Single source of truth for pending_command

	// Active commands per host
	active   map[string]*ActiveCommand
	activeMu sync.RWMutex

	// Shutdown signal
	done chan struct{}
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager(
	log zerolog.Logger,
	registry *Registry,
	sender CommandSender,
	store StateStore,
	events EventLogger,
) *LifecycleManager {
	return &LifecycleManager{
		log:      log.With().Str("component", "lifecycle_manager").Logger(),
		registry: registry,
		sender:   sender,
		store:    store,
		events:   events,
		active:   make(map[string]*ActiveCommand),
		done:     make(chan struct{}),
	}
}

// SetHostProvider sets the host provider (called after dashboard init).
func (lm *LifecycleManager) SetHostProvider(hp HostProvider) {
	lm.hosts = hp
}

// SetBroadcastSender sets the broadcast sender (called after hub init).
func (lm *LifecycleManager) SetBroadcastSender(bs BroadcastSender) {
	lm.broadcast = bs
}

// SetPendingCommandStore sets the pending command store (called after hub init).
// P1100: This makes LifecycleManager the single source of truth for pending_command.
func (lm *LifecycleManager) SetPendingCommandStore(pcs PendingCommandStore) {
	lm.pending = pcs
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND EXECUTION
// ═══════════════════════════════════════════════════════════════════════════

// ExecuteOp starts a command with full lifecycle management.
func (lm *LifecycleManager) ExecuteOp(opID string, host Host, force bool) (*ActiveCommand, error) {
	op := lm.registry.Get(opID)
	if op == nil {
		return nil, &ValidationError{Code: "unknown_op", Message: "Unknown operation: " + opID}
	}

	hostID := host.GetID()

	// Check for existing active command
	lm.activeMu.RLock()
	existing := lm.active[hostID]
	lm.activeMu.RUnlock()

	if existing != nil && !existing.Status.IsTerminal() {
		return nil, &ValidationError{
			Code:    "command_pending",
			Message: "Command '" + existing.OpID + "' already running",
		}
	}

	// Create active command
	now := time.Now()
	cmd := &ActiveCommand{
		Command: Command{
			ID:        generateUUID(),
			HostID:    hostID,
			OpID:      opID,
			Status:    StatusPending,
			CreatedAt: now,
		},
		TimeoutConfig:   GetTimeoutConfig(opID),
		cancelTimeout:   make(chan struct{}),
		cancelReconnect: make(chan struct{}),
	}

	// Capture pre-snapshot for post-validation
	cmd.PreSnapshot = captureHostSnapshot(host)

	// For switch commands, capture agent freshness
	if opID == "switch" || opID == "pull-switch" {
		cmd.PreFreshness = lm.getAgentFreshness(hostID)
	}

	// Persist to store
	if lm.store != nil {
		if err := lm.store.CreateCommand(&cmd.Command); err != nil {
			lm.log.Error().Err(err).Str("op", opID).Str("host", hostID).Msg("failed to persist command")
		}
	}

	// Track as active
	lm.activeMu.Lock()
	lm.active[hostID] = cmd
	lm.activeMu.Unlock()

	// Validate (unless forced)
	cmd.Status = StatusValidating
	lm.updateAndBroadcast(cmd)
	lm.logEvent("info", hostID, opID, "Validating "+opID)

	if !force && op.Validate != nil {
		if verr := op.Validate(host); verr != nil {
			cmd.Status = StatusBlocked
			cmd.Error = verr.Message
			lm.updateAndBroadcast(cmd)
			lm.logEvent("warn", hostID, opID, "Blocked: "+verr.Message)
			return cmd, verr
		}
	}

	// Execute
	cmd.StartedAt = time.Now()
	cmd.Status = StatusExecuting
	lm.updateAndBroadcast(cmd)
	lm.logEvent("info", hostID, opID, "Executing "+opID)

	// Send to agent
	if op.Executor == ExecutorAgent {
		// P1100: Set pending_command in DB BEFORE sending to agent
		// This is the single source of truth - Hub should not update this from heartbeats
		if lm.pending != nil {
			if err := lm.pending.SetPendingCommand(hostID, &opID); err != nil {
				lm.log.Error().Err(err).Str("host", hostID).Msg("failed to set pending_command in DB")
			}
		}

		if !lm.sender.SendCommand(hostID, opID) {
			cmd.Status = StatusError
			cmd.Error = "Failed to send command to agent"
			cmd.FinishedAt = time.Now()
			exitCode := 1
			cmd.ExitCode = &exitCode
			// Clear pending_command since we failed to send
			if lm.pending != nil {
				_ = lm.pending.ClearPendingCommand(hostID)
			}
			lm.updateAndBroadcast(cmd)
			lm.logEvent("error", hostID, opID, cmd.Error)
			return cmd, &ValidationError{Code: "send_failed", Message: cmd.Error}
		}

		// Start timeout watcher
		go lm.watchTimeout(cmd)

		// Command is now running - completion comes via HandleCommandComplete
		return cmd, nil
	}

	// Dashboard-side execution (synchronous)
	if op.Execute != nil {
		if err := op.Execute(nil, host); err != nil {
			return lm.completeWithError(cmd, 1, err.Error())
		}
	}

	return lm.completeWithSuccess(cmd, host)
}

// ═══════════════════════════════════════════════════════════════════════════
// COMMAND COMPLETION
// ═══════════════════════════════════════════════════════════════════════════

// HandleCommandComplete processes command completion from agent.
func (lm *LifecycleManager) HandleCommandComplete(hostID, opID string, exitCode int, message string) (*ActiveCommand, error) {
	lm.activeMu.RLock()
	cmd := lm.active[hostID]
	lm.activeMu.RUnlock()

	if cmd == nil || cmd.OpID != opID {
		lm.log.Debug().Str("host", hostID).Str("op", opID).Msg("completion for untracked command")
		return nil, nil // Not an error - just not tracked by us
	}

	// Stop timeout watcher
	close(cmd.cancelTimeout)

	op := lm.registry.Get(opID)
	isSwitch := opID == "switch" || opID == "pull-switch"

	// For switch with exit 0, enter AWAITING_RECONNECT
	if isSwitch && exitCode == 0 {
		return lm.enterAwaitingReconnect(cmd)
	}

	// For other commands, defer post-check until fresh heartbeat
	if exitCode == 0 && op != nil && op.Executor == ExecutorAgent {
		cmd.PostCheckDeferred = true
		cmd.DeferredExitCode = &exitCode
		cmd.DeferredMessage = message
		lm.log.Debug().Str("host", hostID).Str("op", opID).Msg("post-check deferred until fresh heartbeat")
		return cmd, nil
	}

	// Non-zero exit = immediate failure
	if exitCode != 0 {
		return lm.completeWithError(cmd, exitCode, message)
	}

	// Dashboard commands or forced immediate completion
	host, err := lm.getHost(hostID)
	if err != nil {
		return lm.completeWithError(cmd, exitCode, "Failed to get host state")
	}
	return lm.completeWithPostCheck(cmd, host, exitCode)
}

// HandleHeartbeat processes heartbeat with fresh host data.
// This is where deferred post-checks run.
func (lm *LifecycleManager) HandleHeartbeat(hostID string, freshness *AgentFreshness) {
	lm.activeMu.RLock()
	cmd := lm.active[hostID]
	lm.activeMu.RUnlock()

	if cmd == nil {
		return
	}

	// Update freshness for switch verification
	if freshness != nil {
		lm.activeMu.Lock()
		// Store latest freshness for reconnect verification
		lm.activeMu.Unlock()
	}

	// Check for deferred post-check
	if cmd.PostCheckDeferred {
		host, err := lm.getHost(hostID)
		if err != nil {
			lm.log.Error().Err(err).Str("host", hostID).Msg("failed to get host for deferred post-check")
			return
		}

		cmd.PostCheckDeferred = false
		exitCode := 0
		if cmd.DeferredExitCode != nil {
			exitCode = *cmd.DeferredExitCode
		}

		lm.log.Info().Str("host", hostID).Str("op", cmd.OpID).Msg("running deferred post-check with fresh data")
		_, _ = lm.completeWithPostCheck(cmd, host, exitCode)
	}
}

// HandleAgentReconnect processes agent reconnection after switch.
func (lm *LifecycleManager) HandleAgentReconnect(hostID string, freshness AgentFreshness) {
	lm.activeMu.RLock()
	cmd := lm.active[hostID]
	lm.activeMu.RUnlock()

	if cmd == nil || cmd.Status != StatusAwaitingReconnect {
		return
	}

	// Stop reconnect watcher
	close(cmd.cancelReconnect)

	// Verify binary freshness
	if cmd.PreFreshness == nil {
		lm.log.Warn().Str("host", hostID).Msg("no pre-switch freshness data - skipping verification")
		_, _ = lm.completeWithSuccess(cmd, nil)
		return
	}

	lm.logEvent("info", hostID, cmd.OpID, 
		"Binary check - Before: "+ShortHash(cmd.PreFreshness.SourceCommit)+
		" After: "+ShortHash(freshness.SourceCommit))

	verdict, message := CompareFreshness(*cmd.PreFreshness, freshness)

	switch verdict {
	case FreshnessFresh:
		_, _ = lm.completeWithSuccess(cmd, nil)
	case FreshnessSuspicious:
		cmd.Status = StatusSuspicious
		cmd.Error = message
		cmd.FinishedAt = time.Now()
		lm.updateAndBroadcast(cmd)
		lm.logEvent("warn", hostID, cmd.OpID, message)
		lm.clearActive(hostID)
	case FreshnessStale:
		cmd.Status = StatusStaleBinary
		cmd.Error = message
		cmd.FinishedAt = time.Now()
		lm.updateAndBroadcast(cmd)
		lm.logEvent("error", hostID, cmd.OpID, message)
		lm.clearActive(hostID)
	default:
		_, _ = lm.completeWithSuccess(cmd, nil)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TIMEOUT HANDLING
// ═══════════════════════════════════════════════════════════════════════════

// watchTimeout monitors for timeout conditions.
func (lm *LifecycleManager) watchTimeout(cmd *ActiveCommand) {
	hostID := cmd.HostID
	cfg := cmd.TimeoutConfig

	warningTimer := time.NewTimer(cfg.WarningTimeout + cmd.TimeoutExtended)
	hardTimer := time.NewTimer(cfg.HardTimeout + cmd.TimeoutExtended)

	defer warningTimer.Stop()
	defer hardTimer.Stop()

	for {
		select {
		case <-cmd.cancelTimeout:
			return

		case <-lm.done:
			return

		case <-warningTimer.C:
			lm.activeMu.Lock()
			if cmd.Status == StatusExecuting {
				now := time.Now()
				cmd.WarningAt = &now
				cmd.Status = StatusRunningWarning
				lm.log.Warn().Str("host", hostID).Str("op", cmd.OpID).
					Dur("elapsed", time.Since(cmd.StartedAt)).
					Msg("command exceeded warning timeout")
			}
			lm.activeMu.Unlock()
			lm.updateAndBroadcast(cmd)

		case <-hardTimer.C:
			lm.activeMu.Lock()
			if cmd.Status == StatusExecuting || cmd.Status == StatusRunningWarning {
				now := time.Now()
				cmd.HardTimeoutAt = &now
				cmd.Status = StatusTimeoutPending
				lm.log.Error().Str("host", hostID).Str("op", cmd.OpID).
					Dur("elapsed", time.Since(cmd.StartedAt)).
					Msg("command exceeded hard timeout - user action required")
			}
			lm.activeMu.Unlock()
			lm.updateAndBroadcast(cmd)
			return // Stop watching after hard timeout
		}
	}
}

// ExtendTimeout extends the timeout for a running command.
func (lm *LifecycleManager) ExtendTimeout(hostID string, minutes int) error {
	lm.activeMu.Lock()
	cmd := lm.active[hostID]
	if cmd == nil {
		lm.activeMu.Unlock()
		return &ValidationError{Code: "no_command", Message: "No active command for host"}
	}

	cmd.TimeoutExtended += time.Duration(minutes) * time.Minute
	cmd.Status = StatusExecuting // Reset from warning/timeout_pending
	lm.activeMu.Unlock()

	lm.logEvent("info", hostID, cmd.OpID, "Timeout extended by "+string(rune(minutes))+" minutes")
	lm.updateAndBroadcast(cmd)

	// Restart timeout watcher with new deadline
	close(cmd.cancelTimeout)
	cmd.cancelTimeout = make(chan struct{})
	go lm.watchTimeout(cmd)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// KILL HANDLING
// ═══════════════════════════════════════════════════════════════════════════

// KillCommand sends a kill signal to the running command.
func (lm *LifecycleManager) KillCommand(hostID, signal string, pid int) error {
	lm.activeMu.Lock()
	cmd := lm.active[hostID]
	if cmd == nil {
		lm.activeMu.Unlock()
		return &ValidationError{Code: "no_command", Message: "No active command for host"}
	}

	now := time.Now()
	cmd.KillInitiatedAt = &now
	cmd.KillSignal = signal
	cmd.KillPID = pid
	cmd.Status = StatusKilling
	lm.activeMu.Unlock()

	lm.logEvent("warn", hostID, cmd.OpID, "Sending "+signal+" to PID")
	lm.updateAndBroadcast(cmd)

	// Send kill command to agent
	if !lm.sender.SendCommand(hostID, "kill") {
		lm.activeMu.Lock()
		cmd.Status = StatusError
		cmd.Error = "Failed to send kill signal"
		lm.activeMu.Unlock()
		lm.updateAndBroadcast(cmd)
		return &ValidationError{Code: "kill_failed", Message: "Failed to send kill signal"}
	}

	return nil
}

// MarkKilled marks the command as killed.
func (lm *LifecycleManager) MarkKilled(hostID string) {
	lm.activeMu.Lock()
	cmd := lm.active[hostID]
	if cmd != nil {
		cmd.Status = StatusKilled
		cmd.FinishedAt = time.Now()
	}
	lm.activeMu.Unlock()

	if cmd != nil {
		lm.logEvent("warn", hostID, cmd.OpID, "Command killed by user")
		lm.updateAndBroadcast(cmd)
		lm.clearActive(hostID)
	}
}

// CancelCommand cancels a running command (user chose to ignore).
func (lm *LifecycleManager) CancelCommand(hostID string) error {
	lm.activeMu.Lock()
	cmd := lm.active[hostID]
	if cmd == nil {
		lm.activeMu.Unlock()
		return &ValidationError{Code: "no_command", Message: "No active command for host"}
	}

	cmd.Status = StatusSkipped
	cmd.Error = "Cancelled by user"
	cmd.FinishedAt = time.Now()
	lm.activeMu.Unlock()

	lm.logEvent("warn", hostID, cmd.OpID, "Command cancelled/ignored by user")
	lm.updateAndBroadcast(cmd)
	lm.clearActive(hostID)
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// SWITCH RECONNECT
// ═══════════════════════════════════════════════════════════════════════════

// enterAwaitingReconnect transitions to awaiting agent restart.
func (lm *LifecycleManager) enterAwaitingReconnect(cmd *ActiveCommand) (*ActiveCommand, error) {
	deadline := time.Now().Add(cmd.TimeoutConfig.ReconnectTimeout)
	cmd.ReconnectDeadline = &deadline
	cmd.Status = StatusAwaitingReconnect
	lm.updateAndBroadcast(cmd)
	lm.logEvent("info", cmd.HostID, cmd.OpID, "Switch completed - waiting for agent restart")

	// Start reconnect timeout watcher
	go lm.watchReconnectTimeout(cmd)

	return cmd, nil
}

// watchReconnectTimeout monitors for reconnect timeout.
func (lm *LifecycleManager) watchReconnectTimeout(cmd *ActiveCommand) {
	timer := time.NewTimer(cmd.TimeoutConfig.ReconnectTimeout)
	defer timer.Stop()

	select {
	case <-cmd.cancelReconnect:
		return

	case <-lm.done:
		return

	case <-timer.C:
		lm.activeMu.Lock()
		if cmd.Status == StatusAwaitingReconnect {
			cmd.Status = StatusTimeout
			cmd.Error = "Agent did not reconnect in time"
			cmd.FinishedAt = time.Now()
		}
		lm.activeMu.Unlock()

		lm.logEvent("error", cmd.HostID, cmd.OpID, "Reconnect timeout - verify host manually")
		lm.updateAndBroadcast(cmd)
		lm.clearActive(cmd.HostID)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// COMPLETION HELPERS
// ═══════════════════════════════════════════════════════════════════════════

func (lm *LifecycleManager) completeWithSuccess(cmd *ActiveCommand, host Host) (*ActiveCommand, error) {
	exitCode := 0
	cmd.ExitCode = &exitCode
	cmd.FinishedAt = time.Now()
	cmd.Status = StatusSuccess
	lm.updateAndBroadcast(cmd)
	lm.logEvent("success", cmd.HostID, cmd.OpID, cmd.OpID+" completed successfully")
	lm.clearActive(cmd.HostID)
	return cmd, nil
}

func (lm *LifecycleManager) completeWithError(cmd *ActiveCommand, exitCode int, message string) (*ActiveCommand, error) {
	cmd.ExitCode = &exitCode
	cmd.FinishedAt = time.Now()
	cmd.Status = StatusError
	cmd.Error = message
	lm.updateAndBroadcast(cmd)
	lm.logEvent("error", cmd.HostID, cmd.OpID, message)
	lm.clearActive(cmd.HostID)
	return cmd, &ValidationError{Code: "execution_failed", Message: message}
}

func (lm *LifecycleManager) completeWithPostCheck(cmd *ActiveCommand, host Host, exitCode int) (*ActiveCommand, error) {
	op := lm.registry.Get(cmd.OpID)
	if op == nil || op.PostCheck == nil {
		return lm.completeWithSuccess(cmd, host)
	}

	// Run post-check
	if verr := op.PostCheck(host); verr != nil {
		// Exit 0 but goal not met = partial
		if exitCode == 0 {
			cmd.ExitCode = &exitCode
			cmd.FinishedAt = time.Now()
			cmd.Status = StatusPartial
			cmd.Error = verr.Message
			lm.updateAndBroadcast(cmd)
			lm.logEvent("warn", cmd.HostID, cmd.OpID, "Partial: "+verr.Message)
			lm.clearActive(cmd.HostID)
			return cmd, nil
		}
		return lm.completeWithError(cmd, exitCode, verr.Message)
	}

	return lm.completeWithSuccess(cmd, host)
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════

// GetActiveCommand returns the active command for a host.
func (lm *LifecycleManager) GetActiveCommand(hostID string) *ActiveCommand {
	lm.activeMu.RLock()
	defer lm.activeMu.RUnlock()
	return lm.active[hostID]
}

// HasActiveCommand returns true if the host has an active (non-terminal) command.
// P1100: Used by stale cleanup to avoid clearing pending_command for tracked commands.
func (lm *LifecycleManager) HasActiveCommand(hostID string) bool {
	lm.activeMu.RLock()
	defer lm.activeMu.RUnlock()
	cmd := lm.active[hostID]
	return cmd != nil && !cmd.Status.IsTerminal()
}

// GetAllActiveCommands returns all active commands.
func (lm *LifecycleManager) GetAllActiveCommands() map[string]*ActiveCommand {
	lm.activeMu.RLock()
	defer lm.activeMu.RUnlock()
	result := make(map[string]*ActiveCommand, len(lm.active))
	for k, v := range lm.active {
		result[k] = v
	}
	return result
}

func (lm *LifecycleManager) clearActive(hostID string) {
	lm.activeMu.Lock()
	delete(lm.active, hostID)
	lm.activeMu.Unlock()

	// P1100: Clear pending_command in DB - single source of truth
	if lm.pending != nil {
		if err := lm.pending.ClearPendingCommand(hostID); err != nil {
			lm.log.Error().Err(err).Str("host", hostID).Msg("failed to clear pending_command in DB")
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════

func (lm *LifecycleManager) updateAndBroadcast(cmd *ActiveCommand) {
	// Persist to store
	if lm.store != nil {
		errMsg := ""
		if cmd.Error != "" {
			errMsg = cmd.Error
		}
		_ = lm.store.UpdateCommandStatus(cmd.ID, cmd.Status, cmd.ExitCode, errMsg)
	}

	// Broadcast to browsers
	if lm.broadcast != nil {
		lm.broadcast.BroadcastCommandState(cmd.HostID, cmd)
	}
}

func (lm *LifecycleManager) logEvent(level, hostID, opID, message string) {
	if lm.events != nil {
		lm.events.LogEvent("ops", level, "system", hostID, "op:"+opID, message, nil)
	}

	var event *zerolog.Event
	switch level {
	case "error":
		event = lm.log.Error()
	case "warn":
		event = lm.log.Warn()
	case "success":
		event = lm.log.Info()
	default:
		event = lm.log.Info()
	}
	event.Str("host", hostID).Str("op", opID).Msg(message)
}

func (lm *LifecycleManager) getHost(hostID string) (Host, error) {
	if lm.hosts == nil {
		return nil, &ValidationError{Code: "no_host_provider", Message: "Host provider not set"}
	}
	return lm.hosts.GetHostByID(hostID)
}

func (lm *LifecycleManager) getAgentFreshness(hostID string) *AgentFreshness {
	// TODO: Get from Hub's agent freshness tracking
	return nil
}

func captureHostSnapshot(host Host) *HostSnapshot {
	return &HostSnapshot{
		HostID:        host.GetID(),
		Generation:    host.GetGeneration(),
		AgentVersion:  host.GetAgentVersion(),
		AgentOutdated: host.IsAgentOutdated(),
		GitStatus:     host.GetGitStatus(),
		LockStatus:    host.GetLockStatus(),
		SystemStatus:  host.GetSystemStatus(),
	}
}

func generateUUID() string {
	// Simple UUID generation - in production use github.com/google/uuid
	return time.Now().Format("20060102-150405.000") + "-" + randomHex(4)
}

func randomHex(n int) string {
	const chars = "0123456789abcdef"
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[time.Now().UnixNano()%16]
	}
	return string(result)
}

// Shutdown stops all watchers.
func (lm *LifecycleManager) Shutdown() {
	close(lm.done)
}

