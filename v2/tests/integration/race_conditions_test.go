// Package integration contains integration tests for NixFleet v2.
// This file contains essential race condition tests for P2800.
package integration

import (
	"sync"
	"testing"

	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
	"github.com/markus-barta/nixfleet/v2/internal/templates"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// RAPID CLICKS TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRaceCondition_RapidClicksSameHost(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:     "host1",
		Online: true,
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	// Simulate rapid clicks - 10 concurrent pre-checks
	var wg sync.WaitGroup
	results := make([]dashboard.ValidationResult, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = sm.RunPreChecks(host, "switch")
		}(i)
	}

	wg.Wait()

	// All should get the same result (validators are idempotent)
	for i := 1; i < 10; i++ {
		if results[i].Valid != results[0].Valid {
			t.Errorf("inconsistent results from concurrent pre-checks")
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// CONCURRENT STATE ACCESS TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRaceCondition_ConcurrentStateAccess(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	sm.StartCommand("host1", "switch")

	// Concurrent state reads and transitions
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sm.TransitionTo("host1", dashboard.StateRunning, "test")
		}()
		go func() {
			defer wg.Done()
			_ = sm.GetState("host1")
		}()
	}

	wg.Wait()

	// State should be consistent (no panic, state exists)
	state := sm.GetState("host1")
	if state == nil {
		t.Error("state should not be nil")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SNAPSHOT INTEGRITY TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRaceCondition_SnapshotIntegrity(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	host := &templates.Host{
		ID:         "host1",
		Generation: "abc1234",
		UpdateStatus: &templates.UpdateStatus{
			Git:    templates.StatusCheck{Status: "ok"},
			System: templates.StatusCheck{Status: "outdated"},
		},
	}

	// Concurrent snapshot captures
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.CaptureSnapshot(host)
		}()
	}

	wg.Wait()

	// Snapshot should exist and be valid
	snapshot := sm.GetSnapshot("host1")
	if snapshot == nil {
		t.Error("snapshot should exist")
	}
	if snapshot.Generation != "abc1234" {
		t.Errorf("snapshot generation wrong, got '%s'", snapshot.Generation)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// MULTI-HOST CONCURRENCY TEST
// ═══════════════════════════════════════════════════════════════════════════

func TestRaceCondition_MultiHostConcurrency(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	// Concurrent operations on different hosts
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hostID := string(rune('a' + idx%5))
			host := &templates.Host{ID: hostID, Generation: hostID}
			sm.CaptureSnapshot(host)
			sm.StartCommand(hostID, "switch")
			_ = sm.GetState(hostID)
			_ = sm.GetSnapshot(hostID)
		}(i)
	}

	wg.Wait()
	// No panic = success
}
