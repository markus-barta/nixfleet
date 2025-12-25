# NixFleet v2.0 - Product Requirements Document

> **Single Source of Truth** for NixFleet development.

---

## Vision

NixFleet is a fleet management system for NixOS and macOS hosts. It enables centralized monitoring, configuration deployment, and testing across a personal infrastructure.

**v2.0 Goal**: Complete rewrite in Go with real-time communication, solving the limitations of the v1.0 prototype.

---

## Problem Statement

### v1.0 Limitations (Python/Bash)

1. **Agent becomes unresponsive during switch** - Bash agent is single-threaded; can't heartbeat while executing commands
2. **No live output** - Users wait blindly during long operations
3. **Mixed technology stack** - Python dashboard + Bash agent = inconsistent patterns
4. **HTTP polling inefficiency** - Constant polling even when idle

### v2.0 Solutions

1. **Go agent with goroutines** - Concurrent heartbeats and command execution
2. **WebSocket streaming** - Real-time output from commands to browser
3. **All-Go stack** - Consistent language, single binaries, easier deployment
4. **WebSocket communication** - Persistent connections, instant updates

---

## Critical Requirement: Agent Resilience

> **This is the #1 requirement of NixFleet. Without resilient agents, the entire system is useless.**

NixFleet's core value proposition is remote fleet management. If the agent dies, goes offline, or fails to restart after updates, the operator loses visibility and control. **This is unacceptable.**

### The Fundamental Promise

**After any operation — switch, pull, reboot, crash, network outage, power failure — the most recent working agent MUST be running and connected to the dashboard within 60 seconds of the host being reachable.**

No exceptions. No "it works most of the time." No manual intervention required.

### Command Lifecycle & State Machine

To ensure this promise, every operation follows a strict state machine with pre- and post-validation.

#### Command Lifecycle State Machine

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                    COMMAND LIFECYCLE STATE MACHINE                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌───────────────┐    ┌───────────────┐    ┌───────────────┐               │
│   │  IDLE         │───▶│  VALIDATING   │───▶│  QUEUED       │               │
│   │               │    │  (pre)        │    │               │               │
│   └───────────────┘    └───────┬───────┘    └───────┬───────┘               │
│          ▲                     │ fail               │                       │
│          │              ┌──────▼──────┐             │                       │
│          │              │  BLOCKED    │             │                       │
│          │              │  (show why) │             ▼                       │
│          │              └─────────────┘    ┌───────────────┐                │
│          │                                 │  RUNNING      │                │
│          │                                 │  + progress   │                │
│          │                                 └───────┬───────┘                │
│          │                                         │                        │
│          │                                         ▼                        │
│          │                                 ┌───────────────┐                │
│          │                                 │  VALIDATING   │                │
│          │                                 │  (post)       │                │
│          │                                 └───────┬───────┘                │
│          │                                         │                        │
│          │         ┌───────────────────────────────┼───────────────┐        │
│          │         ▼                               ▼               ▼        │
│   ┌──────┴────────────┐                ┌───────────────┐  ┌─────────────┐   │
│   │  SUCCESS          │                │  PARTIAL      │  │  FAILED     │   │
│   │  (goal achieved)  │                │  (exit 0 but  │  │  (exit ≠ 0) │   │
│   └───────────────────┘                │  goal not met)│  └─────────────┘   │
│                                        └───────────────┘                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### The Switch Lifecycle (Reconnection-Based)

For `switch` commands, completion is signaled by the agent reconnecting with a fresh binary, rather than a message (as the agent must die to restart).

```text
┌───────────┐   ┌─────────┐   ┌─────────┐   ┌───────────────────┐   ┌─────────────┐
│ VALIDATING│──▶│ QUEUED  │──▶│ RUNNING │──▶│ AWAITING_RECONNECT│──▶│ SUCCESS     │
└───────────┘   └─────────┘   └─────────┘   │ (agent died)      │   │ (fresh bin) │
                                            └─────────┬─────────┘   └─────────────┘
                                                      │             ┌─────────────┐
                                                      └────────────▶│ STALE_BINARY│
                                                                    │ (old bin)   │
                                                                    └─────────────┘
```

### 3-Layer Binary Freshness Detection

To prevent the "silent failure" where a switch succeeds but the agent continues running an old binary (due to Nix cache issues), the dashboard performs a paranoid 3-layer check on reconnection:

| Layer            | Method                         | Detection Goal                      |
| ---------------- | ------------------------------ | ----------------------------------- |
| 1. Source Commit | LDFlags injected at build time | Did the source code change?         |
| 2. Store Path    | Resolved `/proc/self/exe`      | Did the Nix store path change?      |
| 3. Binary Hash   | SHA256 of the running binary   | Did the actual bits on disk change? |

**Verdict Logic:**

- **FRESH**: Store Path or Binary Hash changed.
- **STALE**: Nothing changed (Switch reported success but agent is identical).
- **SUSPICIOUS**: Source Commit changed but Path/Hash didn't (possible build cache issue).

### Timeout & Abort Escalation

Commands have defined timeouts to prevent "forever-running" UI states.

1.  **Warning Threshold**: Dashboard logs a warning that the command is taking longer than expected.
2.  **Hard Timeout**: Dashboard enters `TIMEOUT_PENDING` state.
3.  **User Options**:
    - **Wait**: Extend the timeout (e.g., +5m or +30m).
    - **Kill**: Send SIGTERM to the agent (escalates to SIGKILL).
    - **Ignore**: Mark as ignored and return to IDLE.
    - **Reboot (Nuclear)**: If Kill fails, offer host reboot (requires TOTP).

### Verbose Logging Principles

To ensure every state transition is transparent and auditable, the system follows a "No Silent Transitions" policy:

- **WHAT happened**: The state change or validation result.
- **WHY it happened**: The specific condition or error that triggered it.
- **WHAT'S NEXT**: Expected next step or required user action.

---

### Reboot Integration (P6900)

Forced reboots are integrated into the command state machine as a "nuclear option" for stuck hosts.

1.  **Abortion Logic**: If a reboot is triggered while a command is running, the command is transitioned to `ABORTED_BY_REBOOT` and the pre-state snapshot is cleared.
2.  **Post-Reboot Recovery**: Upon reconnection after a reboot, the dashboard detects the aborted state and logs a warning to the user, suggesting manual verification.
3.  **No Auto-Retry**: The system explicitly does **not** auto-retry aborted commands after recovery to ensure safety and prevent reboot loops.

### Why This Matters

We have repeatedly "fixed" agent issues across the fleet, only to discover they weren't actually fixed. This has happened 10+ times. Each time:

- "Now it's fixed for all hosts" → Still broken on some
- "The agent survives switch now" → Agent runs old binary after switch
- "macOS launchd restarts it" → Agent dead after home-manager switch

This cycle ends now. The requirements below are non-negotiable.

### Resilience Requirements

| ID        | Requirement                            | Acceptance Criteria                                                                                                                                  |
| --------- | -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| **RES-1** | Agent survives its own switch          | Agent continues running during `nixos-rebuild switch` or `home-manager switch`. It does NOT get killed by systemd/launchd during the switch process. |
| **RES-2** | Agent restarts after successful switch | After switch completes (exit 0), agent automatically restarts to run the NEW binary. The old binary does not continue running indefinitely.          |
| **RES-3** | Agent survives reboot                  | After system reboot, agent starts automatically and connects to dashboard.                                                                           |
| **RES-4** | Agent survives crash                   | If agent crashes or is killed (SIGKILL, OOM), systemd/launchd restarts it within 30 seconds.                                                         |
| **RES-5** | Agent survives network outage          | Agent reconnects automatically when network returns. Uses exponential backoff (1s → 2s → 4s → ... → 60s max).                                        |
| **RES-6** | Agent survives dashboard restart       | Agent reconnects automatically when dashboard comes back online.                                                                                     |
| **RES-7** | Agent reports correct generation       | After switch + restart, the agent reports the NEW generation hash, not the old one. Dashboard shows accurate status.                                 |
| **RES-8** | Isolated repo stays clean              | `git pull` via dashboard always succeeds. Uses `reset --hard` to avoid merge conflicts.                                                              |

### Implementation Details

#### NixOS (systemd)

```nix
systemd.services.nixfleet-agent = {
  # CRITICAL: Don't stop/restart during switch - agent is running the switch!
  restartIfChanged = false;
  stopIfChanged = false;

  serviceConfig = {
    Restart = "always";
    RestartSec = 3;
    # Exit code 101 = agent requests restart (after successful switch)
    RestartForceExitStatus = "101";
  };
};
```

**Agent behavior after successful switch:**

1. Send success status to dashboard
2. Wait 500ms for message delivery
3. Exit with code 101
4. systemd restarts with new binary

#### macOS (launchd)

```nix
launchd.agents.nixfleet-agent = {
  config = {
    KeepAlive = true;  # Restart on any exit
    RunAtLoad = true;  # Start on login
  };
};
```

**Agent behavior during home-manager switch:**

1. Run `home-manager switch` in a **new session** (`Setsid: true`)
2. This prevents the agent from being killed when launchd reloads
3. After switch, agent exits and launchd restarts with new binary

### Verification Checklist

Before claiming "agent resilience is fixed," verify ALL of these on EVERY host:

- [ ] **Fresh reboot test**: Reboot host → agent connects within 60s
- [ ] **Switch test**: Click Switch in dashboard → agent survives → agent restarts with new binary
- [ ] **Pull test**: Click Pull → repo updated to origin/main (verify with `git log -1`)
- [ ] **Kill test**: `sudo kill -9 $(pgrep nixfleet-agent)` → agent restarts within 30s
- [ ] **Network test**: Disconnect network 60s → reconnect → agent reconnects
- [ ] **Generation test**: After switch, dashboard shows NEW generation hash

### Known Failure Modes

| Failure                              | Root Cause                                        | Fix                                              |
| ------------------------------------ | ------------------------------------------------- | ------------------------------------------------ |
| Agent runs old binary after switch   | `restartIfChanged=false` prevents systemd restart | Agent self-restarts with exit code 101           |
| Agent dead after home-manager switch | launchd kills agent when reloading plist          | Run switch in new session with `Setsid: true`    |
| Pull doesn't update repo             | Regular `git pull` fails with merge conflicts     | Use `git fetch` + `git reset --hard origin/main` |
| Agent shows old generation           | Agent didn't restart after switch                 | Verify exit code 101 triggers restart            |
| Isolated repo on wrong branch/commit | Repo was manually modified or diverged            | `git reset --hard origin/main` + `git clean -fd` |

### Monitoring

The dashboard should make agent health obvious:

1. **Online/Offline indicator** - Green dot = connected, Red = disconnected
2. **Last seen timestamp** - How long since last heartbeat
3. **Generation mismatch** - Yellow "G" indicator if behind target
4. **Agent version mismatch** - Red "A" indicator if agent binary is outdated

---

## User Stories

### US-1: Fleet Overview

> As a fleet operator, I want to see all my hosts at a glance, so I can quickly identify which need attention.

**Acceptance Criteria:**

- Dashboard shows all registered hosts
- Each host displays: name, status (online/offline/stale), last seen, current generation
- Hosts are sorted by criticality, then name
- Online status updates in real-time (no page refresh needed)

### US-2: Deploy Configuration

> As a fleet operator, I want to deploy configuration changes to a host, so the host runs the latest config.

**Acceptance Criteria:**

- Can trigger "Pull" to update git repository
- Can trigger "Switch" to apply configuration
- Can trigger "Pull + Switch" for convenience
- Command status visible in real-time
- Output streams to dashboard as it happens
- Final result (success/failure) clearly indicated

### US-3: Run Tests

> As a fleet operator, I want to run test suites on hosts, so I can verify they're configured correctly.

**Acceptance Criteria:**

- Can trigger "Test" command
- Progress shown (e.g., "3/10 tests complete")
- Results show pass/fail counts
- Failed test details available

### US-4: Monitor Host Health

> As a fleet operator, I want to see host health metrics, so I can identify resource issues.

**Acceptance Criteria:**

- CPU, RAM, swap usage visible (when StaSysMo available)
- Metrics update with each heartbeat
- Visual indicators for high usage

### US-5: Secure Access

> As a fleet operator, I want the dashboard to be secure, so only I can control my infrastructure.

**Acceptance Criteria:**

- Password authentication required
- Optional TOTP (2FA) support
- Sessions expire after 24 hours
- CSRF protection on all mutations
- Rate limiting on login attempts

### US-6: Agent Resilience

> As a fleet operator, I want agents to stay connected reliably, so I always have visibility and control.

**Acceptance Criteria:**

- Agent reconnects automatically after network issues
- Agent continues heartbeating during long commands
- Agent survives configuration switch (doesn't die mid-operation)
- macOS agent restarts reliably after home-manager switch

### US-7: Update Status (P5000)

> As a fleet operator, I want to see at a glance which hosts need updates, so I can keep my fleet current.

**Acceptance Criteria:**

- Three-compartment indicator shows Git, Lock, and System status
- Git: Compare agent's deployed generation with latest from GitHub Pages
- Lock: Show days since last flake.lock update, indicate pending PR
- System: Compare running system with what current config would build
- Click compartment to refresh that specific check
- Bulk actions: Update All, Pull All, Switch All, Test All

### US-8: Automated Flake Updates (P5300)

> As a fleet operator, I want NixFleet to handle flake.lock updates automatically, so I stay current without manual PR management.

**Acceptance Criteria:**

- Detect pending update PRs via GitHub API
- One-click "Merge & Deploy" to merge PR and switch all hosts
- Optional: Full automation (auto-merge + deploy)
- Rollback on failure
- Per-host inclusion/exclusion settings

---

## Functional Requirements

### FR-1: Agent

| ID      | Requirement                                                 | Priority |
| ------- | ----------------------------------------------------------- | -------- |
| FR-1.1  | Connect to dashboard via WebSocket                          | Must     |
| FR-1.2  | Send heartbeat every 5s (configurable, range 1-3600s)       | Must     |
| FR-1.3  | Continue heartbeats during command execution                | Must     |
| FR-1.4  | Execute commands: pull, switch, test, stop, restart, update | Must     |
| FR-1.5  | Stream command output to dashboard in real-time             | Must     |
| FR-1.6  | Report OS version, nixpkgs version, generation              | Must     |
| FR-1.7  | Report StaSysMo metrics (CPU, RAM, swap, load)              | Should   |
| FR-1.8  | Auto-reconnect with exponential backoff                     | Must     |
| FR-1.9  | Support isolated repo mode (agent-managed git clone)        | Must     |
| FR-1.10 | Support SSH key for git operations                          | Should   |

**FR-1.9 Detail: Isolated Repo Mode**

The agent must maintain its own dedicated repository clone, separate from any user-managed repositories:

| Platform                     | Default Isolated Path                |
| ---------------------------- | ------------------------------------ |
| NixOS (systemd)              | `/var/lib/nixfleet-agent/repo`       |
| macOS (launchd/Home Manager) | `~/.local/state/nixfleet-agent/repo` |

Behavior:

1. **Auto-clone**: If `NIXFLEET_REPO_URL` is set and repo doesn't exist, clone it automatically
2. **Clean slate**: Pull command does `git fetch` + `git reset --hard origin/<branch>` + `git clean -fd` (no merge conflicts)
3. **Exclusive access**: Directory owned by agent, mode 0700
4. **Override**: `NIXFLEET_REPO_DIR` can override default path for backward compatibility
   | FR-1.11 | Track command PID for stop capability | Must |
   | FR-1.12 | Work on NixOS (systemd) and macOS (launchd) | Must |
   | FR-1.13 | Report heartbeat interval on registration | Must |
   | FR-1.14 | Report flake.lock last-modified date | Should |
   | FR-1.15 | Detect if system needs rebuild (compare derivations) | Should |
   | FR-1.16 | Support `flakePath` config for status checks | Should |

### FR-2: Dashboard Backend

| ID      | Requirement                                     | Priority |
| ------- | ----------------------------------------------- | -------- |
| FR-2.1  | Accept WebSocket connections from agents        | Must     |
| FR-2.2  | Accept WebSocket connections from browsers      | Must     |
| FR-2.3  | Authenticate agents via token                   | Must     |
| FR-2.4  | Authenticate users via password + optional TOTP | Must     |
| FR-2.5  | Manage sessions with signed cookies             | Must     |
| FR-2.6  | Store host data in SQLite                       | Must     |
| FR-2.7  | Store command logs in files                     | Must     |
| FR-2.8  | Broadcast host updates to connected browsers    | Must     |
| FR-2.9  | Forward command output from agents to browsers  | Must     |
| FR-2.10 | Rate limit login attempts                       | Must     |
| FR-2.11 | CSRF protection on mutations                    | Must     |
| FR-2.12 | Security headers (HSTS, CSP, X-Frame-Options)   | Must     |
| FR-2.13 | Clear stale pending_command for offline hosts   | Must     |
| FR-2.14 | Fetch nixcfg version from GitHub Pages          | Should   |
| FR-2.15 | Compare agent generation with latest version    | Should   |
| FR-2.16 | Store update status per host                    | Should   |

**FR-2.13 Detail**: Stale command detection uses a multiplier-based threshold following industry patterns (Kubernetes, etcd). Default: `120 × heartbeat_interval` with a 5-minute floor. With 5s heartbeat = 10 minutes. This prevents indefinitely stale UI badges when hosts go offline during commands.

### FR-3: Dashboard Frontend

| ID      | Requirement                                      | Priority |
| ------- | ------------------------------------------------ | -------- |
| FR-3.1  | Display host list with status                    | Must     |
| FR-3.2  | Show real-time updates via WebSocket             | Must     |
| FR-3.3  | Action buttons: Pull, Switch, Test               | Must     |
| FR-3.4  | Show command output in expandable log viewer     | Must     |
| FR-3.5  | Show command progress (building X/Y)             | Should   |
| FR-3.6  | Disable buttons while command running            | Must     |
| FR-3.7  | Show host metrics (CPU, RAM)                     | Should   |
| FR-3.8  | Responsive design (mobile-friendly)              | Should   |
| FR-3.9  | Accessible (keyboard navigation, screen readers) | Should   |
| FR-3.10 | Three-compartment update status indicator        | Should   |
| FR-3.11 | Bulk actions: Update All, Pull All, Switch All   | Should   |
| FR-3.12 | Settings page for configurable intervals         | Could    |

---

## Non-Functional Requirements

### NFR-1: Performance

| ID      | Requirement               | Target  |
| ------- | ------------------------- | ------- |
| NFR-1.1 | Dashboard page load time  | < 500ms |
| NFR-1.2 | WebSocket message latency | < 100ms |
| NFR-1.3 | Agent memory usage        | < 20MB  |
| NFR-1.4 | Dashboard memory usage    | < 100MB |
| NFR-1.5 | Support concurrent hosts  | 50+     |

### NFR-2: Reliability

| ID      | Requirement                | Target                  |
| ------- | -------------------------- | ----------------------- |
| NFR-2.1 | Agent uptime               | 99.9% (when host is up) |
| NFR-2.2 | Dashboard uptime           | 99.9%                   |
| NFR-2.3 | Auto-recovery from crashes | < 30 seconds            |
| NFR-2.4 | Data persistence           | Survive restarts        |

### NFR-3: Security

| ID      | Requirement      | Target                   |
| ------- | ---------------- | ------------------------ |
| NFR-3.1 | Password storage | bcrypt hashed            |
| NFR-3.2 | Session tokens   | Cryptographically random |
| NFR-3.3 | Agent tokens     | Per-host, hashed in DB   |
| NFR-3.4 | TLS              | Required in production   |
| NFR-3.5 | Rate limiting    | 5 login attempts/minute  |

### NFR-4: Deployability

| ID      | Requirement          | Target                  |
| ------- | -------------------- | ----------------------- |
| NFR-4.1 | Dashboard deployment | Single Docker container |
| NFR-4.2 | Agent deployment     | Nix flake module        |
| NFR-4.3 | Configuration        | Environment variables   |
| NFR-4.4 | Upgrade path         | Zero-downtime           |

---

## Technical Architecture

### Technology Stack

| Component          | Technology                    |
| ------------------ | ----------------------------- |
| Agent              | Go 1.24+                      |
| Dashboard Backend  | Go 1.24+ with Chi router      |
| Dashboard Frontend | Templ + HTMX + Alpine.js      |
| Database           | SQLite                        |
| Communication      | WebSocket (gorilla/websocket) |
| Authentication     | bcrypt + TOTP (pquerna/otp)   |

### Component Diagram

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                          NixFleet Dashboard (Go)                         │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │   Router    │  │   Auth      │  │   WebSocket │  │   Templ      │   │
│  │   (Chi)     │  │   Middleware│  │   Hub       │  │   Templates  │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────────────┘   │
│                              │                                          │
│  ┌───────────────────────────┴───────────────────────────────────────┐ │
│  │                     SQLite + File Store                            │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │ WebSocket     │ WebSocket     │ WebSocket
              ▼               ▼               ▼
         Go Agent        Go Agent         Browser
         (NixOS)         (macOS)          (HTMX)
```

### Message Protocol

```json
// Agent → Dashboard
{"type": "register", "payload": {"hostname": "hsb0", ...}}
{"type": "heartbeat", "payload": {"metrics": {...}}}
{"type": "output", "payload": {"line": "building...", "command": "switch"}}
{"type": "status", "payload": {"status": "ok", "generation": "abc123"}}

// Dashboard → Agent
{"type": "command", "payload": {"command": "switch"}}
{"type": "ping"}

// Dashboard → Browser
{"type": "host_update", "payload": {"host_id": "hsb0", ...}}
{"type": "command_output", "payload": {"host_id": "hsb0", "line": "..."}}
```

---

## Success Metrics

| Metric                        | Target     | How to Measure        |
| ----------------------------- | ---------- | --------------------- |
| All integration tests pass    | 100%       | CI pipeline           |
| Agent heartbeat during switch | Continuous | Test with long switch |
| Output streaming latency      | < 1 second | Manual test           |
| Successful deployments        | 100%       | Deployment logs       |

---

## Out of Scope (v2.0)

- Multi-user support (single admin only)
- Role-based access control
- Scheduled commands
- Host grouping/tags
- Audit logging to external system
- High availability (single instance)
- nix-darwin support (macOS Home Manager only for now — see P5400)

---

## References

- [Integration Test Specs](../tests/specs/) - Executable specifications
- [Backlog](./backlog/) - Implementation tasks (P-numbered)
- [v1.0 Codebase](../app/, ../agent/) - Reference implementation

---

## Changelog

| Date       | Version | Changes                                                                |
| ---------- | ------- | ---------------------------------------------------------------------- |
| 2025-12-17 | 1.4     | **CRITICAL**: Added Agent Resilience section as #1 requirement         |
|            |         | RES-1 through RES-8: Non-negotiable resilience requirements            |
|            |         | Added verification checklist for agent fixes                           |
|            |         | Documented known failure modes and fixes                               |
|            |         | Added monitoring requirements for agent health                         |
| 2025-12-16 | 1.3     | FR-1.9: Added detailed isolated repo mode spec (P5500)                 |
|            |         | Specified default paths, auto-clone, clean-slate behavior              |
| 2025-12-15 | 1.2     | US-7, US-8: Added Update Status and Automated Flake Updates stories    |
|            |         | FR-1.14-16: Agent update status checks (flakePath, derivation compare) |
|            |         | FR-2.14-16: Dashboard GitHub Pages integration for version compare     |
|            |         | FR-3.10-12: Update status UI, bulk actions, settings page              |
|            |         | Added nix-darwin to Out of Scope (P5400 for future)                    |
| 2025-12-15 | 1.1     | FR-1.2: Fixed heartbeat from 30s to 5s (matching implementation)       |
|            |         | FR-1.13: Added heartbeat interval reporting requirement                |
|            |         | FR-2.13: Added stale command cleanup requirement (multiplier-based)    |
| 2025-12-14 | 1.0     | Initial PRD for v2.0 rewrite                                           |
