// Package sync implements the NixFleet State Sync Protocol (CORE-004).
//
// The State Sync Protocol ensures the browser UI is always live:
// - No stale data after reconnection
// - Automatic drift detection and recovery
// - Version-based state tracking
package sync

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// MessageType defines the types of sync messages.
type MessageType string

const (
	// Server → Client messages
	TypeInit      MessageType = "init"       // Full state on connect
	TypeDelta     MessageType = "delta"      // Incremental change
	TypeSync      MessageType = "sync"       // Periodic version beacon
	TypeFullState MessageType = "full_state" // Response to get_state

	// Client → Server messages
	TypeGetState MessageType = "get_state" // Request full state
)

// Message is the wire format for sync protocol messages.
type Message struct {
	Type    MessageType `json:"type"`
	Version uint64      `json:"version"`
	Payload any         `json:"payload,omitempty"`
}

// ChangeType defines the types of state changes.
type ChangeType string

const (
	ChangeHostAdded       ChangeType = "host_added"
	ChangeHostUpdated     ChangeType = "host_updated"
	ChangeHostRemoved     ChangeType = "host_removed"
	ChangeCommandStarted  ChangeType = "command_started"
	ChangeCommandProgress ChangeType = "command_progress"
	ChangeCommandFinished ChangeType = "command_finished"
	ChangeEvent           ChangeType = "event"
)

// Change represents a single state mutation.
type Change struct {
	Type    ChangeType `json:"type"`
	ID      string     `json:"id,omitempty"`      // Entity ID (host, command, etc.)
	Fields  any        `json:"fields,omitempty"`  // Changed fields
	Payload any        `json:"payload,omitempty"` // Full entity for adds
}

// FullState is the complete state sent on init/full_state.
type FullState struct {
	Hosts     []any `json:"hosts"`     // Host objects
	Commands  []any `json:"commands"`  // Active/recent commands
	Pipelines []any `json:"pipelines"` // Active/recent pipelines
	Events    []any `json:"events"`    // Last N events
}

// VersionStore provides access to the state version.
type VersionStore interface {
	GetVersion() uint64
	IncrementVersion() uint64
}

// StateProvider provides the current full state.
type StateProvider interface {
	GetFullState() FullState
}

// ClientSender can send messages to a client.
type ClientSender interface {
	Send(data []byte) error
}

// StateManager manages state versioning and client synchronization.
type StateManager struct {
	log      zerolog.Logger
	store    VersionStore
	provider StateProvider

	// Connected clients
	clients   map[ClientSender]uint64 // client -> last known version
	clientsMu sync.RWMutex

	// Beacon configuration
	beaconInterval time.Duration
	beaconStop     chan struct{}
}

// NewStateManager creates a new state manager.
func NewStateManager(log zerolog.Logger, store VersionStore, provider StateProvider) *StateManager {
	return &StateManager{
		log:            log.With().Str("component", "state_sync").Logger(),
		store:          store,
		provider:       provider,
		clients:        make(map[ClientSender]uint64),
		beaconInterval: 30 * time.Second,
		beaconStop:     make(chan struct{}),
	}
}

// RegisterClient adds a client and sends initial state.
func (sm *StateManager) RegisterClient(client ClientSender) {
	sm.clientsMu.Lock()
	sm.clients[client] = 0
	sm.clientsMu.Unlock()

	// Send full state
	sm.sendInit(client)
}

// UnregisterClient removes a client.
func (sm *StateManager) UnregisterClient(client ClientSender) {
	sm.clientsMu.Lock()
	delete(sm.clients, client)
	sm.clientsMu.Unlock()
}

// HandleMessage processes incoming client messages.
func (sm *StateManager) HandleMessage(client ClientSender, msgType MessageType) {
	switch msgType {
	case TypeGetState:
		sm.sendFullState(client)
	}
}

// ApplyChange records a state change and broadcasts delta to clients.
func (sm *StateManager) ApplyChange(change Change) {
	version := sm.store.IncrementVersion()

	msg := Message{
		Type:    TypeDelta,
		Version: version,
		Payload: change,
	}

	sm.broadcast(msg)
}

// StartBeacon starts the periodic sync beacon.
func (sm *StateManager) StartBeacon() {
	go func() {
		ticker := time.NewTicker(sm.beaconInterval)
		defer ticker.Stop()

		for {
			select {
			case <-sm.beaconStop:
				return
			case <-ticker.C:
				sm.sendBeacon()
			}
		}
	}()
}

// StopBeacon stops the periodic sync beacon.
func (sm *StateManager) StopBeacon() {
	close(sm.beaconStop)
}

// sendInit sends full state to a client on connect.
func (sm *StateManager) sendInit(client ClientSender) {
	version := sm.store.GetVersion()
	state := sm.provider.GetFullState()

	msg := Message{
		Type:    TypeInit,
		Version: version,
		Payload: state,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		sm.log.Error().Err(err).Msg("failed to marshal init message")
		return
	}

	if err := client.Send(data); err != nil {
		sm.log.Error().Err(err).Msg("failed to send init message")
	}

	sm.clientsMu.Lock()
	sm.clients[client] = version
	sm.clientsMu.Unlock()
}

// sendFullState sends full state in response to get_state request.
func (sm *StateManager) sendFullState(client ClientSender) {
	version := sm.store.GetVersion()
	state := sm.provider.GetFullState()

	msg := Message{
		Type:    TypeFullState,
		Version: version,
		Payload: state,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		sm.log.Error().Err(err).Msg("failed to marshal full_state message")
		return
	}

	if err := client.Send(data); err != nil {
		sm.log.Error().Err(err).Msg("failed to send full_state message")
	}

	sm.clientsMu.Lock()
	sm.clients[client] = version
	sm.clientsMu.Unlock()
}

// sendBeacon sends version beacon to all clients.
func (sm *StateManager) sendBeacon() {
	version := sm.store.GetVersion()

	msg := Message{
		Type:    TypeSync,
		Version: version,
	}

	sm.broadcast(msg)
}

// broadcast sends a message to all connected clients.
func (sm *StateManager) broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		sm.log.Error().Err(err).Msg("failed to marshal broadcast message")
		return
	}

	sm.clientsMu.RLock()
	defer sm.clientsMu.RUnlock()

	for client := range sm.clients {
		if err := client.Send(data); err != nil {
			sm.log.Debug().Err(err).Msg("failed to send to client")
			// Client will be unregistered when connection closes
		}
	}
}

// GetVersion returns the current state version.
func (sm *StateManager) GetVersion() uint64 {
	return sm.store.GetVersion()
}

// ClientCount returns the number of connected clients.
func (sm *StateManager) ClientCount() int {
	sm.clientsMu.RLock()
	defer sm.clientsMu.RUnlock()
	return len(sm.clients)
}

