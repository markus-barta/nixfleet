package dashboard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/github"
	"github.com/rs/zerolog"
)

// mockHub implements the minimal Hub interface needed for testing FlakeUpdateService.
type mockHub struct {
	mu           sync.Mutex
	onlineHosts  []string
	sentCommands []sentCommand
	completions  chan CommandCompletion
}

type sentCommand struct {
	HostID  string
	Command string
}

func newMockHub(hosts []string) *mockHub {
	return &mockHub{
		onlineHosts: hosts,
		completions: make(chan CommandCompletion, 100),
	}
}

func (m *mockHub) GetOnlineHosts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onlineHosts
}

func (m *mockHub) SendCommand(hostID, command string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentCommands = append(m.sentCommands, sentCommand{HostID: hostID, Command: command})
	return true
}

func (m *mockHub) SubscribeCommandCompletion(hostIDs []string) chan CommandCompletion {
	return m.completions
}

func (m *mockHub) UnsubscribeCommandCompletion(ch chan CommandCompletion) {
	// Don't close - test controls the channel
}

func (m *mockHub) BroadcastTypedMessage(msgType string, payload interface{}) {
	// No-op for tests
}

func (m *mockHub) SetFlakeUpdates(fu flakeUpdateGetter) {
	// No-op for tests
}

func (m *mockHub) getSentCommands() []sentCommand {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]sentCommand, len(m.sentCommands))
	copy(result, m.sentCommands)
	return result
}

// simulateCompletion sends a command completion to the hub.
func (m *mockHub) simulateCompletion(hostID, command string, exitCode int) {
	m.completions <- CommandCompletion{
		HostID:   hostID,
		Command:  command,
		ExitCode: exitCode,
		Success:  exitCode == 0,
	}
}

// TestFlakeUpdateService_CheckForUpdates tests PR detection logic.
func TestFlakeUpdateService_CheckForUpdates(t *testing.T) {
	tests := []struct {
		name      string
		prs       []github.PullRequest
		wantPR    bool
		wantPRNum int
	}{
		{
			name:   "no open PRs",
			prs:    nil,
			wantPR: false,
		},
		{
			name: "one flake update PR",
			prs: []github.PullRequest{
				{Number: 42, Title: "Update flake.lock", State: "open", MergeableState: "mergeable"},
			},
			wantPR:    true,
			wantPRNum: 42,
		},
		{
			name: "multiple PRs - picks flake update",
			prs: []github.PullRequest{
				{Number: 1, Title: "Fix bug", State: "open"},
				{Number: 2, Title: "Bump flake inputs", State: "open", MergeableState: "mergeable"},
				{Number: 3, Title: "Add feature", State: "open"},
			},
			wantPR:    true,
			wantPRNum: 2,
		},
		{
			name: "PR with automated label",
			prs: []github.PullRequest{
				{Number: 99, Title: "Weekly update", State: "open", Labels: []github.Label{{Name: "automated"}}},
			},
			wantPR:    true,
			wantPRNum: 99,
		},
		{
			name: "no flake update PRs",
			prs: []github.PullRequest{
				{Number: 1, Title: "Fix bug", State: "open"},
				{Number: 2, Title: "Add feature", State: "open"},
			},
			wantPR: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := github.NewMockClient()
			mockClient.PRs = tt.prs

			cfg := &Config{
				GitHubRepo:    "test/repo",
				GitHubPollTTL: time.Hour,
			}

			svc := &FlakeUpdateService{
				client: mockClient,
				hub:    newMockHub(nil), // Required for broadcast calls
				log:    zerolog.Nop(),
				cfg:    cfg,
			}

			svc.CheckForUpdates(context.Background())

			pr := svc.GetPendingPR()
			if tt.wantPR {
				if pr == nil {
					t.Fatal("expected pending PR, got nil")
				}
				if pr.Number != tt.wantPRNum {
					t.Errorf("expected PR #%d, got #%d", tt.wantPRNum, pr.Number)
				}
			} else {
				if pr != nil {
					t.Errorf("expected no pending PR, got #%d", pr.Number)
				}
			}
		})
	}
}

// TestFlakeUpdateService_MergeAndDeploy_ConcurrentPrevention tests that only one deploy can run at a time.
func TestFlakeUpdateService_MergeAndDeploy_ConcurrentPrevention(t *testing.T) {
	mockClient := github.NewMockClient()
	mockClient.AddFlakeUpdatePR(1, "Update flake.lock")

	cfg := &Config{
		GitHubRepo:    "test/repo",
		GitHubPollTTL: time.Hour,
	}

	svc := &FlakeUpdateService{
		client: mockClient,
		hub:    newMockHub([]string{"host1"}),
		log:    zerolog.Nop(),
		cfg:    cfg,
	}

	// Set up a running job
	svc.deployJob = &DeployJob{
		ID:    "test-job",
		State: "merging",
	}

	// Try to start another deploy
	_, err := svc.MergeAndDeploy(context.Background(), 1, nil)
	if err == nil {
		t.Fatal("expected error when deploy already in progress")
	}

	if _, ok := err.(*ErrDeployInProgress); !ok {
		t.Errorf("expected ErrDeployInProgress, got %T: %v", err, err)
	}
}

// TestFlakeUpdateService_GetPendingPR_ThreadSafe tests concurrent access to pending PR.
func TestFlakeUpdateService_GetPendingPR_ThreadSafe(t *testing.T) {
	svc := &FlakeUpdateService{
		log: zerolog.Nop(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			svc.mu.Lock()
			svc.pendingPR = &github.PullRequest{Number: n}
			svc.mu.Unlock()
		}(i)
		go func() {
			defer wg.Done()
			_ = svc.GetPendingPR()
		}()
	}
	wg.Wait()
}

// TestRunCommandPhase_AllSuccess tests the command phase when all hosts succeed.
func TestRunCommandPhase_AllSuccess(t *testing.T) {
	hub := newMockHub([]string{"host1", "host2", "host3"})

	svc := &FlakeUpdateService{
		hub: hub,
		log: zerolog.Nop(),
	}

	job := &DeployJob{TotalHosts: 3}

	// Simulate completions in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		hub.simulateCompletion("host1", "pull", 0)
		hub.simulateCompletion("host2", "pull", 0)
		hub.simulateCompletion("host3", "pull", 0)
	}()

	failed := svc.runCommandPhase(context.Background(), job, []string{"host1", "host2", "host3"}, "pull", hub.completions)

	if len(failed) != 0 {
		t.Errorf("expected no failures, got %v", failed)
	}
	if job.CompletedHosts != 3 {
		t.Errorf("expected 3 completed hosts, got %d", job.CompletedHosts)
	}
}

// TestRunCommandPhase_PartialFailure tests the command phase when some hosts fail.
func TestRunCommandPhase_PartialFailure(t *testing.T) {
	hub := newMockHub([]string{"host1", "host2", "host3"})

	svc := &FlakeUpdateService{
		hub: hub,
		log: zerolog.Nop(),
	}

	job := &DeployJob{TotalHosts: 3}

	// Simulate completions with one failure
	go func() {
		time.Sleep(10 * time.Millisecond)
		hub.simulateCompletion("host1", "pull", 0)
		hub.simulateCompletion("host2", "pull", 1) // Failed
		hub.simulateCompletion("host3", "pull", 0)
	}()

	failed := svc.runCommandPhase(context.Background(), job, []string{"host1", "host2", "host3"}, "pull", hub.completions)

	if len(failed) != 1 {
		t.Errorf("expected 1 failure, got %v", failed)
	}
	if failed[0] != "host2" {
		t.Errorf("expected host2 to fail, got %v", failed)
	}
}

// TestRunCommandPhase_ContextCancellation tests that context cancellation stops the phase.
func TestRunCommandPhase_ContextCancellation(t *testing.T) {
	hub := newMockHub([]string{"host1", "host2"})

	svc := &FlakeUpdateService{
		hub: hub,
		log: zerolog.Nop(),
	}

	job := &DeployJob{TotalHosts: 2}

	ctx, cancel := context.WithCancel(context.Background())

	// Complete one host, then cancel
	go func() {
		time.Sleep(10 * time.Millisecond)
		hub.simulateCompletion("host1", "pull", 0)
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	failed := svc.runCommandPhase(ctx, job, []string{"host1", "host2"}, "pull", hub.completions)

	// host2 should be in failed list (didn't complete before cancel)
	if len(failed) != 1 {
		t.Errorf("expected 1 failure (cancelled), got %v", failed)
	}
}

// TestRunCommandPhase_IgnoresOtherCommands tests that unrelated command completions are ignored.
func TestRunCommandPhase_IgnoresOtherCommands(t *testing.T) {
	hub := newMockHub([]string{"host1"})

	svc := &FlakeUpdateService{
		hub: hub,
		log: zerolog.Nop(),
	}

	job := &DeployJob{TotalHosts: 1}

	go func() {
		time.Sleep(10 * time.Millisecond)
		// Send completion for different command - should be ignored
		hub.simulateCompletion("host1", "switch", 0)
		time.Sleep(10 * time.Millisecond)
		// Send correct command
		hub.simulateCompletion("host1", "pull", 0)
	}()

	failed := svc.runCommandPhase(context.Background(), job, []string{"host1"}, "pull", hub.completions)

	if len(failed) != 0 {
		t.Errorf("expected no failures, got %v", failed)
	}
}

// TestPRDetection_TitlePatterns tests various title patterns for flake update detection.
func TestPRDetection_TitlePatterns(t *testing.T) {
	tests := []struct {
		title    string
		expected bool
	}{
		{"Update flake.lock", true},
		{"flake.lock: Update nixpkgs", true},
		{"Bump flake inputs", true},
		{"Update flake inputs", true},
		{"FLAKE.LOCK update", true}, // case insensitive
		{"Fix typo in readme", false},
		{"Update dependencies", false}, // no label, no flake keyword
		{"Update package.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			pr := github.PullRequest{
				Number: 1,
				Title:  tt.title,
				State:  "open",
			}
			if pr.IsFlakeLockUpdate() != tt.expected {
				t.Errorf("IsFlakeLockUpdate(%q) = %v, want %v", tt.title, !tt.expected, tt.expected)
			}
		})
	}
}

// TestPRDetection_Labels tests label-based detection.
func TestPRDetection_Labels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []github.Label
		expected bool
	}{
		{"automated label", []github.Label{{Name: "automated"}}, true},
		{"dependencies label", []github.Label{{Name: "dependencies"}}, true},
		{"multiple labels including automated", []github.Label{{Name: "bug"}, {Name: "automated"}}, true},
		{"unrelated labels", []github.Label{{Name: "bug"}, {Name: "enhancement"}}, false},
		{"no labels", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := github.PullRequest{
				Number: 1,
				Title:  "Some random title",
				State:  "open",
				Labels: tt.labels,
			}
			if pr.IsFlakeLockUpdate() != tt.expected {
				t.Errorf("IsFlakeLockUpdate() = %v, want %v", !tt.expected, tt.expected)
			}
		})
	}
}

