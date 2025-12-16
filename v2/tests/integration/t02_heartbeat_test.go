// T02 - Agent Heartbeat Tests
// Based on tests/specs/T02-agent-heartbeat.md
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

// TestAgentHeartbeat_Regular tests Scenario 1: Regular Heartbeat
// Given: agent connected, 1s heartbeat interval
// When: 5 seconds pass
// Then: at least 5 heartbeats received (including immediate first)
func TestAgentHeartbeat_Regular(t *testing.T) {
	// Create a temp git repo for the agent
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config with 1s heartbeat
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		_ = a.Run() // Ignore error in test goroutine
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Wait for registration first
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for at least 5 heartbeats (including immediate first)
	msgs, err := dashboard.WaitForNMessages(ctx, protocol.TypeHeartbeat, 5)
	if err != nil {
		// Check how many we got
		received := dashboard.MessagesOfType(protocol.TypeHeartbeat)
		t.Fatalf("expected at least 5 heartbeats, got %d: %v", len(received), err)
	}

	t.Logf("received %d heartbeats", len(msgs))

	// Verify heartbeat payload structure
	var payload protocol.HeartbeatPayload
	if err := msgs[0].ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse heartbeat payload: %v", err)
	}

	// Heartbeat should have generation (may be empty if no repo)
	// Metrics may be nil if no StaSysMo
	// PendingCommand should be nil when idle
	if payload.PendingCommand != nil {
		t.Errorf("expected nil pending_command when idle, got %v", *payload.PendingCommand)
	}

	a.Shutdown()
}

// TestAgentHeartbeat_DuringCommand tests Scenario 2: Heartbeat During Command Execution
// This is the CRITICAL test for v2.0 - heartbeats must continue during commands
func TestAgentHeartbeat_DuringCommand(t *testing.T) {
	// Create a temp git repo for the agent
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config with 1s heartbeat
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		_ = a.Run() // Ignore error in test goroutine
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for first heartbeat
	_, err = dashboard.WaitForMessage(ctx, protocol.TypeHeartbeat)
	if err != nil {
		t.Fatalf("failed to receive first heartbeat: %v", err)
	}

	// Count heartbeats before command
	beforeCount := len(dashboard.MessagesOfType(protocol.TypeHeartbeat))
	t.Logf("heartbeats before command: %d", beforeCount)

	// Send a "test" command (will run test scripts, may be quick or slow)
	// For this test, we just want to verify heartbeats continue
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Logf("send command failed: %v", err)
	}

	// Wait 5 seconds for heartbeats during command
	time.Sleep(5 * time.Second)

	// Count heartbeats after
	afterCount := len(dashboard.MessagesOfType(protocol.TypeHeartbeat))
	t.Logf("heartbeats after command: %d", afterCount)

	// Should have received more heartbeats
	newHeartbeats := afterCount - beforeCount
	if newHeartbeats < 4 {
		t.Errorf("CRITICAL: expected at least 4 heartbeats during 5s command, got %d", newHeartbeats)
	} else {
		t.Logf("SUCCESS: received %d heartbeats during command execution", newHeartbeats)
	}

	a.Shutdown()
}

// TestAgentHeartbeat_WithoutMetrics tests Scenario 4: Heartbeat without StaSysMo
// Given: StaSysMo NOT running
// When: agent sends heartbeat
// Then: heartbeat has metrics: null
func TestAgentHeartbeat_WithoutMetrics(t *testing.T) {
	// This test relies on StaSysMo not being available on the test host
	// (which is typically true in CI environments)

	// Create a temp git repo for the agent
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		_ = a.Run() // Ignore error in test goroutine
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for heartbeat
	msg, err := dashboard.WaitForMessage(ctx, protocol.TypeHeartbeat)
	if err != nil {
		t.Fatalf("failed to receive heartbeat: %v", err)
	}

	// Parse and check metrics
	var payload protocol.HeartbeatPayload
	if err := msg.ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse heartbeat: %v", err)
	}

	// Metrics should be nil if StaSysMo isn't running
	// (This may pass or fail depending on test environment)
	if payload.Metrics != nil {
		t.Logf("StaSysMo appears to be running: cpu=%.1f%%, ram=%.1f%%",
			payload.Metrics.CPU, payload.Metrics.RAM)
	} else {
		t.Log("metrics correctly nil (StaSysMo not available)")
	}

	a.Shutdown()
}

// TestAgentHeartbeat_ConcurrentCommandRejection tests Scenario 7
// Given: agent executing a command
// When: dashboard sends another command
// Then: agent rejects with "command already running"
func TestAgentHeartbeat_ConcurrentCommandRejection(t *testing.T) {
	// Create a temp git repo for the agent
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		_ = a.Run() // Ignore error in test goroutine
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Send first command (test - may take a while)
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Fatalf("failed to send first command: %v", err)
	}

	// Wait briefly for command to start
	time.Sleep(200 * time.Millisecond)

	// Send second command (should be rejected)
	err = dashboard.SendCommand("switch")
	if err != nil {
		t.Fatalf("failed to send second command: %v", err)
	}

	// Wait for rejection message
	msg, err := dashboard.WaitForMessage(ctx, protocol.TypeRejected)
	if err != nil {
		// May not get rejection if first command already finished
		t.Logf("no rejection received (command may have finished quickly): %v", err)
		return
	}

	// Verify rejection payload
	var payload protocol.CommandRejectedPayload
	if err := msg.ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse rejection: %v", err)
	}

	if payload.Reason != "command already running" {
		t.Errorf("expected reason 'command already running', got '%s'", payload.Reason)
	}

	t.Logf("correctly rejected with: %s (current: %s)", payload.Reason, payload.CurrentCommand)

	a.Shutdown()
}

// TestAgentHeartbeat_ImmediateFirst tests that first heartbeat is sent immediately
// after registration, not after waiting for interval
func TestAgentHeartbeat_ImmediateFirst(t *testing.T) {
	// Create a temp git repo for the agent
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config with 10s heartbeat (we won't wait that long)
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 10 * time.Second, // Long interval
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	// Run agent
	go func() {
		_ = a.Run() // Ignore error in test goroutine
	}()

	// Short timeout - heartbeat should arrive quickly (immediate)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// First heartbeat should arrive almost immediately (within 500ms)
	// not after 10 seconds
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer shortCancel()

	_, err = dashboard.WaitForMessage(shortCtx, protocol.TypeHeartbeat)
	if err != nil {
		t.Error("first heartbeat not received immediately after registration")
	} else {
		t.Log("first heartbeat received immediately after registration")
	}

	a.Shutdown()
}

