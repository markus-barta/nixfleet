// Package integration contains integration tests for NixFleet v2.
// This file tests P2800: Command State Machine validation and state transitions.
package integration

import (
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
	"github.com/markus-barta/nixfleet/v2/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// PRE-VALIDATOR TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestCanExecuteCommand_HostOnline(t *testing.T) {
	host := &templates.Host{
		ID:             "test1",
		Online:         true,
		PendingCommand: "",
	}

	result := dashboard.CanExecuteCommand(host)

	if !result.Valid {
		t.Errorf("expected Valid=true for online host, got Valid=false: %s", result.Message)
	}
	if result.Code != "ok" {
		t.Errorf("expected Code='ok', got Code='%s'", result.Code)
	}
}

func TestCanExecuteCommand_HostOffline(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: false,
	}

	result := dashboard.CanExecuteCommand(host)

	if result.Valid {
		t.Error("expected Valid=false for offline host")
	}
	if result.Code != "host_offline" {
		t.Errorf("expected Code='host_offline', got Code='%s'", result.Code)
	}
}

func TestCanExecuteCommand_CommandPending(t *testing.T) {
	host := &templates.Host{
		ID:             "test1",
		Online:         true,
		PendingCommand: "switch",
	}

	result := dashboard.CanExecuteCommand(host)

	if result.Valid {
		t.Error("expected Valid=false when command pending")
	}
	if result.Code != "command_pending" {
		t.Errorf("expected Code='command_pending', got Code='%s'", result.Code)
	}
}

func TestCanPull_GitOutdated(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated", Message: "Behind origin"},
		},
	}

	result := dashboard.CanPull(host)

	if !result.Valid {
		t.Errorf("expected Valid=true for outdated git, got Valid=false: %s", result.Message)
	}
	if result.Code != "outdated" {
		t.Errorf("expected Code='outdated', got Code='%s'", result.Code)
	}
}

func TestCanPull_GitCurrent(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "ok", Message: "Up to date"},
		},
	}

	result := dashboard.CanPull(host)

	if result.Valid {
		t.Error("expected Valid=false when git is current")
	}
	if result.Code != "already_current" {
		t.Errorf("expected Code='already_current', got Code='%s'", result.Code)
	}
}

func TestCanPull_GitUnknown(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "unknown"},
		},
	}

	result := dashboard.CanPull(host)

	if !result.Valid {
		t.Error("expected Valid=true for unknown git status")
	}
	if result.Code != "unknown_state" {
		t.Errorf("expected Code='unknown_state', got Code='%s'", result.Code)
	}
}

func TestCanSwitch_SystemOutdated(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "outdated", Message: "Needs rebuild"},
		},
	}

	result := dashboard.CanSwitch(host)

	if !result.Valid {
		t.Errorf("expected Valid=true for outdated system, got Valid=false: %s", result.Message)
	}
	if result.Code != "outdated" {
		t.Errorf("expected Code='outdated', got Code='%s'", result.Code)
	}
}

func TestCanSwitch_GitOutdated(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	result := dashboard.CanSwitch(host)

	if result.Valid {
		t.Error("expected Valid=false when git is outdated")
	}
	if result.Code != "git_outdated" {
		t.Errorf("expected Code='git_outdated', got Code='%s'", result.Code)
	}
}

func TestCanSwitch_SystemCurrent(t *testing.T) {
	host := &templates.Host{
		ID:            "test1",
		Online:        true,
		AgentOutdated: false,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.CanSwitch(host)

	if result.Valid {
		t.Error("expected Valid=false when system is current")
	}
	if result.Code != "already_current" {
		t.Errorf("expected Code='already_current', got Code='%s'", result.Code)
	}
}

func TestCanSwitch_AgentOutdated(t *testing.T) {
	host := &templates.Host{
		ID:            "test1",
		Online:        true,
		AgentOutdated: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.CanSwitch(host)

	if !result.Valid {
		t.Error("expected Valid=true when agent is outdated")
	}
	if result.Code != "outdated" {
		t.Errorf("expected Code='outdated', got Code='%s'", result.Code)
	}
}

func TestCanTest_OnlineReady(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
	}

	result := dashboard.CanTest(host)

	if !result.Valid {
		t.Error("expected Valid=true for online host")
	}
	if result.Code != "ok" {
		t.Errorf("expected Code='ok', got Code='%s'", result.Code)
	}
}

func TestCanTest_Offline(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: false,
	}

	result := dashboard.CanTest(host)

	if result.Valid {
		t.Error("expected Valid=false for offline host")
	}
}

func TestCanPullSwitch_BothOutdated(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	result := dashboard.CanPullSwitch(host)

	if !result.Valid {
		t.Errorf("expected Valid=true when both outdated: %s", result.Message)
	}
	if result.Code != "ok" {
		t.Errorf("expected Code='ok', got Code='%s'", result.Code)
	}
}

func TestCanPullSwitch_BothCurrent(t *testing.T) {
	host := &templates.Host{
		ID:            "test1",
		Online:        true,
		AgentOutdated: false,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.CanPullSwitch(host)

	if result.Valid {
		t.Error("expected Valid=false when both current")
	}
	if result.Code != "already_current" {
		t.Errorf("expected Code='already_current', got Code='%s'", result.Code)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// POST-VALIDATOR TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestValidatePullResult_Success(t *testing.T) {
	before := dashboard.HostSnapshot{
		Generation: "abc1234",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		Generation: "def5678",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.ValidatePullResult(before, after, 0)

	if !result.Valid {
		t.Errorf("expected Valid=true: %s", result.Message)
	}
	if result.Code != "goal_achieved" {
		t.Errorf("expected Code='goal_achieved', got Code='%s'", result.Code)
	}
}

func TestValidatePullResult_ExitNonZero(t *testing.T) {
	before := dashboard.HostSnapshot{Generation: "abc1234"}
	after := dashboard.HostSnapshot{Generation: "abc1234"}

	result := dashboard.ValidatePullResult(before, after, 1)

	if result.Valid {
		t.Error("expected Valid=false for non-zero exit")
	}
	if result.Code != "exit_nonzero" {
		t.Errorf("expected Code='exit_nonzero', got Code='%s'", result.Code)
	}
}

func TestValidatePullResult_Partial(t *testing.T) {
	before := dashboard.HostSnapshot{
		Generation: "abc1234",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		Generation: "def5678", // Changed
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated"}, // Still outdated (cache)
		},
	}

	result := dashboard.ValidatePullResult(before, after, 0)

	if !result.Valid {
		t.Error("expected Valid=true for partial success")
	}
	if result.Code != "partial" {
		t.Errorf("expected Code='partial', got Code='%s'", result.Code)
	}
}

func TestValidatePullResult_GoalNotAchieved(t *testing.T) {
	before := dashboard.HostSnapshot{
		Generation: "abc1234",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		Generation: "abc1234", // No change
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated"},
		},
	}

	result := dashboard.ValidatePullResult(before, after, 0)

	if result.Valid {
		t.Error("expected Valid=false when goal not achieved")
	}
	if result.Code != "goal_not_achieved" {
		t.Errorf("expected Code='goal_not_achieved', got Code='%s'", result.Code)
	}
}

func TestValidateSwitchResult_Success(t *testing.T) {
	before := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			System: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.ValidateSwitchResult(before, after, 0)

	if !result.Valid {
		t.Errorf("expected Valid=true: %s", result.Message)
	}
	if result.Code != "goal_achieved" {
		t.Errorf("expected Code='goal_achieved', got Code='%s'", result.Code)
	}
}

func TestValidateSwitchResult_WithAgentUpdate(t *testing.T) {
	before := dashboard.HostSnapshot{
		AgentOutdated: true,
		UpdateStatus: &templates.UpdateStatus{
			System: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		AgentOutdated: false,
		UpdateStatus: &templates.UpdateStatus{
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.ValidateSwitchResult(before, after, 0)

	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.Code != "goal_achieved_with_agent" {
		t.Errorf("expected Code='goal_achieved_with_agent', got Code='%s'", result.Code)
	}
}

func TestValidateTestResult_Pass(t *testing.T) {
	result := dashboard.ValidateTestResult(0)

	if !result.Valid {
		t.Error("expected Valid=true for exit 0")
	}
	if result.Code != "test_passed" {
		t.Errorf("expected Code='test_passed', got Code='%s'", result.Code)
	}
}

func TestValidateTestResult_Fail(t *testing.T) {
	result := dashboard.ValidateTestResult(1)

	if result.Valid {
		t.Error("expected Valid=false for exit 1")
	}
	if result.Code != "test_failed" {
		t.Errorf("expected Code='test_failed', got Code='%s'", result.Code)
	}
}

func TestValidatePullSwitchResult_FullSuccess(t *testing.T) {
	before := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.ValidatePullSwitchResult(before, after, 0)

	if !result.Valid {
		t.Errorf("expected Valid=true: %s", result.Message)
	}
	if result.Code != "goal_achieved" {
		t.Errorf("expected Code='goal_achieved', got Code='%s'", result.Code)
	}
}

func TestValidatePullSwitchResult_PartialGitOnly(t *testing.T) {
	before := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	result := dashboard.ValidatePullSwitchResult(before, after, 0)

	if result.Valid {
		t.Error("expected Valid=false for partial git only")
	}
	if result.Code != "partial_git_only" {
		t.Errorf("expected Code='partial_git_only', got Code='%s'", result.Code)
	}
}

func TestValidatePullSwitchResult_PartialSystemOnly(t *testing.T) {
	before := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}
	after := dashboard.HostSnapshot{
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "ok"},
		},
	}

	result := dashboard.ValidatePullSwitchResult(before, after, 0)

	if !result.Valid {
		t.Error("expected Valid=true for partial system only")
	}
	if result.Code != "partial_system_only" {
		t.Errorf("expected Code='partial_system_only', got Code='%s'", result.Code)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATOR IDEMPOTENCY TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestValidator_Idempotency(t *testing.T) {
	host := &templates.Host{
		ID:     "test1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	// Call each validator twice and verify same result
	validators := []struct {
		name string
		fn   func() dashboard.ValidationResult
	}{
		{"CanExecuteCommand", func() dashboard.ValidationResult { return dashboard.CanExecuteCommand(host) }},
		{"CanPull", func() dashboard.ValidationResult { return dashboard.CanPull(host) }},
		{"CanSwitch", func() dashboard.ValidationResult { return dashboard.CanSwitch(host) }},
		{"CanTest", func() dashboard.ValidationResult { return dashboard.CanTest(host) }},
		{"CanPullSwitch", func() dashboard.ValidationResult { return dashboard.CanPullSwitch(host) }},
	}

	for _, v := range validators {
		t.Run(v.name, func(t *testing.T) {
			result1 := v.fn()
			result2 := v.fn()

			if result1.Valid != result2.Valid {
				t.Errorf("idempotency failed: Valid changed between calls")
			}
			if result1.Code != result2.Code {
				t.Errorf("idempotency failed: Code changed from '%s' to '%s'", result1.Code, result2.Code)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE MACHINE TESTS
// ═══════════════════════════════════════════════════════════════════════════

func newTestStateMachine() *dashboard.CommandStateMachine {
	// Create a mock hub (nil is fine for state tests)
	log := zerolog.Nop()
	return dashboard.NewCommandStateMachine(log, nil)
}

func TestStateMachine_StartCommand(t *testing.T) {
	sm := newTestStateMachine()

	sm.StartCommand("host1", "switch")

	state := sm.GetState("host1")
	if state == nil {
		t.Fatal("expected state to be created")
	}
	if state.State != dashboard.StateValidating {
		t.Errorf("expected state VALIDATING, got %s", state.State)
	}
	if state.Command != "switch" {
		t.Errorf("expected command 'switch', got '%s'", state.Command)
	}
}

func TestStateMachine_TransitionTo(t *testing.T) {
	sm := newTestStateMachine()
	sm.StartCommand("host1", "switch")

	sm.TransitionTo("host1", dashboard.StateRunning, "Command started")

	state := sm.GetState("host1")
	if state.State != dashboard.StateRunning {
		t.Errorf("expected state RUNNING, got %s", state.State)
	}
}

func TestStateMachine_TerminalStateCompletionTime(t *testing.T) {
	sm := newTestStateMachine()
	sm.StartCommand("host1", "switch")

	beforeTransition := time.Now()
	sm.TransitionTo("host1", dashboard.StateSuccess, "Done")
	afterTransition := time.Now()

	state := sm.GetState("host1")
	if state.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set for terminal state")
	}
	if state.CompletedAt.Before(beforeTransition) || state.CompletedAt.After(afterTransition) {
		t.Error("CompletedAt not in expected range")
	}
}

func TestStateMachine_CaptureSnapshot(t *testing.T) {
	sm := newTestStateMachine()

	host := &templates.Host{
		ID:           "host1",
		Generation:   "abc1234",
		AgentVersion: "2.0.0",
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	sm.CaptureSnapshot(host)
	snapshot := sm.GetSnapshot("host1")

	if snapshot == nil {
		t.Fatal("expected snapshot to be captured")
	}
	if snapshot.Generation != "abc1234" {
		t.Errorf("expected generation 'abc1234', got '%s'", snapshot.Generation)
	}
	if snapshot.AgentVersion != "2.0.0" {
		t.Errorf("expected agent version '2.0.0', got '%s'", snapshot.AgentVersion)
	}
}

func TestStateMachine_ClearState(t *testing.T) {
	sm := newTestStateMachine()
	sm.StartCommand("host1", "switch")

	sm.ClearState("host1")

	state := sm.GetState("host1")
	if state != nil {
		t.Error("expected state to be cleared")
	}
}

func TestStateMachine_GetAllHostStates(t *testing.T) {
	sm := newTestStateMachine()
	sm.StartCommand("host1", "switch")
	sm.StartCommand("host2", "pull")

	states := sm.GetAllHostStates()

	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	if states["host1"] == nil || states["host2"] == nil {
		t.Error("expected both hosts to have states")
	}
}

func TestStateMachine_GetRecentLogs(t *testing.T) {
	sm := newTestStateMachine()

	// Log some entries
	sm.Log(dashboard.LogEntry{Level: dashboard.LogLevelInfo, Message: "test1"})
	sm.Log(dashboard.LogEntry{Level: dashboard.LogLevelInfo, Message: "test2"})

	logs := sm.GetRecentLogs(10)

	// We should have at least our 2 entries
	if len(logs) < 2 {
		t.Errorf("expected at least 2 logs, got %d", len(logs))
	}
}

