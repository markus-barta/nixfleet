// Package integration contains integration tests for NixFleet v2.
// This file contains self-healing detection tests for P2800.
// NOTE: Core orphan/reboot tests are in t13_timeout_reconnect_test.go
// This file only contains additional edge cases not covered there.
package integration

import (
	"testing"

	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
	"github.com/rs/zerolog"
)

// ═══════════════════════════════════════════════════════════════════════════
// LOG CLEANUP TESTS (unique tests not in t13)
// ═══════════════════════════════════════════════════════════════════════════

func TestLogCleanup_MaxEntries(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	// Add many log entries
	for i := 0; i < 2000; i++ {
		sm.Log(dashboard.LogEntry{
			Level:   dashboard.LogLevelInfo,
			HostID:  "host1",
			State:   "TEST",
			Message: "test message",
		})
	}

	// Get recent logs - should be capped
	logs := sm.GetRecentLogs(100)

	if len(logs) > 100 {
		t.Errorf("expected at most 100 logs, got %d", len(logs))
	}
}

func TestLogCleanup_OldEntriesRemoved(t *testing.T) {
	log := zerolog.Nop()
	sm := dashboard.NewCommandStateMachine(log, nil)

	// Add entries
	for i := 0; i < 10; i++ {
		sm.Log(dashboard.LogEntry{
			Level:   dashboard.LogLevelInfo,
			HostID:  "host1",
			State:   "TEST",
			Message: "test message",
		})
	}

	logs := sm.GetRecentLogs(5)

	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}
}
