# P1100: Compartment State Machine Overhaul

**Priority**: P1100 (Critical - Core Functionality Broken)  
**Type**: Bug + Refactor  
**Status**: Open  
**Created**: 2025-12-28  
**Supersedes**: P1000 (scope expanded)  
**Last Audit**: 2025-12-29

---

## Summary

The compartment system - the **core UI of NixFleet** - has several state machine issues. While basic status display works, "working" (blue) states are not shown during operations, click behavior is inconsistent, and some compartments show gray when they should show their actual status.

---

## Audit Results (2025-12-29)

| Issue                     | Status         | Notes                                   |
| ------------------------- | -------------- | --------------------------------------- |
| 1. Generation "—"         | ✅ **FIXED**   | All hosts show commit hash              |
| 2. No blue working pulse  | ⚠️ **OPEN**    | Needs testing during operation          |
| 3. System clickable       | ⚠️ **OPEN**    | Shows dialog instead of info-only       |
| 4. Click behavior         | ⚠️ **PARTIAL** | Some states correct, others not         |
| 5. Context bar            | ✅ **OK**      | Hover → context bar works               |
| 6. Compartments gray      | ✅ **OK**      | Was visual analysis error - colors work |
| 7. lockHash missing       | ✅ **OK**      | URL configured correctly via env var    |
| 8. "Switch running"       | ✅ **OK**      | pending_command handling works          |
| 9. Tests "not configured" | ✅ **OK**      | Tests status displays correctly         |

---

## Critical Issues

### ~~Issue 1: Generation Shows "—" Despite Heartbeats~~ ✅ FIXED

**Status**: RESOLVED

Generation column now works correctly. All hosts show git commit hash (e.g., `c3e0094`).

---

### Issue 2: No "Working" (Blue) Status During Operations ⚠️ OPEN

**Symptom**: Compartments jump from gray/green → result without showing blue pulse during execution

**Expected**: While operation runs, compartment shows 🔵 blue pulsing indicator

**Root Cause**: `SetTestsWorking()` exists but may not be called for all operations!

The indicator CSS class `compartment-indicator--working` exists in the code, but the agent needs to:

1. Call `SetXxxWorking()` at operation start
2. Send heartbeat with `working` status
3. Dashboard updates indicator via WebSocket

**Status**: Needs testing during actual operation to verify.

**Affected compartments to check**:

- [ ] Tests: Verify `SetTestsWorking()` is called before running tests
- [ ] System: Verify working state during switch
- [ ] Lock: Verify working state during refresh-lock
- [ ] Git: Dashboard-side pull should show working state

**Fix**: Ensure each operation sets working state before starting.

---

### Issue 3: System Compartment Still Clickable ⚠️ OPEN

**Symptom**: Clicking System compartment shows "expensive refresh" confirmation dialog

**Expected**: System is **inference-only** per P3800 spec - click shows info, no action

**Spec (CORE-006)**:

> System status is inferred from:
>
> - Last command result (pull → outdated, switch → ok)
> - Lock status (lock outdated → system outdated)
> - No manual refresh needed or possible

**Current code** (dashboard.templ line 3341-3344):

```javascript
case 'system':
    // System: show confirmation dialog (refresh is expensive)
    showSystemRefreshDialog(hostId, hostname, description, status);
    break;
```

**Fix**:

- System click should show info only (why it's in current state)
- Remove the refresh dialog entirely
- Show "Inferred from: last switch was ok" or "Lock outdated → system outdated"

---

### Issue 4: Click Behavior Inconsistent ⚠️ PARTIAL

The click behavior in `handleCompartmentClick()` has improved but still needs work:

**Current behavior** (from code review):
| Compartment | Click Action | Status |
|-------------|--------------|--------|
| Agent | Runs `check-version` command | ✅ OK - lightweight |
| Git | Refreshes via `/api/hosts/{id}/refresh-git` | ✅ OK - lightweight |
| Lock | Runs `refresh-lock` command | ✅ OK - lightweight |
| System | Shows confirmation dialog | ❌ WRONG - should be info-only |
| Tests | Runs `test` command always | ⚠️ PARTIAL - doesn't check status |

**Expected per CORE-006**:
| State | Click Action |
|-------|--------------|
| Gray (unknown) | Show "status unknown" info |
| Green (ok) | Show detailed status in log panel (**NO action**) |
| Yellow (outdated) | Trigger appropriate operation |
| Blue (working) | Show progress, offer **STOP** |
| Red (error) | Show error details, offer retry |

**Key gaps**:

1. Clicking GREEN should show info, not re-trigger operation
2. Clicking BLUE should offer STOP, not re-trigger
3. Tests compartment runs `test` regardless of current status

---

### ~~Issue 5-9: Resolved~~

Issues 5-9 were verified as working correctly during the 2025-12-29 audit:

- Context bar works
- Compartment colors display correctly (visual analysis error)
- lockHash/version.json configured via `NIXFLEET_VERSION_URL`
- pending_command handling works
- Tests status displays correctly

---

## State Machine Specification

> **Canonical spec**: See [CORE-006-compartments.md](../spec/CORE-006-compartments.md) for the complete state machine definition including:
>
> - All 5 compartments and their states
> - Click behavior per state
> - Working state lifecycle
> - STOP functionality
> - System inference rules

### Quick Reference (from CORE-006)

| State    | Color     | Click Action                         |
| -------- | --------- | ------------------------------------ |
| unknown  | ⚪ gray   | Show "checking..." or trigger check  |
| ok       | 🟢 green  | Show detailed status (**NO action**) |
| outdated | 🟡 yellow | Trigger appropriate operation        |
| working  | 🔵 blue   | Show progress, offer **STOP**        |
| error    | 🔴 red    | Show error details, offer retry      |

**Key rules:**

- **Green = info only** — never re-triggers operation
- **System = inference only** — no click action, status inferred from commands
- **Blue = STOP available** — user can abort running operations

---

## Investigation Plan (from P1000)

1. **Check GitHub Actions**: Manually trigger `version-pages.yml` and verify lockHash appears
2. **Check Database**: Query `SELECT hostname, pending_command, generation FROM hosts`
3. **Check Agent Logs**: Look for heartbeat payload to verify what's being sent
4. **Check Browser Console**: Look for WebSocket messages to verify what dashboard receives

---

## Files to Modify

### Agent-side

- `src/internal/agent/commands.go` - Add SetXxxWorking() calls at operation start
- `src/internal/agent/status.go` - Add SetSystemWorking(), SetLockWorking(); initialize testsStatus
- `src/internal/agent/heartbeat.go` - Verify generation detection

### Dashboard-side

- `src/internal/dashboard/hub.go` - Verify generation storage, pending_command clearing
- `src/internal/dashboard/state_provider.go` - Initial state loading
- `src/internal/templates/dashboard.templ`:
  - Rewrite `handleCompartmentClick()` with proper state machine
  - Add STOP functionality for working state
  - System compartment → info only, no action trigger

### External (nixcfg repo)

- `nixcfg/.github/workflows/version-pages.yml` - Verify lockHash generation

### Database

- Verify `generation` column is populated and read correctly
- Verify `pending_command` is properly cleared on command complete

---

## Acceptance Criteria

- [x] ~~Generation column shows commit hash for all online hosts~~ ✅ DONE
- [x] ~~Compartments show actual status~~ ✅ DONE (was visual analysis error)
- [ ] All compartments show blue pulse during operations (Issue 2)
- [ ] Clicking blue (working) compartment offers STOP (Issue 4)
- [ ] Clicking green (ok) compartment shows details, NOT re-triggers action (Issue 4)
- [ ] System compartment click shows inference reason, NO action trigger (Issue 3)
- [ ] Click on any compartment opens log panel with detailed info

---

## Validation

Implementation is validated against **[CORE-006-compartments.md](../spec/CORE-006-compartments.md)**:

- Click behavior matches spec tables
- State transitions match diagrams
- System compartment is inference-only (no action trigger)
- Working state shows blue pulse
- STOP functionality works for all operations

## Related

- **CORE-006** - Compartment specification (canonical source of truth)
- P3800 - System Inference (spec says read-only, not implemented)
- P3900 - Tests Compartment (working state not wired)
