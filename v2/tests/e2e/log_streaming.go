// Package e2e provides end-to-end tests for NixFleet functionality.
// Run with: go run ./tests/e2e/log_streaming.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var (
	dashboardURL = flag.String("url", "wss://fleet.barta.cm/ws", "Dashboard WebSocket URL")
	apiURL       = flag.String("api", "https://fleet.barta.cm", "Dashboard API URL")
	hostID       = flag.String("host", "csb0", "Host ID to test")
	command      = flag.String("cmd", "refresh-all", "Command to run (refresh-all is quick and safe)")
	timeout      = flag.Duration("timeout", 30*time.Second, "Test timeout")
	verbose      = flag.Bool("v", false, "Verbose logging of all WS messages")
	password     = flag.String("password", "", "Dashboard password (or set FLEET_PASS env var)")
	totpCode     = flag.String("totp", "", "TOTP code (6 digits) - will prompt if not provided")
)

type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type CommandOutputPayload struct {
	HostID  string `json:"host_id"`
	Line    string `json:"line"`
	Command string `json:"command"`
	IsError bool   `json:"is_error"`
}

type CommandQueuedPayload struct {
	HostID   string `json:"host_id"`
	Hostname string `json:"hostname"`
	Command  string `json:"command"`
}

func main() {
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("🧪 Log Streaming E2E Test")
	log.Printf("   Dashboard WS: %s", *dashboardURL)
	log.Printf("   API URL: %s", *apiURL)
	log.Printf("   Host: %s", *hostID)
	log.Printf("   Command: %s", *command)
	log.Printf("   Timeout: %s", *timeout)
	log.Println()

	// Get password
	pass := *password
	if pass == "" {
		pass = os.Getenv("FLEET_PASS")
	}
	if pass == "" {
		log.Fatalf("❌ Password required: use -password flag or FLEET_PASS env var")
	}

	// Create HTTP client with cookie jar
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
	}
	
	// Parse API URL for cookie handling
	u, _ := url.Parse(*apiURL)

	// Get TOTP code
	code := *totpCode
	if code == "" {
		log.Fatalf("❌ TOTP code required: use -totp flag with 6-digit code")
	}

	// Login to get session cookie
	log.Println("🔐 Logging in...")
	loginURL := fmt.Sprintf("%s/login", *apiURL)
	loginBody := fmt.Sprintf("password=%s&totp=%s", url.QueryEscape(pass), url.QueryEscape(code))
	log.Printf("   Using TOTP code: %s", code)
	
	loginReq, _ := http.NewRequest("POST", loginURL, strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Don't follow redirects - we want to capture the Set-Cookie header
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	
	loginResp, err := httpClient.Do(loginReq)
	if err != nil {
		log.Fatalf("❌ Login request failed: %v", err)
	}
	log.Printf("   Login response: status=%d, location=%s", loginResp.StatusCode, loginResp.Header.Get("Location"))
	log.Printf("   Set-Cookie headers: %v", loginResp.Header.Values("Set-Cookie"))
	loginResp.Body.Close()
	
	// Manually add cookies from response to jar
	if cookies := loginResp.Cookies(); len(cookies) > 0 {
		jar.SetCookies(u, cookies)
	}

	// Check if we got a session cookie
	cookies := jar.Cookies(u)
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "nixfleet_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		log.Printf("⚠️  Cookies received: %v", cookies)
		log.Fatalf("❌ Login failed: no nixfleet_session cookie received")
	}
	log.Println("✅ Logged in!")

	// Fetch dashboard to get CSRF token
	log.Println("🔒 Getting CSRF token...")
	httpClient.CheckRedirect = nil // Allow redirects now
	dashReq, _ := http.NewRequest("GET", *apiURL+"/", nil)
	dashResp, err := httpClient.Do(dashReq)
	if err != nil {
		log.Fatalf("❌ Failed to fetch dashboard: %v", err)
	}
	dashBody, _ := io.ReadAll(dashResp.Body)
	dashResp.Body.Close()

	// Extract CSRF token from HTML (look for data-csrf-token on body element)
	csrfToken := ""
	// Look for: <body data-csrf-token="...">
	if idx := strings.Index(string(dashBody), `data-csrf-token="`); idx != -1 {
		start := idx + len(`data-csrf-token="`)
		end := strings.Index(string(dashBody)[start:], `"`)
		if end != -1 {
			csrfToken = string(dashBody)[start : start+end]
		}
	}
	if csrfToken == "" {
		log.Printf("⚠️  Could not find CSRF token in dashboard HTML")
		log.Printf("   First 2000 chars of response: %s", string(dashBody)[:min(2000, len(dashBody))])
	} else {
		log.Printf("✅ Got CSRF token: %s...", csrfToken[:min(16, len(csrfToken))])
	}

	// Connect to WebSocket with session cookie
	log.Println("📡 Connecting to WebSocket...")
	
	wsURL, err := url.Parse(*dashboardURL)
	if err != nil {
		log.Fatalf("❌ Invalid WebSocket URL: %v", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Jar:              jar,
	}

	headers := http.Header{}
	headers.Set("Origin", *apiURL)
	headers.Set("Cookie", fmt.Sprintf("nixfleet_session=%s", sessionCookie.Value))

	conn, resp, err := dialer.Dial(wsURL.String(), headers)
	if err != nil {
		if resp != nil {
			log.Printf("❌ WebSocket handshake failed: %v (status: %d)", err, resp.StatusCode)
		} else {
			log.Printf("❌ WebSocket connection failed: %v", err)
		}
		os.Exit(1)
	}
	defer conn.Close()

	log.Println("✅ WebSocket connected!")

	// Channel to track received messages
	commandQueued := make(chan bool, 1)
	commandOutputs := make(chan CommandOutputPayload, 100)
	commandComplete := make(chan bool, 1)
	done := make(chan struct{})

	// Start reading WebSocket messages
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if !strings.Contains(err.Error(), "close") {
					log.Printf("❌ WebSocket read error: %v", err)
				}
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("⚠️  Failed to parse message: %v", err)
				continue
			}

			if *verbose {
				log.Printf("📨 WS Message: type=%s payload=%s", msg.Type, string(msg.Payload))
			}

			switch msg.Type {
			case "command_queued":
				var payload CommandQueuedPayload
				if err := json.Unmarshal(msg.Payload, &payload); err == nil {
					if payload.HostID == *hostID {
						log.Printf("✅ command_queued received for %s: %s", payload.HostID, payload.Command)
						select {
						case commandQueued <- true:
						default:
						}
					}
				}

			case "command_output":
				var payload CommandOutputPayload
				if err := json.Unmarshal(msg.Payload, &payload); err == nil {
					if payload.HostID == *hostID {
						log.Printf("📤 command_output: [%s] %s", payload.HostID, payload.Line)
						select {
						case commandOutputs <- payload:
						default:
						}
					}
				} else {
					log.Printf("⚠️  Failed to parse command_output: %v", err)
				}

			case "command_complete":
				var payload struct {
					HostID string `json:"host_id"`
				}
				if err := json.Unmarshal(msg.Payload, &payload); err == nil {
					if payload.HostID == *hostID {
						log.Printf("✅ command_complete received for %s", payload.HostID)
						select {
						case commandComplete <- true:
						default:
						}
					}
				}

			case "host_status", "heartbeat":
				// Ignore these common messages unless verbose
				if *verbose {
					log.Printf("   (ignored: %s)", msg.Type)
				}

			default:
				log.Printf("📨 Other message: %s", msg.Type)
			}
		}
	}()

	// Wait a moment for connection to stabilize
	time.Sleep(500 * time.Millisecond)

	// Send command via API (using same httpClient with session cookie)
	log.Printf("🚀 Sending command '%s' to host '%s'...", *command, *hostID)
	
	cmdURL := fmt.Sprintf("%s/api/hosts/%s/command", *apiURL, *hostID)
	cmdBody := fmt.Sprintf(`{"command":"%s","force":true}`, *command)
	
	req, err := http.NewRequest("POST", cmdURL, strings.NewReader(cmdBody))
	if err != nil {
		log.Fatalf("❌ Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	apiResp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("❌ API request failed: %v", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != http.StatusOK && apiResp.StatusCode != http.StatusAccepted {
		log.Printf("⚠️  API returned status %d", apiResp.StatusCode)
	} else {
		log.Printf("✅ Command sent successfully (status %d)", apiResp.StatusCode)
	}

	// Wait for messages with timeout
	timeoutCh := time.After(*timeout)
	gotQueued := false
	outputCount := 0
	gotComplete := false

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	log.Println()
	log.Println("⏳ Waiting for WebSocket messages...")
	log.Println()

WaitLoop:
	for {
		select {
		case <-commandQueued:
			gotQueued = true

		case output := <-commandOutputs:
			outputCount++
			_ = output // Already logged above

		case <-commandComplete:
			gotComplete = true
			// Give a moment for any trailing output
			time.Sleep(500 * time.Millisecond)
			break WaitLoop

		case <-timeoutCh:
			log.Println("⏰ Timeout reached!")
			break WaitLoop

		case <-interrupt:
			log.Println("🛑 Interrupted!")
			break WaitLoop

		case <-done:
			log.Println("🔌 WebSocket closed!")
			break WaitLoop
		}
	}

	// Summary
	log.Println()
	log.Println("═══════════════════════════════════════════════════")
	log.Println("                    TEST RESULTS                   ")
	log.Println("═══════════════════════════════════════════════════")
	log.Printf("  command_queued received:  %v", gotQueued)
	log.Printf("  command_output count:     %d", outputCount)
	log.Printf("  command_complete received: %v", gotComplete)
	log.Println("═══════════════════════════════════════════════════")

	if outputCount > 0 {
		log.Println("✅ SUCCESS: Host streaming output is working!")
		os.Exit(0)
	} else if gotQueued {
		log.Println("❌ FAIL: Command was queued but NO OUTPUT received!")
		log.Println("   → Check if agent is sending TypeOutput messages")
		log.Println("   → Check dashboard hub.go forwarding logic")
		os.Exit(1)
	} else {
		log.Println("❌ FAIL: Command was NOT queued!")
		log.Println("   → Check if host is connected")
		log.Println("   → Check API endpoint")
		os.Exit(1)
	}
}

