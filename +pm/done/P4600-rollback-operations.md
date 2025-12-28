# P4600 - Rollback Operations

**Created**: 2025-12-28  
**Completed**: 2025-12-28  
**Priority**: P4600 (🟡 High Priority - Sprint 2)  
**Status**: ✅ Done  
**Effort**: 3-4 hours  
**Depends on**: P4500

---

## User Story

**As a** fleet administrator  
**I want** to rollback to a previous generation when a switch breaks my system  
**So that** I can quickly recover from bad updates without manual intervention

---

## Problem

Currently:

- No UI for rollback operations
- When switch succeeds but system is broken, no easy recovery
- When PR breaks multiple hosts, no fleet-wide revert option
- User must SSH to host and manually run `nixos-rebuild --rollback`

### Scenarios Where Rollback is Needed

#### Scenario 1: Switch Succeeds, System Broken

```
Switch exits 0, but:
- X11 won't start
- Networking is down
- GPU drivers broken
- Tests fail (P5400)

System compartment: 🟢 (switch succeeded)
Tests compartment:  🔴 (system actually broken)

User needs: Quick rollback to last working generation
```

#### Scenario 2: PR Breaks Multiple Hosts

```
Merge PR → Deploy to 5 hosts
- gpc0: Switch OK, Tests OK ✓
- imac0: Switch OK, Tests FAIL ✗
- hsb0: Switch OK, Tests FAIL ✗
- csb0: Not attempted

User needs: Revert PR, rollback all hosts
```

---

## Solution

**Two types of rollback:**

### Type 1: Per-Host Generation Rollback (Ellipsis Menu)

```
Host → ⋮ → Rollback System
```

- Rolls back ONE host to previous generation
- Shows target generation before confirming
- Use when: "This host has a problem"

### Type 2: Fleet-Wide PR Revert (After Merge PR Failure)

```
After failed Merge PR:
[Revert PR] button appears
```

- Reverts the merge commit on GitHub
- Pushes revert to nixcfg
- All hosts pull the revert and switch back
- Use when: "This PR broke everyone"

---

## Acceptance Criteria

### Per-Host Rollback

- [ ] "Rollback System" option in host ellipsis menu
- [ ] Shows dialog with available generations
- [ ] Highlights: current gen, target gen, "last known working"
- [ ] Confirmation required before rollback
- [ ] Rollback command dispatched via Op Engine
- [ ] Success: System compartment updates, Tests re-run (if enabled)

### Fleet-Wide PR Revert

- [ ] "Revert PR" button appears after failed Merge PR
- [ ] Shows which hosts succeeded/failed
- [ ] Creates revert commit on nixcfg
- [ ] Pushes to GitHub
- [ ] Updates all host Lock compartments (yellow)
- [ ] User then manually pulls/switches hosts

### UI/UX

- [ ] Rollback dialog shows: gen number, commit hash, timestamp, status
- [ ] Target generation marked as "Last known working" (if tests passed)
- [ ] After rollback: notification "Rolled back to Gen 47"
- [ ] Rollback appears in audit log

---

## Technical Design

### Per-Host Rollback

#### UI Flow

```
User clicks: Host → ⋮ → Rollback System
         ↓
┌──────────────────────────────────────────────────┐
│ Rollback System - gpc0                           │
├──────────────────────────────────────────────────┤
│ Current Generation:                              │
│   Gen 48 (abc123)                                │
│   Dec 28, 14:32                                  │
│   Tests: FAILED (3/8)                            │
│                                                  │
│ Roll back to:                                    │
│ ● Gen 47 (def456)     ← Last known working       │
│   Dec 27, 18:15                                  │
│   Tests: PASSED (8/8) ✓                          │
│                                                  │
│ ○ Gen 46 (ghi789)                                │
│   Dec 26, 12:05                                  │
│   Tests: PASSED (8/8) ✓                          │
│                                                  │
│     [Cancel]  [Rollback to Gen 47]               │
└──────────────────────────────────────────────────┘
         ↓ User clicks "Rollback to Gen 47"
         ↓
1. Dispatch "rollback" op to gpc0
2. Agent runs: nixos-rebuild --rollback switch
3. System compartment: 🟡 (rolling back)
4. After success: System 🟢, Tests 🟡 (need re-run)
5. Auto-run tests (if enabled)
```

#### Agent Command

```go
// internal/agent/commands.go
case "rollback":
    a.sendOutput("Rolling back to previous generation...", "stdout")
    cmd, err = a.buildRollbackCommand()

func (a *Agent) buildRollbackCommand() (*exec.Cmd, error) {
    var args []string

    if runtime.GOOS == "darwin" {
        // macOS: Activate previous home-manager generation
        prevGen := a.findPreviousGeneration()
        args = []string{prevGen + "/activate"}
    } else {
        // NixOS: Use nixos-rebuild --rollback
        args = []string{"sudo", "nixos-rebuild", "--rollback", "switch"}
    }

    return exec.CommandContext(a.ctx, args[0], args[1:]...), nil
}
```

### Fleet-Wide PR Revert

#### UI Flow

```
Merge PR flow fails:
┌─────────────────────────────────────────────────┐
│ ⚠️ Deployment Failed                             │
│                                                 │
│ ✓ gpc0    - Switch OK, Tests OK                │
│ ✗ imac0   - Switch OK, Tests FAIL (X11 broken) │
│ ✗ hsb0    - Switch OK, Tests FAIL (GPU error)  │
│ ⬜ csb0    - Not attempted                      │
│                                                 │
│ [Revert PR]  [View Logs]  [Continue Anyway]     │
└─────────────────────────────────────────────────┘
         ↓ User clicks "Revert PR"
         ↓
┌─────────────────────────────────────────────────┐
│ Revert PR #42?                                  │
│                                                 │
│ This will:                                      │
│ 1. Create revert commit on nixcfg               │
│ 2. Push to GitHub                               │
│ 3. Mark all hosts Lock outdated (yellow)        │
│                                                 │
│ You will need to:                               │
│ - Pull on all hosts (gets revert)               │
│ - Switch on all hosts (applies revert)          │
│                                                 │
│     [Cancel]  [Revert PR #42]                   │
└─────────────────────────────────────────────────┘
         ↓ User confirms
         ↓
1. GitHub API: Create revert commit
2. Push to nixcfg/main
3. All hosts: Lock → 🟡 (revert available)
4. User manually pulls/switches (or uses batch)
```

#### Dashboard Implementation

```go
// internal/dashboard/flake_updates.go
func (s *FlakeUpdateService) RevertPR(ctx context.Context, prNumber int, mergeCommitSHA string) error {
    owner, repo := s.cfg.GitHubOwnerRepo()

    // 1. Create revert commit via GitHub API
    revertMsg := fmt.Sprintf("Revert PR #%d (auto-rollback after failed deployment)", prNumber)
    revertCommit, err := s.client.CreateRevertCommit(ctx, owner, repo, mergeCommitSHA, revertMsg)
    if err != nil {
        return fmt.Errorf("failed to create revert: %w", err)
    }

    // 2. Push revert to main
    err = s.client.UpdateRef(ctx, owner, repo, "heads/main", revertCommit.SHA)
    if err != nil {
        return fmt.Errorf("failed to push revert: %w", err)
    }

    // 3. Broadcast to all hosts that Lock is now outdated
    s.hub.BroadcastToAll("pr_reverted", map[string]any{
        "pr_number": prNumber,
        "message":   "PR reverted - Pull to get reverted flake.lock",
    })

    s.log.Info().
        Int("pr", prNumber).
        Str("revert_sha", revertCommit.SHA).
        Msg("PR reverted successfully")

    return nil
}
```

---

## Op Engine Integration

```go
// internal/ops/registry.go
func opRollback() *Op {
    return &Op{
        ID:          "rollback",
        Description: "Rollback to previous generation",
        Validator: func(ctx context.Context, h Host) error {
            // Can always rollback (NixOS keeps previous gen)
            return nil
        },
        PostCheck: func(ctx context.Context, h Host, exitCode int) error {
            if exitCode != 0 {
                return fmt.Errorf("rollback failed")
            }
            // After successful rollback, system should be OK
            // But tests need re-run
            return nil
        },
        CanRunOnDashboard: false,
        CanRunOnAgent:     true,
    }
}
```

---

## Database Schema

```sql
-- Track rollback events
CREATE TABLE IF NOT EXISTS rollbacks (
    id              TEXT PRIMARY KEY,
    host_id         TEXT NOT NULL,
    from_gen        INTEGER NOT NULL,
    to_gen          INTEGER NOT NULL,
    reason          TEXT,
    initiated_by    TEXT,
    rolled_back_at  DATETIME NOT NULL,
    success         INTEGER DEFAULT 1,
    FOREIGN KEY (host_id) REFERENCES hosts(id)
);
CREATE INDEX IF NOT EXISTS idx_rollbacks_host ON rollbacks(host_id, rolled_back_at DESC);
```

---

## Testing Strategy

### Unit Tests

```go
func TestRollbackCommand(t *testing.T) {
    // NixOS: should use sudo nixos-rebuild --rollback
    // macOS: should activate previous home-manager gen
}

func TestRevertPRFlow(t *testing.T) {
    // Create revert commit
    // Push to GitHub
    // Verify hosts notified
}
```

### Integration Tests

- [ ] Rollback dispatched correctly
- [ ] Agent executes rollback command
- [ ] System compartment updates after rollback
- [ ] Tests re-run after rollback (if enabled)
- [ ] PR revert creates correct commit

### Manual Testing

1. Switch to new config → Tests fail
2. Rollback to previous gen
3. Verify system restored
4. Verify tests pass on old gen

---

## Safety Considerations

### Rollback Limits

- NixOS keeps ~10-20 previous generations (configurable)
- If generation is garbage-collected, rollback will fail
- Show warning if target generation is old (>30 days)

### Confirmation Required

- Always require explicit confirmation for rollback
- Show what you're rolling back FROM and TO
- Highlight "last known working" generation

### Audit Trail

- All rollbacks logged to audit_log table
- Include: who initiated, reason, from/to generations
- Visible in system logs

---

## Out of Scope

- Automatic rollback on test failure (future: P5601)
- Rollback preview (show what will change) (future: P5602)
- Selective rollback (only rollback specific services) (future: P5603)
- Cross-host coordinated rollback (future: P5604)

---

## Related

- **P5400**: Tests Compartment (triggers rollback prompts)
- **P5500**: Generation Tracking (provides rollback targets)
- **P5700**: Merge PR Workflow (uses fleet-wide revert)
- **P4310**: Flake Update Rollback (original spec, now refined)
