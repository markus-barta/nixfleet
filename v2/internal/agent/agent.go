// Package agent implements the NixFleet agent.
package agent

import (
	"context"
	"sync"

	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
	"github.com/rs/zerolog"
)

// Agent is the main agent struct that coordinates all components.
type Agent struct {
	cfg    *config.Config
	log    zerolog.Logger
	ws     *WebSocketClient
	ctx    context.Context
	cancel context.CancelFunc

	// State
	mu             sync.RWMutex
	registered     bool
	pendingCommand *string
	commandPID     *int

	// System info (cached)
	generation     string
	nixpkgsVersion string
	osVersion      string

	// Update status checker
	statusChecker *StatusChecker
}

// New creates a new agent with the given configuration.
func New(cfg *config.Config, log zerolog.Logger) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{
		cfg:    cfg,
		log:    log.With().Str("component", "agent").Logger(),
		ctx:    ctx,
		cancel: cancel,
	}
	a.statusChecker = NewStatusChecker(a)
	// Run initial status checks immediately so first heartbeat has data
	a.statusChecker.ForceRefresh(ctx)
	return a
}

// Run starts the agent and blocks until shutdown.
func (a *Agent) Run() error {
	a.log.Info().
		Str("hostname", a.cfg.Hostname).
		Str("url", a.cfg.DashboardURL).
		Str("repo_dir", a.cfg.RepoDir).
		Bool("isolated_mode", a.cfg.RepoURL != "").
		Msg("starting agent")

	// Ensure repository exists (auto-clone in isolated mode)
	if err := a.ensureRepoExists(); err != nil {
		a.log.Error().Err(err).Msg("failed to ensure repository exists")
		return err
	}

	// Detect system info
	a.detectSystemInfo()

	// Create WebSocket client
	a.ws = NewWebSocketClient(a.cfg, a.log, a)

	// Start goroutines
	var wg sync.WaitGroup

	// Heartbeat loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.heartbeatLoop()
	}()

	// Message handler loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.messageLoop()
	}()

	// WebSocket connection loop (blocks until shutdown)
	a.ws.Run(a.ctx)

	// Wait for goroutines
	wg.Wait()

	a.log.Info().Msg("agent stopped")
	return nil
}

// Shutdown initiates graceful shutdown.
func (a *Agent) Shutdown() {
	a.log.Info().Msg("shutting down")
	a.cancel()
	if a.ws != nil {
		if err := a.ws.Close(); err != nil {
			a.log.Debug().Err(err).Msg("error closing websocket")
		}
	}
}

// OnConnected is called when WebSocket connects.
func (a *Agent) OnConnected() {
	a.log.Info().Msg("connected to dashboard")

	// Send registration
	payload := protocol.RegisterPayload{
		Hostname:          a.cfg.Hostname,
		HostType:          a.detectHostType(),
		AgentVersion:      Version,
		OSVersion:         a.osVersion,
		NixpkgsVersion:    a.nixpkgsVersion,
		Generation:        a.generation,
		ThemeColor:        a.cfg.ThemeColor,
		HeartbeatInterval: int(a.cfg.HeartbeatInterval.Seconds()),
		Location:          a.cfg.Location,
		DeviceType:        a.cfg.DeviceType,
		RepoURL:           a.cfg.RepoURL,
		RepoDir:           a.cfg.RepoDir,
	}

	if err := a.ws.SendMessage(protocol.TypeRegister, payload); err != nil {
		a.log.Error().Err(err).Msg("failed to send registration")
		return
	}

	a.log.Debug().Msg("registration sent")
}

// OnDisconnected is called when WebSocket disconnects.
func (a *Agent) OnDisconnected() {
	a.mu.Lock()
	a.registered = false
	a.mu.Unlock()
	a.log.Warn().Msg("disconnected from dashboard")
}

// OnMessage is called for each incoming message.
func (a *Agent) OnMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.TypeRegistered:
		var payload protocol.RegisteredPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log.Error().Err(err).Msg("failed to parse registered payload")
			return
		}
		a.mu.Lock()
		a.registered = true
		a.mu.Unlock()
		a.log.Info().Str("host_id", payload.HostID).Msg("registered with dashboard")

		// Send first heartbeat immediately
		a.sendHeartbeat()

	case protocol.TypeCommand:
		var payload protocol.CommandPayload
		if err := msg.ParsePayload(&payload); err != nil {
			a.log.Error().Err(err).Msg("failed to parse command payload")
			return
		}
		a.handleCommand(payload.Command)

	default:
		a.log.Warn().Str("type", msg.Type).Msg("unknown message type")
	}
}

// IsRegistered returns whether the agent is registered with the dashboard.
func (a *Agent) IsRegistered() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registered
}

// IsBusy returns whether the agent is executing a command.
func (a *Agent) IsBusy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pendingCommand != nil
}

// messageLoop handles incoming messages.
func (a *Agent) messageLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-a.ws.Messages():
			if msg != nil {
				a.OnMessage(msg)
			}
		}
	}
}

// Version is the agent version.
const Version = "2.1.0"

