// Package e2e provides end-to-end tests for NixFleet functionality.
// Run with: go run ./tests/e2e/log_streaming_test.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
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

	// Connect to WebSocket
	log.Println("📡 Connecting to WebSocket...")
	
	u, err := url.Parse(*dashboardURL)
	if err != nil {
		log.Fatalf("❌ Invalid WebSocket URL: %v", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	headers.Set("Origin", *apiURL)

	conn, resp, err := dialer.Dial(u.String(), headers)
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

	// Send command via API
	log.Printf("🚀 Sending command '%s' to host '%s'...", *command, *hostID)
	
	cmdURL := fmt.Sprintf("%s/api/hosts/%s/command", *apiURL, *hostID)
	cmdBody := fmt.Sprintf(`{"command":"%s","force":true}`, *command)
	
	req, err := http.NewRequest("POST", cmdURL, strings.NewReader(cmdBody))
	if err != nil {
		log.Fatalf("❌ Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	apiResp, err := client.Do(req)
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

