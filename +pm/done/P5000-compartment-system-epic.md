# P5000 - Compartment System Epic

**Created**: 2025-12-28  
**Completed**: 2025-12-28  
**Priority**: P5000 (Epic - High Priority)  
**Status**: ✅ Done  
**Total Effort**: ~18-24 hours

---

## Overview

Complete redesign of the compartment status system from 4 to 5 compartments, with version-based tracking, inference-based status, and comprehensive testing integration.

---

## Problem Statement

Current compartment system has critical flaws:

1. **Lock compartment**: Time-based (broken) - shows green even when updates pending
2. **System compartment**: Always gray (too expensive to check)
3. **No test visibility**: Can't tell if system is actually working
4. **No generation tracking**: Can't see which hosts are on different versions
5. **Aggressive deployment**: "Merge & Deploy" too risky, no per-host control

---

## Solution

Five-compartment pipeline with accurate, lightweight status tracking:

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │ Tests   │
│ (Tool)  │ (Config)│ (Deps)  │ (Deploy)│ (Verify)│
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

---

## Epic Breakdown

### Phase 1: Lock Compartment Fix

**Backlog Item**: [P3700 - Lock Version Tracking](./P3700-lock-version-tracking.md)  
**Effort**: 3-4 hours  
**Priority**: Must Have

**Changes:**

- Agent reports SHA256 hash of flake.lock content
- Dashboard compares with latest from GitHub
- Yellow when outdated (version mismatch), not time-based
- Tooltip shows: "Host: abc123, Latest: def456 (3 commits behind)"

**Value**: Accurate lock status, no false greens

---

### Phase 2: System Compartment Fix

**Backlog Item**: [P3800 - System Inference](./P3800-system-inference.md)  
**Effort**: 2-3 hours  
**Priority**: Must Have

**Changes:**

- Infer status from command results (no expensive checks)
- Pull success → System yellow (need switch)
- Switch success → System green (current)
- Switch fail → System red (broken)
- Lock outdated → System yellow (must be outdated)

**Value**: Always-accurate system status, no gray states

---

### Phase 3: Tests Compartment

**Backlog Item**: [P3900 - Tests Compartment](./P3900-tests-compartment.md)  
**Effort**: 4-5 hours  
**Priority**: Must Have

**Changes:**

- Fifth compartment for test results
- Auto-run tests after switch (configurable)
- Green (pass), Yellow (not run), Red (fail), Blue (running)
- Failed tests prompt rollback

**Value**: Separation of "deployment succeeded" vs "system working"

---

### Phase 4: Generation Tracking

**Backlog Item**: [P4500 - Generation Tracking](./P4500-generation-tracking.md)  
**Effort**: 2-3 hours  
**Priority**: Should Have

**Changes:**

- Add "Gen" column to host table
- Show first 6-7 chars of commit hash
- Highlight generation drift (hosts on different versions)
- Generation history for rollback decisions

**Value**: Visibility into fleet version consistency

---

### Phase 5: Rollback Operations

**Backlog Item**: [P4600 - Rollback Operations](./P4600-rollback-operations.md)  
**Effort**: 3-4 hours  
**Priority**: Should Have

**Changes:**

- Per-host rollback (ellipsis menu)
- Fleet-wide PR revert (after failed merge)
- Show target generation before rollback
- Rollback appears in audit log

**Value**: Quick recovery from bad updates

---

### Phase 6: Merge PR Workflow

**Backlog Item**: [P4700 - Merge PR Workflow](./P4700-merge-pr-workflow.md)  
**Effort**: 2-3 hours  
**Priority**: Should Have

**Changes:**

- "Merge PR" button in dashboard (replaces "Merge & Deploy")
- ONLY merges PR, no automatic deployment
- All Lock compartments turn yellow after merge
- User deploys manually (one-by-one or batch)

**Value**: Safe PR merging with full deployment control

---

### Phase 7: Documentation

**Backlog Item**: [P4800 - Compartment Docs](./P4800-compartment-docs.md)  
**Effort**: 1-2 hours  
**Priority**: Should Have

**Changes:**

- Complete rewrite of FLAKE-UPDATES.md
- State matrix (~20-25 meaningful combinations)
- Troubleshooting guide for each compartment
- Visual examples and flow diagrams

**Value**: User understanding and onboarding

---

## Implementation Order

### Recommended Sequence

1. **P3700** (Lock) - Foundation for everything else
2. **P3800** (System) - Complements Lock, both needed for accurate state
3. **P4500** (Generation) - Needed before Rollback
4. **P3900** (Tests) - Can work independently, but benefits from 1-3
5. **P4600** (Rollback) - Depends on Generation tracking
6. **P4700** (Merge PR) - Depends on Lock tracking
7. **P4800** (Docs) - Last, after all features implemented

### Alternative: Parallel Tracks

**Track A** (Core Status):

- P3700 (Lock) → P3800 (System) → P3900 (Tests)

**Track B** (Operations):

- P4500 (Generation) → P4600 (Rollback) → P4700 (Merge PR)

**Track C** (Documentation):

- P4800 (Docs) - anytime after Track A completes

---

## Dependencies

```
P3700 (Lock)
  ├─→ P3800 (System) - infers from Lock status
  ├─→ P3900 (Tests) - runs after System updates
  └─→ P4700 (Merge PR) - updates Lock compartments

P4500 (Generation)
  └─→ P4600 (Rollback) - shows generation history

All → P4800 (Docs) - documents final system
```

---

## Success Metrics

### Before (Current State)

| Metric                | Current    | Problem                  |
| --------------------- | ---------- | ------------------------ |
| Lock accuracy         | ~60%       | Time-based, false greens |
| System visibility     | 10%        | Always gray              |
| Test visibility       | 0%         | No compartment           |
| Generation visibility | 0%         | Hidden in tooltips       |
| Rollback ease         | Manual SSH | No UI                    |
| Deployment safety     | Low        | Aggressive batch         |

### After (Target State)

| Metric                | Target    | Improvement           |
| --------------------- | --------- | --------------------- |
| Lock accuracy         | 100%      | Version-based         |
| System visibility     | 100%      | Inference-based       |
| Test visibility       | 100%      | Dedicated compartment |
| Generation visibility | 100%      | Table column          |
| Rollback ease         | One-click | UI-driven             |
| Deployment safety     | High      | Manual control        |

---

## Risks & Mitigations

### Risk 1: Lock hash computation overhead

**Impact**: Heartbeat latency  
**Mitigation**: Hash is cheap (~1ms), cache for 5 minutes

### Risk 2: System inference inaccuracy

**Impact**: Wrong status shown  
**Mitigation**: Conservative inference (prefer yellow over green), manual refresh available

### Risk 3: Test auto-run slows deployments

**Impact**: User waits for tests  
**Mitigation**: Tests run async, configurable per-host, can be disabled

### Risk 4: Generation tracking breaks on non-standard setups

**Impact**: Generation column shows wrong data  
**Mitigation**: Graceful degradation (show "unknown" if can't parse)

---

## Out of Scope

- Automatic rollback on test failure (future: P5601)
- Custom test definitions per-host (future: P5401)
- Scheduled merge (e.g., "merge at 2am") (future: P5704)
- Multi-user approval for merges (future: P5705)

---

## Related Specs

- **CORE-006**: Compartment Status System (new spec)
- **CORE-004**: State Sync Protocol (broadcasts compartment updates)
- **CORE-001**: Op Engine (executes operations that change compartments)

---

## Total Effort Estimate

| Phase              | Effort     | Priority    |
| ------------------ | ---------- | ----------- |
| P3700 - Lock       | 3-4h       | Must Have   |
| P3800 - System     | 2-3h       | Must Have   |
| P3900 - Tests      | 4-5h       | Must Have   |
| P4500 - Generation | 2-3h       | Should Have |
| P4600 - Rollback   | 3-4h       | Should Have |
| P4700 - Merge PR   | 2-3h       | Should Have |
| P4800 - Docs       | 1-2h       | Should Have |
| **Total**          | **18-24h** |             |

**Must Have**: 9-12 hours  
**Should Have**: 9-12 hours

---

## Next Steps

1. Review and refine each backlog item (P3700-P4800)
2. Update PRD to reference this epic
3. Create CORE-006 spec (Compartment Status System)
4. Begin implementation with P3700 (Lock tracking)
