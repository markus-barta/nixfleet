// Package integration contains integration tests for NixFleet v2.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
)

// MockDashboard simulates the dashboard WebSocket server for testing.
type MockDashboard struct {
	t         *testing.T
	server    *httptest.Server
	upgrader  websocket.Upgrader
	mu        sync.Mutex
	conns     []*websocket.Conn
	messages  []protocol.Message
	authToken string

	// Callbacks
	OnRegister func(conn *websocket.Conn, payload protocol.RegisterPayload)
}

// NewMockDashboard creates a new mock dashboard.
func NewMockDashboard(t *testing.T) *MockDashboard {
	m := &MockDashboard{
		t:         t,
		authToken: "test-token",
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	m.server = httptest.NewServer(http.HandlerFunc(m.handleWS))
	return m
}

// URL returns the WebSocket URL for the mock dashboard.
func (m *MockDashboard) URL() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http") + "/ws"
}

// Close shuts down the mock dashboard.
func (m *MockDashboard) Close() {
	m.mu.Lock()
	for _, conn := range m.conns {
		_ = conn.Close() // Ignore close errors in cleanup
	}
	m.mu.Unlock()
	m.server.Close()
}

// SetAuthToken sets the expected auth token.
func (m *MockDashboard) SetAuthToken(token string) {
	m.authToken = token
}

// Messages returns all received messages.
func (m *MockDashboard) Messages() []protocol.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]protocol.Message{}, m.messages...)
}

// MessagesOfType returns messages of a specific type.
func (m *MockDashboard) MessagesOfType(msgType string) []protocol.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []protocol.Message
	for _, msg := range m.messages {
		if msg.Type == msgType {
			result = append(result, msg)
		}
	}
	return result
}

// WaitForMessage waits for a message of the given type.
func (m *MockDashboard) WaitForMessage(ctx context.Context, msgType string) (*protocol.Message, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			msgs := m.MessagesOfType(msgType)
			if len(msgs) > 0 {
				return &msgs[len(msgs)-1], nil
			}
		}
	}
}

// WaitForNMessages waits for n messages of the given type.
func (m *MockDashboard) WaitForNMessages(ctx context.Context, msgType string, n int) ([]protocol.Message, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			msgs := m.MessagesOfType(msgType)
			if len(msgs) >= n {
				return msgs[:n], nil
			}
		}
	}
}

// SendCommand sends a command to the first connected agent.
func (m *MockDashboard) SendCommand(command string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.conns) == 0 {
		return nil
	}

	msg, _ := protocol.NewMessage(protocol.TypeCommand, protocol.CommandPayload{
		Command: command,
	})
	data, _ := json.Marshal(msg)
	return m.conns[0].WriteMessage(websocket.TextMessage, data)
}

// SendRaw sends a raw message to the first connected agent.
func (m *MockDashboard) SendRaw(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.conns) == 0 {
		return nil
	}

	return m.conns[0].WriteMessage(websocket.TextMessage, data)
}

// ConnectionCount returns the number of active connections.
func (m *MockDashboard) ConnectionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.conns)
}

// handleWS handles WebSocket upgrade and message processing.
func (m *MockDashboard) handleWS(w http.ResponseWriter, r *http.Request) {
	// Check auth token
	authHeader := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + m.authToken
	if authHeader != expectedAuth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.t.Logf("WebSocket upgrade failed: %v", err)
		return
	}

	m.mu.Lock()
	m.conns = append(m.conns, conn)
	m.mu.Unlock()

	defer func() {
		_ = conn.Close() // Ignore close error in cleanup
		m.mu.Lock()
		for i, c := range m.conns {
			if c == conn {
				m.conns = append(m.conns[:i], m.conns[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
	}()

	// Read messages
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			m.t.Logf("Failed to parse message: %v", err)
			continue
		}

		m.mu.Lock()
		m.messages = append(m.messages, msg)
		m.mu.Unlock()

		// Handle registration
		if msg.Type == protocol.TypeRegister {
			var payload protocol.RegisterPayload
			if err := msg.ParsePayload(&payload); err == nil {
				if m.OnRegister != nil {
					m.OnRegister(conn, payload)
				} else {
					// Default: send registered response
					resp, _ := protocol.NewMessage(protocol.TypeRegistered, protocol.RegisteredPayload{
						HostID: payload.Hostname,
					})
					respData, _ := json.Marshal(resp)
					_ = conn.WriteMessage(websocket.TextMessage, respData) // Ignore error in test
				}
			}
		}
	}
}

// MakeTestConfig creates a test configuration.
func MakeTestConfig(dashboardURL string) map[string]string {
	return map[string]string{
		"NIXFLEET_URL":      dashboardURL,
		"NIXFLEET_TOKEN":    "test-token",
		"NIXFLEET_REPO_DIR": "/tmp/nixfleet-test",
		"NIXFLEET_INTERVAL": "1", // 1 second for faster tests
		"NIXFLEET_HOSTNAME": "test-host",
	}
}

// WithEnv runs a function with temporary environment variables.
func WithEnv(env map[string]string, fn func()) {
	// Save original values
	original := make(map[string]string)
	for k := range env {
		original[k] = ""
		if v, ok := envLookup(k); ok {
			original[k] = v
		}
	}

	// Set new values
	for k, v := range env {
		envSet(k, v)
	}

	// Run function
	fn()

	// Restore original values
	for k, v := range original {
		if v == "" {
			envUnset(k)
		} else {
			envSet(k, v)
		}
	}
}

// envLookup, envSet, envUnset are wrappers to avoid import cycle
func envLookup(key string) (string, bool) {
	return lookupEnv(key)
}

func envSet(key, value string) {
	setEnv(key, value)
}

func envUnset(key string) {
	unsetEnv(key)
}

