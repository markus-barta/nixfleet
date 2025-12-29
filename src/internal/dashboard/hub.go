package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/markus-barta/nixfleet/internal/protocol"
	"github.com/markus-barta/nixfleet/internal/templates"
	syncproto "github.com/markus-barta/nixfleet/internal/sync"
	"github.com/rs/zerolog"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB for logs

	// Broadcast queue size - large enough to buffer bursts
	broadcastQueueSize = 1024

	// Panic recovery delay before restarting
	panicRecoveryDelay = 100 * time.Millisecond
)

// flakeUpdateGetter provides access to pending PR info for browser connections.
// This interface avoids circular dependencies between Hub and FlakeUpdateService.
type flakeUpdateGetter interface {
	GetPendingPR() *PendingPRInfo
}

// lifecycleManagerInterface is the subset of ops.LifecycleManager used by Hub.
// Defined as interface to avoid import cycle.
type lifecycleManagerInterface interface {
	HandleCommandComplete(hostID, opID string, exitCode int, message string) (interface{}, error)
	HandleCommandRejected(hostID, reason, currentCommand string, currentPID int)
	HandleHeartbeat(hostID string, freshness interface{})
	HandleAgentReconnect(hostID string, freshness interface{})
	// P1100: Check if host has an active command in lifecycle manager
	HasActiveCommand(hostID string) bool
}

// Client represents a WebSocket connection (agent or browser).
type Client struct {
	conn       *websocket.Conn
	clientType string // "agent" or "browser"
	clientID   string // hostname for agents, session ID for browsers
	send       chan []byte
	hub        *Hub
	server     *Server

	// Safe close handling - prevents send-on-closed-channel panics
	closeOnce sync.Once
	closed    atomic.Bool
}

// Send implements sync.ClientSender (CORE-004).
// Uses SafeSend to avoid panics on closed channels.
func (c *Client) Send(data []byte) error {
	if ok := c.SafeSend(data); !ok {
		return errors.New("send failed (client closed or buffer full)")
	}
	return nil
}

// SafeSend sends data to the client without panicking on closed channel.
// Returns true if sent successfully, false if channel closed or buffer full.
func (c *Client) SafeSend(data []byte) (sent bool) {
	// Recover from send-on-closed-channel panic
	// This is necessary because there's a race between checking c.closed
	// and actually sending - Close() could run between those two operations.
	defer func() {
		if r := recover(); r != nil {
			sent = false
		}
	}()

	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		// Buffer full, drop message
		return false
	}
}

// Close safely closes the send channel exactly once.
// Uses sync.Once to prevent double-close panics.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.send)
	})
}

// Hub maintains active connections and broadcasts messages.
type Hub struct {
	log            zerolog.Logger
	db             *sql.DB
	cfg            *Config          // Dashboard config for stale command cleanup
	versionFetcher *VersionFetcher  // For Git status in heartbeat broadcasts
	flakeUpdates   flakeUpdateGetter // For PR status on browser connect (P5300)
	stateManager   *syncproto.StateManager // CORE-004: browser state sync (optional)

	// Registered clients
	clients map[*Client]bool

	// Agent connections by hostname
	agents map[string]*Client

	// Browser connections
	browsers map[*Client]bool

	// Channels for registration/unregistration
	register   chan *Client
	unregister chan *Client

	// Channel for messages from agents
	agentMessages chan *agentMessage

	// Async broadcast queue - decouples state changes from notifications
	broadcasts chan []byte

	// Log storage for command output
	logStore *LogStore

	// v3: Lifecycle manager (replaces CommandStateMachine + OpExecutor)
	lifecycleManager lifecycleManagerInterface

	// P2810: Last known agent freshness (updated on register/heartbeat)
	agentFreshness map[string]ops.AgentFreshness

	// Command completion subscribers (P5300 - proper deploy tracking)
	completionSubs   map[string][]chan CommandCompletion
	completionSubsMu sync.Mutex

	mu sync.RWMutex
}

// CommandCompletion represents a completed command for subscriber notification.
type CommandCompletion struct {
	HostID   string
	Command  string
	ExitCode int
	Success  bool
}

type agentMessage struct {
	client  *Client
	message *protocol.Message
}

// NewHub creates a new Hub.
func NewHub(log zerolog.Logger, db *sql.DB, cfg *Config, vf *VersionFetcher) *Hub {
	return &Hub{
		log:            log.With().Str("component", "hub").Logger(),
		db:             db,
		cfg:            cfg,
		versionFetcher: vf,
		clients:        make(map[*Client]bool),
		agents:         make(map[string]*Client),
		browsers:       make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		agentMessages:  make(chan *agentMessage, 256),
		broadcasts:     make(chan []byte, broadcastQueueSize),
		completionSubs: make(map[string][]chan CommandCompletion),
		agentFreshness: make(map[string]ops.AgentFreshness), // P2810
	}
}

// SetFlakeUpdates sets the flake update service reference.
// Called after FlakeUpdateService is created to avoid circular dependencies.
func (h *Hub) SetFlakeUpdates(fu flakeUpdateGetter) {
	h.flakeUpdates = fu
}

// SetLifecycleManager sets the lifecycle manager reference.
// Called by Server after creation to avoid circular dependencies.
func (h *Hub) SetLifecycleManager(lm lifecycleManagerInterface) {
	h.lifecycleManager = lm
}

// SetStateManager wires the CORE-004 StateManager for browser state sync.
func (h *Hub) SetStateManager(sm *syncproto.StateManager) {
	h.stateManager = sm
}

// GetAgentFreshness returns the last known freshness data for an agent (P2810).
func (h *Hub) GetAgentFreshness(hostID string) *ops.AgentFreshness {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if f, ok := h.agentFreshness[hostID]; ok {
		return &f
	}
	return nil
}

// Run starts the hub's main loop with panic recovery and context support.
// It will automatically recover from panics and restart the loop.
func (h *Hub) Run(ctx context.Context) {
	// Start broadcast loop in separate goroutine
	go h.broadcastLoop(ctx)

	// Start stale command cleanup loop (PRD FR-2.13)
	go h.staleCommandCleanupLoop(ctx)

	// Main loop with panic recovery
	for {
		if err := h.runLoop(ctx); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				h.log.Info().Msg("hub shutting down gracefully")
				return
			}
			h.log.Error().Err(err).Msg("hub loop crashed, restarting...")
			time.Sleep(panicRecoveryDelay)
		}
	}
}

// runLoop is the main processing loop with panic recovery.
func (h *Hub) runLoop(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("hub panic: %v\n%s", r, debug.Stack())
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.agentMessages:
			h.handleAgentMessage(msg)
		}
	}
}

// handleRegister processes client registration.
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	h.clients[client] = true
	if client.clientType == "browser" {
		h.browsers[client] = true
	}
	h.mu.Unlock()

	h.log.Debug().
		Str("type", client.clientType).
		Str("id", client.clientID).
		Msg("client registered")

	// CORE-004: send init/full_state on connect for browsers
	if client.clientType == "browser" && h.stateManager != nil {
		h.stateManager.RegisterClient(client)
	}

	// P7000: PR status is now fetched on-demand via per-host refresh button
	// No longer pushed on browser connect
}

// handleUnregister processes client unregistration.
// CRITICAL: All external operations happen OUTSIDE the mutex lock to prevent deadlocks.
func (h *Hub) handleUnregister(client *Client) {
	var (
		shouldNotify bool
		hostID       string
		wasKnown     bool
	)

	// Phase 1: State changes under lock (fast, no external calls)
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		wasKnown = true
		delete(h.clients, client)
		delete(h.browsers, client)
		if client.clientType == "agent" && client.clientID != "" {
			if h.agents[client.clientID] == client {
				delete(h.agents, client.clientID)
				shouldNotify = true
				hostID = client.clientID
			}
		}
	}
	h.mu.Unlock()

	// Phase 2: External operations OUTSIDE lock (prevents deadlock)
	if wasKnown {
		// CORE-004: unregister browser client from state sync
		if client.clientType == "browser" && h.stateManager != nil {
			h.stateManager.UnregisterClient(client)
		}
		// Close channel safely (uses sync.Once)
		client.Close()
	}

	if hostID != "" {
		// Database update outside lock
		_, err := h.db.Exec(`UPDATE hosts SET status = 'offline' WHERE hostname = ?`, hostID)
		if err != nil {
			h.log.Error().Err(err).Str("host", hostID).Msg("failed to mark host offline")
		}

		// CORE-004: Emit host_updated delta (offline)
		if h.stateManager != nil {
			h.stateManager.ApplyChange(syncproto.Change{
				Type:   syncproto.ChangeHostUpdated,
				ID:     hostID,
				Fields: map[string]any{"status": "offline"},
			})
		}
	}

	if shouldNotify {
		// Legacy host_offline broadcast removed (CORE-004 delta is the source of truth).
	}

	h.log.Debug().
		Str("type", client.clientType).
		Str("id", client.clientID).
		Msg("client unregistered")
}

// broadcastLoop runs in a separate goroutine and handles all browser broadcasts.
// This decouples broadcasts from the main hub loop, preventing blocking.
func (h *Hub) broadcastLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("broadcast loop crashed, restarting...")
			// Only restart if context is still active
			if ctx.Err() == nil {
				go h.broadcastLoop(ctx)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-h.broadcasts:
			h.doBroadcast(data)
		}
	}
}

// doBroadcast sends data to all connected browsers.
func (h *Hub) doBroadcast(data []byte) {
	h.mu.RLock()
	browsers := make([]*Client, 0, len(h.browsers))
	for client := range h.browsers {
		browsers = append(browsers, client)
	}
	h.mu.RUnlock()

	for _, client := range browsers {
		client.SafeSend(data) // Never panics
	}
}

// staleCommandCleanupLoop periodically cleans up stale pending_command for offline hosts.
// This implements PRD FR-2.13: Uses multiplier × heartbeat_interval (like Kubernetes liveness probes).
func (h *Hub) staleCommandCleanupLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("stale command cleanup loop crashed, restarting...")
			if ctx.Err() == nil {
				go h.staleCommandCleanupLoop(ctx)
			}
		}
	}()

	ticker := time.NewTicker(h.cfg.StaleCleanupInterval)
	defer ticker.Stop()

	h.log.Info().
		Dur("interval", h.cfg.StaleCleanupInterval).
		Dur("threshold", h.cfg.StaleCommandTimeout()).
		Int("multiplier", h.cfg.StaleMultiplier).
		Msg("stale command cleanup loop started")

	for {
		select {
		case <-ctx.Done():
			h.log.Info().Msg("stale command cleanup loop shutting down")
			return
		case <-ticker.C:
			h.cleanupStaleCommands()
		}
	}
}

// cleanupStaleCommands clears pending_command for hosts that have been
// unresponsive longer than the stale threshold. This handles:
// - Offline hosts that went down during a command
// - "Running" hosts where the agent died mid-command without sending status
// Any host with a pending_command and stale last_seen is cleaned up.
// P1100: Now also checks LifecycleManager to avoid clearing tracked commands.
func (h *Hub) cleanupStaleCommands() {
	timeout := h.cfg.StaleCommandTimeout()
	thresholdMinutes := int(timeout.Minutes())

	// First, query which hosts will be affected (for broadcasting)
	// Check if host is currently connected - don't cleanup active agents
	h.mu.RLock()
	connectedAgents := make(map[string]bool)
	for hostname := range h.agents {
		connectedAgents[hostname] = true
	}
	h.mu.RUnlock()

	rows, err := h.db.Query(`
		SELECT hostname, pending_command, status
		FROM hosts
		WHERE pending_command IS NOT NULL
		AND last_seen < datetime('now', '-' || ? || ' minutes')
	`, thresholdMinutes)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to query stale commands")
		return
	}

	var hostsToUpdate []string
	for rows.Next() {
		var hostname, pendingCommand, status string
		if err := rows.Scan(&hostname, &pendingCommand, &status); err != nil {
			continue
		}
		// Skip if agent is currently connected (heartbeats will handle it)
		if connectedAgents[hostname] {
			continue
		}
		// P1100: Skip if LifecycleManager is tracking an active command
		// This can happen for AWAITING_RECONNECT state where we're waiting for agent restart
		if h.lifecycleManager != nil && h.lifecycleManager.HasActiveCommand(hostname) {
			h.log.Debug().
				Str("host", hostname).
				Str("command", pendingCommand).
				Msg("skipping stale cleanup - LifecycleManager has active command")
			continue
		}
		hostsToUpdate = append(hostsToUpdate, hostname)
		h.log.Info().
			Str("host", hostname).
			Str("command", pendingCommand).
			Str("status", status).
			Dur("threshold", timeout).
			Msg("clearing stale pending_command for unresponsive host")
	}
	rows.Close()

	if len(hostsToUpdate) == 0 {
		return
	}

	// Update the database - also set status to 'offline' since host is clearly not running
	for _, hostname := range hostsToUpdate {
		_, err := h.db.Exec(`
			UPDATE hosts
			SET pending_command = NULL, status = 'offline'
			WHERE hostname = ?
		`, hostname)
		if err != nil {
			h.log.Error().Err(err).Str("host", hostname).Msg("failed to cleanup stale command")
			continue
		}
	}

	h.log.Info().
		Int("count", len(hostsToUpdate)).
		Dur("threshold", timeout).
		Msg("cleared stale commands for unresponsive hosts")

	// Legacy host_offline broadcast removed (CORE-004 delta is the source of truth).
}

// queueBroadcast queues a message for async broadcast to all browsers.
// Non-blocking: drops message with warning if queue is full.
func (h *Hub) queueBroadcast(msg map[string]any) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal broadcast message")
		return
	}

	select {
	case h.broadcasts <- data:
		// Queued successfully
	default:
		h.log.Warn().Msg("broadcast queue full, dropping message")
	}
}

// BroadcastToBrowsers queues a message for broadcast to all connected browsers.
// This is the public API - internally uses the async queue.
func (h *Hub) BroadcastToBrowsers(msg map[string]any) {
	h.queueBroadcast(msg)
}

// BroadcastHostStatus fetches the full status for a host and broadcasts it to all browsers.
// This should be called after any event that changes the host's compartment status.
func (h *Hub) BroadcastHostStatus(hostID string) {
	// Query host from database
	var host struct {
		Hostname         string
		Generation       *string
		AgentVersion     *string
		LockStatusJSON   *string
		SystemStatusJSON *string
		TestsStatusJSON  *string
		TestsGeneration  *string
		RepoURL          *string
		RepoDir          *string
		Status           string
	}

	err := h.db.QueryRow(`
		SELECT hostname, generation, agent_version, lock_status_json,
		       system_status_json, tests_status_json, tests_generation,
		       repo_url, repo_dir, status
		FROM hosts WHERE id = ? OR hostname = ?
	`, hostID, hostID).Scan(
		&host.Hostname, &host.Generation, &host.AgentVersion,
		&host.LockStatusJSON, &host.SystemStatusJSON,
		&host.TestsStatusJSON, &host.TestsGeneration,
		&host.RepoURL, &host.RepoDir, &host.Status,
	)
	if err != nil {
		h.log.Debug().Err(err).Str("host", hostID).Msg("BroadcastHostStatus: host not found")
		return
	}

	// Build update status
	var lockStatus, systemStatus, testsStatus map[string]any
	if host.LockStatusJSON != nil {
		_ = json.Unmarshal([]byte(*host.LockStatusJSON), &lockStatus)
	}
	if host.SystemStatusJSON != nil {
		_ = json.Unmarshal([]byte(*host.SystemStatusJSON), &systemStatus)
	}
	if host.TestsStatusJSON != nil {
		_ = json.Unmarshal([]byte(*host.TestsStatusJSON), &testsStatus)
	}

	// Get git status if version fetcher is available
	var gitStatus map[string]any
	generation := ""
	if host.Generation != nil {
		generation = *host.Generation
	}
	if h.versionFetcher != nil {
		status, msg, checked := h.versionFetcher.GetGitStatus(generation)
		gitStatus = map[string]any{"status": status, "message": msg, "checked_at": checked}
		// P7100: Debug logging
		h.log.Debug().
			Str("host", hostID).
			Str("db_generation", generation).
			Str("git_status", status).
			Msg("BroadcastHostStatus: git status computed")
	}
	if h.versionFetcher == nil {
		gitStatus = map[string]any{
			"status":     "error",
			"message":    "Version tracking not configured (remote desired state unavailable)",
			"checked_at": "",
		}
	}

	// Get repo URL/dir
	repoURL := ""
	repoDir := ""
	if host.RepoURL != nil {
		repoURL = *host.RepoURL
	}
	if host.RepoDir != nil {
		repoDir = *host.RepoDir
	}

	updateStatus := map[string]any{
		"git":      gitStatus,
		"lock":     lockStatus,
		"system":   systemStatus,
		"tests":    testsStatus,
		"repo_url": repoURL,
		"repo_dir": repoDir,
	}

	// CORE-006: Tests are generation-scoped (old-generation pass => 🟡 on new deployment)
	testsGen := ""
	if host.TestsGeneration != nil {
		testsGen = *host.TestsGeneration
	}
	if testsGen != "" && generation != "" && testsGen != generation {
		if ts, ok := updateStatus["tests"].(map[string]any); ok && ts != nil {
			updateStatus["tests"] = map[string]any{
				"status":     "outdated",
				"message":    "Tests outdated for current deployed state",
				"checked_at": "",
			}
		}
	}

	// CORE-006: Remote-gate System (avoid gray when Git/Lock clearly show not-current)
	gs, _ := gitStatus["status"].(string)
	ls := ""
	if lockStatus != nil {
		ls, _ = lockStatus["status"].(string)
	}
	gitOutdated := gs == "outdated"
	gitError := gs == "error"
	lockOutdated := ls == "outdated"
	lockError := ls == "error"
	gitOK := gs == "ok"
	lockOK := ls == "ok"
	gitUnknown := gs == "" || gs == "unknown"
	lockUnknown := ls == "" || ls == "unknown"

	if ss, ok := updateStatus["system"].(map[string]any); ok && ss != nil {
		if ssv, _ := ss["status"].(string); ssv == "ok" && (!gitOK || !lockOK) {
			switch {
			case gitOutdated || lockOutdated:
				updateStatus["system"] = map[string]any{"status": "outdated", "message": "System not current vs remote (Git/Lock behind)", "checked_at": ""}
			case gitError || lockError:
				updateStatus["system"] = map[string]any{"status": "outdated", "message": "Remote verification degraded (Git/Lock error)", "checked_at": ""}
			case gitUnknown || lockUnknown:
				updateStatus["system"] = map[string]any{"status": "unknown", "message": "Cannot verify System vs remote (insufficient signal)", "checked_at": ""}
			}
		}
	} else {
		switch {
		case gitOutdated || lockOutdated:
			updateStatus["system"] = map[string]any{"status": "outdated", "message": "System not current vs remote (Git/Lock behind)", "checked_at": ""}
		case gitError || lockError:
			updateStatus["system"] = map[string]any{"status": "outdated", "message": "Remote verification degraded (Git/Lock error)", "checked_at": ""}
		}
	}

	// CORE-004: Emit host_updated delta for update_status refresh
	if h.stateManager != nil {
		h.stateManager.ApplyChange(syncproto.Change{
			Type: syncproto.ChangeHostUpdated,
			ID:   hostID,
			Fields: map[string]any{
				"generation":    generation,
				"update_status": updateStatus,
			},
		})
	}

	h.log.Debug().Str("host", hostID).Msg("broadcast host status update")
}

// handleAgentMessage processes messages from agents.
func (h *Hub) handleAgentMessage(msg *agentMessage) {
	switch msg.message.Type {
	case protocol.TypeRegister:
		h.handleAgentRegister(msg)

	case protocol.TypeHeartbeat:
		var payload protocol.HeartbeatPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			h.log.Error().Err(err).Msg("failed to parse heartbeat payload")
			return
		}
		h.handleHeartbeat(msg.client.clientID, payload)

	case protocol.TypeOutput:
		var payload protocol.OutputPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			return
		}

		// Log to file
		if h.logStore != nil {
			_ = h.logStore.AppendLine(msg.client.clientID, payload.Command, payload.Line, payload.IsError)
		}

		// Forward to browsers
		h.BroadcastToBrowsers(map[string]any{
			"type": "command_output",
			"payload": map[string]any{
				"host_id":  msg.client.clientID,
				"line":     payload.Line,
				"command":  payload.Command,
				"is_error": payload.IsError,
			},
		})

	case protocol.TypeStatus:
		var payload protocol.StatusPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			return
		}
		h.handleStatus(msg.client.clientID, payload)

	case protocol.TypeRejected:
		var payload protocol.CommandRejectedPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			return
		}

		h.log.Warn().
			Str("host", msg.client.clientID).
			Str("reason", payload.Reason).
			Str("agent_current", payload.CurrentCommand).
			Int("agent_pid", payload.CurrentPID).
			Msg("agent rejected command")

		// Reconcile lifecycle state so we don't get stuck "busy/pulling"
		if h.lifecycleManager != nil {
			h.lifecycleManager.HandleCommandRejected(msg.client.clientID, payload.Reason, payload.CurrentCommand, payload.CurrentPID)
		}

		// NOTE: We intentionally don't send legacy toast here.
		// UI consumes events via CORE-004 full_state/delta(event).
		if h.stateManager != nil {
			h.stateManager.ApplyChange(syncproto.Change{
				Type: syncproto.ChangeEvent,
				Payload: map[string]any{
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"category":  "command",
					"level":     "error",
					"host_id":   msg.client.clientID,
					"message":   "Agent rejected command: " + payload.Reason,
					"details": map[string]any{
						"current_command": payload.CurrentCommand,
						"current_pid":     payload.CurrentPID,
					},
				},
			})
		}

	case protocol.TypeTestProgress:
		var payload protocol.TestProgressPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			return
		}

		// Forward to browsers
		h.BroadcastToBrowsers(map[string]any{
			"type": "test_progress",
			"payload": map[string]any{
				"host_id": msg.client.clientID,
				"current": payload.Current,
				"total":   payload.Total,
				"passed":  payload.Passed,
				"running": payload.Running,
				"result":  payload.Result,
			},
		})

	case protocol.TypeOperationProgress:
		// P2800: Operation progress for status dots
		var payload protocol.OperationProgressPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			h.log.Error().Err(err).Msg("failed to parse operation_progress payload")
			return
		}

		h.log.Debug().
			Str("host", msg.client.clientID).
			Interface("progress", payload.Progress).
			Msg("operation_progress received")

		// Forward to browsers for status dot animation
		h.BroadcastToBrowsers(map[string]any{
			"type": "operation_progress",
			"payload": map[string]any{
				"host_id":  msg.client.clientID,
				"progress": payload.Progress,
			},
		})
	}
}

// handleAgentRegister processes agent registration.
// CRITICAL: External operations happen OUTSIDE the mutex lock.
// P2800: Also handles reconnection-based switch completion verification.
func (h *Hub) handleAgentRegister(msg *agentMessage) {
	var payload protocol.RegisterPayload
	if err := msg.message.ParsePayload(&payload); err != nil {
		h.log.Error().Err(err).Msg("failed to parse register payload")
		return
	}

	var oldClient *Client

	// Phase 1: State changes under lock
	h.mu.Lock()
	if existing, ok := h.agents[payload.Hostname]; ok && existing != msg.client {
		oldClient = existing
		h.log.Warn().Str("hostname", payload.Hostname).Msg("replaced duplicate agent")
	}
	msg.client.clientID = payload.Hostname
	h.agents[payload.Hostname] = msg.client
	h.mu.Unlock()

	// Phase 2: External operations OUTSIDE lock
	if oldClient != nil {
		oldClient.Close() // Safe close
	}

	// Update host in database
	h.updateHost(payload)

	// Send registered response (uses SafeSend)
	resp, _ := protocol.NewMessage(protocol.TypeRegistered, protocol.RegisteredPayload{
		HostID: payload.Hostname,
	})
	respData, _ := json.Marshal(resp)
	msg.client.SafeSend(respData)

	h.log.Info().
		Str("hostname", payload.Hostname).
		Str("agent_version", payload.AgentVersion).
		Str("source_commit", payload.SourceCommit).
		Msg("agent registered")

	// P2810: Store agent freshness for pre-switch snapshot capture
	freshness := ops.AgentFreshness{
		SourceCommit: payload.SourceCommit,
		StorePath:    payload.StorePath,
		BinaryHash:   payload.BinaryHash,
	}
	h.mu.Lock()
	h.agentFreshness[payload.Hostname] = freshness
	h.mu.Unlock()

	// v3: Notify lifecycle manager of agent reconnection
	if h.lifecycleManager != nil {
		h.lifecycleManager.HandleAgentReconnect(payload.Hostname, freshness)
	}
}

func (h *Hub) updateHost(payload protocol.RegisterPayload) {
	// Use default color if not provided
	themeColor := payload.ThemeColor
	if themeColor == "" {
		if payload.HostType == "macos" {
			themeColor = "#bb9af7" // Tokyo Night purple for macOS
		} else {
			themeColor = "#7aa2f7" // Tokyo Night blue for NixOS
		}
	}

	// Use defaults for location and device_type if not provided
	location := payload.Location
	if location == "" {
		location = "home"
	}
	deviceType := payload.DeviceType
	if deviceType == "" {
		deviceType = "desktop"
	}

	// Upsert host record
	// On re-registration (after switch/restart), clear pending_command and set online
	_, err := h.db.Exec(`
		INSERT INTO hosts (id, hostname, host_type, agent_version, os_version, nixpkgs_version, generation, theme_color, location, device_type, repo_url, repo_dir, last_seen, status, pending_command)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), 'online', NULL)
		ON CONFLICT(hostname) DO UPDATE SET
			host_type = excluded.host_type,
			agent_version = excluded.agent_version,
			os_version = excluded.os_version,
			nixpkgs_version = excluded.nixpkgs_version,
			generation = excluded.generation,
			theme_color = excluded.theme_color,
			location = excluded.location,
			device_type = excluded.device_type,
			repo_url = excluded.repo_url,
			repo_dir = excluded.repo_dir,
			last_seen = datetime('now'),
			status = 'online',
			pending_command = NULL
	`, payload.Hostname, payload.Hostname, payload.HostType, payload.AgentVersion,
		payload.OSVersion, payload.NixpkgsVersion, payload.Generation, themeColor, location, deviceType,
		payload.RepoURL, payload.RepoDir)

	if err != nil {
		h.log.Error().Err(err).Str("hostname", payload.Hostname).Msg("failed to upsert host")
		return
	}

	h.log.Debug().
		Str("hostname", payload.Hostname).
		Str("host_type", payload.HostType).
		Str("os_version", payload.OSVersion).
		Str("theme_color", themeColor).
		Msg("updated host record")

	// Legacy host_heartbeat broadcast removed (CORE-004 delta is the source of truth).

	// CORE-004: Emit host_updated delta (registration refreshes many fields)
	if h.stateManager != nil {
		h.stateManager.ApplyChange(syncproto.Change{
			Type: syncproto.ChangeHostUpdated,
			ID:   payload.Hostname,
			Fields: map[string]any{
				"hostname":        payload.Hostname,
				"host_type":       payload.HostType,
				"agent_version":   payload.AgentVersion,
				"os_version":      payload.OSVersion,
				"nixpkgs_version": payload.NixpkgsVersion,
				"generation":      payload.Generation,
				"theme_color":     themeColor,
				"location":        location,
				"device_type":     deviceType,
				"repo_url":        payload.RepoURL,
				"repo_dir":        payload.RepoDir,
				"status":          "online",
				"pending_command": nil,
				"last_seen":       time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

func (h *Hub) handleHeartbeat(hostID string, payload protocol.HeartbeatPayload) {
	// Serialize metrics to JSON if available
	var metricsJSON *string
	if payload.Metrics != nil {
		data, err := json.Marshal(payload.Metrics)
		if err == nil {
			s := string(data)
			metricsJSON = &s
		}
	}

	// P3700: Compute Lock status from hash comparison (version-based, not time-based)
	var lockStatus protocol.StatusCheck
	if h.versionFetcher != nil {
		status, msg, checked := h.versionFetcher.GetLockStatus(payload.LockHash)
		lockStatus = protocol.StatusCheck{
			Status:    status,
			Message:   msg,
			CheckedAt: checked,
		}
	} else if payload.UpdateStatus != nil && payload.UpdateStatus.Lock.Status != "" {
		// Fallback to agent's time-based status if no lock hash or version fetcher
		lockStatus = payload.UpdateStatus.Lock
	}

	// P3800: Compute System status with inference
	// Rule: If Lock is outdated → System MUST be outdated (can't be current with old deps)
	var systemStatus protocol.StatusCheck
	if payload.UpdateStatus != nil && payload.UpdateStatus.System.Status != "" {
		systemStatus = payload.UpdateStatus.System
	}
	// P3800: Override system status if lock is outdated
	if lockStatus.Status == "outdated" && systemStatus.Status == "ok" {
		// Lock outdated means system can't be current - override to outdated
		systemStatus = protocol.StatusCheck{
			Status:    "outdated",
			Message:   "System outdated (flake.lock is behind)",
			CheckedAt: lockStatus.CheckedAt,
		}
	}

	// Serialize update status to JSON
	var lockStatusJSON, systemStatusJSON, testsStatusJSON *string
	if lockStatus.Status != "" {
		if data, err := json.Marshal(lockStatus); err == nil {
			s := string(data)
			lockStatusJSON = &s
		}
	}
	if systemStatus.Status != "" {
		if data, err := json.Marshal(systemStatus); err == nil {
			s := string(data)
			systemStatusJSON = &s
		}
	}
	// P1110: Persist tests compartment status (generation-scoped)
	var testsGenerationPtr *string
	if payload.UpdateStatus != nil && payload.UpdateStatus.Tests.Status != "" {
		if data, err := json.Marshal(payload.UpdateStatus.Tests); err == nil {
			s := string(data)
			testsStatusJSON = &s
		}
		if payload.Generation != "" {
			testsGenerationPtr = &payload.Generation
		}
	}

	// P3700: Store lock_hash in database
	var lockHashPtr *string
	if payload.LockHash != "" {
		lockHashPtr = &payload.LockHash
	}

	// P1100: DO NOT update pending_command from heartbeat!
	// LifecycleManager is the SINGLE SOURCE OF TRUTH for pending_command.
	// Heartbeat reports what the agent *thinks* it's running, but the dashboard
	// tracks command lifecycle independently to handle edge cases like:
	// - Agent crash mid-command
	// - Switch command awaiting reconnect
	// - Dashboard restart during command
	//
	// The agent's PendingCommand is still logged for debugging but not persisted.
	if payload.PendingCommand != nil && *payload.PendingCommand != "" {
		h.log.Debug().
			Str("host", hostID).
			Str("agent_pending", *payload.PendingCommand).
			Msg("agent reports pending command (informational only)")
	}

	// Update host last_seen and status in database (WITHOUT pending_command)
	_, err := h.db.Exec(`
		UPDATE hosts SET 
			last_seen = datetime('now'),
			status = 'online',
			generation = ?,
			nixpkgs_version = ?,
			metrics_json = ?,
			lock_status_json = COALESCE(?, lock_status_json),
			system_status_json = COALESCE(?, system_status_json),
			tests_status_json = COALESCE(?, tests_status_json),
			tests_generation = COALESCE(?, tests_generation),
			lock_hash = ?
		WHERE hostname = ?
	`, payload.Generation, payload.NixpkgsVersion, metricsJSON, lockStatusJSON, systemStatusJSON, testsStatusJSON, testsGenerationPtr, lockHashPtr, hostID)

	if err != nil {
		h.log.Error().Err(err).Str("host", hostID).Msg("failed to update heartbeat")
	}

	h.log.Debug().
		Str("host", hostID).
		Str("generation", payload.Generation).
		Str("lock_hash", payload.LockHash).
		Msg("heartbeat received")

	// P2810: Update agent freshness (for pre-switch snapshot capture)
	if payload.SourceCommit != "" || payload.StorePath != "" || payload.BinaryHash != "" {
		h.mu.Lock()
		h.agentFreshness[hostID] = ops.AgentFreshness{
			SourceCommit: payload.SourceCommit,
			StorePath:    payload.StorePath,
			BinaryHash:   payload.BinaryHash,
		}
		h.mu.Unlock()
	}

	// Build update status for broadcast
	var gitStatus map[string]any
	if h.versionFetcher != nil {
		status, msg, checked := h.versionFetcher.GetGitStatus(payload.Generation)
		gitStatus = map[string]any{"status": status, "message": msg, "checked_at": checked}
	} else {
		gitStatus = map[string]any{
			"status":     "error",
			"message":    "Version tracking not configured (remote desired state unavailable)",
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		}
	}

	updateStatus := map[string]any{
		"git": gitStatus,
	}
	// P3700: Always use the computed lockStatus (version-based)
	if lockStatus.Status != "" {
		updateStatus["lock"] = lockStatus
	}
	// P3800: Use inferred systemStatus
	if systemStatus.Status != "" {
		updateStatus["system"] = systemStatus
	}
	// P3900: Include tests status from agent
	if payload.UpdateStatus != nil && payload.UpdateStatus.Tests.Status != "" {
		updateStatus["tests"] = payload.UpdateStatus.Tests
	}

	// CORE-006: Remote-gate System/Tests
	// - System/Tests MUST NOT be green unless Git+Lock are green.
	gs, _ := gitStatus["status"].(string)
	ls := ""
	if lock, ok := updateStatus["lock"].(protocol.StatusCheck); ok {
		ls = lock.Status
	}

	gitOK := gs == "ok"
	gitOutdated := gs == "outdated"
	gitError := gs == "error"
	gitUnknown := gs == "" || gs == "unknown"

	lockOK := ls == "ok"
	lockOutdated := ls == "outdated"
	lockError := ls == "error"
	lockUnknown := ls == "" || ls == "unknown"

	// Gate System
	if ss, ok := updateStatus["system"].(protocol.StatusCheck); ok {
		if ss.Status == "ok" && (!gitOK || !lockOK) {
			switch {
			case gitOutdated || lockOutdated:
				updateStatus["system"] = protocol.StatusCheck{
					Status:    "outdated",
					Message:   "System not current vs remote (Git/Lock behind)",
					CheckedAt: ss.CheckedAt,
				}
			case gitError || lockError:
				updateStatus["system"] = protocol.StatusCheck{
					Status:    "outdated",
					Message:   "Remote verification degraded (Git/Lock error)",
					CheckedAt: ss.CheckedAt,
				}
			case gitUnknown || lockUnknown:
				updateStatus["system"] = protocol.StatusCheck{
					Status:    "unknown",
					Message:   "Cannot verify System vs remote (insufficient signal)",
					CheckedAt: ss.CheckedAt,
				}
			}
		}
	} else {
		// If we have no System signal but Git/Lock clearly indicate not-current, avoid gray.
		switch {
		case gitOutdated || lockOutdated:
			updateStatus["system"] = protocol.StatusCheck{
				Status:    "outdated",
				Message:   "System not current vs remote (Git/Lock behind)",
				CheckedAt: time.Now().UTC().Format(time.RFC3339),
			}
		case gitError || lockError:
			updateStatus["system"] = protocol.StatusCheck{
				Status:    "outdated",
				Message:   "Remote verification degraded (Git/Lock error)",
				CheckedAt: time.Now().UTC().Format(time.RFC3339),
			}
		}
	}

	// Gate Tests (green only when System+Git+Lock are green)
	if ts, ok := updateStatus["tests"].(protocol.StatusCheck); ok && ts.Status == "ok" {
		systemOK := false
		if ss, ok := updateStatus["system"].(protocol.StatusCheck); ok && ss.Status == "ok" {
			systemOK = true
		}
		if !systemOK || !gitOK || !lockOK {
			switch {
			case gitOutdated || lockOutdated:
				updateStatus["tests"] = protocol.StatusCheck{
					Status:    "outdated",
					Message:   "Tests outdated for current remote state",
					CheckedAt: ts.CheckedAt,
				}
			case gitError || lockError:
				updateStatus["tests"] = protocol.StatusCheck{
					Status:    "outdated",
					Message:   "Remote verification degraded (Git/Lock error)",
					CheckedAt: ts.CheckedAt,
				}
			case gitUnknown || lockUnknown:
				updateStatus["tests"] = protocol.StatusCheck{
					Status:    "unknown",
					Message:   "Cannot verify Tests vs remote (insufficient signal)",
					CheckedAt: ts.CheckedAt,
				}
			default:
				updateStatus["tests"] = protocol.StatusCheck{
					Status:    "outdated",
					Message:   "Tests outdated for current deployed state",
					CheckedAt: ts.CheckedAt,
				}
			}
		}
	}

	// v3: Notify lifecycle manager of heartbeat for deferred post-checks
	if h.lifecycleManager != nil {
		h.mu.RLock()
		freshness := h.agentFreshness[hostID]
		h.mu.RUnlock()
		h.lifecycleManager.HandleHeartbeat(hostID, &freshness)
	}

	// Legacy host_heartbeat broadcast removed (CORE-004 delta is the source of truth).

	// CORE-004: Emit host_updated delta with the same status data (authoritative state)
	if h.stateManager != nil {
		h.stateManager.ApplyChange(syncproto.Change{
			Type: syncproto.ChangeHostUpdated,
			ID:   hostID,
			Fields: map[string]any{
				"status":        "online",
				"last_seen":     time.Now().UTC().Format(time.RFC3339),
				"generation":    payload.Generation,
				"metrics":       payload.Metrics,
				"update_status": updateStatus,
			},
		})
	}
}

func (h *Hub) handleStatus(hostID string, payload protocol.StatusPayload) {
	h.log.Info().
		Str("host", hostID).
		Str("command", payload.Command).
		Str("status", payload.Status).
		Int("exit_code", payload.ExitCode).
		Msg("command status")

	// P1100: DO NOT clear pending_command here!
	// LifecycleManager is the SINGLE SOURCE OF TRUTH for pending_command.
	// It will call ClearPendingCommand() when the command reaches a terminal state.
	// This includes handling switch commands that need to await reconnect.
	//
	// The old code cleared pending_command directly here, but that created
	// race conditions with LifecycleManager's state tracking.

	// Complete the log file
	if h.logStore != nil {
		_ = h.logStore.CompleteCommand(hostID, payload.Command, payload.ExitCode)
	}

	// Notify completion subscribers (P5300 - deploy tracking)
	h.notifyCommandCompletion(hostID, payload.Command, payload.ExitCode)

	// P7100: Update generation and refresh caches BEFORE post-validation
	// This was a critical bug - post-validation was using stale data
	if payload.Generation != "" {
		_, err := h.db.Exec(`UPDATE hosts SET generation = ? WHERE hostname = ?`, payload.Generation, hostID)
		if err != nil {
			h.log.Error().Err(err).Str("host", hostID).Msg("failed to update generation")
		}
	}

	// P1110: Persist System/Tests compartment status from command outcome (dashboard-side inference)
	// This is cheap and survives agent restarts.
	now := time.Now().UTC()
	switch payload.Command {
	case "switch", "rollback":
		var sys protocol.StatusCheck
		if payload.ExitCode == 0 {
			sys = protocol.StatusCheck{Status: "ok", Message: "Last " + payload.Command + " succeeded", CheckedAt: now.Format(time.RFC3339)}
		} else {
			msg := payload.Message
			if msg == "" {
				msg = "Last " + payload.Command + " failed"
			}
			sys = protocol.StatusCheck{Status: "error", Message: msg, CheckedAt: now.Format(time.RFC3339)}
		}
		if data, err := json.Marshal(sys); err == nil {
			s := string(data)
			_, _ = h.db.Exec(`UPDATE hosts SET system_status_json = ? WHERE hostname = ?`, s, hostID)
		}
		// After a successful deployment, tests become outdated for the new deployed state.
		if payload.ExitCode == 0 && payload.Generation != "" {
			tests := protocol.StatusCheck{Status: "outdated", Message: "Tests need re-run after deploy", CheckedAt: now.Format(time.RFC3339)}
			if data, err := json.Marshal(tests); err == nil {
				s := string(data)
				_, _ = h.db.Exec(`UPDATE hosts SET tests_status_json = ?, tests_generation = ? WHERE hostname = ?`, s, payload.Generation, hostID)
			}
		}

	case "test":
		var tests protocol.StatusCheck
		if payload.ExitCode == 0 {
			tests = protocol.StatusCheck{Status: "ok", Message: "Tests passed", CheckedAt: now.Format(time.RFC3339)}
		} else {
			msg := payload.Message
			if msg == "" {
				msg = "Tests failed"
			}
			tests = protocol.StatusCheck{Status: "error", Message: msg, CheckedAt: now.Format(time.RFC3339)}
		}
		if data, err := json.Marshal(tests); err == nil {
			s := string(data)
			gen := payload.Generation
			if gen == "" {
				// Fallback: read current generation from DB
				var g sql.NullString
				_ = h.db.QueryRow(`SELECT generation FROM hosts WHERE hostname = ?`, hostID).Scan(&g)
				if g.Valid {
					gen = g.String
				}
			}
			if gen != "" {
				_, _ = h.db.Exec(`UPDATE hosts SET tests_status_json = ?, tests_generation = ? WHERE hostname = ?`, s, gen, hostID)
			} else {
				_, _ = h.db.Exec(`UPDATE hosts SET tests_status_json = ? WHERE hostname = ?`, s, hostID)
			}
		}
	}

	// Force refresh version fetcher for pull commands BEFORE post-validation
	isPull := payload.Command == "pull" || payload.Command == "pull-switch"
	if h.versionFetcher != nil && isPull {
		h.versionFetcher.ForceRefresh()
		// P7100: Debug logging for git status comparison
		if latest := h.versionFetcher.GetLatest(); latest != nil {
			status, msg, _ := h.versionFetcher.GetGitStatus(payload.Generation)
			h.log.Info().
				Str("host", hostID).
				Str("agent_generation", payload.Generation).
				Str("remote_commit", latest.GitCommit).
				Str("git_status", status).
				Str("git_msg", msg).
				Msg("P7100: git status after pull (pre-validation)")
		}
	}

	// v3: Notify lifecycle manager of command completion
	if h.lifecycleManager != nil {
		_, err := h.lifecycleManager.HandleCommandComplete(hostID, payload.Command, payload.ExitCode, payload.Message)
		if err != nil {
			h.log.Debug().Err(err).Str("host", hostID).Str("command", payload.Command).
				Msg("lifecycle manager did not track this command")
		}
	}

	// Legacy command_complete broadcast removed:
	// - Command lifecycle is tracked via CORE-004 command deltas
	// - Host busy state is tracked via pending_command deltas

	// Broadcast updated host status to browsers
	h.BroadcastHostStatus(hostID)
}

// getHostForPostValidation queries the database for current host state.
// Used by post-validation to compare before/after snapshots.
func (h *Hub) getHostForPostValidation(hostID string) *templates.Host {
	var generation, agentVersion, lockJSON, systemJSON sql.NullString

	err := h.db.QueryRow(`
		SELECT generation, agent_version, lock_status_json, system_status_json
		FROM hosts WHERE hostname = ?
	`, hostID).Scan(&generation, &agentVersion, &lockJSON, &systemJSON)
	if err != nil {
		h.log.Debug().Err(err).Str("host", hostID).Msg("failed to get host for post-validation")
		return nil
	}

	host := &templates.Host{
		ID:       hostID,
		Hostname: hostID,
	}
	if generation.Valid {
		host.Generation = generation.String
	}
	if agentVersion.Valid {
		host.AgentVersion = agentVersion.String
	}

	// Parse status JSON
	var lockStatus, systemStatus templates.StatusCheck
	if lockJSON.Valid {
		_ = json.Unmarshal([]byte(lockJSON.String), &lockStatus)
	}
	if systemJSON.Valid {
		_ = json.Unmarshal([]byte(systemJSON.String), &systemStatus)
	}

	// Get git status from version fetcher
	var gitStatus templates.StatusCheck
	if h.versionFetcher != nil {
		status, msg, checked := h.versionFetcher.GetGitStatus(host.Generation)
		gitStatus = templates.StatusCheck{Status: status, Message: msg, CheckedAt: checked}
	}

	host.UpdateStatus = &templates.UpdateStatus{
		Git:    gitStatus,
		Lock:   lockStatus,
		System: systemStatus,
	}

	return host
}

// GetAgent returns the agent client for a given host ID.
func (h *Hub) GetAgent(hostID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agents[hostID]
}

// GetOnlineHosts returns a list of all currently connected agent host IDs.
func (h *Hub) GetOnlineHosts() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hosts := make([]string, 0, len(h.agents))
	for hostID := range h.agents {
		hosts = append(hosts, hostID)
	}
	return hosts
}

// SendCommand sends a command to a specific agent by host ID.
func (h *Hub) SendCommand(hostID, command string) bool {
	agent := h.GetAgent(hostID)
	if agent == nil {
		h.log.Warn().Str("host", hostID).Msg("cannot send command: agent not connected")
		return false
	}

	msg, err := protocol.NewMessage(protocol.TypeCommand, protocol.CommandPayload{
		Command: command,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create command message")
		return false
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal command message")
		return false
	}

	select {
	case agent.send <- data:
		h.log.Debug().Str("host", hostID).Str("command", command).Msg("command sent")
		return true
	default:
		h.log.Warn().Str("host", hostID).Msg("agent send buffer full")
		return false
	}
}

// SubscribeCommandCompletion creates a channel that receives command completions for specific hosts.
// Caller must call UnsubscribeCommandCompletion when done to prevent leaks.
// The channel is buffered to prevent blocking the hub.
func (h *Hub) SubscribeCommandCompletion(hostIDs []string) chan CommandCompletion {
	ch := make(chan CommandCompletion, len(hostIDs)*2) // Buffer for pull + switch per host

	h.completionSubsMu.Lock()
	defer h.completionSubsMu.Unlock()

	for _, hostID := range hostIDs {
		h.completionSubs[hostID] = append(h.completionSubs[hostID], ch)
	}

	return ch
}

// UnsubscribeCommandCompletion removes a completion channel from all host subscriptions.
func (h *Hub) UnsubscribeCommandCompletion(ch chan CommandCompletion) {
	h.completionSubsMu.Lock()
	defer h.completionSubsMu.Unlock()

	for hostID, subs := range h.completionSubs {
		filtered := make([]chan CommandCompletion, 0, len(subs))
		for _, sub := range subs {
			if sub != ch {
				filtered = append(filtered, sub)
			}
		}
		if len(filtered) == 0 {
			delete(h.completionSubs, hostID)
		} else {
			h.completionSubs[hostID] = filtered
		}
	}

	close(ch)
}

// notifyCommandCompletion sends completion notification to all subscribers for a host.
func (h *Hub) notifyCommandCompletion(hostID, command string, exitCode int) {
	h.completionSubsMu.Lock()
	defer h.completionSubsMu.Unlock()

	subs := h.completionSubs[hostID]
	if len(subs) == 0 {
		return
	}

	completion := CommandCompletion{
		HostID:   hostID,
		Command:  command,
		ExitCode: exitCode,
		Success:  exitCode == 0,
	}

	for _, ch := range subs {
		select {
		case ch <- completion:
		default:
			// Channel full, subscriber not keeping up - skip to avoid blocking hub
			h.log.Warn().Str("host", hostID).Msg("completion subscriber not keeping up")
		}
	}
}

// BroadcastTypedMessage broadcasts a typed message to all browsers.
// This is a convenience wrapper that creates the standard message format.
func (h *Hub) BroadcastTypedMessage(msgType string, payload interface{}) {
	h.queueBroadcast(map[string]any{
		"type":    msgType,
		"payload": payload,
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// P1100: PendingCommandStore implementation
// Hub implements ops.PendingCommandStore to be the single source of truth
// for pending_command state. LifecycleManager calls these methods.
// ═══════════════════════════════════════════════════════════════════════════

// SetPendingCommand sets the pending_command for a host.
// Called by LifecycleManager when a command starts.
func (h *Hub) SetPendingCommand(hostID string, command *string) error {
	var cmd interface{}
	if command != nil {
		cmd = *command
	}
	_, err := h.db.Exec(`UPDATE hosts SET pending_command = ? WHERE hostname = ? OR id = ?`, cmd, hostID, hostID)
	if err != nil {
		h.log.Error().Err(err).Str("host", hostID).Msg("failed to set pending_command")
	}

	// CORE-004: Emit host_updated delta for pending_command
	if h.stateManager != nil {
		var cmd any
		if command != nil {
			cmd = *command
		} else {
			cmd = nil
		}
		h.stateManager.ApplyChange(syncproto.Change{
			Type:   syncproto.ChangeHostUpdated,
			ID:     hostID,
			Fields: map[string]any{"pending_command": cmd},
		})
	}
	return err
}

// ClearPendingCommand clears the pending_command for a host.
// Called by LifecycleManager when a command completes (any terminal state).
func (h *Hub) ClearPendingCommand(hostID string) error {
	_, err := h.db.Exec(`UPDATE hosts SET pending_command = NULL WHERE hostname = ? OR id = ?`, hostID, hostID)
	if err != nil {
		h.log.Error().Err(err).Str("host", hostID).Msg("failed to clear pending_command")
	}

	// CORE-004: Emit host_updated delta for pending_command clear
	if h.stateManager != nil {
		h.stateManager.ApplyChange(syncproto.Change{
			Type:   syncproto.ChangeHostUpdated,
			ID:     hostID,
			Fields: map[string]any{"pending_command": nil},
		})
	}
	return err
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	// Explicitly handle pings by sending pongs (gorilla/websocket does this by default,
	// but being explicit ensures it works through all proxies)
	c.conn.SetPingHandler(func(appData string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.log.Error().Err(err).Msg("read error")
			}
			return
		}

		// Reset read deadline on any received message
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if c.clientType == "agent" {
			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				c.hub.log.Warn().Err(err).Msg("failed to parse message")
				continue
			}
			c.hub.agentMessages <- &agentMessage{client: c, message: &msg}
		} else {
			// Handle browser messages (subscriptions, etc.)
			c.handleBrowserMessage(data)
		}
	}
}

// writePump pumps messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleBrowserMessage processes messages from browser clients.
func (c *Client) handleBrowserMessage(data []byte) {
	var msg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case string(syncproto.TypeGetState):
		// CORE-004: explicit full_state request
		if c.server != nil && c.server.stateManager != nil {
			c.server.stateManager.HandleMessage(c, syncproto.TypeGetState)
		}
	case "subscribe":
		// Browser subscribing to host updates
		c.hub.log.Debug().Str("browser", c.clientID).Msg("browser subscribed")
	case "unsubscribe":
		// Browser unsubscribing
		c.hub.log.Debug().Str("browser", c.clientID).Msg("browser unsubscribed")
	}
}
