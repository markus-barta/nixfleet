package ops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// CommandSender is the interface for sending commands to agents.
// This abstracts the Hub dependency.
type CommandSender interface {
	SendCommand(hostID, command string) bool
	GetOnlineHosts() []string
}

// StateStore is the interface for persisting command state.
// This abstracts the State Store dependency (CORE-003).
type StateStore interface {
	CreateCommand(cmd *Command) error
	UpdateCommandStatus(cmdID string, status OpStatus, exitCode *int, errMsg string) error
	GetCommand(cmdID string) (*Command, error)
	GetPendingCommands(hostID string) ([]*Command, error)
}

// EventLogger is the interface for logging events.
// This abstracts the event logging dependency (CORE-003).
type EventLogger interface {
	LogEvent(category, level, actor, hostID, action, message string, details map[string]any)
}

// Executor runs ops with full lifecycle management.
type Executor struct {
	log       zerolog.Logger
	registry  *Registry
	sender    CommandSender
	store     StateStore
	events    EventLogger
	
	// Active commands per host (one at a time per host)
	active   map[string]*Command
	activeMu sync.RWMutex
}

// NewExecutor creates a new op executor.
func NewExecutor(log zerolog.Logger, registry *Registry, sender CommandSender, store StateStore, events EventLogger) *Executor {
	return &Executor{
		log:      log.With().Str("component", "op_executor").Logger(),
		registry: registry,
		sender:   sender,
		store:    store,
		events:   events,
		active:   make(map[string]*Command),
	}
}

// ExecuteOp runs a single op on a host with full lifecycle.
// Returns the command record and any error.
func (e *Executor) ExecuteOp(ctx context.Context, opID string, host Host, force bool) (*Command, error) {
	// 1. Get op from registry
	op := e.registry.Get(opID)
	if op == nil {
		return nil, fmt.Errorf("unknown op: %s", opID)
	}

	hostID := host.GetID()
	
	// 2. Check for existing active command
	e.activeMu.RLock()
	existing := e.active[hostID]
	e.activeMu.RUnlock()
	
	if existing != nil && !existing.Status.IsTerminal() {
		return nil, fmt.Errorf("host %s has active command: %s", hostID, existing.OpID)
	}

	// 3. Create command record
	cmd := &Command{
		ID:        uuid.New().String(),
		HostID:    hostID,
		OpID:      opID,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}

	// Persist to store
	if e.store != nil {
		if err := e.store.CreateCommand(cmd); err != nil {
			e.log.Error().Err(err).Str("op", opID).Str("host", hostID).Msg("failed to persist command")
			// Continue anyway - in-memory tracking still works
		}
	}

	// Track as active
	e.activeMu.Lock()
	e.active[hostID] = cmd
	e.activeMu.Unlock()

	// 4. Validate (unless forced)
	cmd.Status = StatusValidating
	e.updateStatus(cmd, StatusValidating, nil, "")
	e.logEvent("audit", "info", "user", hostID, "op:"+opID, fmt.Sprintf("Validating %s", opID), nil)

	if !force && op.Validate != nil {
		if verr := op.Validate(host); verr != nil {
			cmd.Status = StatusBlocked
			cmd.Error = verr.Message
			e.updateStatus(cmd, StatusBlocked, nil, verr.Message)
			e.logEvent("audit", "warn", "user", hostID, "op:"+opID, fmt.Sprintf("Blocked: %s", verr.Message), map[string]any{"code": verr.Code})
			return cmd, verr
		}
	}

	// 5. Execute
	cmd.StartedAt = time.Now()
	cmd.Status = StatusExecuting
	e.updateStatus(cmd, StatusExecuting, nil, "")
	e.logEvent("audit", "info", "user", hostID, "op:"+opID, fmt.Sprintf("Executing %s", opID), nil)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, op.Timeout)
	defer cancel()

	var execErr error
	if op.Executor == ExecutorAgent {
		// Send to agent
		if !e.sender.SendCommand(hostID, opID) {
			execErr = fmt.Errorf("failed to send command to agent")
		}
		// Note: actual execution result comes back via WebSocket
		// This is an async operation - we return here and status updates
		// come through HandleCommandComplete
	} else {
		// Dashboard-side execution
		if op.Execute != nil {
			execErr = op.Execute(execCtx, host)
		}
	}

	if execErr != nil {
		cmd.Status = StatusError
		cmd.Error = execErr.Error()
		now := time.Now()
		cmd.FinishedAt = now
		exitCode := 1
		cmd.ExitCode = &exitCode
		e.updateStatus(cmd, StatusError, &exitCode, execErr.Error())
		e.logEvent("audit", "error", "system", hostID, "op:"+opID, fmt.Sprintf("Failed: %s", execErr.Error()), nil)
		return cmd, execErr
	}

	// For dashboard ops, we can run post-check immediately
	if op.Executor == ExecutorDashboard {
		return e.completeCommand(cmd, op, host, 0, "")
	}

	// For agent ops, command is now running - completion handled by HandleCommandComplete
	return cmd, nil
}

// HandleCommandComplete processes command completion from agent.
func (e *Executor) HandleCommandComplete(hostID, opID string, exitCode int, output string) (*Command, error) {
	e.activeMu.RLock()
	cmd := e.active[hostID]
	e.activeMu.RUnlock()

	if cmd == nil || cmd.OpID != opID {
		e.log.Warn().Str("host", hostID).Str("op", opID).Msg("completion for unknown command")
		return nil, fmt.Errorf("no active command for %s on %s", opID, hostID)
	}

	op := e.registry.Get(opID)
	if op == nil {
		e.log.Error().Str("op", opID).Msg("completion for unknown op")
		return nil, fmt.Errorf("unknown op: %s", opID)
	}

	// Need to get current host state for post-check
	// This is a simplified version - actual implementation will fetch from DB
	var errMsg string
	if exitCode != 0 {
		errMsg = fmt.Sprintf("exit code %d", exitCode)
	}

	return e.completeCommand(cmd, op, nil, exitCode, errMsg)
}

// completeCommand finalizes a command with post-checks.
func (e *Executor) completeCommand(cmd *Command, op *Op, host Host, exitCode int, errMsg string) (*Command, error) {
	cmd.ExitCode = &exitCode
	cmd.FinishedAt = time.Now()

	if exitCode != 0 {
		cmd.Status = StatusError
		cmd.Error = errMsg
		e.updateStatus(cmd, StatusError, &exitCode, errMsg)
		e.logEvent("audit", "error", "system", cmd.HostID, "op:"+cmd.OpID, 
			fmt.Sprintf("Failed with exit code %d", exitCode), nil)
		return cmd, fmt.Errorf("command failed: %s", errMsg)
	}

	// Run post-check if available
	if op.PostCheck != nil && host != nil {
		if verr := op.PostCheck(host); verr != nil {
			cmd.Status = StatusError
			cmd.Error = verr.Message
			e.updateStatus(cmd, StatusError, &exitCode, verr.Message)
			e.logEvent("audit", "warn", "system", cmd.HostID, "op:"+cmd.OpID,
				fmt.Sprintf("Post-check failed: %s", verr.Message), map[string]any{"code": verr.Code})
			return cmd, verr
		}
	}

	cmd.Status = StatusSuccess
	e.updateStatus(cmd, StatusSuccess, &exitCode, "")
	e.logEvent("audit", "success", "system", cmd.HostID, "op:"+cmd.OpID,
		fmt.Sprintf("%s completed successfully", cmd.OpID), nil)

	// Clear from active
	e.activeMu.Lock()
	delete(e.active, cmd.HostID)
	e.activeMu.Unlock()

	return cmd, nil
}

// GetActiveCommand returns the active command for a host, if any.
func (e *Executor) GetActiveCommand(hostID string) *Command {
	e.activeMu.RLock()
	defer e.activeMu.RUnlock()
	return e.active[hostID]
}

// CancelCommand cancels a running command.
func (e *Executor) CancelCommand(hostID string) error {
	e.activeMu.Lock()
	cmd := e.active[hostID]
	if cmd != nil {
		cmd.Status = StatusError
		cmd.Error = "Cancelled by user"
		now := time.Now()
		cmd.FinishedAt = now
		delete(e.active, hostID)
	}
	e.activeMu.Unlock()

	if cmd == nil {
		return fmt.Errorf("no active command for %s", hostID)
	}

	e.updateStatus(cmd, StatusError, nil, "Cancelled by user")
	e.logEvent("audit", "warn", "user", hostID, "op:"+cmd.OpID, "Command cancelled", nil)
	return nil
}

// updateStatus persists status change to store.
func (e *Executor) updateStatus(cmd *Command, status OpStatus, exitCode *int, errMsg string) {
	if e.store != nil {
		if err := e.store.UpdateCommandStatus(cmd.ID, status, exitCode, errMsg); err != nil {
			e.log.Error().Err(err).Str("cmd", cmd.ID).Msg("failed to update command status")
		}
	}
}

// logEvent logs to event logger.
func (e *Executor) logEvent(category, level, actor, hostID, action, message string, details map[string]any) {
	if e.events != nil {
		e.events.LogEvent(category, level, actor, hostID, action, message, details)
	}
	
	// Also log to zerolog
	event := e.log.Info()
	if level == "error" {
		event = e.log.Error()
	} else if level == "warn" {
		event = e.log.Warn()
	}
	event.
		Str("category", category).
		Str("actor", actor).
		Str("host", hostID).
		Str("action", action).
		Msg(message)
}

