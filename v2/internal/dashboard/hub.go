package dashboard

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
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
)

// Client represents a WebSocket connection (agent or browser).
type Client struct {
	conn       *websocket.Conn
	clientType string // "agent" or "browser"
	clientID   string // hostname for agents, session ID for browsers
	send       chan []byte
	hub        *Hub
	server     *Server
}

// Hub maintains active connections and broadcasts messages.
type Hub struct {
	log zerolog.Logger
	db  *sql.DB

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

	// Log storage for command output
	logStore *LogStore

	mu sync.RWMutex
}

type agentMessage struct {
	client  *Client
	message *protocol.Message
}

// NewHub creates a new Hub.
func NewHub(log zerolog.Logger, db *sql.DB) *Hub {
	return &Hub{
		log:           log.With().Str("component", "hub").Logger(),
		db:            db,
		clients:       make(map[*Client]bool),
		agents:        make(map[string]*Client),
		browsers:      make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		agentMessages: make(chan *agentMessage, 256),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
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

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.browsers, client)
				if client.clientType == "agent" && client.clientID != "" {
					if h.agents[client.clientID] == client {
						delete(h.agents, client.clientID)
						// Mark host as offline in database
						_, err := h.db.Exec(`UPDATE hosts SET status = 'offline' WHERE hostname = ?`, client.clientID)
						if err != nil {
							h.log.Error().Err(err).Str("host", client.clientID).Msg("failed to mark host offline")
						}
						// Broadcast offline status to browsers
						h.broadcastHostOffline(client.clientID)
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
			h.log.Debug().
				Str("type", client.clientType).
				Str("id", client.clientID).
				Msg("client unregistered")

		case msg := <-h.agentMessages:
			h.handleAgentMessage(msg)
		}
	}
}

// handleAgentMessage processes messages from agents.
func (h *Hub) handleAgentMessage(msg *agentMessage) {
	switch msg.message.Type {
	case protocol.TypeRegister:
		var payload protocol.RegisterPayload
		if err := msg.message.ParsePayload(&payload); err != nil {
			h.log.Error().Err(err).Msg("failed to parse register payload")
			return
		}

		h.mu.Lock()
		// Check for duplicate hostname
		if existing, ok := h.agents[payload.Hostname]; ok && existing != msg.client {
			// Close old connection
			close(existing.send)
			h.log.Warn().Str("hostname", payload.Hostname).Msg("replaced duplicate agent")
		}
		msg.client.clientID = payload.Hostname
		h.agents[payload.Hostname] = msg.client
		h.mu.Unlock()

		// Update host in database
		h.updateHost(payload)

		// Send registered response
		resp, _ := protocol.NewMessage(protocol.TypeRegistered, protocol.RegisteredPayload{
			HostID: payload.Hostname,
		})
		respData, _ := json.Marshal(resp)
		msg.client.send <- respData

		h.log.Info().
			Str("hostname", payload.Hostname).
			Str("agent_version", payload.AgentVersion).
			Msg("agent registered")

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

	// Upsert host record
	_, err := h.db.Exec(`
		INSERT INTO hosts (id, hostname, host_type, agent_version, os_version, nixpkgs_version, generation, theme_color, last_seen, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), 'online')
		ON CONFLICT(hostname) DO UPDATE SET
			host_type = excluded.host_type,
			agent_version = excluded.agent_version,
			os_version = excluded.os_version,
			nixpkgs_version = excluded.nixpkgs_version,
			generation = excluded.generation,
			theme_color = excluded.theme_color,
			last_seen = datetime('now'),
			status = 'online'
	`, payload.Hostname, payload.Hostname, payload.HostType, payload.AgentVersion,
		payload.OSVersion, payload.NixpkgsVersion, payload.Generation, themeColor)

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

	// Update host last_seen and status in database
	_, err := h.db.Exec(`
		UPDATE hosts SET 
			last_seen = datetime('now'),
			status = 'online',
			generation = ?,
			nixpkgs_version = ?,
			pending_command = ?,
			metrics_json = ?
		WHERE hostname = ?
	`, payload.Generation, payload.NixpkgsVersion, payload.PendingCommand, metricsJSON, hostID)

	if err != nil {
		h.log.Error().Err(err).Str("host", hostID).Msg("failed to update heartbeat")
	}

	h.log.Debug().
		Str("host", hostID).
		Str("generation", payload.Generation).
		Msg("heartbeat received")

	// Broadcast to browsers
	h.BroadcastToBrowsers(map[string]any{
		"type": "host_update",
		"payload": map[string]any{
			"host_id":         hostID,
			"online":          true,
			"last_seen":       time.Now().Format(time.RFC3339),
			"generation":      payload.Generation,
			"nixpkgs_version": payload.NixpkgsVersion,
			"pending_command": payload.PendingCommand,
			"metrics":         payload.Metrics,
		},
	})
}

func (h *Hub) handleStatus(hostID string, payload protocol.StatusPayload) {
	h.log.Info().
		Str("host", hostID).
		Str("command", payload.Command).
		Str("status", payload.Status).
		Msg("command status")

	// Complete the log file
	if h.logStore != nil {
		_ = h.logStore.CompleteCommand(hostID, payload.Command, payload.ExitCode)
	}

	// Broadcast to browsers
	h.BroadcastToBrowsers(map[string]any{
		"type": "command_complete",
		"payload": map[string]any{
			"host_id":   hostID,
			"command":   payload.Command,
			"status":    payload.Status,
			"message":   payload.Message,
			"exit_code": payload.ExitCode,
		},
	})
}

// GetAgent returns the agent client for a given host ID.
func (h *Hub) GetAgent(hostID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agents[hostID]
}

// BroadcastToBrowsers sends a message to all connected browsers.
func (h *Hub) BroadcastToBrowsers(msg map[string]any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	browsers := make([]*Client, 0, len(h.browsers))
	for client := range h.browsers {
		browsers = append(browsers, client)
	}
	h.mu.RUnlock()

	for _, client := range browsers {
		select {
		case client.send <- data:
		default:
			// Client send buffer full, skip
		}
	}
}

// broadcastHostOffline notifies browsers that a host went offline.
func (h *Hub) broadcastHostOffline(hostID string) {
	h.BroadcastToBrowsers(map[string]any{
		"type": "host_update",
		"payload": map[string]any{
			"host_id": hostID,
			"online":  false,
			"status":  "offline",
		},
	})
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
	case "subscribe":
		// Browser subscribing to host updates
		c.hub.log.Debug().Str("browser", c.clientID).Msg("browser subscribed")
	case "unsubscribe":
		// Browser unsubscribing
		c.hub.log.Debug().Str("browser", c.clientID).Msg("browser unsubscribed")
	}
}

