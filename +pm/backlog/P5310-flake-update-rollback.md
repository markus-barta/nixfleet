# P5310 - Flake Update Rollback

**Created**: 2025-12-17  
**Priority**: P5310 (Medium)  
**Status**: Backlog  
**Depends on**: P5300 (Automated Flake Lock Updates)

---

## User Story

**As a** fleet administrator  
**I want** NixFleet to automatically rollback failed flake updates  
**So that** a bad update doesn't leave my fleet in an inconsistent state

---

## Overview

When a flake update deployment fails on one or more hosts, provide options to:

1. Stop deployment to remaining hosts (already implemented in P5300)
2. Revert the merge commit on GitHub
3. Trigger rollback on hosts that already deployed

---

## Acceptance Criteria

### Automatic Rollback

- [ ] When switch fails on any host, stop deployment to remaining hosts ✅ (done in P5300)
- [ ] Option to revert the merge commit via GitHub API
- [ ] Option to trigger `rollback` command on hosts that successfully switched
- [ ] Dashboard shows clear status: which hosts succeeded, which failed, which reverted

### Manual Rollback

- [ ] "Rollback" button appears after failed deployment
- [ ] User can choose: revert commit only, or revert + rollback hosts
- [ ] Confirmation dialog with list of affected hosts

### Rollback Command

- [ ] Implement `rollback` command in agent (use `nixos-rebuild --rollback` or boot prev generation)
- [ ] Track rollback success/failure per host
- [ ] WebSocket broadcast of rollback progress

---

## Technical Notes

### GitHub Revert

```go
// Create revert commit
POST /repos/{owner}/{repo}/git/commits
{
  "message": "Revert flake.lock update (auto-rollback)",
  "tree": "<original-tree-sha>",
  "parents": ["<merge-commit-sha>"]
}

// Or create revert PR for safer approach
POST /repos/{owner}/{repo}/pulls
{
  "title": "Revert: <original-pr-title>",
  "head": "revert-<sha>",
  "base": "main"
}
```

### NixOS Rollback

```bash
# Roll back to previous generation
nixos-rebuild --rollback switch

# Or boot to previous generation (requires reboot)
/nix/var/nix/profiles/system-<N-1>-link/bin/switch-to-configuration switch
```

### Home Manager Rollback

```bash
# List generations
home-manager generations

# Activate previous generation
/nix/var/nix/profiles/per-user/$USER/home-manager-<N-1>-link/activate
```

---

## UI Design

After failed deployment:

```
┌────────────────────────────────────────────────────────────────┐
│ ⚠️ Deployment Failed                                           │
│                                                                │
│ Switch failed on: hsb0, csb0                                   │
│ Successfully deployed: gpc0, hsb1                              │
│ Not attempted: csb1                                            │
│                                                                │
│ [Revert Merge]  [Rollback Deployed Hosts]  [Dismiss]           │
└────────────────────────────────────────────────────────────────┘
```

---

## Security Considerations

- Rollback requires same GitHub token permissions as merge
- Agent rollback command should be protected (only dashboard can trigger)
- Audit log: record who triggered rollback, when, what was reverted

---

## Related

- **P5300**: Automated Flake Lock Updates (prerequisite)
- **P5301**: Flake Updates E2E Test Suite
