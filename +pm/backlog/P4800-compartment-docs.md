# P4800 - Compartment State Documentation

**Created**: 2025-12-28  
**Priority**: P4800 (🟡 High Priority - Sprint 2)  
**Status**: Backlog  
**Effort**: 1-2 hours  
**Depends on**: P3700, P3800, P3900

---

## User Story

**As a** NixFleet user  
**I want** comprehensive documentation of all compartment states  
**So that** I understand what each color means and what action to take

---

## Problem

Current `docs/FLAKE-UPDATES.md`:

- Only documents 3 compartments (missing Agent and Tests)
- Time-based Lock description (outdated)
- No comprehensive state matrix
- Missing troubleshooting guide

---

## Solution

**Update `docs/FLAKE-UPDATES.md` with:**

1. **Five Compartments Overview** - What each one means
2. **State Matrix** - ~20-25 meaningful combinations
3. **Common Scenarios** - Normal workflow, failures, edge cases
4. **Troubleshooting Guide** - When things go wrong
5. **Visual Examples** - ASCII diagrams of typical states

---

## Acceptance Criteria

### Documentation Structure

- [ ] Five compartments clearly explained (Agent, Git, Lock, System, Tests)
- [ ] State matrix with ~20-25 common combinations
- [ ] Each state shows: colors, meaning, action needed
- [ ] Common scenarios documented with flow diagrams
- [ ] Troubleshooting section for each compartment

### Content Quality

- [ ] Clear, concise explanations (no jargon)
- [ ] Visual examples using ASCII tables/diagrams
- [ ] Real-world scenarios (not just theory)
- [ ] Links to related backlog items for deeper dives

---

## Content Outline

### 1. The Five Compartments

```markdown
## The Five Compartments Explained

NixFleet uses a **five-stage pipeline** to track your fleet's state:

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
└─────────┴─────────┴─────────┴─────────┴─────────┘

Each compartment answers a specific question:

| #   | Compartment | Question                                  | Managed By              |
| --- | ----------- | ----------------------------------------- | ----------------------- |
| 1   | **Agent**   | Is my nixfleet-agent binary current?      | Dashboard version check |
| 2   | **Git**     | Is my local repo up to date with GitHub?  | GitHub API comparison   |
| 3   | **Lock**    | Are my dependencies (flake.lock) current? | Content hash comparison |
| 4   | **System**  | Is my running system current?             | Inferred from commands  |
| 5   | **Tests**   | Is my system actually working?            | Test execution results  |

### Compartment States

| Color     | Meaning               | Action              |
| --------- | --------------------- | ------------------- |
| 🟢 Green  | Current / Passed      | No action needed    |
| 🟡 Yellow | Outdated / Not run    | Update needed       |
| 🔴 Red    | Failed / Error        | Fix required        |
| 🔵 Blue   | Working / In progress | Wait for completion |
| ⚪ Gray   | Unknown / Disabled    | Check configuration |
```

### 2. Common State Combinations

```markdown
## Common Scenarios

### Scenario 1: Everything Current ✓

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: All up to date, system working correctly
**Action**: None - enjoy your day ☕

### Scenario 2: Config Updated on GitHub

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟡 │ 🟢 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: New config available on GitHub (you or someone else pushed)
**Action**: Click Git → Pull

### Scenario 3: After Pull

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟢 │ 🟡 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Pulled new config, but not yet applied
**Action**: Click System → Switch

### Scenario 4: After Switch

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🟡 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Switch succeeded, tests not run yet
**Action**: Click Tests → Run Tests (or wait for auto-run)

### Scenario 5: Lock Outdated (PR Pending)

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟡 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: New flake.lock available (PR merged on GitHub)
**Action**: Click Git → Pull (to get new flake.lock)

### Scenario 6: Agent Outdated

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🔴 │ 🟢 │ 🟢 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Agent binary is outdated (dashboard was updated)
**Action**: Pull + Switch (this updates the agent binary)

### Scenario 7: Switch Failed ⚠️

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟢 │ 🔴 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Switch command failed (config error)
**Action**: Click System → View logs, fix config error

### Scenario 8: Tests Failed ⚠️

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🔴 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Switch succeeded, but system is broken
**Action**: Host → ⋮ → Rollback System

### Scenario 9: During Update (In Progress)

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🟢 │ 🔵 │ 🟢 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Pull command is running
**Action**: Wait for completion

### Scenario 10: Everything Broken (Oh No!)

┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent │ Git │ Lock │ System │ Tests │
│ 🔴 │ 🟡 │ 🟡 │ 🔴 │ 🔴 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

**Meaning**: Multiple issues, system in bad state
**Action**: Rollback System, then investigate logs
```

### 3. Troubleshooting Guide

```markdown
## Troubleshooting

### Agent Compartment

#### Problem: Agent shows red (outdated)

**Cause**: Dashboard was updated, agent binary is old
**Fix**: Pull + Switch (this updates agent binary)
**Note**: On macOS, may need "Restart Agent" after switch

#### Problem: Agent shows gray (unknown)

**Cause**: Host offline or never connected
**Fix**: Check host connectivity, restart agent

---

### Git Compartment

#### Problem: Git stuck on yellow after pull

**Cause**: Pull failed (local changes, conflicts)
**Fix**:

1. SSH to host
2. Run: `cd ~/Code/nixcfg && git status`
3. Resolve conflicts manually

#### Problem: Git shows outdated but I didn't push anything

**Cause**: Someone else pushed, or automated commit
**Fix**: Check GitHub commit history, then pull

---

### Lock Compartment

#### Problem: Lock shows yellow but no PR visible

**Cause**: PR was merged directly (not via dashboard)
**Fix**: Pull to get new flake.lock

#### Problem: Lock shows green but packages are old

**Cause**: flake.lock hasn't been updated in weeks
**Fix**: Wait for weekly GitHub Action to create update PR

---

### System Compartment

#### Problem: System stuck on yellow after switch

**Cause**: Switch command failed or timed out
**Fix**: Check logs, retry switch

#### Problem: System shows red (switch failed)

**Cause**: Config error (syntax, missing option, etc.)
**Fix**:

1. Click System → View logs
2. Find error message
3. Fix in nixcfg
4. Push, Pull, Switch again

---

### Tests Compartment

#### Problem: Tests always gray

**Cause**: Tests disabled for this host
**Fix**: Enable tests in host configuration

#### Problem: Tests fail after successful switch

**Cause**: Config broke something (GPU, X11, networking)
**Fix**: Rollback to previous generation
```

### 4. The Update Flow Visualized

```markdown
## Complete Update Flow

### Normal Weekly Update
```

Week 1: GitHub Action runs
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🟢 │
└─────────┴─────────┴─────────┴─────────┴─────────┘

GitHub Action creates PR with new flake.lock
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟢 │ 🟡 │ 🟢 │ 🟢 │ <- Lock yellow (PR pending)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Merge PR" in dashboard
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟡 │ 🟡 │ 🟢 │ 🟢 │ <- Git yellow (PR merged)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Pull" on host
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟢 │ 🟢 │ 🟡 │ 🟢 │ <- System yellow (need switch)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Switch" on host
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🟡 │ <- Tests yellow (need run)
└─────────┴─────────┴─────────┴─────────┴─────────┘

Tests auto-run and pass
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ 🟢 │ 🟢 │ 🟢 │ 🟢 │ 🟢 │ <- All good! ✓
└─────────┴─────────┴─────────┴─────────┴─────────┘

```

```

---

## Files to Update

### Primary

- [ ] `docs/FLAKE-UPDATES.md` - Complete rewrite with five compartments

### Secondary

- [ ] `docs/UPDATE-ARCHITECTURE.md` - Update compartment descriptions
- [ ] `README.md` - Update compartment screenshots (if any)
- [ ] `+pm/PRD.md` - Update compartment descriptions in PRD

---

## Testing Strategy

### Documentation Review

- [ ] Technical accuracy verified
- [ ] All scenarios tested manually
- [ ] Links work correctly
- [ ] ASCII diagrams render properly

### User Testing

- [ ] Show to new user, get feedback
- [ ] Verify they can understand states without help
- [ ] Check if troubleshooting guide is helpful

---

## Out of Scope

- Interactive documentation (future: P5801)
- Video tutorials (future: P5802)
- In-app help tooltips (future: P5803)

---

## Related

- **P5200**: Lock Compartment - Version-Based Tracking
- **P5300**: System Compartment - Inference-Based Status
- **P5400**: Tests Compartment - Fifth Compartment
- **P5500**: Generation Tracking and Visibility
