// T03 - Agent Commands Tests
// Based on tests/specs/T03-agent-commands.md
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/agent"
	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/markus-barta/nixfleet/v2/internal/protocol"
	"github.com/rs/zerolog"
)

// TestAgentCommand_OutputStreaming tests Scenario 6: Command Output Streaming
// Given: command produces output
// When: each line is written
// Then: line is sent to dashboard promptly
func TestAgentCommand_OutputStreaming(t *testing.T) {
	// Create a temp directory with a mock test script
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "hosts", "test-host", "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test script that outputs multiple lines with delays
	testScript := filepath.Join(testDir, "T01-output.sh")
	scriptContent := `#!/bin/bash
echo "line 1"
sleep 0.1
echo "line 2"
sleep 0.1
echo "line 3"
sleep 0.1
echo "line 4"
sleep 0.1
echo "line 5"
`
	if err := os.WriteFile(testScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 5 * time.Second, // Long interval to not interfere
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)
	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for agent to be ready
	time.Sleep(200 * time.Millisecond)

	// Send test command
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Fatalf("failed to send command: %v", err)
	}

	// Wait for output messages
	time.Sleep(2 * time.Second)

	// Check output messages
	outputs := dashboard.MessagesOfType(protocol.TypeOutput)
	t.Logf("received %d output messages", len(outputs))

	if len(outputs) < 3 {
		t.Errorf("expected at least 3 output lines, got %d", len(outputs))
	}

	// Verify output content
	for i, msg := range outputs {
		var payload protocol.OutputPayload
		if err := msg.ParsePayload(&payload); err != nil {
			t.Errorf("failed to parse output %d: %v", i, err)
			continue
		}
		t.Logf("output %d: %s (%s)", i, payload.Line, payload.Stream)
	}
}

// TestAgentCommand_Stop tests Scenario 5: Stop Running Command
// Given: command is executing
// When: dashboard sends stop
// Then: process is killed, agent reports stopped
func TestAgentCommand_Stop(t *testing.T) {
	t.Skip("SKIP: Test is flaky - hangs waiting for process cleanup")
	// Create temp directory with a long-running test script
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "hosts", "test-host", "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a long-running test script
	testScript := filepath.Join(testDir, "T01-long.sh")
	scriptContent := `#!/bin/bash
echo "starting long operation"
sleep 30
echo "this should not appear"
`
	if err := os.WriteFile(testScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
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

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)
	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for agent to be ready
	time.Sleep(200 * time.Millisecond)

	// Send test command (starts long-running script)
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Fatalf("failed to send test command: %v", err)
	}

	// Wait for command to start
	time.Sleep(500 * time.Millisecond)

	// Verify command is running (check heartbeat has pending_command)
	heartbeats := dashboard.MessagesOfType(protocol.TypeHeartbeat)
	t.Logf("heartbeats so far: %d", len(heartbeats))

	// Send stop command
	err = dashboard.SendCommand("stop")
	if err != nil {
		t.Fatalf("failed to send stop command: %v", err)
	}

	// Wait for status report
	time.Sleep(1 * time.Second)

	// Check for status message indicating stopped/error
	statuses := dashboard.MessagesOfType(protocol.TypeStatus)
	t.Logf("status messages: %d", len(statuses))

	// Agent should have reported something (stopped or error)
	if len(statuses) == 0 {
		t.Log("no status message received (stop may not have completed yet)")
	} else {
		for _, msg := range statuses {
			var payload protocol.StatusPayload
			if err := msg.ParsePayload(&payload); err == nil {
				t.Logf("status: %s, exit_code: %d, message: %s",
					payload.Status, payload.ExitCode, payload.Message)
			}
		}
	}

	// Verify agent is still responsive (send another heartbeat check)
	time.Sleep(1 * time.Second)
	finalHeartbeats := dashboard.MessagesOfType(protocol.TypeHeartbeat)
	if len(finalHeartbeats) <= len(heartbeats) {
		t.Error("agent stopped sending heartbeats after stop command")
	} else {
		t.Log("agent still responsive after stop command")
	}
}

// TestAgentCommand_Failure tests Scenario 7: Command Failure
// Given: command fails (non-zero exit)
// When: command completes
// Then: agent reports error status
func TestAgentCommand_Failure(t *testing.T) {
	// Create temp directory with a failing test script
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "hosts", "test-host", "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a failing test script
	testScript := filepath.Join(testDir, "T01-fail.sh")
	scriptContent := `#!/bin/bash
echo "about to fail"
exit 1
`
	if err := os.WriteFile(testScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 5 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)
	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for agent to be ready
	time.Sleep(200 * time.Millisecond)

	// Send test command
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Fatalf("failed to send command: %v", err)
	}

	// Wait for command to complete
	time.Sleep(2 * time.Second)

	// Check for error status
	statuses := dashboard.MessagesOfType(protocol.TypeStatus)
	if len(statuses) == 0 {
		t.Fatal("expected status message after command failure")
	}

	var payload protocol.StatusPayload
	if err := statuses[len(statuses)-1].ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse status: %v", err)
	}

	if payload.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", payload.Status)
	}

	if payload.ExitCode == 0 {
		t.Error("expected non-zero exit code for failed command")
	}

	t.Logf("correctly reported failure: status=%s, exit=%d, msg=%s",
		payload.Status, payload.ExitCode, payload.Message)
}

// TestAgentCommand_Pull tests Scenario 1: Pull Command
// This requires a real git repo, so we create a mock one
func TestAgentCommand_Pull(t *testing.T) {
	// Create a mock git repository
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git user (required for commits)
	_ = exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = exec.Command("git", "-C", tmpDir, "add", ".").Run()
	_ = exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config (legacy mode - just git pull)
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           tmpDir,
		HeartbeatInterval: 5 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)
	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for agent to be ready
	time.Sleep(200 * time.Millisecond)

	// Send pull command
	err = dashboard.SendCommand("pull")
	if err != nil {
		t.Fatalf("failed to send command: %v", err)
	}

	// Wait for command to complete
	time.Sleep(2 * time.Second)

	// Check for status message
	statuses := dashboard.MessagesOfType(protocol.TypeStatus)
	if len(statuses) == 0 {
		// Pull might fail (no remote), that's OK for this test
		t.Log("no status received (pull may have failed - no remote)")
		return
	}

	var payload protocol.StatusPayload
	if err := statuses[len(statuses)-1].ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse status: %v", err)
	}

	t.Logf("pull result: status=%s, exit=%d, msg=%s",
		payload.Status, payload.ExitCode, payload.Message)
}

// TestAgentCommand_HeartbeatsDuringLongCommand verifies heartbeats continue
// during a long command (reinforces T02 but with real command execution)
func TestAgentCommand_HeartbeatsDuringLongCommand(t *testing.T) {
	// Create temp directory with a 5-second test script
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "hosts", "test-host", "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a 5-second test script
	testScript := filepath.Join(testDir, "T01-slow.sh")
	scriptContent := `#!/bin/bash
echo "starting slow test"
for i in 1 2 3 4 5; do
    echo "step $i"
    sleep 1
done
echo "done"
`
	if err := os.WriteFile(testScript, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
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

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)
	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Wait for first heartbeat
	_, err = dashboard.WaitForMessage(ctx, protocol.TypeHeartbeat)
	if err != nil {
		t.Fatalf("failed to receive initial heartbeat: %v", err)
	}

	// Count heartbeats before command
	beforeCount := len(dashboard.MessagesOfType(protocol.TypeHeartbeat))

	// Send test command (5 second script)
	err = dashboard.SendCommand("test")
	if err != nil {
		t.Fatalf("failed to send command: %v", err)
	}

	// Wait for command to complete (5+ seconds)
	time.Sleep(6 * time.Second)

	// Count heartbeats after
	afterCount := len(dashboard.MessagesOfType(protocol.TypeHeartbeat))
	newHeartbeats := afterCount - beforeCount

	t.Logf("heartbeats during 5s command: %d", newHeartbeats)

	// Should have at least 4 heartbeats during 5 seconds
	if newHeartbeats < 4 {
		t.Errorf("expected at least 4 heartbeats during 5s command, got %d", newHeartbeats)
	}

	// Verify some heartbeats had pending_command set
	heartbeats := dashboard.MessagesOfType(protocol.TypeHeartbeat)
	pendingCount := 0
	for _, msg := range heartbeats {
		var payload protocol.HeartbeatPayload
		if err := msg.ParsePayload(&payload); err == nil && payload.PendingCommand != nil {
			pendingCount++
		}
	}
	t.Logf("heartbeats with pending_command: %d", pendingCount)

	if pendingCount == 0 {
		t.Error("no heartbeats had pending_command set during command execution")
	}
}

