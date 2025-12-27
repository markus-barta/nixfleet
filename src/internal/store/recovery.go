package store

import (
	"context"
	"time"

	"github.com/markus-barta/nixfleet/internal/ops"
)

// RecoveryHandler processes orphaned commands on startup.
type RecoveryHandler func(cmd *ops.Command) error

// RecoverOrphanedCommands finds commands stuck in EXECUTING state
// and processes them with the given handler.
// This implements CORE-003 startup recovery.
func (s *StateStore) RecoverOrphanedCommands(handler RecoveryHandler) error {
	orphaned, err := s.GetOrphanedCommands()
	if err != nil {
		return err
	}

	if len(orphaned) == 0 {
		s.log.Info().Msg("no orphaned commands to recover")
		return nil
	}

	s.log.Warn().Int("count", len(orphaned)).Msg("found orphaned commands, recovering")

	for _, cmd := range orphaned {
		// Mark as orphaned
		if err := s.UpdateCommandStatus(cmd.ID, "ORPHANED", nil, "Dashboard restarted"); err != nil {
			s.log.Error().Err(err).Str("cmd", cmd.ID).Msg("failed to mark command as orphaned")
			continue
		}

		// Log to event log
		s.LogEvent("system", "warn", "system", cmd.HostID, "recovery",
			"Command orphaned due to dashboard restart: "+cmd.OpID, map[string]any{
				"command_id": cmd.ID,
				"op":         cmd.OpID,
			})

		// Call handler if provided
		if handler != nil {
			if err := handler(cmd); err != nil {
				s.log.Error().Err(err).Str("cmd", cmd.ID).Msg("recovery handler failed")
			}
		}
	}

	return nil
}

// StartRetentionCleanup runs periodic cleanup of old records.
// Uses default retention periods from CORE-003:
// - Commands: 30 days
// - Pipelines: 30 days
// - Events: 7 days
func (s *StateStore) StartRetentionCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.log.Info().Dur("interval", interval).Msg("starting retention cleanup loop")

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("retention cleanup loop stopped")
			return
		case <-ticker.C:
			s.runCleanup()
		}
	}
}

func (s *StateStore) runCleanup() {
	// Default retention periods from CORE-003
	const (
		commandRetention  = 30 * 24 * time.Hour // 30 days
		pipelineRetention = 30 * 24 * time.Hour // 30 days
		eventRetention    = 7 * 24 * time.Hour  // 7 days
	)

	cmds, _ := s.CleanupOldCommands(commandRetention)
	pipes, _ := s.CleanupOldPipelines(pipelineRetention)
	events, _ := s.CleanupOldEvents(eventRetention)

	if cmds > 0 || pipes > 0 || events > 0 {
		s.log.Info().
			Int64("commands", cmds).
			Int64("pipelines", pipes).
			Int64("events", events).
			Msg("retention cleanup complete")
	}
}

