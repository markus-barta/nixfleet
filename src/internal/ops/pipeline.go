package ops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PipelineStatus represents the current status of a pipeline execution.
type PipelineStatus string

const (
	PipelineIdle      PipelineStatus = "IDLE"
	PipelineRunning   PipelineStatus = "RUNNING"
	PipelineComplete  PipelineStatus = "COMPLETE"
	PipelinePartial   PipelineStatus = "PARTIAL"
	PipelineFailed    PipelineStatus = "FAILED"
	PipelineCancelled PipelineStatus = "CANCELLED"
)

// Pipeline defines an ordered sequence of op IDs with && semantics.
// See CORE-002 for the full specification.
type Pipeline struct {
	// ID is the unique identifier: "do-all", "merge-deploy", etc.
	ID string

	// Ops is the ordered list of op IDs to execute.
	Ops []string

	// Description is a human-readable description.
	Description string
}

// PipelineRecord represents a pipeline execution record.
// Persisted in the State Store (CORE-003).
type PipelineRecord struct {
	ID           string         `json:"id"`            // UUID
	PipelineID   string         `json:"pipeline_id"`   // Pipeline definition ID
	Hosts        []string       `json:"hosts"`         // Host IDs participating
	CurrentStage int            `json:"current_stage"` // 0-indexed stage
	Status       PipelineStatus `json:"status"`        // Current status
	CreatedAt    time.Time      `json:"created_at"`    // When started
	FinishedAt   time.Time      `json:"finished_at"`   // When completed (or failed)
}

// HostPipelineState tracks a host's progress through a pipeline.
type HostPipelineState struct {
	HostID     string     `json:"host_id"`
	StageIndex int        `json:"stage_index"` // Which stage we're on
	Status     OpStatus   `json:"status"`      // Current op status
	Error      string     `json:"error"`       // Error message if failed
	Skipped    bool       `json:"skipped"`     // True if host was excluded due to earlier failure
}

// PipelineRegistry holds all registered pipelines.
type PipelineRegistry struct {
	pipelines map[string]*Pipeline
	mu        sync.RWMutex
}

// NewPipelineRegistry creates an empty pipeline registry.
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{
		pipelines: make(map[string]*Pipeline),
	}
}

// Register adds a pipeline to the registry.
func (r *PipelineRegistry) Register(p *Pipeline) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pipelines[p.ID] = p
}

// Get returns a pipeline by ID, or nil if not found.
func (r *PipelineRegistry) Get(id string) *Pipeline {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pipelines[id]
}

// DefaultPipelineRegistry creates the standard NixFleet pipeline registry.
// All pipelines from CORE-002 are registered here.
func DefaultPipelineRegistry() *PipelineRegistry {
	r := NewPipelineRegistry()

	r.Register(&Pipeline{
		ID:          "do-all",
		Ops:         []string{"pull", "switch", "test"},
		Description: "Full update cycle",
	})

	r.Register(&Pipeline{
		ID:          "merge-deploy",
		Ops:         []string{"merge-pr", "pull", "switch", "test"},
		Description: "Merge PR then deploy",
	})

	r.Register(&Pipeline{
		ID:          "update-agent",
		Ops:         []string{"bump-flake", "pull", "switch", "restart"},
		Description: "Update agent to latest version",
	})

	r.Register(&Pipeline{
		ID:          "force-update",
		Ops:         []string{"force-rebuild", "restart"},
		Description: "Force rebuild with cache bypass",
	})

	return r
}

// PipelineExecutor orchestrates multi-op sequences.
type PipelineExecutor struct {
	log       zerolog.Logger
	opExec    *Executor
	registry  *PipelineRegistry
	store     PipelineStore
	events    EventLogger

	// Active pipelines
	active   map[string]*PipelineRecord
	activeMu sync.RWMutex
}

// PipelineStore is the interface for persisting pipeline state.
type PipelineStore interface {
	CreatePipeline(p *PipelineRecord) error
	UpdatePipelineStage(pipelineID string, stage int) error
	FinishPipeline(pipelineID string, status PipelineStatus) error
	GetPipeline(pipelineID string) (*PipelineRecord, error)
}

// NewPipelineExecutor creates a new pipeline executor.
func NewPipelineExecutor(log zerolog.Logger, opExec *Executor, registry *PipelineRegistry, store PipelineStore, events EventLogger) *PipelineExecutor {
	return &PipelineExecutor{
		log:      log.With().Str("component", "pipeline_executor").Logger(),
		opExec:   opExec,
		registry: registry,
		store:    store,
		events:   events,
		active:   make(map[string]*PipelineRecord),
	}
}

// Execute runs a pipeline on the given hosts with && semantics.
// Hosts that fail are excluded from subsequent ops.
func (pe *PipelineExecutor) Execute(ctx context.Context, pipelineID string, hosts []Host) (*PipelineRecord, error) {
	// Get pipeline definition
	pipeline := pe.registry.Get(pipelineID)
	if pipeline == nil {
		return nil, fmt.Errorf("unknown pipeline: %s", pipelineID)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts specified")
	}

	// Create pipeline record
	hostIDs := make([]string, len(hosts))
	for i, h := range hosts {
		hostIDs[i] = h.GetID()
	}

	record := &PipelineRecord{
		ID:           uuid.New().String(),
		PipelineID:   pipelineID,
		Hosts:        hostIDs,
		CurrentStage: 0,
		Status:       PipelineRunning,
		CreatedAt:    time.Now(),
	}

	// Persist
	if pe.store != nil {
		if err := pe.store.CreatePipeline(record); err != nil {
			pe.log.Error().Err(err).Str("pipeline", pipelineID).Msg("failed to persist pipeline")
		}
	}

	// Track as active
	pe.activeMu.Lock()
	pe.active[record.ID] = record
	pe.activeMu.Unlock()

	pe.logEvent("audit", "info", "user", "", "pipeline:"+pipelineID,
		fmt.Sprintf("Starting pipeline %s on %d hosts", pipelineID, len(hosts)), nil)

	// Execute stages
	activeHosts := hosts
	var hostStates []HostPipelineState

	for stageIdx, opID := range pipeline.Ops {
		record.CurrentStage = stageIdx
		if pe.store != nil {
			_ = pe.store.UpdatePipelineStage(record.ID, stageIdx)
		}

		pe.log.Info().
			Str("pipeline", record.ID).
			Int("stage", stageIdx).
			Str("op", opID).
			Int("hosts", len(activeHosts)).
			Msg("executing pipeline stage")

		pe.logEvent("audit", "info", "system", "", "pipeline:"+pipelineID,
			fmt.Sprintf("Stage %d/%d: %s on %d hosts", stageIdx+1, len(pipeline.Ops), opID, len(activeHosts)), nil)

		// Execute op on all active hosts (parallel)
		results := pe.executeStage(ctx, opID, record.ID, activeHosts)

		// Filter to successful hosts for next stage
		var stillActive []Host
		for _, result := range results {
			if result.Error == nil && result.ExitCode == 0 {
				stillActive = append(stillActive, result.Host)
			} else {
				// Mark host as skipped for remaining stages
				hostStates = append(hostStates, HostPipelineState{
					HostID:     result.Host.GetID(),
					StageIndex: stageIdx,
					Status:     StatusSkipped,
					Error:      result.Error.Error(),
					Skipped:    true,
				})
			}
		}

		activeHosts = stillActive

		// Check if any hosts remain
		if len(activeHosts) == 0 {
			record.Status = PipelineFailed
			record.FinishedAt = time.Now()
			if pe.store != nil {
				_ = pe.store.FinishPipeline(record.ID, PipelineFailed)
			}
			pe.logEvent("audit", "error", "system", "", "pipeline:"+pipelineID,
				fmt.Sprintf("Pipeline failed: all hosts failed at stage %d (%s)", stageIdx, opID), nil)
			return record, fmt.Errorf("all hosts failed at stage %d", stageIdx)
		}
	}

	// Determine final status
	if len(activeHosts) < len(hosts) {
		record.Status = PipelinePartial
	} else {
		record.Status = PipelineComplete
	}
	record.FinishedAt = time.Now()

	if pe.store != nil {
		_ = pe.store.FinishPipeline(record.ID, record.Status)
	}

	// Clear from active
	pe.activeMu.Lock()
	delete(pe.active, record.ID)
	pe.activeMu.Unlock()

	pe.logEvent("audit", "success", "system", "", "pipeline:"+pipelineID,
		fmt.Sprintf("Pipeline %s: %d/%d hosts completed all stages", string(record.Status), len(activeHosts), len(hosts)), nil)

	return record, nil
}

// executeStage runs an op on all hosts in parallel and collects results.
func (pe *PipelineExecutor) executeStage(ctx context.Context, opID, pipelineID string, hosts []Host) []OpResult {
	var wg sync.WaitGroup
	results := make([]OpResult, len(hosts))

	for i, host := range hosts {
		wg.Add(1)
		go func(idx int, h Host) {
			defer wg.Done()

			cmd, err := pe.opExec.ExecuteOp(ctx, opID, h, false)
			results[idx] = OpResult{
				Command: cmd,
				Host:    h,
				Error:   err,
			}
			if cmd != nil && cmd.ExitCode != nil {
				results[idx].ExitCode = *cmd.ExitCode
			}
		}(i, host)
	}

	wg.Wait()
	return results
}

// Cancel cancels a running pipeline.
func (pe *PipelineExecutor) Cancel(pipelineID string) error {
	pe.activeMu.Lock()
	record := pe.active[pipelineID]
	if record != nil {
		record.Status = PipelineCancelled
		record.FinishedAt = time.Now()
		delete(pe.active, pipelineID)
	}
	pe.activeMu.Unlock()

	if record == nil {
		return fmt.Errorf("no active pipeline: %s", pipelineID)
	}

	if pe.store != nil {
		_ = pe.store.FinishPipeline(pipelineID, PipelineCancelled)
	}

	pe.logEvent("audit", "warn", "user", "", "pipeline:"+record.PipelineID,
		"Pipeline cancelled by user", nil)

	return nil
}

// GetActive returns the active pipeline record, if any.
func (pe *PipelineExecutor) GetActive(pipelineID string) *PipelineRecord {
	pe.activeMu.RLock()
	defer pe.activeMu.RUnlock()
	return pe.active[pipelineID]
}

func (pe *PipelineExecutor) logEvent(category, level, actor, hostID, action, message string, details map[string]any) {
	if pe.events != nil {
		pe.events.LogEvent(category, level, actor, hostID, action, message, details)
	}
}

