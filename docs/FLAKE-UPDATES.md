# Understanding Flake Updates in NixFleet

## The Five Compartments Explained

NixFleet uses a **five-stage pipeline** to track your fleet's state:

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

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

---

## Common Scenarios

### Scenario 1: Everything Current ✓

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟢    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: All up to date, system working correctly  
**Action**: None - enjoy your day ☕

### Scenario 2: Config Updated on GitHub

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟡    │   🟢    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: New config available on GitHub (you or someone else pushed)  
**Action**: Click Git → Pull

### Scenario 3: After Pull

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟢    │   🟡    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Pulled new config, but not yet applied  
**Action**: Click System → Switch

### Scenario 4: After Switch

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟢    │   🟢    │   🟡    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Switch succeeded, tests not run yet  
**Action**: Click Tests → Run Tests (or wait for auto-run)

### Scenario 5: Lock Outdated (PR Merged)

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟡    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: New flake.lock available (PR merged on GitHub)  
**Action**: Click Git → Pull (to get new flake.lock)

### Scenario 6: Agent Outdated

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🔴    │   🟢    │   🟢    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Agent binary is outdated (dashboard was updated)  
**Action**: Pull + Switch (this updates the agent binary)

### Scenario 7: Switch Failed ⚠️

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟢    │   🔴    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Switch command failed (config error)  
**Action**: Click System → View logs, fix config error

### Scenario 8: Tests Failed ⚠️

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🟢    │   🟢    │   🟢    │   🔴    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Switch succeeded, but system is broken  
**Action**: Host → ⋮ → Rollback System

### Scenario 9: During Update (In Progress)

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🟢    │   🔵    │   🟢    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Pull command is running  
**Action**: Wait for completion

### Scenario 10: Everything Broken (Oh No!)

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  Agent  │   Git   │  Lock   │ System  │  Tests  │
│   🔴    │   🟡    │   🟡    │   🔴    │   🔴    │
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Meaning**: Multiple issues, system in bad state  
**Action**: Rollback System, then investigate logs

---

## Complete Update Flow

### Normal Weekly Update

```
Week 1: GitHub Action runs
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟢    │   🟢    │
└─────────┴─────────┴─────────┴─────────┴─────────┘

GitHub Action creates PR with new flake.lock
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟡    │   🟢    │   🟢    │  <- Lock yellow (PR pending)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Merge PR" in dashboard
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟡    │   🟡    │   🟢    │   🟢    │  <- Git yellow (PR merged)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Pull" on host
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟡    │   🟢    │  <- System yellow (need switch)
└─────────┴─────────┴─────────┴─────────┴─────────┘

You click "Switch" on host
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟢    │   🟡    │  <- Tests yellow (need run)
└─────────┴─────────┴─────────┴─────────┴─────────┘

Tests auto-run and pass
↓
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟢    │   🟢    │  <- All good! ✓
└─────────┴─────────┴─────────┴─────────┴─────────┘
```

---

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

---

## Rollback Operations

### Per-Host Rollback (⋮ → Rollback System)

Use when: "This host has a problem"

1. Click host **⋮** menu → **Rollback System**
2. Confirm the rollback
3. Agent runs `nixos-rebuild --rollback switch`
4. Tests compartment turns yellow (need re-run)

### Fleet-Wide PR Revert

Use when: "This PR broke everyone"

1. If tests fail on multiple hosts after merge
2. Dashboard offers **Revert PR** button
3. Creates revert commit on GitHub
4. All hosts can then pull the revert

---

## Glossary

| Term           | Meaning                                                      |
| -------------- | ------------------------------------------------------------ |
| **flake.nix**  | Defines your Nix configuration and its inputs (dependencies) |
| **flake.lock** | Pins exact versions of all inputs (like package-lock.json)   |
| **nixpkgs**    | The main Nix package repository (80,000+ packages)           |
| **generation** | A commit hash representing a deployed configuration version  |
| **switch**     | Apply a new system configuration (`nixos-rebuild switch`)    |
| **rollback**   | Revert to a previous NixOS generation                        |
| **PR**         | Pull Request on GitHub                                       |

---

## Related Documentation

- [UPDATE-ARCHITECTURE.md](./UPDATE-ARCHITECTURE.md) — Complete update flow and troubleshooting
- [BUILD-DEPLOY.md](./BUILD-DEPLOY.md) — How components are built and deployed
- [P3700](../+pm/done/P3700-lock-version-tracking.md) — Lock compartment version tracking
- [P3800](../+pm/done/P3800-system-inference.md) — System compartment inference
- [P3900](../+pm/done/P3900-tests-compartment.md) — Tests compartment (5th)
- [P4500](../+pm/done/P4500-generation-tracking.md) — Generation tracking
- [P4600](../+pm/done/P4600-rollback-operations.md) — Rollback operations
- [P4700](../+pm/done/P4700-merge-pr-workflow.md) — Merge PR workflow
