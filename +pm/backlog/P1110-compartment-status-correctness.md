# P1110: Compartment Status Correctness (Remote-Verified)

**Priority**: P1110 (Critical - Core UX Incorrect)  
**Type**: Bug + Spec/Implementation Alignment  
**Status**: ✅ COMPLETE  
**Created**: 2025-12-29  
**Supersedes**: P1100 (partially implemented; superseded due to correctness gaps)  
**Canonical Spec**: `+pm/spec/CORE-006-compartments.md`

---

## Summary

The compartment row is NixFleet’s core UX. Today, it has correctness gaps (e.g., System staying gray, stale “busy/pulling” state, inconsistent inference vs spec). This item makes the implementation match `CORE-006`:

- **🟢 means current vs remote desired state**
- **🟡 means action needed OR verification degraded**
- **🔴 means a real problem (including remote fetch failures for Git/Lock)**
- **⚪ is warm-up/insufficient host signal only**

---

## Known Problems (Observed)

- ✅ System dot persistence + remote-gating implemented (no more “stays gray” due to DB wipe).
- ✅ Agent `command_rejected` handled; lifecycle reconciles and clears stuck “busy/pulling”.
- ✅ Remote fetch failures are 🔴 for Git/Lock (VersionFetcher returns `error`).
- ✅ State hydration is drift-safe via CORE-004 init/full_state/delta (P1120).

---

## Audit (Reality Check vs CORE-006) — 2025-12-29

### Compartment data sources (today)

| Compartment | Intended by spec (`CORE-006`)                                                     | Implemented today                                                                                                                              | Gap                                                            |
| ----------- | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Agent       | Dashboard-side version compare                                                    | Dashboard-side version compare                                                                                                                 | ✅ OK                                                          |
| Git         | Dashboard-side remote desired; **remote failure = 🔴**                            | `VersionFetcher.GetGitStatus()` returns only `ok/outdated/unknown` and does not surface fetch errors                                           | 🔴 not representable; remote failure conflated with unknown    |
| Lock        | Agent reports hash; dashboard compares vs remote desired; **remote failure = 🔴** | `VersionFetcher.GetLockStatus()` returns only `ok/outdated/unknown`; fallback uses agent time-based status when lock hash/version data missing | 🔴 not representable; fallback violates “version-based” intent |
| System      | Dashboard-side inference + **remote-gated**                                       | `Hub.handleHeartbeat()` largely uses **agent-provided** `UpdateStatus.System` and can overwrite DB with NULL                                   | Not dashboard-inferred; not remote-gated; can be wiped → gray  |
| Tests       | Agent runs; results are **generation-scoped**; remote-gated (no false 🟢)         | Only forwarded if agent sends `UpdateStatus.Tests`; no generation scoping                                                                      | No persistence/scoping; can be missing after restarts          |

**Code touchpoints (current):**

- Git/Lock remote comparison: `src/internal/dashboard/version_fetcher.go`
- Heartbeat persistence/broadcast: `src/internal/dashboard/hub.go` (`handleHeartbeat`)
- Server-side host view: `src/internal/dashboard/handlers.go` (`getUpdateStatus`)

### State machine / lifecycle mismatches affecting compartments

- **`command_rejected` is not handled** (agent → dashboard), so Op Engine can believe an op is running when agent refused it.
- **pending_command “single source of truth” is violated in multiple places** (cleared on registration and via ad-hoc API paths).
- **Lifecycle HostProvider returns `UpdateStatus` as always-unknown** due to stub parsing, which breaks validation/post-check logic.
- **CORE-004 State Sync is not wired to browser WebSocket** (browser messages only `subscribe/unsubscribe`), and the `full_state` host payload does not include compartment data.

---

## Required Behavior (from CORE-006)

- **Git/Lock**:
  - Remote fetch failure ⇒ **🔴** (dashboard/network/config problem)
- **System/Tests**:
  - Must not be **🟢** unless **remote desired state is verifiable** (Git/Lock not 🔴)
  - Remote verification degraded ⇒ **🟡** with explicit message
- **Offline hosts**:
  - Show last known compartment states (UI can dim the row; pure UX)

---

## Implementation Work (High Level)

### Dashboard: Compartment State Computation

- **Git/Lock remote fetch failure**:
  - Compute and store compartment status as `error` (🔴) when remote desired state can’t be fetched.
- **System**:
  - Implement **dashboard-side inference** and **remote gating** per `CORE-006` (no expensive agent checks).
  - Prevent “no-op pull” from turning System 🟡 (track `PullChangedDesired` / equivalent).
- **Tests**:
  - Make tests **generation-scoped**: old-generation pass ⇒ 🟡 on new deployment; never ran ⇒ ⚪.

### Lifecycle / Stale Command Correctness

- **Handle `command_rejected` (agent → dashboard)**:
  - When agent rejects because it is busy, the dashboard must reconcile Op Engine state and clear/transition appropriately (no stuck `pending_command` / no false “pulling”).

### State Hydration & Persistence

- Ensure **full state** payload includes enough data to render compartments immediately (not only after subsequent heartbeats).
- Ensure DB updates do **not** accidentally wipe stable compartment state (e.g., by overwriting JSON columns with NULL).

---

## Concrete Fix List (Actionable)

### Git/Lock: make remote failures truly 🔴

- ✅ Implemented in `src/internal/dashboard/version_fetcher.go` (+ tests updated).

### System: dashboard-inferred + remote-gated; no more “wiped to NULL”

- ✅ Persist System from command outcomes in `src/internal/dashboard/hub.go` and prevent JSON wipe via `COALESCE(...)`.
- ✅ Remote gating enforced (System/Tests cannot be 🟢 unless Git+Lock are 🟢).

### Tests: generation-scoped semantics

- ✅ Persist `tests_status_json` + `tests_generation` and degrade on generation change.

### Command lifecycle: handle agent rejection and stop ad-hoc pending_command writes

- ✅ `protocol.TypeRejected` handled in `src/internal/dashboard/hub.go` and reconciled via `LifecycleManager.HandleCommandRejected`.
- Remove/replace “clear pending_command here” hotfixes that bypass LifecycleManager, e.g. in `src/internal/dashboard/handlers.go` (refresh path) and host registration upsert.

### Ops validation must understand `error`

- ✅ Ops validation blocks on Git `error` (`src/internal/ops/validation.go`).

### Lifecycle HostProvider must return real compartment data

- ✅ Implemented real JSON parsing + Git status fill in `src/internal/dashboard/lifecycle_adapter.go`.

### State sync (scope candidate)

- ✅ Done in `P1120` (CORE-004 fully wired; `full_state` includes compartment fields).

---

## Acceptance Criteria

- **Row of 🟢** only when host is truly current vs remote desired (Git/Lock/System/Tests aligned).
- **Remote desired fetch failure**:
  - Git = 🔴, Lock = 🔴, System = 🟡, Tests = 🟡, plus explicit messaging.
- **System** transitions correctly through:
  - Remote ahead → 🟡
  - Pull changes desired inputs → 🟡 (until switch)
  - Switch succeeds → 🟢 (subject to remote verifiability)
  - Switch fails → 🔴
- **No stuck “busy/pulling”** due solely to rejection or lost messages.

---

## Notes / History

- Prior work and context: `+pm/cancelled/P1100-compartment-state-machine-overhaul.md`
