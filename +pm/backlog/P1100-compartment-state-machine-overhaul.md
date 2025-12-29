# P1100: Compartment State Machine Overhaul

**Priority**: P1100 (Critical - Core Functionality Broken)  
**Type**: Bug + Refactor  
**Status**: Open  
**Created**: 2025-12-28  
**Supersedes**: P1000 (scope expanded)

---

## Summary

The compartment system - the **core UI of NixFleet** - has fundamental state machine bugs. Status indicators don't reflect reality, click actions are inconsistent, and "working" states are not shown during operations.

---

## Critical Issues

### Issue 1: Generation Shows "—" Despite Heartbeats

**Symptom**: All hosts show "—" in Generation column, tooltip says "Generation unknown"

**Expected**: Shows short git commit hash (e.g., `db2edff`)

**Investigation needed**:

- [ ] Check dashboard DB: `SELECT hostname, generation FROM hosts`
- [ ] Check agent logs for "system info detected" with generation value
- [ ] Verify `handleHeartbeat()` is storing `payload.Generation`
- [ ] Verify `RepoDir` config is correct on each agent

**Likely causes**:

1. Agent's `detectGeneration()` returning empty (`.git/HEAD` not found)
2. Agent `repo_dir` config pointing to wrong path
3. Dashboard not reading generation from DB into Host struct

---

### Issue 2: No "Working" (Blue) Status During Operations

**Symptom**: Compartments jump from gray/green → result without showing blue pulse during execution

**Expected**: While operation runs, compartment shows 🔵 blue pulsing indicator

**Root Cause**: `SetTestsWorking()` exists but is **never called**!

```go
// commands.go line 140-143 - MISSING SetTestsWorking() call
case "test":
    a.sendOperationProgress("tests", "in_progress", 0, 8)
    cmd, err = a.buildTestCommand()
    // BUG: Should call a.statusChecker.SetTestsWorking() here!
```

**Affected compartments**:

- [ ] Tests: `SetTestsWorking()` not called
- [ ] System: Need `SetSystemWorking()` during switch
- [ ] Lock: Need `SetLockWorking()` during refresh-lock
- [ ] Git: Dashboard-side, needs working state during pull

**Fix**: Add working state calls at start of each operation

---

### Issue 3: System Compartment Still Clickable

**Symptom**: Clicking System compartment shows "expensive refresh" confirmation dialog

**Expected**: System is **inference-only** per P3800 spec - no click action

**Spec (CORE-006)**:

> System status is inferred from:
>
> - Last command result (pull → outdated, switch → ok)
> - Lock status (lock outdated → system outdated)
> - No manual refresh needed or possible

**Current code** (dashboard.templ line 3341):

```javascript
case 'system':
    showSystemRefreshDialog(hostId, hostname, description, status);
    break;
```

**Fix**: System click should show info modal (read-only), not trigger action

---

### Issue 4: Click Behavior Inconsistent

**Current behavior** (broken):
| State | Click Action | Problem |
|-------|--------------|---------|
| Gray (unknown) | Triggers operation | OK |
| Green (ok) | Triggers operation again | WRONG - should show info |
| Yellow (outdated) | Triggers operation | OK |
| Blue (working) | Triggers operation again | WRONG - should offer STOP |
| Red (error) | Triggers operation | Maybe show error details first? |

**Expected behavior**:
| State | Click Action |
|-------|--------------|
| Gray (unknown) | Show "status unknown" info |
| Green (ok) | Show detailed status in log panel |
| Yellow (outdated) | Trigger appropriate operation |
| Blue (working) | Offer to STOP/KILL the running operation |
| Red (error) | Show error details, offer retry |

---

### Issue 5: Context Bar vs Log Panel Information

**Current**: Hover shows brief status in context bar at bottom

**Expected**:

- Hover → brief status in context bar (current behavior, OK)
- Click → detailed status/history in log panel (NOT triggering action when already ok)

---

## State Machine Specification

### Per-Compartment States

```
     ┌─────────────────────────────────────────────────────────┐
     │                    COMPARTMENT STATES                    │
     ├──────────┬────────┬──────────────────────────────────────┤
     │  State   │ Color  │ Click Action                         │
     ├──────────┼────────┼──────────────────────────────────────┤
     │ unknown  │ ⚪ gray │ Show "checking..." or trigger check  │
     │ ok       │ 🟢 green│ Show detailed status (no action)     │
     │ outdated │ 🟡 yellow│ Trigger update operation            │
     │ working  │ 🔵 blue │ Show progress, offer STOP            │
     │ error    │ 🔴 red  │ Show error details, offer retry      │
     └──────────┴────────┴──────────────────────────────────────┘
```

### Working State Lifecycle

```
User clicks compartment (outdated state)
         │
         ▼
    ┌─────────────┐
    │ Set WORKING │ ← Agent/Dashboard sets blue state
    │   (blue)    │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Operation  │ ← Command executes
    │   Running   │
    └──────┬──────┘
           │
     ┌─────┴─────┐
     ▼           ▼
┌─────────┐ ┌─────────┐
│   OK    │ │  ERROR  │
│ (green) │ │  (red)  │
└─────────┘ └─────────┘
```

### Per-Compartment Click Logic

#### Agent Compartment

- **Gray**: "Agent version unknown"
- **Green**: "Agent v3.1.4 - current"
- **Yellow**: "Agent outdated (v3.1.2 → v3.1.4 available)" → offer Update
- **Blue**: N/A (agent updates are instant)
- **Red**: "Agent update failed" → show error, offer retry

#### Git Compartment

- **Gray**: "Checking GitHub..."
- **Green**: "Git current (abc123)" → show commit details
- **Yellow**: "2 commits behind" → trigger Pull
- **Blue**: "Pulling..." → show progress, offer Stop
- **Red**: "Pull failed" → show error, offer retry

#### Lock Compartment

- **Gray**: "Checking flake.lock..."
- **Green**: "Lock current (hash matches)" → show hash
- **Yellow**: "Lock outdated (new deps available)" → trigger Pull
- **Blue**: "Refreshing..." → show progress
- **Red**: "Lock check failed" → show error

#### System Compartment (INFERENCE ONLY - NO TRIGGER)

- **Gray**: "System status unknown"
- **Green**: "System current (gen abc123)"
- **Yellow**: "System outdated (needs switch)" → show WHY (inferred from what)
- **Blue**: "Switching..." → show progress, offer Stop
- **Red**: "Switch failed" → show error

#### Tests Compartment

- **Gray**: "Tests not configured"
- **Green**: "All tests passed" → show test results
- **Yellow**: "Tests not run yet" → trigger Test
- **Blue**: "Tests running..." → show progress, offer Stop
- **Red**: "Tests failed" → show failures, offer retry

---

## Files to Modify

### Agent-side

- `src/internal/agent/commands.go` - Add SetXxxWorking() calls
- `src/internal/agent/status.go` - Add missing Set methods (SetSystemWorking, SetLockWorking)

### Dashboard-side

- `src/internal/dashboard/hub.go` - Verify generation storage, add git working state
- `src/internal/templates/dashboard.templ`:
  - Rewrite `handleCompartmentClick()` with proper state machine
  - Add STOP functionality for working state
  - System compartment → info only, no action trigger

### Database

- Verify `generation` column is populated and read correctly

---

## Acceptance Criteria

- [ ] Generation column shows commit hash for all online hosts
- [ ] All compartments show blue pulse during operations
- [ ] Clicking blue (working) compartment offers STOP
- [ ] Clicking green (ok) compartment shows details, NOT re-triggers action
- [ ] System compartment click shows inference reason, NO action trigger
- [ ] Click on any compartment opens log panel with detailed info

---

## Related

- P3800 - System Inference (spec says read-only, not implemented)
- P3900 - Tests Compartment (working state not wired)
- CORE-006 - Compartment specification
