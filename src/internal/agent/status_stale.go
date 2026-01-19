package agent

import (
	"time"

	"github.com/markus-barta/nixfleet/internal/protocol"
)

// P1110: detectAndResolveStaleStatus detects and resolves stale status states.
// This handles the case where the agent crashes during a command (e.g., switch)
// and the status remains "working" indefinitely.
func (s *StatusChecker) detectAndResolveStaleStatus(now time.Time) {
	// Only check for stale "working" states
	if s.systemStatus.Status != "working" {
		return
	}

	// Check if status has been "working" for too long
	if s.lastStatusUpdate.IsZero() {
		return
	}

	elapsed := now.Sub(s.lastStatusUpdate)
	if elapsed > s.staleThreshold {
		s.a.log.Warn().
			Str("old_status", s.systemStatus.Status).
			Str("old_message", s.systemStatus.Message).
			Dur("elapsed", elapsed).
			Dur("threshold", s.staleThreshold).
			Msg("P1110: detected stale system status - resolving to unknown")

		// Resolve stale status to "unknown" with explanation
		s.systemStatus = protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Status stale (agent may have restarted during command)",
			CheckedAt: now.UTC().Format(time.RFC3339),
		}
		s.lastSystemCheck = now
		s.lastStatusUpdate = now
	}
}
