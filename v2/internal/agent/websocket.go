package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
	"github.com/rs/zerolog"
)

// ConnectionHandler is called on connection events.
type ConnectionHandler interface {
	OnConnected()
	OnDisconnected()
}

// WebSocketClient manages the WebSocket connection to the dashboard.
type WebSocketClient struct {
	cfg     *config.Config
	log     zerolog.Logger
	handler ConnectionHandler

	conn     *websocket.Conn
	mu       sync.Mutex
	messages chan *protocol.Message

	// Reconnection
	connected bool
	backoff   time.Duration
}

// Connection parameters (from T01 spec)
const (
	pingInterval     = 30 * time.Second
	pongWait         = 45 * time.Second
	writeWait        = 10 * time.Second
	maxBackoff       = 60 * time.Second
	initialBackoff   = 1 * time.Second
	closeGracePeriod = 5 * time.Second
)

// NewWebSocketClient creates a new WebSocket client.
func NewWebSocketClient(cfg *config.Config, log zerolog.Logger, handler ConnectionHandler) *WebSocketClient {
	return &WebSocketClient{
		cfg:      cfg,
		log:      log.With().Str("component", "websocket").Logger(),
		handler:  handler,
		messages: make(chan *protocol.Message, 100),
		backoff:  initialBackoff,
	}
}

// Run connects to the dashboard and maintains the connection.
// It blocks until the context is cancelled.
func (c *WebSocketClient) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.log.Debug().Msg("context cancelled, stopping")
			return
		default:
		}

		if err := c.connect(ctx); err != nil {
			c.log.Error().Err(err).Dur("backoff", c.backoff).Msg("connection failed, retrying")
			c.waitBackoff(ctx)
			continue
		}

		// Connected - reset backoff
		c.backoff = initialBackoff

		// Read messages until disconnect
		c.readLoop(ctx)

		// Disconnected - wait before reconnecting
		c.waitBackoff(ctx)
	}
}

// connect establishes the WebSocket connection.
func (c *WebSocketClient) connect(ctx context.Context) error {
	c.log.Debug().Str("url", c.cfg.DashboardURL).Msg("connecting")

	// Create request with auth header
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.cfg.Token)

	// Connect with context
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, c.cfg.DashboardURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			c.log.Error().Msg("authentication failed: 401 Unauthorized")
		}
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// Configure connection
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start ping goroutine
	go c.pingLoop(ctx)

	// Notify handler
	c.handler.OnConnected()

	return nil
}

// readLoop reads messages from the WebSocket.
func (c *WebSocketClient) readLoop(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		c.handler.OnDisconnected()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Error().Err(err).Msg("read error")
			}
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.log.Error().Err(err).Str("data", string(data)).Msg("failed to parse message")
			continue // Continue on malformed messages (per T01 spec)
		}

		c.log.Debug().Str("type", msg.Type).Msg("received message")

		select {
		case c.messages <- &msg:
		default:
			c.log.Warn().Msg("message queue full, dropping message")
		}
	}
}

// pingLoop sends periodic pings.
func (c *WebSocketClient) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			conn := c.conn
			connected := c.connected
			c.mu.Unlock()

			if !connected || conn == nil {
				return
			}

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				c.log.Debug().Err(err).Msg("ping failed")
				return
			}
		}
	}
}

// waitBackoff waits for the current backoff duration.
func (c *WebSocketClient) waitBackoff(ctx context.Context) {
	timer := time.NewTimer(c.backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}

	// Exponential backoff
	c.backoff *= 2
	if c.backoff > maxBackoff {
		c.backoff = maxBackoff
	}
}

// SendMessage sends a message to the dashboard.
func (c *WebSocketClient) SendMessage(msgType string, payload any) error {
	msg, err := protocol.NewMessage(msgType, payload)
	if err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return websocket.ErrCloseSent
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Messages returns the channel for incoming messages.
func (c *WebSocketClient) Messages() <-chan *protocol.Message {
	return c.messages
}

// Close closes the connection gracefully.
func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// Send close message
	deadline := time.Now().Add(closeGracePeriod)
	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"),
		deadline,
	)
	if err != nil {
		c.conn.Close()
		return err
	}

	// Wait briefly for close acknowledgment
	time.Sleep(100 * time.Millisecond)
	return c.conn.Close()
}

// IsConnected returns whether the client is connected.
func (c *WebSocketClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

