package dashboard

import (
	"context"
	"sync"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/github"
	"github.com/rs/zerolog"
)

// FlakeUpdateService monitors GitHub for flake.lock update PRs and manages deployments.
type FlakeUpdateService struct {
	client github.Client
	hub    *Hub
	log    zerolog.Logger
	cfg    *Config

	mu        sync.RWMutex
	pendingPR *github.PullRequest // Current pending flake update PR
	lastCheck time.Time

	// Deployment state
	deployMu    sync.Mutex
	deployJob   *DeployJob
	deployJobID int
}

// DeployJob represents an ongoing merge-and-deploy operation.
type DeployJob struct {
	ID        string    `json:"id"`
	PRNumber  int       `json:"pr_number"`
	State     string    `json:"state"` // "merging", "pulling", "switching", "completed", "failed"
	StartedAt time.Time `json:"started_at"`
	Message   string    `json:"message,omitempty"`

	// Progress tracking
	TotalHosts     int      `json:"total_hosts"`
	CompletedHosts int      `json:"completed_hosts"`
	FailedHosts    []string `json:"failed_hosts,omitempty"`
}

// PendingPRInfo is a simplified view of a pending PR for the frontend.
type PendingPRInfo struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
	Mergeable bool   `json:"mergeable"`
}

// NewFlakeUpdateService creates a new flake update service.
func NewFlakeUpdateService(cfg *Config, hub *Hub, log zerolog.Logger) *FlakeUpdateService {
	client := github.NewClient(github.ClientConfig{
		Token:   cfg.GitHubToken,
		BaseURL: cfg.GitHubAPIURL,
		Timeout: 30 * time.Second,
	})

	return &FlakeUpdateService{
		client: client,
		hub:    hub,
		log:    log.With().Str("component", "flake-updates").Logger(),
		cfg:    cfg,
	}
}

// Start begins the background polling loop for update PRs.
func (s *FlakeUpdateService) Start(ctx context.Context) {
	// Do initial check
	s.CheckForUpdates(ctx)

	ticker := time.NewTicker(s.cfg.GitHubPollTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("flake update service stopped")
			return
		case <-ticker.C:
			s.CheckForUpdates(ctx)
		}
	}
}

// CheckForUpdates queries GitHub for pending flake.lock update PRs.
func (s *FlakeUpdateService) CheckForUpdates(ctx context.Context) {
	owner, repo := s.cfg.GitHubOwnerRepo()
	if owner == "" || repo == "" {
		s.log.Debug().Msg("GitHub repo not configured, skipping PR check")
		return
	}

	s.log.Debug().Str("repo", s.cfg.GitHubRepo).Msg("checking for update PRs")

	prs, err := s.client.ListOpenPRs(ctx, owner, repo)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to list open PRs")
		return
	}

	// Find flake.lock update PRs
	var updatePR *github.PullRequest
	for i := range prs {
		if prs[i].IsFlakeLockUpdate() {
			updatePR = &prs[i]
			break // Take the first (oldest) one
		}
	}

	s.mu.Lock()
	oldPR := s.pendingPR
	s.pendingPR = updatePR
	s.lastCheck = time.Now()
	s.mu.Unlock()

	// Log and broadcast if status changed
	if updatePR != nil && (oldPR == nil || oldPR.Number != updatePR.Number) {
		s.log.Info().
			Int("pr", updatePR.Number).
			Str("title", updatePR.Title).
			Msg("detected flake update PR")

		// Broadcast to all connected browsers
		s.broadcastPRStatus()
	} else if updatePR == nil && oldPR != nil {
		s.log.Info().Msg("no pending update PRs")
		s.broadcastPRStatus()
	}
}

// GetPendingPR returns the current pending PR info, if any.
func (s *FlakeUpdateService) GetPendingPR() *PendingPRInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.pendingPR == nil {
		return nil
	}

	return &PendingPRInfo{
		Number:    s.pendingPR.Number,
		Title:     s.pendingPR.Title,
		URL:       s.pendingPR.HTMLURL,
		CreatedAt: s.pendingPR.CreatedAt.Format(time.RFC3339),
		Mergeable: s.pendingPR.IsMergeable(),
	}
}

// GetLastCheck returns when the last PR check was performed.
func (s *FlakeUpdateService) GetLastCheck() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCheck
}

// GetCurrentJob returns the current deployment job, if any.
func (s *FlakeUpdateService) GetCurrentJob() *DeployJob {
	s.deployMu.Lock()
	defer s.deployMu.Unlock()
	return s.deployJob
}

// MergeAndDeploy starts a merge-and-deploy operation.
// Returns the job ID for tracking progress.
func (s *FlakeUpdateService) MergeAndDeploy(ctx context.Context, prNumber int, hostIDs []string) (string, error) {
	s.deployMu.Lock()

	// Check if already running
	if s.deployJob != nil && (s.deployJob.State == "merging" || s.deployJob.State == "pulling" || s.deployJob.State == "switching") {
		s.deployMu.Unlock()
		return "", &ErrDeployInProgress{JobID: s.deployJob.ID}
	}

	// Create new job
	s.deployJobID++
	jobID := time.Now().Format("20060102-150405") + "-" + string(rune('0'+s.deployJobID%10))

	job := &DeployJob{
		ID:        jobID,
		PRNumber:  prNumber,
		State:     "merging",
		StartedAt: time.Now(),
	}
	s.deployJob = job
	s.deployMu.Unlock()

	// Run deployment in background
	go s.runDeploy(context.Background(), job, hostIDs)

	return jobID, nil
}

// runDeploy executes the merge-and-deploy workflow.
func (s *FlakeUpdateService) runDeploy(ctx context.Context, job *DeployJob, hostIDs []string) {
	owner, repo := s.cfg.GitHubOwnerRepo()

	s.log.Info().
		Str("job", job.ID).
		Int("pr", job.PRNumber).
		Msg("starting merge-and-deploy")

	// 1. Merge the PR
	s.updateJobState(job, "merging", "Merging PR...")
	result, err := s.client.MergePR(ctx, owner, repo, job.PRNumber, "merge")
	if err != nil {
		s.updateJobState(job, "failed", "Failed to merge: "+err.Error())
		s.log.Error().Err(err).Int("pr", job.PRNumber).Msg("merge failed")
		return
	}

	s.log.Info().
		Int("pr", job.PRNumber).
		Str("sha", result.SHA).
		Msg("PR merged successfully")

	// Clear pending PR
	s.mu.Lock()
	s.pendingPR = nil
	s.mu.Unlock()
	s.broadcastPRStatus()

	// 2. Wait a moment for GitHub to process merge
	time.Sleep(3 * time.Second)

	// 3. Get target hosts
	hosts := s.hub.GetOnlineHosts()
	if len(hostIDs) > 0 {
		// Filter to specified hosts
		filtered := make([]string, 0)
		hostSet := make(map[string]bool)
		for _, id := range hostIDs {
			hostSet[id] = true
		}
		for _, h := range hosts {
			if hostSet[h] {
				filtered = append(filtered, h)
			}
		}
		hosts = filtered
	}

	if len(hosts) == 0 {
		s.updateJobState(job, "completed", "No online hosts to deploy to")
		return
	}

	job.TotalHosts = len(hosts)

	// 4. Pull on all hosts
	s.updateJobState(job, "pulling", "Pulling updates...")
	for _, hostID := range hosts {
		s.log.Debug().Str("host", hostID).Msg("sending pull command")
		s.hub.SendCommand(hostID, "pull")
	}

	// Wait for pulls to complete (simplified - in production would wait for command_result)
	time.Sleep(10 * time.Second)

	// 5. Switch on all hosts
	s.updateJobState(job, "switching", "Switching configurations...")
	for _, hostID := range hosts {
		s.log.Debug().Str("host", hostID).Msg("sending switch command")
		s.hub.SendCommand(hostID, "switch")
		job.CompletedHosts++
		s.broadcastJobStatus(job)
	}

	// 6. Done
	s.updateJobState(job, "completed", "Deployment complete")
	s.log.Info().
		Str("job", job.ID).
		Int("hosts", len(hosts)).
		Msg("merge-and-deploy completed")
}

// updateJobState updates the job state and broadcasts to clients.
func (s *FlakeUpdateService) updateJobState(job *DeployJob, state, message string) {
	s.deployMu.Lock()
	job.State = state
	job.Message = message
	s.deployMu.Unlock()

	s.broadcastJobStatus(job)
}

// broadcastPRStatus sends pending PR info to all connected browsers.
func (s *FlakeUpdateService) broadcastPRStatus() {
	pr := s.GetPendingPR()
	s.hub.BroadcastTypedMessage("flake_update_pr", map[string]interface{}{
		"pending_pr": pr,
		"checked_at": s.GetLastCheck().Format(time.RFC3339),
	})
}

// broadcastJobStatus sends job status to all connected browsers.
func (s *FlakeUpdateService) broadcastJobStatus(job *DeployJob) {
	s.hub.BroadcastTypedMessage("flake_update_job", job)
}

// ErrDeployInProgress is returned when a deploy is already running.
type ErrDeployInProgress struct {
	JobID string
}

func (e *ErrDeployInProgress) Error() string {
	return "deployment already in progress: " + e.JobID
}

