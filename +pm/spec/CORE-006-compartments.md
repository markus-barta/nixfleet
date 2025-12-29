# CORE-006: Compartment Status System

**Version**: 1.0  
**Status**: Draft  
**Created**: 2025-12-28  
**Related**: P5200, P5300, P5400, P5500

---

## Overview

The Compartment Status System is a **five-stage pipeline** that provides at-a-glance visibility into fleet state. Each compartment represents a specific stage in the deployment process, from tooling to testing.

---

## Goals

1. **Clear visibility** - User knows exactly what state each host is in
2. **Actionable information** - Each color indicates what action to take
3. **No expensive checks** - Status updates without heavy operations
4. **Accurate tracking** - Version-based, not time-based heuristics

---

## The Five Compartments

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│ Agent   │   Git   │  Lock   │ System  │ Tests   │
└─────────┴─────────┴─────────┴─────────┴─────────┘
    ↓         ↓         ↓         ↓         ↓
 Tooling   Config     Deps     Deploy     Verify
```

| #   | Name       | Question                   | Data Source        | Check Type                        |
| --- | ---------- | -------------------------- | ------------------ | --------------------------------- |
| 1   | **Agent**  | Is nixfleet-agent current? | Version comparison | Dashboard-side                    |
| 2   | **Git**    | Is local repo up to date?  | GitHub API         | Dashboard-side                    |
| 3   | **Lock**   | Is flake.lock current?     | Content hash       | Agent reports, Dashboard compares |
| 4   | **System** | Is running system current? | Command inference  | Dashboard-side                    |
| 5   | **Tests**  | Is system working?         | Test execution     | Agent runs, reports back          |

---

## Color States

Each compartment uses a **5-color system**:

| Color         | Meaning               | When to Use                                                        | Action Required      |
| ------------- | --------------------- | ------------------------------------------------------------------ | -------------------- |
| 🟢 **Green**  | Current / Passed      | Everything OK                                                      | None                 |
| 🟡 **Yellow** | Outdated / Pending    | Update needed                                                      | Pull / Switch / Test |
| 🔴 **Red**    | Failed / Error        | Something broke                                                    | Fix / Rollback       |
| 🔵 **Blue**   | Working / In progress | Operation running                                                  | Wait                 |
| ⚪ **Gray**   | Unknown / Warm-up     | No host signal yet (never connected / never ran / not enough data) | Wait                 |

**Important semantics:**

- **Gray is _not_ used for remote fetch failures.** If the dashboard cannot fetch the remote desired state, that is a **real problem** → **Red** on the compartment that depends on it.
- **Yellow can also mean “verification degraded”.** If remote desired state is unavailable, **System/Tests must not show green** (because “current vs remote” can’t be proven). In that case they show **Yellow** with an explicit message (not Gray).

---

## Compartment 1: Agent

### Purpose

Verifies the nixfleet-agent binary is current with the dashboard.

### States

```go
func computeAgentStatus(host Host, dashboardVersion string) string {
    if host.AgentVersion == "" {
        return "unknown"  // Gray: never connected
    }
    if host.AgentVersion != dashboardVersion {
        return "error"  // Red: outdated
    }
    return "ok"  // Green: current
}
```

| State     | Color | Meaning                         | Action             |
| --------- | ----- | ------------------------------- | ------------------ |
| `ok`      | 🟢    | Agent version matches dashboard | None               |
| `error`   | 🔴    | Agent outdated                  | Pull + Switch      |
| `unknown` | ⚪    | Agent version unknown           | Wait for heartbeat |

### Data Flow

```
Agent Heartbeat:
{
  "agent_version": "3.0.1",
  ...
}
         ↓
Dashboard compares:
host.AgentVersion == dashboard.Version
         ↓
Agent compartment: 🟢/🔴/⚪
```

---

## Compartment 2: Git

### Purpose

Checks if local repo is up to date with GitHub.

### States

```go
func computeGitStatus(hostGeneration string, latestCommit string) string {
    if hostGeneration == "" {
        return "unknown"  // Gray: host has not reported a generation yet
    }
    if latestCommit == "" {
        return "error"  // Red: remote desired commit unavailable (fetch/config problem)
    }
    if hostGeneration == latestCommit {
        return "ok"  // Green: up to date
    }
    return "outdated"  // Yellow: behind
}
```

| State      | Color | Meaning                           | Action                       |
| ---------- | ----- | --------------------------------- | ---------------------------- |
| `ok`       | 🟢    | Host repo matches remote desired  | None                         |
| `outdated` | 🟡    | Host repo behind remote desired   | Pull                         |
| `error`    | 🔴    | Remote desired state unavailable  | Fix dashboard/network/config |
| `unknown`  | ⚪    | Host generation unknown (warm-up) | Wait for first heartbeat     |

### Data Flow

```
Dashboard fetches (cached 5s):
https://example.github.io/nixcfg/version.json
→ { "gitCommit": "abc123..." }
         ↓
Agent reports in heartbeat:
{ "generation": "def456..." }
         ↓
Dashboard compares:
host.Generation == latestCommit
         ↓
Git compartment: 🟢/🟡/🔴/⚪
```

---

## Compartment 3: Lock

### Purpose

Checks if flake.lock is current (dependencies up to date).

### States

```go
func computeLockStatus(hostLockHash string, latestLockHash string) string {
    if hostLockHash == "" {
        return "unknown"  // Gray: host has not reported lock hash yet
    }
    if latestLockHash == "" {
        return "error"  // Red: remote desired lock hash unavailable (fetch/config problem)
    }
    if hostLockHash == latestLockHash {
        return "ok"  // Green: current
    }
    return "outdated"  // Yellow: outdated
}
```

| State      | Color | Meaning                          | Action                       |
| ---------- | ----- | -------------------------------- | ---------------------------- |
| `ok`       | 🟢    | Host lock matches remote desired | None                         |
| `outdated` | 🟡    | Host lock behind remote desired  | Pull (after PR merged)       |
| `error`    | 🔴    | Remote desired lock unavailable  | Fix dashboard/network/config |
| `unknown`  | ⚪    | Host lock hash unknown (warm-up) | Wait for first heartbeat     |

### Data Flow

```
Agent computes (every 5 min):
lockHash = SHA256(flake.lock content)
         ↓
Agent reports in heartbeat:
{ "lock_hash": "abc123..." }
         ↓
Dashboard fetches latest flake.lock from GitHub:
latestLockHash = SHA256(latest flake.lock)
         ↓
Dashboard compares:
host.LockHash == latestLockHash
         ↓
Lock compartment: 🟢/🟡/🔴/⚪
```

### Why Content Hash, Not Time?

**Old approach (broken):**

```
Days since last update: 5 days
Status: Green (< 7 days)

Problem: PR merged yesterday with new flake.lock
Reality: Host is outdated!
```

**New approach (correct):**

```
Host lock hash: abc123
Latest lock hash: def456
Status: Yellow (different)

Accurate: Host IS outdated
```

---

## Compartment 4: System

### Purpose

Checks if running system matches current config.

### States

**Inputs required (cheap):**

- `Git.Status` and `Lock.Status` computed from compartments 2–3 (remote desired vs host-reported).
- `LastCommand` / `LastExitCode` from command history (Op Engine / State Store).
- `PullChangedDesired` (boolean): true **only if** the last successful `pull` changed the host’s desired inputs (e.g., `generation` and/or `lock_hash` changed vs the previously known values). This prevents “no-op pull” from incorrectly turning System yellow.

```go
func inferSystemStatus(host Host) string {
    // System is defined as: "deployed and current vs *remote desired*"
    //
    // Therefore System MUST NOT be green unless Git and Lock are green.

    // Remote verification degraded: Git/Lock checks failed upstream
    if host.Git.Status == "error" || host.Lock.Status == "error" {
        return "outdated" // Yellow: cannot verify current vs remote right now
    }

    // If Git outdated → cannot be current vs remote
    if host.Git.Status == "outdated" {
        return "outdated"
    }

    // If Lock outdated → System MUST be outdated
    if host.Lock.Status == "outdated" {
        return "outdated"
    }

    // Deployment inference (cheap):
    // - Switch exit != 0 => error
    // - Switch exit == 0 => ok
    // - Pull that changed desired inputs => outdated until switch
    switch {
    case host.LastCommand == "switch" && host.LastExitCode != 0:
        return "error"
    case host.LastCommand == "switch" && host.LastExitCode == 0:
        return "ok"
    case host.LastCommand == "pull" && host.LastExitCode == 0 && host.PullChangedDesired == true:
        return "outdated"
    default:
        // Gray only when we genuinely have insufficient host history
        return "unknown"
    }
}
```

| State      | Color | Meaning                                            | Action                 |
| ---------- | ----- | -------------------------------------------------- | ---------------------- |
| `ok`       | 🟢    | Deployed and current vs remote desired             | None                   |
| `outdated` | 🟡    | Not current vs remote (or cannot verify vs remote) | Pull / Switch          |
| `error`    | 🔴    | Switch failed (deployment problem)                 | Check logs, fix config |
| `unknown`  | ⚪    | Insufficient host history (warm-up)                | Wait for first command |

### Why Inference, Not Expensive Checks?

**Old approach (broken):**

```
Run: nix build --dry-run (30-60 seconds!)
Problem: Too expensive for automatic checks
Result: Always gray (check never runs)
```

**New approach (correct):**

```
User runs Pull (exit 0)
→ System: Yellow (know config changed)

User runs Switch (exit 0)
→ System: Green (know system current)

No expensive checks needed!
```

**Note:** If remote desired state cannot be fetched (Git/Lock = 🔴), System must not be 🟢.
In that scenario System becomes 🟡 with a message like “Cannot verify vs remote (remote check failing)”.

---

## Compartment 5: Tests

### Purpose

Verifies system is actually working after deployment.

### States

| State      | Color | Meaning                                                                              | Action         |
| ---------- | ----- | ------------------------------------------------------------------------------------ | -------------- |
| `ok`       | 🟢    | Tests passed **for the currently deployed (remote) state**                           | None           |
| `outdated` | 🟡    | Tests missing/outdated for the currently deployed state (or cannot verify vs remote) | Run tests      |
| `error`    | 🔴    | Tests failed (on current deployed state)                                             | Fix / Rollback |
| `working`  | 🔵    | Tests running                                                                        | Wait           |
| `unknown`  | ⚪    | Tests never ran yet (warm-up)                                                        | Run tests      |

### Data Flow

```
User runs Switch (exit 0)
         ↓
System compartment: 🟢
Tests compartment: 🟡 (tests now outdated for new deployed state)
         ↓
User runs Test
         ↓
Tests compartment: 🔵 (running)
         ↓
Tests complete (exit code)
         ↓
Tests compartment: 🟢 (pass) or 🔴 (fail)
```

### Why Separate from System?

**System compartment:** "Did the deployment succeed?"  
**Tests compartment:** "Is the system actually working?"

**Example:**

```
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│   🟢    │   🟢    │   🟢    │   🟢    │   🔴    │
└─────────┴─────────┴─────────┴─────────┴─────────┘

System: 🟢 (switch succeeded)
Tests:  🔴 (X11 won't start)

Clear signal: deployment worked, but system is broken
```

---

## Click Behavior

Each compartment responds to clicks based on its current state:

### State → Action Matrix

```
┌──────────┬────────┬──────────────────────────────────────┐
│  State   │ Color  │ Click Action                         │
├──────────┼────────┼──────────────────────────────────────┤
│ unknown  │ ⚪ gray │ Show "checking..." or trigger check  │
│ ok       │ 🟢 green│ Show detailed status (NO action)     │
│ outdated │ 🟡 yellow│ Trigger appropriate operation       │
│ working  │ 🔵 blue │ Show progress, offer STOP            │
│ error    │ 🔴 red  │ Show error details, offer retry      │
└──────────┴────────┴──────────────────────────────────────┘
```

### Per-Compartment Click Logic

#### Agent Compartment

| State | Click Response                                         |
| ----- | ------------------------------------------------------ |
| Gray  | "Agent version unknown"                                |
| Green | "Agent v3.1.4 - current" (info only)                   |
| Red   | "Agent outdated (v3.1.2 → v3.1.4)" → offer Pull+Switch |

#### Git Compartment

| State  | Click Response                                                          |
| ------ | ----------------------------------------------------------------------- |
| Gray   | "Git status unknown (host not reporting yet)"                           |
| Green  | "Git current vs remote (abc123)" → show details                         |
| Yellow | "Git behind remote" → trigger Pull                                      |
| Blue   | "Pulling..." → show progress, offer Stop                                |
| Red    | "Remote git check failed (cannot fetch desired state)" → show fix hints |

#### Lock Compartment

| State  | Click Response                                                          |
| ------ | ----------------------------------------------------------------------- |
| Gray   | "Lock status unknown (host not reporting yet)"                          |
| Green  | "Lock current vs remote (hash matches)" → show hash                     |
| Yellow | "Lock behind remote" → trigger Pull                                     |
| Blue   | "Refreshing..." → show progress                                         |
| Red    | "Remote lock check failed (cannot fetch desired lock)" → show fix hints |

#### System Compartment (INFERENCE ONLY)

> **⚠️ CRITICAL**: System compartment does NOT trigger actions.
> Status is inferred from command results and lock state.
> Click shows information only — no refresh, no switch trigger.

| State  | Click Response                                                           |
| ------ | ------------------------------------------------------------------------ |
| Gray   | "System status unknown (insufficient host history)" (info)               |
| Green  | "System current vs remote (abc123)" (info)                               |
| Yellow | "System not current vs remote (or cannot verify vs remote)" + WHY (info) |
| Blue   | "Switching..." → show progress, offer Stop                               |
| Red    | "Switch failed" → show error (info)                                      |

**Why no action?** Running `nix build --dry-run` to check system status takes 30-60+ seconds and consumes significant resources. Instead, we infer status from:

- Lock outdated → System MUST be outdated
- Last command was `pull` (exit 0) → System outdated
- Last command was `switch` (exit 0) → System current

#### Tests Compartment

| State  | Click Response                                                                          |
| ------ | --------------------------------------------------------------------------------------- |
| Gray   | "Tests never ran yet" (info)                                                            |
| Green  | "Tests passed (current deployed state)" → show results                                  |
| Yellow | "Tests outdated for current deployed state (or cannot verify vs remote)" → trigger Test |
| Blue   | "Tests running..." → show progress, offer Stop                                          |
| Red    | "Tests failed" → show failures, offer retry                                             |

### Working State Lifecycle

```
User clicks compartment (yellow/outdated state)
         │
         ▼
    ┌─────────────┐
    │ Set WORKING │ ← Immediately show blue pulse
    │   (blue)    │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Operation  │ ← Command executes on agent
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

### STOP Functionality

When a compartment is in **working** (blue) state:

1. Click shows current progress
2. Offers **STOP** button
3. STOP sends `SIGTERM` to running process
4. If process doesn't exit in 3s, sends `SIGKILL`
5. Compartment transitions to **error** (red) with "Stopped by user"

---

## State Transitions

### Normal Update Flow

```
Initial:  [ 🟢  🟢  🟢  🟢  🟢 ]  All current

PR merged on GitHub:
          [ 🟢  🟡  🟡  🟢  🟢 ]  Git & Lock outdated

After Pull:
          [ 🟢  🟢  🟢  🟡  🟢 ]  System needs switch

After Switch:
          [ 🟢  🟢  🟢  🟢  🟡 ]  Tests need run

After Tests Pass:
          [ 🟢  🟢  🟢  🟢  🟢 ]  All current
```

### Remote Verification Degraded (Dashboard can’t fetch desired state)

```
Remote fetch fails:
          [ 🟢  🔴  🔴  🟡  🟡 ]

Git/Lock: 🔴 (cannot verify remote desired state)
System/Tests: 🟡 (must not be 🟢 because “current vs remote” cannot be proven)
```

### Failed Update Flow

```
Initial:  [ 🟢  🟢  🟢  🟢  🟢 ]

After Pull:
          [ 🟢  🟢  🟢  🟡  🟢 ]

Switch fails:
          [ 🟢  🟢  🟢  🔴  🟢 ]  System error

User fixes config:
          [ 🟢  🟡  🟢  🔴  🟢 ]  Git outdated (fix pushed)

After Pull:
          [ 🟢  🟢  🟢  🔴  🟢 ]  Still broken (old system)

After Switch (success):
          [ 🟢  🟢  🟢  🟢  🟡 ]  System fixed

After Tests:
          [ 🟢  🟢  🟢  🟢  🟢 ]  All good
```

---

## Command Lifecycle State Machine

This diagram shows the **complete command lifecycle** from user click to completion.
This is the canonical reference for all state transitions.

### State Definitions

| Status               | Description                                  | Terminal? |
| -------------------- | -------------------------------------------- | --------- |
| `PENDING`            | Command queued, not yet validated            | No        |
| `VALIDATING`         | Checking preconditions                       | No        |
| `BLOCKED`            | Validation failed, cannot proceed            | Yes       |
| `EXECUTING`          | Command running on agent                     | No        |
| `RUNNING_WARNING`    | Exceeded warning timeout, still running      | No        |
| `TIMEOUT_PENDING`    | Exceeded hard timeout, user action required  | No        |
| `AWAITING_RECONNECT` | Switch completed, waiting for agent restart  | No        |
| `KILLING`            | SIGTERM sent, waiting for process to die     | No        |
| `KILLED`             | Command terminated by user                   | Yes       |
| `SUCCESS`            | Command completed successfully               | Yes       |
| `ERROR`              | Command failed (non-zero exit or validation) | Yes       |
| `TIMEOUT`            | Agent never reconnected after switch         | Yes       |
| `PARTIAL`            | Exit 0 but post-check failed                 | Yes       |
| `STALE_BINARY`       | Agent reconnected with old binary            | Yes       |
| `SUSPICIOUS`         | Source commit changed but binary hash didn't | Yes       |
| `SKIPPED`            | Command cancelled by user                    | Yes       |

### Full State Transition Diagram

```
                              ┌──────────────────────────────────────────────────────────────┐
                              │                    COMMAND LIFECYCLE                          │
                              └──────────────────────────────────────────────────────────────┘

        ┌─────────┐                                                               ┌─────────┐
        │  IDLE   │───────────── User clicks compartment ─────────────────────────▶│ PENDING │
        └─────────┘                                                               └────┬────┘
             ▲                                                                         │
             │                                                                    Validate
             │                                                                         │
             │         ┌──────────────────────────────────────────────────────────────┐│
             │         │                EDGE CASE: Validation fails                    ││
             │         │  - Host offline                                               ││
             │         │  - Another command already running                            ││
             │         │  - Precondition not met                                       ││
             │         └──────────────────────────────────────────────────────────────┘│
             │                           │                                             ▼
             │                           ▼                                       ┌──────────┐
             │◀──── (clearActive) ─────│ BLOCKED  │                              │VALIDATING│
             │                         └──────────┘                              └────┬─────┘
             │                                                                        │
             │                                                                   Send to agent
             │                                                                        │
             │         ┌──────────────────────────────────────────────────────────────┐│
             │         │                EDGE CASE: Agent offline                       ││
             │         │  - WebSocket disconnected                                     ││
             │         │  - Send buffer full                                           ││
             │         └──────────────────────────────────────────────────────────────┘│
             │                           │                                             ▼
             │                           ▼                                       ┌──────────┐
             │◀──── (clearActive) ──┬──│  ERROR   │◀─────── send_failed ─────────│EXECUTING │
             │                      │  └──────────┘                              └────┬─────┘
             │                      │        ▲                                        │
             │                      │        │                                  Agent runs cmd
             │                      │        │                                        │
             │                      │  ┌─────┴────────────────────────────────────────┤
             │                      │  │           TIMEOUT PATH:                      │
             │                      │  │                                              │
             │                      │  │  warning_timeout exceeded                    │
             │                      │  │           │                                  │
             │                      │  │           ▼                                  │
             │                      │  │  ┌─────────────────┐                         │
             │                      │  │  │ RUNNING_WARNING │ (still executing)       │
             │                      │  │  └────────┬────────┘                         │
             │                      │  │           │                                  │
             │                      │  │  hard_timeout exceeded                       │
             │                      │  │           │                                  │
             │                      │  │           ▼                                  │
             │                      │  │  ┌─────────────────┐                         │
             │                      │  │  │ TIMEOUT_PENDING │ → User must act         │
             │                      │  │  └────────┬────────┘                         │
             │                      │  │           │                                  │
             │                      │  │     ┌─────┴─────┐                            │
             │                      │  │     │           │                            │
             │                      │  │   Extend     Kill                            │
             │                      │  │     │           │                            │
             │                      │  │     ▼           ▼                            │
             │                      │  │  (restart   ┌─────────┐                      │
             │                      │  │   watcher)  │ KILLING │                      │
             │                      │  │             └────┬────┘                      │
             │                      │  │                  │                           │
             │                      │  │            SIGKILL fallback                  │
             │                      │  │                  │                           │
             │                      │  │                  ▼                           │
             │                      │  │             ┌────────┐                       │
             │                      │  └─────────────│ KILLED │                       │
             │                      │                └────────┘                       │
             │                      │                     │                           │
             │◀─────────────────────┴─────────────────────┘                           │
             │                                                                        │
             │         ┌──────────────────────────────────────────────────────────────┤
             │         │           STOP PATH (user clicks STOP):                      │
             │         │  1. SIGTERM sent to process group                            │
             │         │  2. 3s grace period                                          │
             │         │  3. SIGKILL if still running                                 │
             │         │  4. → KILLED                                                 │
             │         └──────────────────────────────────────────────────────────────┤
             │                                                                        │
             │         ┌──────────────────────────────────────────────────────────────┤
             │         │           CRASH PATH:                                        │
             │         │  - Agent crashes mid-command                                 │
             │         │  - Agent disconnects (network failure)                       │
             │         │  - Host reboots unexpectedly                                 │
             │         │                                                              │
             │         │  Recovery: staleCommandCleanupLoop runs every 1m             │
             │         │  - Checks: pending_command != NULL AND last_seen stale       │
             │         │  - Clears pending_command, sets status = 'offline'           │
             │         └──────────────────────────────────────────────────────────────┤
             │                                                                        │
             │                                                                   Exit code
             │                                                                    received
             │                                                                        │
             │         ┌──────────────────────────────────────────────────────────────┤
             │         │  BRANCH: exit != 0 → immediate ERROR                         │
             │         │  BRANCH: switch && exit == 0 → AWAITING_RECONNECT            │
             │         │  BRANCH: other && exit == 0 → post-check (deferred)          │
             │         └──────────────────────────────────────────────────────────────┤
             │                           │                         │                   │
             │                      Switch path               Other path               │
             │                           │                         │                   │
             │                           ▼                         ▼                   │
             │                    ┌─────────────┐          ┌─────────────┐             │
             │                    │ AWAITING_   │          │ Post-check  │             │
             │                    │ RECONNECT   │          │ (deferred)  │             │
             │                    └──────┬──────┘          └──────┬──────┘             │
             │                           │                        │                    │
             │         ┌─────────────────┴────────────────────────┤                    │
             │         │  SWITCH VERIFICATION:                    │                    │
             │         │                                          │                    │
             │         │  On agent reconnect:                     │                    │
             │         │  1. Compare PreFreshness vs new          │                    │
             │         │  2. Check: SourceCommit changed?         │                    │
             │         │  3. Check: StorePath changed?            │                    │
             │         │  4. Check: BinaryHash changed?           │                    │
             │         │                                          │                    │
             │         │  Verdicts:                                │                    │
             │         │  - All changed → SUCCESS                  │                    │
             │         │  - Commit changed, hash same → SUSPICIOUS │                    │
             │         │  - Nothing changed → STALE_BINARY         │                    │
             │         │  - Timeout → TIMEOUT                      │                    │
             │         └─────────────────┬────────────────────────┘                    │
             │                           │                                             │
             │           ┌───────────────┼───────────────────────────────────┐         │
             │           │               │                                   │         │
             │           ▼               ▼                                   ▼         ▼
             │     ┌─────────┐    ┌─────────────┐                     ┌─────────┐┌─────────┐
             │◀────│ SUCCESS │    │STALE_BINARY │                     │ PARTIAL ││  ERROR  │
             │     └─────────┘    │ SUSPICIOUS  │                     └─────────┘└─────────┘
             │                    │   TIMEOUT   │                          │          │
             │                    └─────────────┘                          │          │
             │                           │                                 │          │
             │◀──────── clearActive() ───┴─────────────────────────────────┴──────────┘

```

### Edge Cases and Recovery

| Scenario                                | Detection                    | Recovery                                          |
| --------------------------------------- | ---------------------------- | ------------------------------------------------- |
| Agent crashes mid-command               | `last_seen` becomes stale    | `staleCommandCleanupLoop` clears after 10m        |
| Switch succeeds, agent never reconnects | `ReconnectDeadline` exceeded | `watchReconnectTimeout` → TIMEOUT                 |
| Switch completes but binary unchanged   | Freshness comparison         | → STALE_BINARY, user must investigate             |
| User closes browser during command      | N/A                          | Command continues; UI syncs on reconnect          |
| Dashboard restarts during command       | `active` map lost            | Agent reconnect clears pending_command            |
| Network partition                       | WebSocket disconnects        | Agent reconnects; registration clears stale state |

### Invariants (MUST always hold)

1. **Single source of truth**: LifecycleManager's `active` map is authoritative; `hosts.pending_command` is a cache
2. **No orphaned commands**: Every command in `active` must eventually reach a terminal state
3. **No stuck UI**: Every non-terminal state has a timeout or cleanup mechanism
4. **Idempotent reconnect**: Agent re-registration always clears stale `pending_command`
5. **Heartbeats continue**: Agent sends heartbeats even during command execution

---

## Implementation Notes

### Performance

- **Agent status**: Dashboard-side, instant (<1ms)
- **Git status**: Dashboard-side, cached (5s TTL), instant
- **Lock status**: Agent computes hash (~1ms), Dashboard compares
- **System status**: Dashboard-side inference, instant
- **Tests status**: Agent runs tests (~10-60s), results cached

**Total heartbeat overhead**: ~1-2ms (no expensive operations)

### Caching Strategy

```go
type CompartmentCache struct {
    mu            sync.RWMutex
    latestCommit  string
    latestLockHash string
    lastFetch     time.Time
    cacheTTL      time.Duration  // 5 seconds
}
```

### Database Schema

```sql
-- Host state (existing + new columns)
ALTER TABLE hosts ADD COLUMN lock_hash TEXT;
ALTER TABLE hosts ADD COLUMN gen_number INTEGER;
ALTER TABLE hosts ADD COLUMN last_command TEXT;
ALTER TABLE hosts ADD COLUMN last_exit_code INTEGER;
ALTER TABLE hosts ADD COLUMN test_status_json TEXT;
```

**Note:** These can also be derived from the command journal (`commands` table) instead of denormalizing onto `hosts`:

- `last_command`, `last_exit_code`
- `pull_changed_desired`
- `last_successful_switch_at` / `last_successful_switch_generation`
- `last_tests_result_at` / `last_tests_generation`

---

## API

### WebSocket Messages

```json
{
  "type": "host_status_update",
  "payload": {
    "host_id": "gpc0",
    "compartments": {
      "agent": {
        "status": "ok",
        "message": "Agent 3.0.1 (current)"
      },
      "git": {
        "status": "ok",
        "message": "Up to date with origin/main"
      },
      "lock": {
        "status": "ok",
        "message": "flake.lock is current"
      },
      "system": {
        "status": "ok",
        "message": "System matches config"
      },
      "tests": {
        "status": "ok",
        "message": "All tests passed (8/8)"
      }
    }
  }
}
```

---

## Testing

### Unit Tests

```go
func TestCompartmentStates(t *testing.T) {
    // Test all 5 compartments
    // Test state transitions
    // Test inference logic
}
```

### Integration Tests

```go
func TestUpdateFlow(t *testing.T) {
    // Simulate full update flow
    // Verify compartments update correctly
    // Verify state sync broadcasts
}
```

---

## Related Specs

- **CORE-004**: State Sync Protocol (broadcasts compartment updates)
- **CORE-001**: Op Engine (executes operations that change compartments)
- **CORE-003**: State Store (persists compartment state)

---

## Related Backlog Items

- **P5200**: Lock Compartment - Version-Based Tracking
- **P5300**: System Compartment - Inference-Based Status
- **P5400**: Tests Compartment - Fifth Compartment
- **P5500**: Generation Tracking and Visibility
- **P5800**: Compartment State Documentation
