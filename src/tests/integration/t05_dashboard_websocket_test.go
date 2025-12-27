package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/internal/dashboard"
	"github.com/markus-barta/nixfleet/internal/protocol"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// setupDashboardForWS creates a test dashboard for WebSocket tests.
func setupDashboardForWS(t *testing.T) *testDashboardWS {
	t.Helper()

	password := "testpassword123"
	agentToken := "test-agent-token"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	_ = os.Setenv("NIXFLEET_PASSWORD_HASH", string(hash))
	_ = os.Setenv("NIXFLEET_SESSION_SECRET", "test-session-secret-32-bytes-xx")
	_ = os.Setenv("NIXFLEET_AGENT_TOKEN", agentToken)
	_ = os.Unsetenv("NIXFLEET_TOTP_SECRET")
	_ = os.Setenv("NIXFLEET_DB_PATH", ":memory:")

	cfg, err := dashboard.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	db, err := dashboard.InitDatabase(":memory:")
	if err != nil {
		t.Fatalf("failed to init database: %v", err)
	}

	log := zerolog.New(io.Discard)
	server := dashboard.New(cfg, db, log)

	ts := httptest.NewServer(server.Router())

	return &testDashboardWS{
		t:          t,
		server:     ts,
		password:   password,
		agentToken: agentToken,
	}
}

type testDashboardWS struct {
	t          *testing.T
	server     *httptest.Server
	password   string
	agentToken string
}

func (td *testDashboardWS) Close() {
	td.server.Close()
}

func (td *testDashboardWS) WSURL() string {
	return "ws" + strings.TrimPrefix(td.server.URL, "http") + "/ws"
}

func (td *testDashboardWS) URL() string {
	return td.server.URL
}

// connectAgent creates an agent WebSocket connection.
func (td *testDashboardWS) connectAgent(t *testing.T, hostname string) *websocket.Conn {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+td.agentToken)

	conn, resp, err := websocket.DefaultDialer.Dial(td.WSURL(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("failed to connect agent: %v, status: %d, body: %s", err, resp.StatusCode, body)
		}
		t.Fatalf("failed to connect agent: %v", err)
	}

	// Send registration
	regMsg, _ := protocol.NewMessage(protocol.TypeRegister, protocol.RegisterPayload{
		Hostname:          hostname,
		HostType:          "nixos",
		AgentVersion:      "2.0.0-test",
		OSVersion:         "24.11",
		NixpkgsVersion:    "abc123",
		Generation:        "gen-1",
		HeartbeatInterval: 30,
	})
	data, _ := json.Marshal(regMsg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send registration: %v", err)
	}

	// Wait for registered response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read registered response: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})

	var respMsg protocol.Message
	if err := json.Unmarshal(respData, &respMsg); err != nil {
		t.Fatalf("failed to parse registered response: %v", err)
	}
	if respMsg.Type != protocol.TypeRegistered {
		t.Fatalf("expected 'registered' message, got: %s", respMsg.Type)
	}

	return conn
}

// connectBrowser creates a browser WebSocket connection (requires login first).
func (td *testDashboardWS) connectBrowser(t *testing.T) *websocket.Conn {
	// Login to get session
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_ = resp.Body.Close()

	// Get session cookie
	serverURL, _ := url.Parse(td.URL())
	cookies := jar.Cookies(serverURL)

	var sessionCookie string
	for _, c := range cookies {
		if c.Name == "nixfleet_session" {
			sessionCookie = c.Value
			break
		}
	}
	if sessionCookie == "" {
		t.Fatal("session cookie not found after login")
	}

	// Connect WebSocket with cookie
	header := http.Header{}
	header.Set("Cookie", "nixfleet_session="+sessionCookie)

	conn, resp, err := websocket.DefaultDialer.Dial(td.WSURL(), header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("failed to connect browser: %v, status: %d, body: %s", err, resp.StatusCode, body)
		}
		t.Fatalf("failed to connect browser: %v", err)
	}

	return conn
}

// TestDashboardWebSocket_AgentConnection tests agent WebSocket connection.
func TestDashboardWebSocket_AgentConnection(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	// Connect agent
	conn := td.connectAgent(t, "test-host")
	defer func() { _ = conn.Close() }()

	t.Log("agent connected and registered successfully")

	// Send heartbeat
	hbMsg, _ := protocol.NewMessage(protocol.TypeHeartbeat, protocol.HeartbeatPayload{
		Generation:     "gen-1",
		NixpkgsVersion: "abc123",
	})
	hbData, _ := json.Marshal(hbMsg)
	if err := conn.WriteMessage(websocket.TextMessage, hbData); err != nil {
		t.Fatalf("failed to send heartbeat: %v", err)
	}

	t.Log("heartbeat sent successfully")
}

// TestDashboardWebSocket_BrowserConnection tests browser WebSocket connection.
func TestDashboardWebSocket_BrowserConnection(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	// Connect browser
	conn := td.connectBrowser(t)
	defer func() { _ = conn.Close() }()

	t.Log("browser connected successfully")
}

// TestDashboardWebSocket_RejectUnauthenticated tests rejection of unauthenticated connections.
func TestDashboardWebSocket_RejectUnauthenticated(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	// Try to connect without any auth
	conn, resp, err := websocket.DefaultDialer.Dial(td.WSURL(), nil)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected connection to be rejected")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	t.Log("unauthenticated connection correctly rejected")
}

// TestDashboardWebSocket_RejectInvalidToken tests rejection of invalid agent tokens.
func TestDashboardWebSocket_RejectInvalidToken(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer wrong-token")

	conn, resp, err := websocket.DefaultDialer.Dial(td.WSURL(), header)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected connection with invalid token to be rejected")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	t.Log("invalid token correctly rejected")
}

// TestDashboardWebSocket_MessageRouting tests that agent messages are routed to browsers.
func TestDashboardWebSocket_MessageRouting(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	// Connect agent first
	agentConn := td.connectAgent(t, "routing-test-host")
	defer func() { _ = agentConn.Close() }()

	// Give hub time to register agent
	time.Sleep(100 * time.Millisecond)

	// Connect browser
	browserConn := td.connectBrowser(t)
	defer func() { _ = browserConn.Close() }()

	// Collect messages from browser in background
	var browserMessages []map[string]any
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		for {
			_ = browserConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, data, err := browserConn.ReadMessage()
			if err != nil {
				close(done)
				return
			}
			var msg map[string]any
			if err := json.Unmarshal(data, &msg); err == nil {
				mu.Lock()
				browserMessages = append(browserMessages, msg)
				mu.Unlock()
			}
		}
	}()

	// Agent sends heartbeat
	hbMsg, _ := protocol.NewMessage(protocol.TypeHeartbeat, protocol.HeartbeatPayload{
		Generation:     "gen-2",
		NixpkgsVersion: "def456",
		Metrics: &protocol.Metrics{
			CPU:  25.5,
			RAM:  40.0,
			Swap: 5.0,
			Load: 1.2,
		},
	})
	hbData, _ := json.Marshal(hbMsg)
	if err := agentConn.WriteMessage(websocket.TextMessage, hbData); err != nil {
		t.Fatalf("failed to send heartbeat: %v", err)
	}

	// Wait for message propagation
	time.Sleep(500 * time.Millisecond)

	// Check browser received host_heartbeat
	mu.Lock()
	defer mu.Unlock()

	var foundHostUpdate bool
	for _, msg := range browserMessages {
		if msg["type"] == "host_heartbeat" {
			foundHostUpdate = true
			payload := msg["payload"].(map[string]any)
			if payload["host_id"] != "routing-test-host" {
				t.Errorf("expected host_id 'routing-test-host', got %v", payload["host_id"])
			}
			t.Logf("browser received host_heartbeat: %v", payload)
			break
		}
	}

	if !foundHostUpdate {
		t.Error("browser did not receive host_heartbeat")
		t.Logf("received messages: %v", browserMessages)
	}
}

// TestDashboardWebSocket_MultipleBrowsers tests that multiple browsers receive updates.
func TestDashboardWebSocket_MultipleBrowsers(t *testing.T) {
	td := setupDashboardForWS(t)
	defer td.Close()

	// Connect agent
	agentConn := td.connectAgent(t, "multi-browser-host")
	defer func() { _ = agentConn.Close() }()

	time.Sleep(100 * time.Millisecond)

	// Connect multiple browsers
	numBrowsers := 3
	browsers := make([]*websocket.Conn, numBrowsers)
	for i := 0; i < numBrowsers; i++ {
		browsers[i] = td.connectBrowser(t)
		defer func(c *websocket.Conn) { _ = c.Close() }(browsers[i])
	}

	// Collect messages from all browsers
	type browserMsg struct {
		browserIdx int
		msg        map[string]any
	}
	msgChan := make(chan browserMsg, 100)

	for i, conn := range browsers {
		go func(idx int, c *websocket.Conn) {
			for {
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, data, err := c.ReadMessage()
				if err != nil {
					return
				}
				var msg map[string]any
				if err := json.Unmarshal(data, &msg); err == nil {
					msgChan <- browserMsg{browserIdx: idx, msg: msg}
				}
			}
		}(i, conn)
	}

	// Agent sends heartbeat
	hbMsg, _ := protocol.NewMessage(protocol.TypeHeartbeat, protocol.HeartbeatPayload{
		Generation:     "gen-3",
		NixpkgsVersion: "ghi789",
	})
	hbData, _ := json.Marshal(hbMsg)
	if err := agentConn.WriteMessage(websocket.TextMessage, hbData); err != nil {
		t.Fatalf("failed to send heartbeat: %v", err)
	}

	// Wait and collect
	time.Sleep(500 * time.Millisecond)
	close(msgChan)

	// Count host_heartbeats per browser
	receivedByBrowser := make(map[int]bool)
	for bm := range msgChan {
		if bm.msg["type"] == "host_heartbeat" {
			receivedByBrowser[bm.browserIdx] = true
		}
	}

	for i := 0; i < numBrowsers; i++ {
		if !receivedByBrowser[i] {
			t.Errorf("browser %d did not receive host_heartbeat", i)
		}
	}

	t.Logf("all %d browsers received host_heartbeat", numBrowsers)
}

