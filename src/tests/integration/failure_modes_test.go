// Package integration contains integration tests for NixFleet v2.
// This file contains essential failure mode and edge case tests for P2800.
package integration

import (
	"testing"

	"github.com/markus-barta/nixfleet/internal/dashboard"
	"github.com/markus-barta/nixfleet/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// NETWORK FAILURE TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestFailureMode_AgentDisconnectMidCommand(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	// Start command
	sm.StartCommand("host1", "switch")
	sm.TransitionTo("host1", dashboard.StateRunning, "running")

	// Simulate disconnect - state should be preserved
	state := sm.GetState("host1")
	if state == nil {
		t.Fatal("state should be preserved during disconnect")
	}
	if state.State != dashboard.StateRunning {
		t.Errorf("expected RUNNING, got %s", state.State)
	}

	// Enter awaiting reconnect (what happens on disconnect)
	sm.EnterAwaitingReconnect("host1")

	state = sm.GetState("host1")
	if state.State != dashboard.StateAwaitingReconnect {
		t.Errorf("expected AWAITING_RECONNECT, got %s", state.State)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// EDGE CASE TESTS (essential only)
// ═══════════════════════════════════════════════════════════════════════════

func TestEdgeCase_NilUpdateStatus(t *testing.T) {
	host := &templates.Host{
		ID:           "host1",
		Online:       true,
		UpdateStatus: nil,
	}

	// Should not panic and should allow pull (unknown state)
	result := dashboard.CanPull(host)
	if !result.Valid {
		t.Error("nil UpdateStatus should allow pull (unknown state)")
	}
	if result.Code != "unknown_state" {
		t.Errorf("expected 'unknown_state', got '%s'", result.Code)
	}
}

func TestEdgeCase_EmptyHostID(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	// Empty host ID should not panic
	sm.StartCommand("", "switch")

	state := sm.GetState("")
	if state == nil {
		t.Error("state should exist even for empty host ID")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE CONSISTENCY TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestStateConsistency_StartCommandSetsFields(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	sm.StartCommand("host1", "switch")

	state := sm.GetState("host1")
	if state.Command != "switch" {
		t.Errorf("expected command 'switch', got '%s'", state.Command)
	}
	if state.StartedAt == nil || state.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if state.HostID != "host1" {
		t.Errorf("expected HostID 'host1', got '%s'", state.HostID)
	}
}

func TestStateConsistency_CompletionSetsTime(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	sm.StartCommand("host1", "pull")
	sm.TransitionTo("host1", dashboard.StateSuccess, "done")

	state := sm.GetState("host1")
	if state.CompletedAt == nil {
		t.Error("CompletedAt should be set on success")
	}
}
