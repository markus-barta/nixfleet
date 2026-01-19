package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/markus-barta/nixfleet/internal/ops"
)

// ═══════════════════════════════════════════════════════════════════════════
// OP ENGINE HANDLERS (v3)
// These replace the legacy handleCommand and related handlers
// ═══════════════════════════════════════════════════════════════════════════

// handleDispatchOp dispatches an op to one or more hosts using the Op Engine.
// This is the primary entry point for all operations in v3.
// POST /api/dispatch
func (s *Server) handleDispatchOp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Op    string   `json:"op"`              // Op ID: "pull", "switch", "test", etc.
		Hosts []string `json:"hosts"`           // Host IDs to execute on
		Force bool     `json:"force,omitempty"` // Skip pre-validation
		TOTP  string   `json:"totp,omitempty"`  // For ops requiring TOTP (reboot)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Op == "" {
		s.jsonError(w, "op is required", http.StatusBadRequest)
		return
	}

	if len(req.Hosts) == 0 {
		s.jsonError(w, "hosts is required", http.StatusBadRequest)
		return
	}

	// Check if op exists
	op := s.opRegistry.Get(req.Op)
	if op == nil {
		s.jsonError(w, fmt.Sprintf("unknown op: %s", req.Op), http.StatusBadRequest)
		return
	}

	// Check TOTP for ops that require it
	if op.RequiresTotp {
		if !s.cfg.HasTOTP() {
			s.jsonError(w, "TOTP must be configured for this operation", http.StatusForbidden)
			return
		}
		if req.TOTP == "" || !s.auth.CheckTOTP(req.TOTP) {
			s.jsonError(w, "Invalid TOTP code", http.StatusUnauthorized)
			return
		}
	}

	// Execute on each host
	results := make([]map[string]any, 0, len(req.Hosts))
	var successCount, errorCount int

	for _, hostID := range req.Hosts {
		host, err := s.getHostByID(hostID)
		if err != nil {
			results = append(results, map[string]any{
				"host_id": hostID,
				"status":  "error",
				"error":   "Host not found",
			})
			errorCount++
			continue
		}

		// Create host adapter for Op Engine
		hostAdapter := ops.NewHostAdapter(host)

		// Execute the op via lifecycle manager
		cmd, err := s.lifecycleManager.ExecuteOp(req.Op, hostAdapter, req.Force)
		if err != nil {
			// Check if it's a validation error (blocked)
			if verr, ok := err.(*ops.ValidationError); ok {
				results = append(results, map[string]any{
					"host_id": hostID,
					"status":  "blocked",
					"code":    verr.Code,
					"message": verr.Message,
				})
			} else {
				results = append(results, map[string]any{
					"host_id": hostID,
					"status":  "error",
					"error":   err.Error(),
				})
			}
			errorCount++
			continue
		}

		results = append(results, map[string]any{
			"host_id":    hostID,
			"status":     string(cmd.Status),
			"command_id": cmd.ID,
		})
		if cmd.Status == ops.StatusExecuting || cmd.Status == ops.StatusPending {
			successCount++
		} else if cmd.Status == ops.StatusBlocked {
			errorCount++
		} else {
			successCount++
		}

		// P8900: Don't broadcast here - LifecycleManager already broadcasts via BroadcastCommandState()
		// Duplicate emission was causing "pull started" to appear twice in logs
	}

	// Determine overall status
	status := "success"
	if errorCount > 0 && successCount == 0 {
		status = "failed"
	} else if errorCount > 0 {
		status = "partial"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"results": results,
		"summary": map[string]int{
			"success": successCount,
			"error":   errorCount,
			"total":   len(req.Hosts),
		},
	})
}

// handleDispatchPipeline dispatches a pipeline to hosts.
// POST /api/dispatch/pipeline
func (s *Server) handleDispatchPipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pipeline string   `json:"pipeline"`        // Pipeline ID: "do-all", "merge-deploy"
		Hosts    []string `json:"hosts"`           // Host IDs to execute on
		TOTP     string   `json:"totp,omitempty"`  // For pipelines with TOTP ops
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Pipeline == "" {
		s.jsonError(w, "pipeline is required", http.StatusBadRequest)
		return
	}

	if len(req.Hosts) == 0 {
		s.jsonError(w, "hosts is required", http.StatusBadRequest)
		return
	}

	// Check if pipeline exists
	pipeline := s.pipelineRegistry.Get(req.Pipeline)
	if pipeline == nil {
		s.jsonError(w, fmt.Sprintf("unknown pipeline: %s", req.Pipeline), http.StatusBadRequest)
		return
	}

	// Check if any op in the pipeline requires TOTP
	for _, opID := range pipeline.Ops {
		op := s.opRegistry.Get(opID)
		if op != nil && op.RequiresTotp {
			if !s.cfg.HasTOTP() {
				s.jsonError(w, "TOTP must be configured for this pipeline", http.StatusForbidden)
				return
			}
			if req.TOTP == "" || !s.auth.CheckTOTP(req.TOTP) {
				s.jsonError(w, "Invalid TOTP code", http.StatusUnauthorized)
				return
			}
			break
		}
	}

	// Build host list
	hosts := make([]ops.Host, 0, len(req.Hosts))
	for _, hostID := range req.Hosts {
		host, err := s.getHostByID(hostID)
		if err != nil {
			s.jsonError(w, fmt.Sprintf("host not found: %s", hostID), http.StatusNotFound)
			return
		}
		hosts = append(hosts, ops.NewHostAdapter(host))
	}

	// Execute pipeline (async - returns immediately, completion comes via WebSocket)
	// Use background context since HTTP request context is canceled after response
	go func() {
		record, err := s.pipelineExecutor.Execute(context.Background(), req.Pipeline, hosts)
		if err != nil {
			s.log.Error().Err(err).Str("pipeline", req.Pipeline).Msg("pipeline execution failed")
		} else {
			s.log.Info().
				Str("pipeline", record.ID).
				Str("status", string(record.Status)).
				Msg("pipeline completed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "started",
		"pipeline": req.Pipeline,
		"hosts":    req.Hosts,
	})
}

// handleGetOps returns the list of available ops.
// GET /api/ops
func (s *Server) handleGetOps(w http.ResponseWriter, r *http.Request) {
	allOps := s.opRegistry.All()

	opList := make([]map[string]any, 0, len(allOps))
	for _, op := range allOps {
		opList = append(opList, map[string]any{
			"id":            op.ID,
			"description":   op.Description,
			"executor":      string(op.Executor),
			"timeout":       op.Timeout.String(),
			"retryable":     op.Retryable,
			"requires_totp": op.RequiresTotp,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ops": opList})
}

// handleGetPipelines returns the list of available pipelines.
// GET /api/pipelines
func (s *Server) handleGetPipelines(w http.ResponseWriter, r *http.Request) {
	allPipelines := s.pipelineRegistry.All()

	pipelineList := make([]map[string]any, 0, len(allPipelines))
	for _, p := range allPipelines {
		pipelineList = append(pipelineList, map[string]any{
			"id":          p.ID,
			"ops":         p.Ops,
			"description": p.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"pipelines": pipelineList})
}

// handleGetEventLog returns recent events from the event log.
// GET /api/events
func (s *Server) handleGetEventLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseIntLimit(limitStr, 1, 500); err == nil {
			limit = l
		}
	}

	events, err := s.stateStore.GetRecentEvents(limit)
	if err != nil {
		s.jsonError(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
}

// handleGetHostEvents returns events for a specific host.
// GET /api/hosts/{hostID}/events
func (s *Server) handleGetHostEvents(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostID")

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseIntLimit(limitStr, 1, 200); err == nil {
			limit = l
		}
	}

	events, err := s.stateStore.GetHostEvents(hostID, limit)
	if err != nil {
		s.jsonError(w, "Failed to fetch host events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════

// jsonError sends a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// parseIntLimit parses a string to an int with bounds checking.
func parseIntLimit(s string, min, max int) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return 0, err
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v, nil
}

