// Package integration contains integration tests for NixFleet v2.
// This file tests P2800: Timeout handling and reconnection verification.
package integration

import (
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/internal/dashboard"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// TIMEOUT CONFIGURATION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestGetTimeoutConfig_Pull(t *testing.T) {
	cfg := dashboard.GetTimeoutConfig("pull")

	if cfg.WarningTimeout != 2*time.Minute {
		t.Errorf("expected warning 2m, got %v", cfg.WarningTimeout)
	}
	if cfg.HardTimeout != 5*time.Minute {
		t.Errorf("expected hard 5m, got %v", cfg.HardTimeout)
	}
	if cfg.ReconnectTimeout != 0 {
		t.Errorf("expected reconnect 0 (N/A), got %v", cfg.ReconnectTimeout)
	}
}

func TestGetTimeoutConfig_Switch(t *testing.T) {
	cfg := dashboard.GetTimeoutConfig("switch")

	if cfg.WarningTimeout != 10*time.Minute {
		t.Errorf("expected warning 10m, got %v", cfg.WarningTimeout)
	}
	if cfg.HardTimeout != 30*time.Minute {
		t.Errorf("expected hard 30m, got %v", cfg.HardTimeout)
	}
	if cfg.ReconnectTimeout != 90*time.Second {
		t.Errorf("expected reconnect 90s, got %v", cfg.ReconnectTimeout)
	}
}

func TestGetTimeoutConfig_PullSwitch(t *testing.T) {
	cfg := dashboard.GetTimeoutConfig("pull-switch")

	if cfg.WarningTimeout != 12*time.Minute {
		t.Errorf("expected warning 12m, got %v", cfg.WarningTimeout)
	}
	if cfg.HardTimeout != 35*time.Minute {
		t.Errorf("expected hard 35m, got %v", cfg.HardTimeout)
	}
	if cfg.ReconnectTimeout != 90*time.Second {
		t.Errorf("expected reconnect 90s, got %v", cfg.ReconnectTimeout)
	}
}

func TestGetTimeoutConfig_Test(t *testing.T) {
	cfg := dashboard.GetTimeoutConfig("test")

	if cfg.WarningTimeout != 5*time.Minute {
		t.Errorf("expected warning 5m, got %v", cfg.WarningTimeout)
	}
	if cfg.HardTimeout != 10*time.Minute {
		t.Errorf("expected hard 10m, got %v", cfg.HardTimeout)
	}
}

func TestGetTimeoutConfig_Unknown(t *testing.T) {
	cfg := dashboard.GetTimeoutConfig("unknown-command")

	// Should return sensible defaults
	if cfg.WarningTimeout <= 0 {
		t.Error("expected positive warning timeout for unknown command")
	}
	if cfg.HardTimeout <= 0 {
		t.Error("expected positive hard timeout for unknown command")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE TRANSITION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestStateMachine_ExtendTimeout(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	sm.ExtendTimeout("host1", 5)

	state := sm.GetState("host1")
	if state.ExtendedBy != 5 {
		t.Errorf("expected ExtendedBy=5, got %d", state.ExtendedBy)
	}

	// Extend again
	sm.ExtendTimeout("host1", 10)
	state = sm.GetState("host1")
	if state.ExtendedBy != 15 {
		t.Errorf("expected ExtendedBy=15, got %d", state.ExtendedBy)
	}
}

func TestStateMachine_InitiateKill(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	sm.InitiateKill("host1", "SIGTERM", 12345)

	state := sm.GetState("host1")
	if state.State != dashboard.StateKilling {
		t.Errorf("expected state KILLING, got %s", state.State)
	}
	if state.KillSignal != "SIGTERM" {
		t.Errorf("expected signal SIGTERM, got %s", state.KillSignal)
	}
	if state.KillPID == nil || *state.KillPID != 12345 {
		t.Errorf("expected PID 12345")
	}
}

func TestStateMachine_MarkKillFailed(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateKilling, "Killing")

	sm.MarkKillFailed("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateKillFailed {
		t.Errorf("expected state KILL_FAILED, got %s", state.State)
	}
}

func TestStateMachine_MarkIgnored(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateTimeoutPending, "Timeout")

	sm.MarkIgnored("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateIgnored {
		t.Errorf("expected state IGNORED, got %s", state.State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AWAITING RECONNECT TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestStateMachine_EnterAwaitingReconnect(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	sm.EnterAwaitingReconnect("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateAwaitingReconnect {
		t.Errorf("expected state AWAITING_RECONNECT, got %s", state.State)
	}
}

func TestStateMachine_HandleAgentReconnect_Fresh(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")

	// Capture freshness before
	preFreshness := &dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/old",
		BinaryHash:   "hash-old",
	}
	sm.GetState("host1").PreFreshness = preFreshness
	sm.TransitionTo("host1", dashboard.StateAwaitingReconnect, "Waiting")

	// Agent reconnects with new freshness
	newFreshness := dashboard.AgentFreshness{
		SourceCommit: "def5678",
		StorePath:    "/nix/store/new",
		BinaryHash:   "hash-new",
	}
	sm.HandleAgentReconnect("host1", newFreshness)

	state := sm.GetState("host1")
	if state.State != dashboard.StateSuccess {
		t.Errorf("expected state SUCCESS, got %s", state.State)
	}
}

func TestStateMachine_HandleAgentReconnect_Stale(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")

	// Capture freshness before
	preFreshness := &dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/old",
		BinaryHash:   "hash-old",
	}
	sm.GetState("host1").PreFreshness = preFreshness
	sm.TransitionTo("host1", dashboard.StateAwaitingReconnect, "Waiting")

	// Agent reconnects with SAME freshness (stale!)
	sameFreshness := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/old",
		BinaryHash:   "hash-old",
	}
	sm.HandleAgentReconnect("host1", sameFreshness)

	state := sm.GetState("host1")
	if state.State != dashboard.StateStaleBinary {
		t.Errorf("expected state STALE_BINARY, got %s", state.State)
	}
}

func TestStateMachine_HandleAgentReconnect_Suspicious(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")

	// Capture freshness before
	preFreshness := &dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/old",
		BinaryHash:   "hash-old",
	}
	sm.GetState("host1").PreFreshness = preFreshness
	sm.TransitionTo("host1", dashboard.StateAwaitingReconnect, "Waiting")

	// Agent reconnects with different commit but same binary (suspicious!)
	suspiciousFreshness := dashboard.AgentFreshness{
		SourceCommit: "def5678", // Changed
		StorePath:    "/nix/store/old", // Same
		BinaryHash:   "hash-old", // Same
	}
	sm.HandleAgentReconnect("host1", suspiciousFreshness)

	state := sm.GetState("host1")
	if state.State != dashboard.StateSuspicious {
		t.Errorf("expected state SUSPICIOUS, got %s", state.State)
	}
}

func TestStateMachine_HandleAgentReconnect_NotWaiting(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	// Try to handle reconnect when not in AWAITING_RECONNECT
	freshness := dashboard.AgentFreshness{
		SourceCommit: "def5678",
		StorePath:    "/nix/store/new",
		BinaryHash:   "hash-new",
	}
	sm.HandleAgentReconnect("host1", freshness)

	// State should not change
	state := sm.GetState("host1")
	if state.State != dashboard.StateRunning {
		t.Errorf("expected state RUNNING (unchanged), got %s", state.State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// REBOOT INTEGRATION TESTS (P6900)
// ═══════════════════════════════════════════════════════════════════════════

func TestStateMachine_HandleRebootTriggered(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	sm.HandleRebootTriggered("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateAbortedByReboot {
		t.Errorf("expected state ABORTED_BY_REBOOT, got %s", state.State)
	}

	// Check pending reboot recovery
	aborted := sm.GetPendingRebootRecovery("host1")
	if aborted != "switch" {
		t.Errorf("expected aborted command 'switch', got '%s'", aborted)
	}
}

func TestStateMachine_HandleRebootTriggered_NoCommand(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)

	// No command running - should do nothing
	sm.HandleRebootTriggered("host1")

	state := sm.GetState("host1")
	if state != nil && state.State != dashboard.StateIdle {
		t.Errorf("expected nil or IDLE state, got %v", state)
	}
}

func TestStateMachine_HandlePostRebootReconnect(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")
	sm.HandleRebootTriggered("host1")

	// Simulate agent reconnect after reboot
	handled := sm.HandlePostRebootReconnect("host1")

	if !handled {
		t.Error("expected HandlePostRebootReconnect to return true")
	}

	state := sm.GetState("host1")
	if state.State != dashboard.StateIdle {
		t.Errorf("expected state IDLE after reboot recovery, got %s", state.State)
	}

	// Pending recovery should be cleared
	aborted := sm.GetPendingRebootRecovery("host1")
	if aborted != "" {
		t.Errorf("expected pending reboot recovery to be cleared, got '%s'", aborted)
	}
}

func TestStateMachine_HandlePostRebootReconnect_NoRecovery(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)

	// No pending reboot recovery
	handled := sm.HandlePostRebootReconnect("host1")

	if handled {
		t.Error("expected HandlePostRebootReconnect to return false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SELF-HEALING TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestStateMachine_DetectOrphanedStates(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)

	// Start a command with an old timestamp
	sm.StartCommand("host1", "switch")
	state := sm.GetState("host1")
	oldTime := time.Now().Add(-2 * time.Hour)
	state.StartedAt = &oldTime
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	// Detect orphaned states with 1 hour threshold
	orphaned := sm.DetectOrphanedStates(1 * time.Hour)

	if len(orphaned) != 1 {
		t.Errorf("expected 1 orphaned state, got %d", len(orphaned))
	}
	if len(orphaned) > 0 && orphaned[0] != "host1" {
		t.Errorf("expected host1 to be orphaned, got %v", orphaned)
	}
}

func TestStateMachine_DetectOrphanedStates_NotOrphaned(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)

	// Start a recent command
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "Running")

	// Detect orphaned states with 1 hour threshold
	orphaned := sm.DetectOrphanedStates(1 * time.Hour)

	if len(orphaned) != 0 {
		t.Errorf("expected 0 orphaned states, got %d", len(orphaned))
	}
}

func TestStateMachine_DetectOrphanedStates_TerminalIgnored(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)

	// Command in terminal state should be ignored even if old
	sm.StartCommand("host1", "switch")
	state := sm.GetState("host1")
	oldTime := time.Now().Add(-2 * time.Hour)
	state.StartedAt = &oldTime
	sm.TransitionTo("host1", dashboard.StateSuccess, "Done")

	// Detect orphaned states
	orphaned := sm.DetectOrphanedStates(1 * time.Hour)

	if len(orphaned) != 0 {
		t.Errorf("expected 0 orphaned (terminal states ignored), got %d", len(orphaned))
	}
}

func TestStateMachine_CleanupOrphanedState(t *testing.T) {
	sm := dashboard.NewCommandStateMachine(zerolog.Nop(), nil)
	sm.StartCommand("host1", "switch")

	sm.CleanupOrphanedState("host1")

	// State transitions to CLEANUP instead of being removed
	state := sm.GetState("host1")
	if state == nil {
		t.Error("expected state to exist after cleanup")
	} else if state.State != dashboard.StateCleanup {
		t.Errorf("expected state CLEANUP, got %s", state.State)
	}

	snapshot := sm.GetSnapshot("host1")
	if snapshot != nil {
		t.Error("expected snapshot to be cleaned up")
	}
}

