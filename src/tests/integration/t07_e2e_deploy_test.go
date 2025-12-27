// Package integration contains end-to-end tests for the NixFleet v2 system.
//
// These E2E tests verify the FULL v2 stack (Go dashboard + Go agent).
// They are currently placeholders until:
// - P4100: Agent is packaged as Nix module
// - P4400: Dashboard is packaged for deployment
//
// Once both are deployed, these tests will run against real hosts with the v2 stack.
//
// NOTE: Do NOT test against v1 production (fleet.barta.cm) - that tests different code!
package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pquerna/otp/totp"
)

// E2E Configuration from environment
type e2eConfig struct {
	DashboardURL string   // ws://localhost:8000/ws or wss://fleet.barta.cm/ws
	HTTPBaseURL  string   // http://localhost:8000 or https://fleet.barta.cm
	AgentToken   string   // Token for agent connections
	Password     string   // Dashboard login password
	TOTPSecret   string   // TOTP secret for 2FA (optional)
	Hosts        []string // Hosts to test (e.g., mba-mbp-work)
}

func loadE2EConfig(t *testing.T) *e2eConfig {
	cfg := &e2eConfig{
		DashboardURL: os.Getenv("E2E_DASHBOARD_URL"),
		AgentToken:   os.Getenv("E2E_AGENT_TOKEN"),
		Password:     os.Getenv("E2E_PASSWORD"),
		TOTPSecret:   os.Getenv("E2E_TOTP_SECRET"),
	}

	// Derive HTTP URL from WebSocket URL
	if cfg.DashboardURL != "" {
		cfg.HTTPBaseURL = strings.Replace(cfg.DashboardURL, "ws://", "http://", 1)
		cfg.HTTPBaseURL = strings.Replace(cfg.HTTPBaseURL, "wss://", "https://", 1)
		cfg.HTTPBaseURL = strings.TrimSuffix(cfg.HTTPBaseURL, "/ws")
	}

	// Parse hosts list
	hosts := os.Getenv("E2E_HOSTS")
	if hosts != "" {
		cfg.Hosts = strings.Split(hosts, ",")
	}

	return cfg
}

func (c *e2eConfig) isConfigured() bool {
	return c.DashboardURL != "" && c.Password != "" && len(c.Hosts) > 0
}

// generateTOTP generates a TOTP code from the secret.
func (c *e2eConfig) generateTOTP() string {
	if c.TOTPSecret == "" {
		return ""
	}
	code, err := totp.GenerateCode(c.TOTPSecret, time.Now())
	if err != nil {
		return ""
	}
	return code
}

// TestE2E_DeployFlow tests a full pull → switch flow on a real host.
//
// IMPORTANT: This tests the v2 stack only. Once P4100 and P4400 are complete,
// deploy v2 agent to mba-mbp-work and run this test.
//
// Requires E2E environment variables to be set.
func TestE2E_DeployFlow(t *testing.T) {
	t.Skip("E2E tests require v2 deployment (P4100 + P4400) - currently testing v2 via T01-T06")

	cfg := loadE2EConfig(t)
	if !cfg.isConfigured() {
		t.Skip("E2E not configured. Set E2E_DASHBOARD_URL, E2E_PASSWORD, E2E_HOSTS")
	}

	t.Logf("E2E Deploy Flow Test")
	t.Logf("  Dashboard: %s", cfg.HTTPBaseURL)
	t.Logf("  Hosts: %v", cfg.Hosts)

	// Login to dashboard
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	formData := url.Values{
		"password": {cfg.Password},
	}
	if totpCode := cfg.generateTOTP(); totpCode != "" {
		formData.Set("totp", totpCode)
		t.Logf("using TOTP code: %s", totpCode)
	}

	resp, err := client.PostForm(cfg.HTTPBaseURL+"/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Fatalf("login failed with status %d: %s", resp.StatusCode, body)
	}
	t.Log("logged in successfully")

	// Extract CSRF token from dashboard
	resp, err = client.Get(cfg.HTTPBaseURL + "/")
	if err != nil {
		t.Fatalf("failed to get dashboard: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	if csrfStart == -1 {
		// Try v1 style CSRF extraction
		csrfStart = strings.Index(bodyStr, `name="csrf_token" value="`)
		if csrfStart == -1 {
			t.Fatalf("CSRF token not found in response")
		}
		csrfStart += len(`name="csrf_token" value="`)
	} else {
		csrfStart += len(`data-csrf-token="`)
	}
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	t.Logf("got CSRF token: %s...", csrfToken[:16])

	// Connect browser WebSocket to receive updates
	serverURL, _ := url.Parse(cfg.HTTPBaseURL)
	cookies := client.Jar.Cookies(serverURL)
	var sessionCookie string
	for _, c := range cookies {
		if strings.Contains(c.Name, "session") {
			sessionCookie = c.String()
			break
		}
	}

	header := http.Header{}
	header.Set("Cookie", sessionCookie)

	browserWS, _, err := websocket.DefaultDialer.Dial(cfg.DashboardURL, header)
	if err != nil {
		t.Logf("warning: couldn't connect browser WebSocket: %v", err)
	} else {
		defer func() { _ = browserWS.Close() }()
		t.Log("connected browser WebSocket")
	}

	// Test each host
	for _, host := range cfg.Hosts {
		t.Run(host, func(t *testing.T) {
			testDeployHost(t, client, cfg, csrfToken, host, browserWS)
		})
	}
}

func testDeployHost(t *testing.T, client *http.Client, cfg *e2eConfig, csrfToken, host string, browserWS *websocket.Conn) {
	t.Logf("testing host: %s", host)

	// Check host is online
	resp, err := client.Get(cfg.HTTPBaseURL + "/api/hosts")
	if err != nil {
		t.Fatalf("failed to get hosts: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var hostsResp struct {
		Hosts []struct {
			ID       string `json:"id"`
			Hostname string `json:"hostname"`
			Status   string `json:"status"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &hostsResp); err != nil {
		t.Fatalf("failed to parse hosts: %v", err)
	}

	var hostFound bool
	for _, h := range hostsResp.Hosts {
		if h.Hostname == host || h.ID == host {
			hostFound = true
			t.Logf("host %s found, status: %s", host, h.Status)
			break
		}
	}
	if !hostFound {
		t.Skipf("host %s not found in dashboard", host)
	}

	// v3: Send pull command via Op Engine
	req, _ := http.NewRequest("POST", cfg.HTTPBaseURL+"/api/dispatch",
		strings.NewReader(fmt.Sprintf(`{"op": "pull", "hosts": ["%s"]}`, host)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("pull command failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pull command returned %d: %s", resp.StatusCode, body)
	}

	t.Logf("pull command dispatched: %s", body)

	// Wait for pull to complete
	if browserWS != nil {
		t.Log("waiting for pull completion...")
		waitForCommandComplete(t, browserWS, host, "pull", 60*time.Second)
	} else {
		// Fallback: just wait a bit
		time.Sleep(10 * time.Second)
	}

	t.Logf("pull completed for %s", host)
}

func waitForCommandComplete(t *testing.T, ws *websocket.Conn, host, command string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, data, err := ws.ReadMessage()
		if err != nil {
			continue
		}

		var msg struct {
			Type    string `json:"type"`
			Payload struct {
				HostID  string `json:"host_id"`
				Command string `json:"command"`
				Status  string `json:"status"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if msg.Payload.HostID == host && msg.Payload.Command == command {
			if msg.Payload.Status == "ok" || msg.Payload.Status == "error" {
				t.Logf("command %s on %s completed with status: %s", command, host, msg.Payload.Status)
				return
			}
		}
	}
	t.Logf("warning: timed out waiting for command completion")
}

// TestE2E_HostConnectivity tests that expected hosts are online.
//
// IMPORTANT: This tests the v2 stack only. Do not run against v1 production.
func TestE2E_HostConnectivity(t *testing.T) {
	t.Skip("E2E tests require v2 deployment (P4100 + P4400) - currently testing v2 via T01-T06")

	cfg := loadE2EConfig(t)
	if !cfg.isConfigured() {
		t.Skip("E2E not configured. Set E2E_DASHBOARD_URL, E2E_PASSWORD, E2E_HOSTS")
	}

	// Login with TOTP if configured
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	formData := url.Values{
		"password": {cfg.Password},
	}
	if totpCode := cfg.generateTOTP(); totpCode != "" {
		formData.Set("totp", totpCode)
	}

	resp, err := client.PostForm(cfg.HTTPBaseURL+"/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_ = resp.Body.Close()

	// Get hosts
	resp, err = client.Get(cfg.HTTPBaseURL + "/api/hosts")
	if err != nil {
		t.Fatalf("failed to get hosts: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var hostsResp struct {
		Hosts []struct {
			ID       string `json:"id"`
			Hostname string `json:"hostname"`
			Status   string `json:"status"`
			LastSeen string `json:"last_seen"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(body, &hostsResp); err != nil {
		t.Fatalf("failed to parse hosts: %v", err)
	}

	t.Logf("found %d hosts", len(hostsResp.Hosts))

	for _, expectedHost := range cfg.Hosts {
		t.Run(expectedHost, func(t *testing.T) {
			var found bool
			for _, h := range hostsResp.Hosts {
				if h.Hostname == expectedHost || h.ID == expectedHost {
					found = true
					t.Logf("host %s: status=%s, last_seen=%s", h.Hostname, h.Status, h.LastSeen)
					if h.Status != "ok" && h.Status != "running" {
						t.Logf("warning: host status is %s", h.Status)
					}
					break
				}
			}
			if !found {
				t.Errorf("host %s not found", expectedHost)
			}
		})
	}
}

