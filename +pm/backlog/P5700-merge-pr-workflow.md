# P5700 - Merge PR Workflow

**Created**: 2025-12-28  
**Priority**: P5700 (Medium - Workflow Feature)  
**Status**: Backlog  
**Effort**: 2-3 hours  
**Depends on**: P5200

---

## User Story

**As a** fleet administrator  
**I want** to merge flake.lock update PRs from the dashboard  
**So that** I don't have to go to GitHub to merge, then come back to deploy

---

## Problem

Current workflow for flake.lock updates:

1. GitHub Action creates PR with new `flake.lock`
2. Go to GitHub → find PR → review → **merge** (leave NixFleet)
3. Go back to NixFleet → Pull → Switch hosts
4. **Disjointed experience**, context switching

---

## Solution

**"Merge PR" button in dashboard:**

- Detects pending flake.lock update PR
- One-click merge from dashboard
- Updates all Lock compartments to yellow
- User then deploys using existing batch operations (manual control)

### Key Principle

**Manual deployment control!** The button ONLY merges the PR. Deployment is still manual (one-by-one or batch, user's choice).

---

## Acceptance Criteria

### PR Detection

- [ ] Dashboard polls GitHub API for open PRs (once per hour)
- [ ] Filter: PRs with label "automated" or title containing "flake.lock"
- [ ] Store pending PR in State Store
- [ ] Broadcast PR status to browsers via State Sync

### Merge PR Button

- [ ] Button appears in context bar when PR detected
- [ ] Shows: "Merge PR #42: Update flake.lock"
- [ ] Click opens confirmation dialog
- [ ] Dialog shows: PR title, changes summary, mergeable status

### Merge Flow

- [ ] Merge via GitHub API (merge commit, not squash/rebase)
- [ ] Wait 3-5 seconds for GitHub to process
- [ ] Clear pending PR from state
- [ ] Broadcast: "PR merged, Lock compartments updating"
- [ ] All hosts: Lock → 🟡 (yellow, need pull)

### Error Handling

- [ ] If PR not mergeable: show error "PR has conflicts"
- [ ] If merge fails: show error message from GitHub
- [ ] If GitHub API unavailable: disable button, show reason

---

## Technical Design

### PR Detection

```go
// internal/dashboard/flake_updates.go
func (s *FlakeUpdateService) CheckForPendingPR(ctx context.Context) error {
    owner, repo := s.cfg.GitHubOwnerRepo()

    // Fetch open PRs
    prs, err := s.client.ListOpenPRs(ctx, owner, repo)
    if err != nil {
        return err
    }

    // Filter to flake.lock updates
    var updatePR *github.PullRequest
    for _, pr := range prs {
        if pr.IsFlakeLockUpdate() && pr.IsMergeable() {
            updatePR = &pr
            break
        }
    }

    // Store in state
    s.mu.Lock()
    s.pendingPR = updatePR
    s.lastCheck = time.Now()
    s.mu.Unlock()

    // Broadcast to browsers
    if updatePR != nil {
        s.hub.BroadcastTypedMessage("pending_pr", map[string]any{
            "number":    updatePR.Number,
            "title":     updatePR.Title,
            "url":       updatePR.HTMLURL,
            "mergeable": updatePR.IsMergeable(),
        })
    }

    return nil
}
```

### Merge PR Handler

```go
// internal/dashboard/handlers_ops.go
func (s *Server) handleMergePR(w http.ResponseWriter, r *http.Request) {
    var req struct {
        PRNumber int `json:"pr_number"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // Merge via GitHub API
    owner, repo := s.cfg.GitHubOwnerRepo()
    result, err := s.flakeUpdates.client.MergePR(r.Context(), owner, repo, req.PRNumber, "merge")
    if err != nil {
        s.log.Error().Err(err).Int("pr", req.PRNumber).Msg("merge failed")
        http.Error(w, "failed to merge: "+err.Error(), http.StatusInternalServerError)
        return
    }

    s.log.Info().
        Int("pr", req.PRNumber).
        Str("sha", result.SHA).
        Msg("PR merged successfully")

    // Clear pending PR
    s.flakeUpdates.ClearPendingPR()

    // Broadcast: all hosts now have outdated Lock
    s.hub.BroadcastToAll("pr_merged", map[string]any{
        "pr_number": req.PRNumber,
        "sha":       result.SHA,
        "message":   "flake.lock updated - Pull to get new version",
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "status":  "merged",
        "sha":     result.SHA,
        "message": "PR merged successfully",
    })
}
```

### UI Component

```html
<!-- Context bar (Alpine.js) -->
<div x-show="pendingPR" class="context-row context-row-pr">
  <div class="context-row-info">
    <svg class="icon icon-pr"><use href="#icon-git-pull-request"></use></svg>
    <span class="pr-label">PR #<span x-text="pendingPR.number"></span></span>
    <span class="pr-detail" x-text="pendingPR.title"></span>
  </div>
  <button class="btn btn-merge" @click="mergePR()">
    <svg class="icon"><use href="#icon-git-merge"></use></svg>
    <span>Merge PR</span>
  </button>
</div>
```

### Merge Confirmation Dialog

```javascript
async function mergePR() {
  const pr = pendingPR;

  const confirmed = confirm(
    `Merge PR #${pr.number}?\n\n` +
      `"${pr.title}"\n\n` +
      `This will update flake.lock on GitHub.\n` +
      `You'll then need to Pull + Switch hosts.`,
  );

  if (!confirmed) return;

  try {
    const response = await fetch("/api/flake-updates/merge-pr", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrfToken,
      },
      body: JSON.stringify({ pr_number: pr.number }),
    });

    if (!response.ok) {
      const error = await response.text();
      alert(`Failed to merge PR: ${error}`);
      return;
    }

    const result = await response.json();
    showNotification("success", `PR #${pr.number} merged successfully`);

    // pendingPR will be cleared by State Sync delta
  } catch (err) {
    alert(`Failed to merge PR: ${err.message}`);
  }
}
```

---

## Post-Merge Flow

```
User clicks "Merge PR" button:
         ↓
1. Dashboard merges PR via GitHub API
         ↓
2. GitHub processes merge (~3-5 seconds)
         ↓
3. Dashboard broadcasts: "PR merged"
         ↓
4. All host Lock compartments: 🟡 (yellow)
   Tooltip: "flake.lock updated (3 commits behind)"
         ↓
5. User decides deployment strategy:
   Option A: One-by-one (click Pull on each host)
   Option B: Batch (select all → Pull All)
         ↓
6. After Pull: Lock 🟢, System 🟡
         ↓
7. User deploys: One-by-one or batch Switch
         ↓
8. After Switch: System 🟢, Tests 🟡
         ↓
9. Tests auto-run (if enabled): Tests 🟢 or 🔴
```

---

## Integration with Existing Features

### With P5200 (Lock Tracking)

- After merge: all hosts show Lock yellow (version mismatch)
- Tooltip shows: "Host: abc123, Latest: def456 (merged PR #42)"

### With P5400 (Tests)

- After switch post-merge: tests auto-run
- If tests fail on any host: offer rollback

### With P5600 (Rollback)

- If multiple hosts fail tests after merge: "Revert PR" button appears
- User can revert the merge, undo for everyone

### With P4300 (Automated Flake Updates)

- P4300 created the PR
- P5700 merges the PR (replaces old "Merge & Deploy")
- Deployment is still manual

---

## Testing Strategy

### Unit Tests

```go
func TestMergePR(t *testing.T) {
    // Mock GitHub client
    // Call MergePR
    // Verify API called with correct params
    // Verify pending PR cleared
}

func TestPRDetection(t *testing.T) {
    // Mock GitHub PRs
    // Filter to flake.lock updates
    // Verify correct PR selected
}
```

### Integration Tests

- [ ] PR detected and stored
- [ ] Merge button appears in UI
- [ ] Merge succeeds via API
- [ ] Lock compartments update to yellow
- [ ] State Sync broadcasts merge event

### Manual Testing

1. GitHub Action creates PR → Button appears
2. Click "Merge PR" → Confirmation dialog
3. Confirm → PR merges, button disappears
4. All hosts: Lock yellow
5. Pull one host → Lock green, System yellow
6. Switch → System green, Tests yellow/green/red

---

## Configuration

```yaml
# nixfleet.yaml
github:
  repo: "markus-barta/nixcfg"
  token: "${NIXFLEET_GITHUB_TOKEN}" # Required for merge
  pr_check_interval: 3600 # seconds (1 hour)
```

---

## Security Considerations

- Requires GitHub token with `repo` scope (write access)
- Token stored as environment variable, not in code
- Audit log records who merged PR
- Only one PR can be merged at a time (prevent conflicts)

---

## Out of Scope

- Automatic merge (always requires user confirmation) (future: P5701)
- PR review comments from dashboard (future: P5702)
- PR diff viewer in dashboard (future: P5703)
- Scheduled merge (e.g., "merge at 2am") (future: P5704)
- "Merge & Deploy All" combined operation (explicitly rejected - too aggressive)

---

## Related

- **P5200**: Lock Compartment - Version-Based Tracking (shows outdated after merge)
- **P5600**: Rollback Operations (revert PR if merge breaks fleet)
- **P4300**: Automated Flake Updates (creates the PRs that this merges)
- **P4310**: Flake Update Rollback (original spec, now split)
