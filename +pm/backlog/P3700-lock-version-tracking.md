# P3700 - Lock Compartment: Version-Based Tracking

**Created**: 2025-12-28  
**Priority**: P3700 (🔴 Critical Path - Sprint 1)  
**Status**: Backlog  
**Effort**: 3-4 hours  
**Depends on**: None

---

## User Story

**As a** fleet administrator  
**I want** the Lock compartment to show if my flake.lock is outdated compared to the latest version  
**So that** I know exactly which hosts need updating, without time-based heuristics

---

## Problem

Current Lock compartment is time-based:

- Shows green if `flake.lock` updated within 7 days
- Shows yellow if older than 7 days
- **Doesn't know if there's a newer version available**
- **Can show green even when updates are pending**

### Example of Current Broken Behavior

```
PR #42 merged on GitHub (new flake.lock available)

┌─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │
│   🟢    │   🟡    │   🟢    │   🟢    │  <- Lock shows GREEN (wrong!)
└─────────┴─────────┴─────────┴─────────┘

User thinks: "Everything is up to date"
Reality: flake.lock is outdated, host needs Pull
```

---

## Solution

**Version-based tracking** instead of time-based:

1. **Agent reports**: SHA256 hash of `flake.lock` content
2. **Dashboard fetches**: Latest `flake.lock` hash from nixcfg GitHub
3. **Comparison**: Exact match = green, different = yellow

### After Fix

```
PR #42 merged on GitHub (new flake.lock available)

┌─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │
│   🟢    │   🟡    │   🟡    │   🟢    │  <- Lock shows YELLOW (correct!)
└─────────┴─────────┴─────────┴─────────┘

Tooltip: "flake.lock outdated (3 commits behind)"
```

---

## Acceptance Criteria

### Agent Side

- [ ] Agent computes SHA256 hash of `flake.lock` content
- [ ] Agent sends `lock_hash` in heartbeat payload
- [ ] Hash computation is cheap (<1ms)
- [ ] Works on both NixOS and macOS

### Dashboard Side

- [ ] Dashboard fetches latest `flake.lock` from nixcfg repo (GitHub API or Pages)
- [ ] Cache the latest hash (5-second TTL)
- [ ] Compare `host.lock_hash` vs `latest_lock_hash`
- [ ] Lock compartment: green if match, yellow if different

### UI/UX

- [ ] Lock tooltip shows: "flake.lock outdated by X commits"
- [ ] Lock tooltip shows: "Latest: abc123, Host: def456"
- [ ] When PR pending: tooltip shows "Merge PR #42 to update"
- [ ] Lock never shows green when updates available

---

## Technical Design

### Agent Changes

```go
// internal/agent/heartbeat.go
func (a *Agent) sendHeartbeat() {
    // ... existing code ...

    lockHash := a.computeLockHash()

    payload := protocol.HeartbeatPayload{
        // ... existing fields ...
        LockHash: lockHash,
    }
}

// internal/agent/status.go
func (a *Agent) computeLockHash() string {
    lockPath := filepath.Join(a.cfg.RepoDir, "flake.lock")
    data, err := os.ReadFile(lockPath)
    if err != nil {
        return ""  // Empty if can't read
    }
    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:])
}
```

### Dashboard Changes

```go
// internal/dashboard/version_fetcher.go
type LockVersion struct {
    Hash      string
    Commit    string
    UpdatedAt time.Time
}

func (vf *VersionFetcher) GetLatestLockVersion() *LockVersion {
    // Fetch flake.lock from GitHub API
    // https://api.github.com/repos/{owner}/{repo}/contents/flake.lock
    // Compute SHA256 of content
    // Return hash + commit info
}

func (vf *VersionFetcher) GetLockStatus(hostHash string) (status, message string) {
    latest := vf.GetLatestLockVersion()
    if latest == nil {
        return "unknown", "Cannot fetch latest flake.lock"
    }

    if hostHash == latest.Hash {
        return "ok", "flake.lock is current"
    }

    // Count commits between host and latest
    commits := vf.countCommitsBetween(hostHash, latest.Hash)
    return "outdated", fmt.Sprintf("flake.lock outdated by %d commits", commits)
}
```

### Database Schema

```sql
-- Add lock_hash to hosts table
ALTER TABLE hosts ADD COLUMN lock_hash TEXT;
```

---

## Testing Strategy

### Unit Tests

```go
func TestComputeLockHash(t *testing.T) {
    // Create test flake.lock with known content
    // Verify SHA256 matches expected
}

func TestLockStatusComparison(t *testing.T) {
    // Host hash == latest → "ok"
    // Host hash != latest → "outdated"
}
```

### Integration Tests

- [ ] Agent sends lock_hash in heartbeat
- [ ] Dashboard receives and stores lock_hash
- [ ] Dashboard fetches latest from GitHub
- [ ] Lock compartment updates correctly

### Manual Testing

1. Deploy with current flake.lock → Lock shows green
2. Merge PR with new flake.lock → Lock shows yellow
3. Pull on host → Lock shows green again

---

## Migration Notes

- Old hosts without `lock_hash` field show "unknown" (gray)
- After agent update, all hosts report hash within 5 minutes (heartbeat interval)
- No breaking changes to protocol

---

## Out of Scope

- Showing diff of flake.lock changes (future: P5201)
- Automatic notifications when new lock available (future: P5202)
- Per-input version tracking (e.g., "nixpkgs updated, home-manager same") (future: P5203)

---

## Related

- **P5300**: System Compartment - Inference-Based Status
- **P5400**: Tests Compartment - Fifth Compartment
- **P5700**: Merge PR Workflow
- **P4300**: Automated Flake Updates (completed, needs update)
