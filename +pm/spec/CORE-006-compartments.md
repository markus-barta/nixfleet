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

| Color         | Meaning               | When to Use       | Action Required      |
| ------------- | --------------------- | ----------------- | -------------------- |
| 🟢 **Green**  | Current / Passed      | Everything OK     | None                 |
| 🟡 **Yellow** | Outdated / Pending    | Update needed     | Pull / Switch / Test |
| 🔴 **Red**    | Failed / Error        | Something broke   | Fix / Rollback       |
| 🔵 **Blue**   | Working / In progress | Operation running | Wait                 |
| ⚪ **Gray**   | Unknown / Disabled    | No data yet       | Configure / Wait     |

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
    if latestCommit == "" {
        return "unknown"  // Gray: can't fetch from GitHub
    }
    if hostGeneration == latestCommit {
        return "ok"  // Green: up to date
    }
    return "outdated"  // Yellow: behind
}
```

| State      | Color | Meaning                         | Action                  |
| ---------- | ----- | ------------------------------- | ----------------------- |
| `ok`       | 🟢    | Local repo matches origin/main  | None                    |
| `outdated` | 🟡    | Local repo behind               | Pull                    |
| `unknown`  | ⚪    | Cannot fetch latest from GitHub | Check GitHub API config |

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
Git compartment: 🟢/🟡/⚪
```

---

## Compartment 3: Lock

### Purpose

Checks if flake.lock is current (dependencies up to date).

### States

```go
func computeLockStatus(hostLockHash string, latestLockHash string) string {
    if latestLockHash == "" {
        return "unknown"  // Gray: can't fetch latest
    }
    if hostLockHash == latestLockHash {
        return "ok"  // Green: current
    }
    return "outdated"  // Yellow: outdated
}
```

| State      | Color | Meaning                      | Action                          |
| ---------- | ----- | ---------------------------- | ------------------------------- |
| `ok`       | 🟢    | flake.lock matches latest    | None                            |
| `outdated` | 🟡    | flake.lock outdated          | Pull (if PR merged) or Merge PR |
| `unknown`  | ⚪    | Cannot determine lock status | Check configuration             |

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
Lock compartment: 🟢/🟡/⚪
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

```go
func inferSystemStatus(host Host) string {
    // If Lock outdated, System MUST be outdated
    if host.Lock.Status == "outdated" {
        return "outdated"
    }

    // Infer from last command
    switch {
    case host.LastCommand == "pull" && host.LastExitCode == 0:
        return "outdated"  // Pulled new config, need switch
    case host.LastCommand == "switch" && host.LastExitCode == 0:
        return "ok"  // Successfully applied
    case host.LastCommand == "switch" && host.LastExitCode != 0:
        return "error"  // Switch failed
    case host.Lock.Status == "ok":
        return "ok"  // Assume current
    default:
        return "unknown"  // First heartbeat
    }
}
```

| State      | Color | Meaning                           | Action                 |
| ---------- | ----- | --------------------------------- | ---------------------- |
| `ok`       | 🟢    | System matches config             | None                   |
| `outdated` | 🟡    | System needs rebuild              | Switch                 |
| `error`    | 🔴    | Switch failed                     | Check logs, fix config |
| `unknown`  | ⚪    | Status unknown (first connection) | Wait for first command |

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

---

## Compartment 5: Tests

### Purpose

Verifies system is actually working after deployment.

### States

| State      | Color | Meaning           | Action                  |
| ---------- | ----- | ----------------- | ----------------------- |
| `ok`       | 🟢    | All tests passed  | None                    |
| `outdated` | 🟡    | Tests not run yet | Run tests               |
| `error`    | 🔴    | Tests failed      | Rollback or fix         |
| `working`  | 🔵    | Tests running     | Wait                    |
| `unknown`  | ⚪    | Tests disabled    | Enable tests (optional) |

### Data Flow

```
User runs Switch (exit 0)
         ↓
System compartment: 🟢
Tests compartment: 🟡 (not run yet)
         ↓
[If auto-run enabled]
Test command dispatched
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

| State  | Click Response                               |
| ------ | -------------------------------------------- |
| Gray   | "Checking GitHub..."                         |
| Green  | "Git current (abc123)" → show commit details |
| Yellow | "2 commits behind" → trigger Pull            |
| Blue   | "Pulling..." → show progress, offer Stop     |
| Red    | "Pull failed" → show error, offer retry      |

#### Lock Compartment

| State  | Click Response                            |
| ------ | ----------------------------------------- |
| Gray   | "Checking flake.lock..."                  |
| Green  | "Lock current (hash matches)" → show hash |
| Yellow | "Lock outdated" → trigger Pull            |
| Blue   | "Refreshing..." → show progress           |
| Red    | "Lock check failed" → show error          |

#### System Compartment (INFERENCE ONLY)

> **⚠️ CRITICAL**: System compartment does NOT trigger actions.
> Status is inferred from command results and lock state.
> Click shows information only — no refresh, no switch trigger.

| State  | Click Response                                              |
| ------ | ----------------------------------------------------------- |
| Gray   | "System status unknown" (info)                              |
| Green  | "System current (gen abc123)" (info)                        |
| Yellow | "System outdated — needs switch" + WHY it's outdated (info) |
| Blue   | "Switching..." → show progress, offer Stop                  |
| Red    | "Switch failed" → show error (info)                         |

**Why no action?** Running `nix build --dry-run` to check system status takes 30-60+ seconds and consumes significant resources. Instead, we infer status from:

- Lock outdated → System MUST be outdated
- Last command was `pull` (exit 0) → System outdated
- Last command was `switch` (exit 0) → System current

#### Tests Compartment

| State  | Click Response                                 |
| ------ | ---------------------------------------------- |
| Gray   | "Tests not configured" (info)                  |
| Green  | "All tests passed" → show test results         |
| Yellow | "Tests not run yet" → trigger Test             |
| Blue   | "Tests running..." → show progress, offer Stop |
| Red    | "Tests failed" → show failures, offer retry    |

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
