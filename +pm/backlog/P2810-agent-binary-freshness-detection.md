# P2810 - Agent Binary Freshness Detection

**Created**: 2025-12-21
**Priority**: P2810 (High - Critical Gap)
**Status**: Backlog
**Effort**: Medium (2-3 days)
**Depends on**: None

---

## Problem

The Lock/System compartments don't detect when the **agent binary itself** is outdated. This leads to:

1. User triggers `switch` via dashboard
2. Switch completes successfully (exit 0)
3. Lock compartment shows GREEN (repo is current)
4. But the **running agent** is still the OLD binary!
5. New features (like P2800 operation_progress) don't work
6. User is confused - "I switched but nothing changed"

### Root Cause

The agent binary is a Nix derivation built from the `nixfleet` flake input. When:

- `nixcfg/flake.lock` is updated to point to new nixfleet commit
- `home-manager switch` or `nixos-rebuild switch` runs
- Nix may **substitute** (download) the binary from cache instead of building
- If cache has OLD binary, user gets old code even though switch "succeeded"

This has happened **>10 times** already!

---

## Solution

### Option A: Agent Self-Check (Recommended)

The agent should know its own source commit and report it:

```go
// Embedded at build time
var (
    Version      = "2.1.0"
    SourceCommit = "unknown"  // Set by ldflags: -X main.SourceCommit=$(git rev-parse HEAD)
)

// In heartbeat payload
type HeartbeatPayload struct {
    // ... existing fields ...
    SourceCommit string `json:"source_commit"` // Git commit agent was built from
}
```

Dashboard can then compare:

- `agent.SourceCommit` vs `nixfleet` flake input commit
- If different → Agent Outdated indicator (separate from Lock/System)

### Option B: Store Path Tracking

Track the expected Nix store path for the agent:

1. Dashboard knows expected `nixfleet-agent` derivation hash
2. Agent reports its own store path
3. Compare and warn if mismatched

### Option C: Build Timestamp

Less reliable but simpler:

- Agent reports its build timestamp
- Dashboard compares to expected (based on flake.lock update time)

---

## Implementation Plan

### Phase 1: Agent Reports Source Commit

1. Update `packages/nixfleet-agent-v2.nix`:

   ```nix
   ldflags = [
     "-s" "-w"
     "-X main.Version=${version}"
     "-X main.SourceCommit=${src.rev or "unknown"}"  # NEW
   ];
   ```

2. Update agent to include `SourceCommit` in heartbeat

3. Dashboard stores and displays source commit

### Phase 2: Dashboard Comparison

1. Dashboard knows its own `NIXFLEET_SOURCE_COMMIT` (from build)
2. Compare agent's reported commit to expected
3. New compartment or badge: "Agent Binary Outdated"

### Phase 3: Force Rebuild Detection

When user triggers `switch`:

1. Before: capture agent store path
2. After: check if store path changed
3. If unchanged but nixfleet input updated → warn "Agent binary not rebuilt"

---

## Acceptance Criteria

- [ ] Agent reports its source commit in heartbeat
- [ ] Dashboard displays agent source commit (tooltip or details)
- [ ] Dashboard warns when agent commit doesn't match expected
- [ ] Post-switch validation detects unchanged agent binary
- [ ] Clear user guidance: "Run `nix-collect-garbage -d` and switch again"

---

## Related Issues

- **Nix Binary Cache**: Sometimes `cache.nixos.org` serves stale binaries
- **Workaround**: `nix build --no-substitute` or garbage collect

---

## Notes

This issue has occurred >10 times during development. It's a silent failure that wastes significant debugging time. Must fix!
