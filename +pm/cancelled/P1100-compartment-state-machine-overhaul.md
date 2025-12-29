# P1100: Compartment State Machine Overhaul

**Priority**: P1100 (Critical - Core Functionality Broken)  
**Type**: Bug + Refactor  
**Status**: 🟡 SUPERSEDED (partial; see below)  
**Created**: 2025-12-28  
**Supersedes**: P1000 (scope expanded)  
**Last Audit**: 2025-12-29  
**State Machine Refactor**: 2025-12-29  
**Click Behavior Fixed**: 2025-12-29  
**Superseded By**: `+pm/backlog/P1110-compartment-status-correctness.md` (2025-12-29)

---

## Superseded / Correction Note (2025-12-29)

This document contains useful implementation notes and an initial audit, but it was **incorrectly marked “✅ COMPLETE”**. Subsequent analysis found fundamental gaps between spec and implementation (and between UI behavior and intended semantics).

Use this file as **historical context**, not as the active tracking item.

**Key gaps that remained (moved to P1110):**

- **System compartment correctness**: System can remain gray / be overwritten; System must be **remote-gated** per `CORE-006` (🟢 only when current vs remote desired).
- **Remote fetch failures**: should be **🔴 for Git/Lock** and must block **🟢 for System/Tests** (System/Tests become 🟡 “verification degraded”, not gray).
- **Stale command correctness**: agent-side `command_rejected` must be handled to avoid stuck “busy/pulling” state.
- **State hydration/persistence**: initial/full-state and DB persistence must not drop or wipe compartment data.

Canonical compartment semantics: `+pm/spec/CORE-006-compartments.md`.

---

## Summary

The compartment system - the **core UI of NixFleet** - has several state machine issues. While basic status display works, "working" (blue) states are not shown during operations, click behavior is inconsistent, and some compartments show gray when they should show their actual status.

---

## Audit Results (2025-12-29) — Later Found Incomplete

| Issue                     | Status       | Notes                                                    |
| ------------------------- | ------------ | -------------------------------------------------------- |
| 1. Generation "—"         | ✅ **FIXED** | All hosts show commit hash                               |
| 2. No blue working pulse  | ✅ **FIXED** | SetXxxWorking() wired for all ops                        |
| 3. System clickable       | ✅ **FIXED** | Now info-only per CORE-006                               |
| 4. Click behavior         | ✅ **FIXED** | Full state-based click behavior                          |
| 5. Context bar            | ✅ **OK**    | Hover → context bar works                                |
| 6. Compartments gray      | ⚠️ **GAPS**  | Later analysis found System correctness gaps (see P1110) |
| 7. lockHash missing       | ✅ **OK**    | URL configured correctly via env var                     |
| 8. "Switch running"       | ✅ **FIXED** | State machine refactored (see below)                     |
| 9. Tests "not configured" | ⚠️ **GAPS**  | Tests semantics refined in CORE-006; see P1110           |

---

## State Machine Refactor (2025-12-29)

### Changes Made

| Priority | Fix                                                                        | Files Changed                         |
| -------- | -------------------------------------------------------------------------- | ------------------------------------- |
| **P0**   | LifecycleManager is now SINGLE SOURCE OF TRUTH for `pending_command`       | `lifecycle.go`, `hub.go`, `server.go` |
| **P0**   | Reconnect timeout now clears DB via `clearActive()`                        | `lifecycle.go`                        |
| **P1**   | `SetSystemWorking()`, `SetLockWorking()` wired before operations           | `status.go`, `commands.go`            |
| **P1**   | macOS switch now restarts agent (like NixOS) for proper reconnect flow     | `commands.go`                         |
| **P2**   | Stale cleanup checks LifecycleManager before clearing `pending_command`    | `hub.go`, `lifecycle_adapter.go`      |
| **UI**   | Complete rewrite of `handleCompartmentClick()` per CORE-006                | `dashboard.templ`                     |
| **UI**   | System compartment is now info-only (deprecated `showSystemRefreshDialog`) | `dashboard.templ`                     |
| **UI**   | Green (ok) state shows info, never re-triggers action                      | `dashboard.templ`                     |
| **UI**   | Blue (working) state dispatches `show-stop-action` event                   | `dashboard.templ`                     |

### Architectural Changes

**Before:** Dual state tracking created edge cases

- `hosts.pending_command` (DB) updated by heartbeats
- `LifecycleManager.active` (memory) tracked by ops engine
- Race conditions when agent crashed or disconnected

**After:** Single source of truth

- `LifecycleManager` controls `pending_command` via `PendingCommandStore` interface
- Hub implements `SetPendingCommand()` / `ClearPendingCommand()`
- Heartbeats no longer update `pending_command` (informational only)
- Stale cleanup respects LifecycleManager state

### Full State Diagram

See **[CORE-006-compartments.md](../spec/CORE-006-compartments.md)** for the complete command lifecycle state machine diagram.

---

## Critical Issues

### ~~Issue 1: Generation Shows "—" Despite Heartbeats~~ ✅ FIXED

**Status**: RESOLVED

Generation column now works correctly. All hosts show git commit hash (e.g., `c3e0094`).

---

### ~~Issue 2: No "Working" (Blue) Status During Operations~~ ✅ FIXED

**Status**: RESOLVED (2025-12-29)

**Fix Applied**: Added `SetXxxWorking()` calls before all operations:

- ✅ `SetTestsWorking()` called before `test` command
- ✅ `SetSystemWorking()` called before `switch` and `pull-switch` commands
- ✅ `SetLockWorking()` called before `refresh-lock` command

**Files changed**: `status.go`, `commands.go`

---

### ~~Issue 3: System Compartment Still Clickable~~ ✅ FIXED

**Status**: RESOLVED (2025-12-29)

**Fix Applied**: System compartment is now info-only per CORE-006:

- Click shows status in log panel with explanation
- Shows "Inferred from Lock status and last command"
- Suggests action via Deploy menu, never triggers directly
- `showSystemRefreshDialog()` deprecated

---

### ~~Issue 4: Click Behavior Inconsistent~~ ✅ FIXED

**Status**: RESOLVED (2025-12-29)

**Fix Applied**: Complete rewrite of `handleCompartmentClick()` per CORE-006 state machine:

| State                | Click Action                                     |
| -------------------- | ------------------------------------------------ |
| ⚪ Gray (unknown)    | Trigger lightweight check                        |
| 🟢 Green (ok)        | Show detailed status (**NO action**)             |
| 🟡 Yellow (outdated) | Trigger appropriate operation                    |
| 🔵 Blue (working)    | Show progress, dispatch `show-stop-action` event |
| 🔴 Red (error)       | Show error details, offer retry                  |

**Per-compartment behavior now correct**:
| Compartment | Click Action |
|-------------|--------------|
| Agent | Green=info, Error=info+suggest action, Unknown=check-version |
| Git | Green=info, Outdated/Error=triggerGitRefresh, Unknown=refresh |
| Lock | Green=info, Outdated/Error=refresh-lock, Unknown=refresh |
| System | **INFO ONLY** - never triggers action (inference-only) |
| Tests | Green=info, Outdated/Error=run tests, Unknown=info |

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

## Acceptance Criteria (Historical; see P1110 for current)

- [x] ~~Generation column shows commit hash for all online hosts~~ ✅ DONE
- [x] ~~Compartments show actual status~~ ⚠️ PARTIAL (see P1110)
- [x] ~~All compartments show blue pulse during operations~~ ✅ DONE (Issue 2 - SetXxxWorking wired)
- [x] ~~State machine has single source of truth~~ ✅ DONE (LifecycleManager controls pending_command)
- [x] ~~No stale commands after disconnect~~ ⚠️ PARTIAL (rejections / other edge cases; see P1110)
- [x] ~~Clicking blue (working) compartment offers STOP~~ ✅ DONE
- [x] ~~Clicking green (ok) compartment shows details, NOT re-triggers action~~ ✅ DONE
- [x] ~~System compartment click shows inference reason, NO action trigger~~ ✅ DONE
- [x] ~~Click on any compartment opens log panel with detailed info~~ ✅ DONE

---

## Related

- **CORE-006** - Compartment specification (canonical source of truth)
- **P1110** - Current correctness work item (superseding this)
