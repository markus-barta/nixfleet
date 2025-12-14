// T01 - Agent Connection Tests
// Based on tests/specs/T01-agent-connection.md
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/agent"
	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
	"github.com/rs/zerolog"
)

// TestAgentConnection_Success tests Scenario 1: Successful Connection
// Given: dashboard running, agent has valid token
// When: agent starts
// Then: agent connects, sends register message, dashboard acknowledges
func TestAgentConnection_Success(t *testing.T) {
	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           "/tmp/nixfleet-test",
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent in goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		a.Run()
	}()

	// Wait for registration message
	msg, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Verify registration payload
	var payload protocol.RegisterPayload
	if err := msg.ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse registration payload: %v", err)
	}

	// Verify fields (from T01 spec)
	if payload.Hostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got '%s'", payload.Hostname)
	}
	if payload.AgentVersion != agent.Version {
		t.Errorf("expected agent version '%s', got '%s'", agent.Version, payload.AgentVersion)
	}
	if payload.HeartbeatInterval != 1 {
		t.Errorf("expected heartbeat interval 1, got %d", payload.HeartbeatInterval)
	}
	if payload.HostType != "nixos" && payload.HostType != "macos" {
		t.Errorf("expected host_type 'nixos' or 'macos', got '%s'", payload.HostType)
	}

	// Shutdown
	a.Shutdown()
}

// TestAgentConnection_InvalidToken tests Scenario 2: Invalid Token
// Given: dashboard running, agent has invalid token
// When: agent attempts to connect
// Then: connection rejected with 401, agent retries with backoff
func TestAgentConnection_InvalidToken(t *testing.T) {
	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	dashboard.SetAuthToken("correct-token") // Agent will use wrong token
	defer dashboard.Close()

	// Create agent config with wrong token
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "wrong-token", // Wrong token!
		RepoDir:           "/tmp/nixfleet-test",
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent briefly
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		a.Run()
	}()

	// Wait a bit for connection attempts
	time.Sleep(2 * time.Second)

	// Verify no registration (connection should be rejected)
	msgs := dashboard.MessagesOfType(protocol.TypeRegister)
	if len(msgs) > 0 {
		t.Error("expected no registration with invalid token")
	}

	// Verify no connections (all should be rejected)
	if dashboard.ConnectionCount() > 0 {
		t.Error("expected no connections with invalid token")
	}

	a.Shutdown()
	<-ctx.Done()
}

// TestAgentConnection_Reconnect tests Scenario 4: Connection Lost
// Given: agent connected
// When: dashboard closes connection
// Then: agent detects disconnect, reconnects with backoff
func TestAgentConnection_Reconnect(t *testing.T) {
	// Start mock dashboard
	dashboard := NewMockDashboard(t)

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           "/tmp/nixfleet-test",
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		a.Run()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for first registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive first registration: %v", err)
	}
	t.Log("first registration received")

	// Close dashboard to simulate disconnect
	dashboard.Close()
	t.Log("dashboard closed")

	// Wait a moment
	time.Sleep(500 * time.Millisecond)

	// Start new dashboard on same port (need new server)
	dashboard2 := NewMockDashboard(t)
	defer dashboard2.Close()

	// Update agent's URL (simulating same URL coming back up)
	// Note: In real test, we'd need to use fixed port or update config
	// For now, this demonstrates the pattern

	// Agent should reconnect
	// (In a real test, we'd verify the reconnection)

	a.Shutdown()
}

// TestAgentConnection_MalformedMessage tests Scenario 6: Malformed Messages
// Given: agent connected
// When: dashboard sends malformed JSON
// Then: agent logs error, continues operating
func TestAgentConnection_MalformedMessage(t *testing.T) {
	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           "/tmp/nixfleet-test",
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		a.Run()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Send malformed JSON
	err = dashboard.SendRaw([]byte("this is not valid json"))
	if err != nil {
		t.Logf("send raw failed (expected if conn closed): %v", err)
	}

	// Wait a moment
	time.Sleep(500 * time.Millisecond)

	// Agent should still be connected (verify with heartbeat)
	// Wait for a heartbeat (proves agent is still running)
	_, err = dashboard.WaitForMessage(ctx, protocol.TypeHeartbeat)
	if err != nil {
		t.Errorf("agent stopped after malformed message: %v", err)
	}

	a.Shutdown()
}

// TestAgentConnection_DuplicateHostname is a placeholder
// (Would require dashboard implementation to test properly)
func TestAgentConnection_DuplicateHostname(t *testing.T) {
	t.Skip("requires dashboard implementation")
}

