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
┌─────────────────────────────────────────────────────────────┐
│  BEFORE (manual)                                            │
│  ───────────────                                            │
│  GitHub Action → PR → You review (???) → Merge → Deploy     │
└─────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  AFTER (NixFleet automated)                                 │
│  ──────────────────────────                                 │
│  GitHub Action → PR → NixFleet detects → Auto/Click merge   │
│  → Pull all hosts → Switch all hosts → Rollback on failure  │
└─────────────────────────────────────────────────────────────┘
```

---

## Features

### 1. PR Detection

NixFleet dashboard checks GitHub API for open PRs:

- Repo: configured via `NIXFLEET_GITHUB_REPO` (e.g., `markus-barta/nixcfg`)
- Filter: PRs with label `automated` or title containing "flake.lock"
- Polling: Every 5 minutes (or on dashboard load)

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
5. Notifies admin of success/failure

### 5. Rollback on Failure

If any host fails during switch:

- Stop deployment to remaining hosts
- Offer rollback options:
  - Revert the merge commit
  - Create revert PR
  - Or just alert admin

---

## Configuration

### Global Settings (Dashboard)

```yaml
flakeUpdates:
  enabled: true
  githubRepo: "markus-barta/nixcfg"
  githubToken: "${NIXFLEET_GITHUB_TOKEN}" # PAT with repo access

  automation:
    enabled: false # Full auto-merge + deploy
    delayMinutes: 60 # Wait for CI before merge
    deployStrategy: "all" # all | online-only | none

  rollback:
    enabled: true
    strategy: "revert-commit" # revert-commit | revert-pr | alert-only
```

### Per-Host Settings

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

- [ ] GitHub API integration to detect open PRs
- [ ] Lock compartment shows "PR pending" when update available
- [ ] Click → "Merge & Deploy" button
- [ ] Merge PR via API
- [ ] Sequential deploy: pull → switch on all online hosts
- [ ] Success/failure notification

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
