# P4300 - Automated Flake Lock Updates

**Created**: 2025-12-15  
**Updated**: 2025-12-19  
**Priority**: P4300 (Medium)  
**Status**: MVP Complete  
**Depends on**: P5000 (Update Status Indicator)

---

## User Story

**As a** fleet administrator  
**I want** NixFleet to handle flake.lock updates automatically  
**So that** I stay current without manual PR reviews I don't understand

---

## Overview

Turn the manual "review and merge PR" workflow into a one-click (or fully automated) experience, while preserving the ability to do each step manually.

```text
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE (manual)                                                │
│  ───────────────                                                │
│  GitHub Action → PR → You review (???) → Merge → Deploy         │
└─────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  AFTER (NixFleet - flexible modes)                              │
│  ─────────────────────────────────                              │
│                                                                 │
│  MODE 1: Manual per-step                                        │
│  Click Pull → Click Switch → per host, full control             │
│                                                                 │
│  MODE 2: Per-host automatic                                     │
│  Click "Update Host" → Pull + Switch + Verify automatically     │
│                                                                 │
│  MODE 3: Fleet-wide automatic                                   │
│  Click "Merge & Deploy" → All hosts updated in one action       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

See [UPDATE-ARCHITECTURE.md](../../docs/UPDATE-ARCHITECTURE.md) for the complete update flow documentation.

---

## Implementation Plan

### Phase 1: GitHub API Client

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

### Phase 2: PR Detection Service

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

### Phase 3: Lock Compartment Enhancement

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

### Phase 4: Merge & Deploy Endpoint

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

## Testing Strategy

### Live Testing (Primary)

Test with **real GitHub PRs** from the weekly `nix flake update` Action:

1. Wait for next PR from `.github/workflows/update-flake-lock.yml`
2. Verify Lock compartment shows "PR pending"
3. Click "Merge & Deploy"
4. Watch the deployment happen
5. Verify all hosts update successfully

### Unit Tests

- `github/client_test.go` - Test API response parsing
- `flake_updates_test.go` - Test service logic (with stubbed client)

### E2E Tests (Future)

See **P4301 - Flake Updates E2E Test Suite** for comprehensive automated testing with mock infrastructure.

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

### MVP

- [x] GitHub API client with interface for testing
- [x] PR detection polling (hourly + manual trigger)
- [x] Lock compartment shows "PR pending" when update available
- [x] API endpoint: `POST /api/flake-updates/check`
- [x] API endpoint: `POST /api/flake-updates/merge-and-deploy`
- [x] Merge PR via GitHub API
- [x] Sequential deploy: pull → switch on all online hosts (with proper completion tracking)
- [x] WebSocket broadcast of job progress
- [x] Basic error handling and status reporting

### Tests

- [x] Unit tests for GitHub client (response parsing) — `types_test.go`
- [x] Unit tests for FlakeUpdateService logic — `flake_updates_test.go`
- [ ] Manual live test with real GitHub PR

_See P4301 for comprehensive E2E test suite (future)_

---

## Future Enhancements

See separate backlog items:

- **P4310**: Rollback on failure (revert merge commit, per-host rollback)
- **P4301**: Flake Updates E2E Test Suite

### Automation Features (Future)

- Full automation toggle (auto-merge + deploy)
- Configurable delay before auto-merge
- Per-host inclusion/exclusion from auto-deploy
- Canary deployment (deploy to one host first, wait, then rest)
- Deploy priority ordering

### Power User Features (Future)

- "Hold" a host from updates
- Update history with diff view
- Scheduled maintenance windows (only deploy during certain hours)

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
