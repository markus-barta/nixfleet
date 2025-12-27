// Package integration contains integration tests for NixFleet v2.
// This file contains E2E mock tests simulating full command flows for P2800.
package integration

import (
	"testing"

	"github.com/markus-barta/nixfleet/internal/dashboard"
	"github.com/markus-barta/nixfleet/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// FULL FLOW: PULL COMMAND
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_PullCommand_Success(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:         "host1",
		Online:     true,
		Generation: "gen-before",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "outdated", Message: "Behind origin/main"},
		},
	}

	// Step 1: Pre-checks
	preResult := sm.RunPreChecks(host, "pull")
	if !preResult.Valid {
		t.Fatalf("pre-check failed: %s", preResult.Message)
	}

	// Step 2: Capture snapshot
	sm.CaptureSnapshot(host)

	// Step 3: Start command
	sm.StartCommand("host1", "pull")

	// Step 4: Transition to running
	sm.TransitionTo("host1", dashboard.StateRunning, "executing git pull")

	// Step 5: Command completes - simulate status update
	hostAfter := &templates.Host{
		ID:         "host1",
		Online:     true,
		Generation: "gen-after",
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "ok", Message: "Up to date"},
		},
	}

	// Step 6: Post-checks
	postResult := sm.RunPostChecks("host1", "pull", 0, hostAfter, "")
	if !postResult.Valid {
		t.Fatalf("post-check failed: %s", postResult.Message)
	}

	// Step 7: Transition to success
	sm.TransitionTo("host1", dashboard.StateSuccess, postResult.Message)

	state := sm.GetState("host1")
	if state.State != dashboard.StateSuccess {
		t.Errorf("expected SUCCESS, got %s", state.State)
	}
}

func TestE2E_PullCommand_Blocked(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:     "host1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git: templates.StatusCheck{Status: "ok", Message: "Up to date"},
		},
	}

	// Pre-checks should fail - already current
	preResult := sm.RunPreChecks(host, "pull")
	if preResult.Valid {
		t.Error("pre-check should fail for already current git")
	}
	if preResult.Code != "already_current" {
		t.Errorf("expected 'already_current', got '%s'", preResult.Code)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FULL FLOW: SWITCH COMMAND
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_SwitchCommand_SuccessWithReconnect(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:         "host1",
		Online:     true,
		Generation: "gen-before",
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "outdated", Message: "Rebuild needed"},
		},
	}

	// Pre-checks
	preResult := sm.RunPreChecks(host, "switch")
	if !preResult.Valid {
		t.Fatalf("pre-check failed: %s", preResult.Message)
	}

	// Capture snapshot with freshness
	preFreshness := &dashboard.AgentFreshness{
		SourceCommit: "commit-before",
		StorePath:    "/nix/store/before",
		BinaryHash:   "hash-before",
	}
	sm.CaptureSnapshotWithFreshness(host, preFreshness)

	// Start and run
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "executing nixos-rebuild switch")

	// Switch completes, agent will restart
	sm.EnterAwaitingReconnect("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateAwaitingReconnect {
		t.Errorf("expected AWAITING_RECONNECT, got %s", state.State)
	}

	// Agent reconnects with new binary
	postFreshness := dashboard.AgentFreshness{
		SourceCommit: "commit-after",
		StorePath:    "/nix/store/after",
		BinaryHash:   "hash-after",
	}
	sm.HandleAgentReconnect("host1", postFreshness)

	// State should transition based on freshness comparison
	state = sm.GetState("host1")
	if state.State == dashboard.StateAwaitingReconnect {
		t.Error("should have transitioned out of AWAITING_RECONNECT")
	}
}

func TestE2E_SwitchCommand_GitOutdated(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:     "host1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "outdated"}, // Git not up to date
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	// Pre-checks should fail - git needs pull first
	preResult := sm.RunPreChecks(host, "switch")
	if preResult.Valid {
		t.Error("pre-check should fail when git is outdated")
	}
	if preResult.Code != "git_outdated" {
		t.Errorf("expected 'git_outdated', got '%s'", preResult.Code)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FULL FLOW: KILL COMMAND
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_KillCommand(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "running")

	// Kill initiated
	sm.InitiateKill("host1", "SIGTERM", 12345)

	state := sm.GetState("host1")
	if state.State != dashboard.StateKilling {
		t.Errorf("expected KILLING, got %s", state.State)
	}

	// Kill successful
	sm.TransitionTo("host1", dashboard.StateKilled, "process terminated")

	state = sm.GetState("host1")
	if state.State != dashboard.StateKilled {
		t.Errorf("expected KILLED, got %s", state.State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FULL FLOW: TIMEOUT HANDLING
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_TimeoutHandling_Ignore(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "running")

	// User chooses to ignore (dismiss warning)
	sm.MarkIgnored("host1")

	state := sm.GetState("host1")
	if state.State != dashboard.StateIgnored {
		t.Errorf("expected IGNORED, got %s", state.State)
	}
}
