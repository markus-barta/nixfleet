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
	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// setupDashboardForCommands creates a test dashboard for command tests.
func setupDashboardForCommands(t *testing.T) *testDashboardCmd {
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

	return &testDashboardCmd{
		t:          t,
		server:     ts,
		password:   password,
		agentToken: agentToken,
	}
}

type testDashboardCmd struct {
	t          *testing.T
	server     *httptest.Server
	password   string
	agentToken string
}

func (td *testDashboardCmd) Close() {
	td.server.Close()
}

func (td *testDashboardCmd) WSURL() string {
	return "ws" + strings.TrimPrefix(td.server.URL, "http") + "/ws"
}

func (td *testDashboardCmd) URL() string {
	return td.server.URL
}

// connectAgentCmd creates an agent WebSocket connection.
func (td *testDashboardCmd) connectAgentCmd(t *testing.T, hostname string) *websocket.Conn {
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

// loginAndGetCSRF logs in and returns the session cookie + CSRF token.
func (td *testDashboardCmd) loginAndGetCSRF(t *testing.T) (*http.Client, string) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Login
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Extract CSRF token
	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	if csrfStart == -1 {
		t.Fatalf("CSRF token not found in response")
	}
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	return client, csrfToken
}

// TestDashboardCommand_QueuePull tests queuing a pull command.
func TestDashboardCommand_QueuePull(t *testing.T) {
	t.Skip("TODO: Fix goroutine cleanup - test hangs")
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Connect agent
	agentConn := td.connectAgentCmd(t, "cmd-test-host")
	defer func() { _ = agentConn.Close() }()

	// Collect messages from agent in background with timeout
	var agentMessages []protocol.Message
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_ = agentConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, data, err := agentConn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err == nil {
				mu.Lock()
				agentMessages = append(agentMessages, msg)
				mu.Unlock()
			}
		}
	}()

	// Give hub time to register agent
	time.Sleep(100 * time.Millisecond)

	// Login and get CSRF token
	client, csrfToken := td.loginAndGetCSRF(t)

	// Send pull command
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/cmd-test-host/command",
		strings.NewReader(`{"command": "pull"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("command request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", result["status"])
	}
	if result["command"] != "pull" {
		t.Errorf("expected command 'pull', got %v", result["command"])
	}

	// Wait briefly for agent to receive command, then close connection to stop goroutine
	time.Sleep(300 * time.Millisecond)
	_ = agentConn.Close()

	// Wait for goroutine to finish with timeout
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("warning: message reader did not exit cleanly")
	}

	// Check agent received command
	mu.Lock()
	defer mu.Unlock()

	var foundCommand bool
	for _, msg := range agentMessages {
		if msg.Type == protocol.TypeCommand {
			foundCommand = true
			var payload protocol.CommandPayload
			if err := msg.ParsePayload(&payload); err == nil {
				if payload.Command != "pull" {
					t.Errorf("expected command 'pull', got %s", payload.Command)
				}
				t.Logf("agent received command: %s", payload.Command)
			}
			break
		}
	}

	if !foundCommand {
		t.Error("agent did not receive command")
		t.Logf("agent messages: %v", agentMessages)
	}
}

// TestDashboardCommand_NonExistentHost tests rejection for non-existent hosts.
func TestDashboardCommand_NonExistentHost(t *testing.T) {
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Login without connecting any agent
	client, csrfToken := td.loginAndGetCSRF(t)

	// Send command to non-existent host
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/non-existent-host/command",
		strings.NewReader(`{"command": "pull"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("command request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Should return 404 Not Found for non-existent host
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent host, got %d: %s", resp.StatusCode, body)
	}

	t.Logf("non-existent host correctly rejected: %s", body)
}

// TestDashboardCommand_UnauthenticatedReject tests rejection without auth.
func TestDashboardCommand_UnauthenticatedReject(t *testing.T) {
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Send command without login
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/any-host/command",
		strings.NewReader(`{"command": "pull"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	// Should redirect to login (302) since requireAuth middleware redirects
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect to login, got %d", resp.StatusCode)
	}

	t.Log("unauthenticated request correctly rejected")
}

// TestDashboardCommand_CSRFReject tests rejection without CSRF token.
func TestDashboardCommand_CSRFReject(t *testing.T) {
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Login but don't use CSRF token
	client, _ := td.loginAndGetCSRF(t)

	// Send command without CSRF token
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/any-host/command",
		strings.NewReader(`{"command": "pull"}`))
	req.Header.Set("Content-Type", "application/json")
	// Intentionally NOT setting X-CSRF-Token

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	// Should return 403 Forbidden
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without CSRF, got %d", resp.StatusCode)
	}

	t.Log("missing CSRF token correctly rejected")
}

// TestDashboardCommand_StatusUpdate tests that status updates are broadcast to browsers.
func TestDashboardCommand_StatusUpdate(t *testing.T) {
	t.Skip("SKIP: Test hangs waiting for agent connection")
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Connect agent
	agentConn := td.connectAgentCmd(t, "status-test-host")
	defer func() { _ = agentConn.Close() }()

	time.Sleep(100 * time.Millisecond)

	// Login and connect browser
	client, csrfToken := td.loginAndGetCSRF(t)

	// Get session cookie for browser WebSocket
	serverURL, _ := url.Parse(td.URL())
	cookies := client.Jar.Cookies(serverURL)
	var sessionCookie string
	for _, c := range cookies {
		if c.Name == "nixfleet_session" {
			sessionCookie = c.Value
			break
		}
	}

	// Connect browser WebSocket
	header := http.Header{}
	header.Set("Cookie", "nixfleet_session="+sessionCookie)
	browserConn, _, err := websocket.DefaultDialer.Dial(td.WSURL(), header)
	if err != nil {
		t.Fatalf("failed to connect browser: %v", err)
	}
	defer func() { _ = browserConn.Close() }()

	// Collect browser messages
	var browserMessages []map[string]any
	var mu sync.Mutex

	go func() {
		for {
			_ = browserConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, data, err := browserConn.ReadMessage()
			if err != nil {
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

	// Send command
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/status-test-host/command",
		strings.NewReader(`{"command": "switch"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp, _ := client.Do(req)
	_ = resp.Body.Close()

	// Wait for command to be delivered to agent
	time.Sleep(200 * time.Millisecond)

	// Agent sends status update
	statusMsg, _ := protocol.NewMessage(protocol.TypeStatus, protocol.StatusPayload{
		Status:   "ok",
		Command:  "switch",
		ExitCode: 0,
		Message:  "Switched to generation 42",
	})
	statusData, _ := json.Marshal(statusMsg)
	if err := agentConn.WriteMessage(websocket.TextMessage, statusData); err != nil {
		t.Fatalf("failed to send status: %v", err)
	}

	// Wait for broadcast
	time.Sleep(500 * time.Millisecond)

	// Check browser received command_complete
	mu.Lock()
	defer mu.Unlock()

	var foundStatus bool
	for _, msg := range browserMessages {
		if msg["type"] == "command_complete" {
			foundStatus = true
			payload := msg["payload"].(map[string]any)
			if payload["status"] != "ok" {
				t.Errorf("expected status 'ok', got %v", payload["status"])
			}
			t.Logf("browser received status: %v", payload)
			break
		}
	}

	if !foundStatus {
		t.Error("browser did not receive command_complete")
		t.Logf("browser messages: %v", browserMessages)
	}
}

// TestDashboardCommand_StopCommand tests the stop command.
func TestDashboardCommand_StopCommand(t *testing.T) {
	t.Skip("SKIP: Test hangs waiting for agent connection")
	td := setupDashboardForCommands(t)
	defer td.Close()

	// Connect agent
	agentConn := td.connectAgentCmd(t, "stop-test-host")
	defer func() { _ = agentConn.Close() }()

	// Collect agent messages
	var agentMessages []protocol.Message
	var mu sync.Mutex

	go func() {
		for {
			_ = agentConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, data, err := agentConn.ReadMessage()
			if err != nil {
				return
			}
			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err == nil {
				mu.Lock()
				agentMessages = append(agentMessages, msg)
				mu.Unlock()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Login
	client, csrfToken := td.loginAndGetCSRF(t)

	// Send stop command
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts/stop-test-host/command",
		strings.NewReader(`{"command": "stop"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("stop command failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Wait for agent to receive
	time.Sleep(500 * time.Millisecond)

	// Verify agent received stop
	mu.Lock()
	defer mu.Unlock()

	var foundStop bool
	for _, msg := range agentMessages {
		if msg.Type == protocol.TypeCommand {
			var payload protocol.CommandPayload
			if err := msg.ParsePayload(&payload); err == nil {
				if payload.Command == "stop" {
					foundStop = true
					t.Log("agent received stop command")
					break
				}
			}
		}
	}

	if !foundStop {
		t.Error("agent did not receive stop command")
	}
}

