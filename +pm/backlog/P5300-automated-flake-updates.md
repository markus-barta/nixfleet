# P5300 - Automated Flake Lock Updates

**Created**: 2025-12-15  
**Priority**: P5300 (Medium)  
**Status**: Backlog  
**Depends on**: P5000 (Update Status Indicator)

---

## User Story

**As a** fleet administrator  
**I want** NixFleet to handle flake.lock updates automatically  
**So that** I stay current without manual PR reviews I don't understand

---

## Overview

Turn the manual "review and merge PR" workflow into a one-click (or fully automated) experience.

```text
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE (manual)                                                │
│  ───────────────                                                │
│  GitHub Action → PR → You review (???) → Merge → Deploy         │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  AFTER (NixFleet automated)                                     │
│  ──────────────────────────                                     │
│  GitHub Action → PR → NixFleet detects → Auto/Click merge       │
│  → Pull all hosts → Switch all hosts → Rollback on failure      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Plan

### Phase 1: GitHub API Client (P5300a-1)

Create abstracted GitHub client with interface for testing:

```go
// internal/github/client.go

type Client interface {
    ListOpenPRs(ctx context.Context, owner, repo string) ([]PullRequest, error)
    GetPR(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
    MergePR(ctx context.Context, owner, repo string, number int, method string) error
    GetLatestCommit(ctx context.Context, owner, repo, branch string) (string, error)
}

type PullRequest struct {
    Number    int
    Title     string
    State     string      // "open", "closed", "merged"
    Labels    []string
    CreatedAt time.Time
    UpdatedAt time.Time
    MergeableState string // "mergeable", "conflicting", "unknown"
    HeadSHA   string
    BaseBranch string
}
```

Files to create:

- `v2/internal/github/client.go` - Interface + real implementation
- `v2/internal/github/mock.go` - Mock implementation for tests
- `v2/internal/github/types.go` - Shared types

### Phase 2: PR Detection Service (P5300a-2)

Background service that polls GitHub for update PRs:

```go
// internal/dashboard/flake_updates.go

type FlakeUpdateService struct {
    github    github.Client
    hub       *Hub
    config    FlakeUpdateConfig
    lastCheck time.Time
    pendingPR *github.PullRequest
}

func (s *FlakeUpdateService) Start(ctx context.Context) {
    // Poll every hour (configurable)
    ticker := time.NewTicker(s.config.PollInterval)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkForUpdates(ctx)
        }
    }
}

func (s *FlakeUpdateService) checkForUpdates(ctx context.Context) {
    prs, err := s.github.ListOpenPRs(ctx, s.config.Owner, s.config.Repo)
    // Filter for flake.lock update PRs (label: "automated" or title contains "flake.lock")
    // Broadcast update to all browser clients
}
```

### Phase 3: Lock Compartment Enhancement (P5300a-3)

Update Lock compartment to show "PR pending" state:

```go
// New status in UpdateStatus
type UpdateStatus struct {
    Git     StatusCheck `json:"git"`
    Lock    StatusCheck `json:"lock"`
    System  StatusCheck `json:"system"`
    // NEW
    PendingPR *PendingPR `json:"pending_pr,omitempty"`
}

type PendingPR struct {
    Number    int    `json:"number"`
    Title     string `json:"title"`
    URL       string `json:"url"`
    CreatedAt string `json:"created_at"`
}
```

CSS for Lock compartment when PR pending:

```css
.update-compartment.lock.pr-pending {
  /* Distinct glow/pulse to indicate action available */
  animation: pr-pending-glow 2s ease-in-out infinite;
}
```

### Phase 4: Merge & Deploy Endpoint (P5300a-4)

New API endpoint and handler:

```go
// POST /api/flake-updates/merge-and-deploy
type MergeAndDeployRequest struct {
    PRNumber int      `json:"pr_number"`
    Hosts    []string `json:"hosts"` // empty = all online hosts
}

type MergeAndDeployResponse struct {
    Status   string   `json:"status"` // "started", "error"
    JobID    string   `json:"job_id"`
    Message  string   `json:"message,omitempty"`
}
```

Deployment job:

```go
func (s *Server) runMergeAndDeploy(ctx context.Context, jobID string, prNumber int, hosts []string) {
    // 1. Merge PR via GitHub API
    if err := s.github.MergePR(ctx, owner, repo, prNumber, "merge"); err != nil {
        s.broadcastJobUpdate(jobID, "error", "Failed to merge: "+err.Error())
        return
    }
    s.broadcastJobUpdate(jobID, "merged", "PR merged successfully")

    // 2. Wait for merge to propagate (GitHub needs a moment)
    time.Sleep(5 * time.Second)

    // 3. Pull on all hosts (sequential or parallel based on config)
    for _, hostID := range hosts {
        s.sendCommand(hostID, "pull")
        // Wait for completion
    }
    s.broadcastJobUpdate(jobID, "pulled", "All hosts pulled")

    // 4. Switch on all hosts
    for _, hostID := range hosts {
        s.sendCommand(hostID, "switch")
        // Wait for completion, handle failures
    }
    s.broadcastJobUpdate(jobID, "completed", "Deployment complete")
}
```

---

## End-to-End Test Strategy

### The Challenge

E2E testing this feature is tricky because it involves:

1. GitHub API (external service)
2. Multiple agents responding to commands
3. Time-sensitive operations (polling, waiting for completion)
4. Rollback scenarios

### Test Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                         TEST HARNESS                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│   │ MockGitHub   │    │  Dashboard   │    │ MockAgent(s) │     │
│   │   Server     │◄───│   (real)     │◄───│   (test)     │     │
│   └──────────────┘    └──────────────┘    └──────────────┘     │
│         ▲                    ▲                    ▲              │
│         │                    │                    │              │
│   ┌─────┴────────────────────┴────────────────────┴─────┐       │
│   │                    Test Driver                       │       │
│   │  1. Setup mock GitHub with pending PR                │       │
│   │  2. Start dashboard + mock agents                    │       │
│   │  3. Verify Lock compartment shows PR pending         │       │
│   │  4. Trigger "Merge & Deploy" via API                 │       │
│   │  5. Verify mock GitHub received merge call           │       │
│   │  6. Verify agents received pull + switch             │       │
│   │  7. Verify success broadcast to browsers             │       │
│   └──────────────────────────────────────────────────────┘       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Test File: `t13_flake_updates_test.go`

```go
package integration

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

// MockGitHubAPI simulates GitHub API for testing
type MockGitHubAPI struct {
    t           *testing.T
    server      *httptest.Server
    pendingPRs  []MockPR
    mergedPRs   []int
    mu          sync.Mutex
}

type MockPR struct {
    Number int
    Title  string
    Labels []string
    State  string
}

func NewMockGitHubAPI(t *testing.T) *MockGitHubAPI {
    m := &MockGitHubAPI{t: t}
    m.server = httptest.NewServer(http.HandlerFunc(m.handleRequest))
    return m
}

func (m *MockGitHubAPI) AddPendingPR(number int, title string, labels []string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.pendingPRs = append(m.pendingPRs, MockPR{
        Number: number,
        Title:  title,
        Labels: labels,
        State:  "open",
    })
}

func (m *MockGitHubAPI) WasMerged(number int) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, n := range m.mergedPRs {
        if n == number {
            return true
        }
    }
    return false
}

func (m *MockGitHubAPI) handleRequest(w http.ResponseWriter, r *http.Request) {
    // Handle GET /repos/{owner}/{repo}/pulls - list PRs
    // Handle PUT /repos/{owner}/{repo}/pulls/{number}/merge - merge PR
    // etc.
}

// TestFlakeUpdate_E2E_MergeAndDeploy tests the full flow
func TestFlakeUpdate_E2E_MergeAndDeploy(t *testing.T) {
    // 1. Setup mock GitHub
    mockGH := NewMockGitHubAPI(t)
    defer mockGH.Close()
    mockGH.AddPendingPR(42, "Update flake.lock", []string{"automated"})

    // 2. Setup mock dashboard with GitHub URL override
    // (dashboard needs to point to mockGH.URL() instead of api.github.com)

    // 3. Setup mock agents (3 of them)
    agent1 := NewMockAgent(t, "host-1")
    agent2 := NewMockAgent(t, "host-2")
    agent3 := NewMockAgent(t, "host-3")
    defer agent1.Close()
    defer agent2.Close()
    defer agent3.Close()

    // 4. Wait for agents to register
    time.Sleep(2 * time.Second)

    // 5. Trigger PR check
    resp, err := http.Post(dashboardURL+"/api/flake-updates/check", "", nil)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp.StatusCode)

    // 6. Verify Lock compartment shows PR pending
    hosts, err := getHosts(dashboardURL)
    require.NoError(t, err)
    require.NotNil(t, hosts[0].UpdateStatus.PendingPR)
    require.Equal(t, 42, hosts[0].UpdateStatus.PendingPR.Number)

    // 7. Trigger Merge & Deploy
    resp, err = http.Post(dashboardURL+"/api/flake-updates/merge-and-deploy",
        "application/json",
        strings.NewReader(`{"pr_number": 42}`))
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp.StatusCode)

    // 8. Wait for deployment to complete (with timeout)
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // 9. Verify GitHub merge was called
    require.Eventually(t, func() bool {
        return mockGH.WasMerged(42)
    }, 10*time.Second, 100*time.Millisecond)

    // 10. Verify all agents received pull command
    require.Eventually(t, func() bool {
        return agent1.ReceivedCommand("pull") &&
               agent2.ReceivedCommand("pull") &&
               agent3.ReceivedCommand("pull")
    }, 30*time.Second, 100*time.Millisecond)

    // 11. Simulate agents completing pull
    agent1.SendCommandResult("pull", "ok", "")
    agent2.SendCommandResult("pull", "ok", "")
    agent3.SendCommandResult("pull", "ok", "")

    // 12. Verify all agents received switch command
    require.Eventually(t, func() bool {
        return agent1.ReceivedCommand("switch") &&
               agent2.ReceivedCommand("switch") &&
               agent3.ReceivedCommand("switch")
    }, 30*time.Second, 100*time.Millisecond)

    // 13. Simulate agents completing switch
    agent1.SendCommandResult("switch", "ok", "")
    agent2.SendCommandResult("switch", "ok", "")
    agent3.SendCommandResult("switch", "ok", "")

    // 14. Verify deployment marked complete
    status, err := getDeploymentStatus(dashboardURL)
    require.NoError(t, err)
    require.Equal(t, "completed", status.State)
}

// TestFlakeUpdate_E2E_RollbackOnFailure tests rollback when a host fails
func TestFlakeUpdate_E2E_RollbackOnFailure(t *testing.T) {
    // Similar setup...
    // But agent2 returns error on switch
    // Verify rollback is triggered (if enabled)
}

// TestFlakeUpdate_E2E_PartialDeployment tests when some hosts are offline
func TestFlakeUpdate_E2E_PartialDeployment(t *testing.T) {
    // Only agent1 and agent3 are online
    // Verify deployment only targets online hosts
}
```

### Mock Agent Enhancement

Extend the existing `MockDashboard` pattern to create `MockAgent`:

```go
// MockAgent simulates an agent for testing dashboard flows
type MockAgent struct {
    t              *testing.T
    hostname       string
    ws             *websocket.Conn
    receivedCmds   []string
    mu             sync.Mutex
}

func NewMockAgent(t *testing.T, hostname string) *MockAgent {
    // Connect to dashboard WebSocket
    // Send register message
    // Handle incoming commands
}

func (m *MockAgent) ReceivedCommand(cmd string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, c := range m.receivedCmds {
        if c == cmd {
            return true
        }
    }
    return false
}

func (m *MockAgent) SendCommandResult(cmd, status, message string) {
    // Send command_result message to dashboard
}
```

---

## Features

### 1. PR Detection

NixFleet dashboard checks GitHub API for open PRs:

- Repo: configured via `NIXFLEET_GITHUB_REPO` (e.g., `markus-barta/nixcfg`)
- Filter: PRs with label `automated` or title containing "flake.lock"
- Polling: Once a day at e.g. midnight UTC (or via dashboard manual check)

### 2. Lock Compartment Integration

When PR detected:

- Lock compartment glows
- Tooltip: "Update PR pending: nixpkgs abc123 → def456"
- Click opens action menu

### 3. One-Click Update

User clicks "Merge & Deploy":

1. Merge PR via GitHub API
2. Wait for merge to complete
3. Trigger `pull` on all (or selected) hosts
4. Trigger `switch` on all (or selected) hosts
5. Monitor for failures

### 4. Full Automation (Optional)

If enabled, NixFleet automatically:

1. Detects new update PR
2. Waits configurable delay (default: 1 hour) for CI to pass
3. Merges PR
4. Deploys to all hosts
5. Notifies admin of success/failure via apprise notification service (on csb0 or csb1)

### 5. Rollback on Failure

If any host fails during switch:

- Stop deployment to remaining hosts
- Offer rollback options:
  - Revert the merge commit
  - Create revert PR
  - Or just alert admin

---

## Configuration

### Environment Variables

```bash
# GitHub integration
NIXFLEET_GITHUB_TOKEN=ghp_xxxxx          # PAT with repo scope
NIXFLEET_GITHUB_REPO=markus-barta/nixcfg # owner/repo format
NIXFLEET_GITHUB_API_URL=                 # Optional: for testing with mock

# Automation settings (future)
NIXFLEET_AUTO_MERGE=false                # Enable full automation
NIXFLEET_AUTO_MERGE_DELAY=60             # Minutes to wait before auto-merge
```

### Per-Host Settings (Future)

```nix
services.nixfleet-agent = {
  # ...
  flakeUpdates = {
    autoUpdate = true;      # Include in auto-deploy (default: true)
    priority = 1;           # Deploy order (lower = first)
    canary = false;         # Deploy first as canary, wait, then others
  };
};
```

---

## Acceptance Criteria

### MVP (P5300a)

- [ ] GitHub API client with interface for testing
- [ ] PR detection polling (hourly + manual trigger)
- [ ] Lock compartment shows "PR pending" when update available
- [ ] API endpoint: `POST /api/flake-updates/check`
- [ ] API endpoint: `POST /api/flake-updates/merge-and-deploy`
- [ ] Merge PR via GitHub API
- [ ] Sequential deploy: pull → switch on all online hosts
- [ ] WebSocket broadcast of job progress
- [ ] Basic error handling and status reporting

### Tests (P5300a-test)

- [ ] Unit tests for GitHub client
- [ ] Integration test: MockGitHub + MockAgents + real dashboard
- [ ] E2E test: Full merge-and-deploy flow
- [ ] E2E test: Failure handling (agent fails switch)
- [ ] E2E test: Partial deployment (offline hosts)

### Pro Features (P5300b)

- [ ] Full automation toggle (auto-merge + deploy)
- [ ] Configurable delay before auto-merge
- [ ] Per-host inclusion/exclusion from auto-deploy
- [ ] Canary deployment (deploy to one host first, wait, then rest)
- [ ] Rollback on failure (revert merge commit)
- [ ] Deploy priority ordering

### Power User (P5300c)

- [ ] Per-host rollback controls
- [ ] "Hold" a host from updates
- [ ] Update history with diff view
- [ ] Scheduled maintenance windows (only deploy during certain hours)

---

## Technical Notes

### GitHub API

```go
// Check for open update PRs
GET /repos/{owner}/{repo}/pulls?state=open&labels=automated

// Merge a PR
PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge
{
  "merge_method": "merge"  // or "squash" or "rebase"
}
```

Requires GitHub PAT with `repo` scope.

### Deployment Order

Default order for safer deployments:

1. Non-critical hosts (gaming PCs, personal laptops)
2. Home servers (hsb0, hsb1)
3. Cloud servers (csb0, csb1) — last, most critical

Or use canary pattern:

1. Deploy to `gpc0` (gaming PC, low risk)
2. Wait 10 minutes
3. If healthy, deploy to rest

### Rollback Strategy

```bash
# Revert the merge commit
git revert -m 1 HEAD
git push origin main

# This creates a new commit that undoes the flake.lock change
# Hosts will see "behind remote" and need to pull + switch again
```

---

## Security Considerations

- GitHub token stored securely (env var or secret manager)
- Token should have minimal scope (only repo access needed)
- Consider: separate token per action (merge vs read)
- Audit log: record who triggered merge, when, what changed

---

## Related

- **P5000**: Update status indicator (shows Lock compartment)
- **P5100**: Queue offline commands (deploy to offline hosts when they come back)
- Existing: `.github/workflows/update-flake-lock.yml` creates weekly PRs

---

## Open Questions

1. **What if CI fails?** Don't merge, but should we notify?
2. **What if deploy takes hours?** Show progress? Allow cancel?
3. **Multiple update PRs?** Merge oldest first? Or newest?
4. **Dependency updates only?** Or also flake.nix changes in same PR?
