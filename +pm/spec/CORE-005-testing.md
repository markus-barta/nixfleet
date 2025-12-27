# CORE-005: Testing Strategy

> **Spec Type**: Core Building Block  
> **Status**: Stable  
> **Last Updated**: 2025-12-27

---

## Purpose

Defines NixFleet's testing approach: what we test, when, and how. Balances fast iteration with confidence in critical operations.

---

## Testing Layers

| Layer               | Trigger             | Runtime  | Purpose                    |
| ------------------- | ------------------- | -------- | -------------------------- |
| **Smoke tests**     | CI (every push)     | < 30s    | Build works, basic sanity  |
| **Unit tests**      | CI (every push)     | < 2 min  | Core logic, no (heavy) I/O |
| **Integration**     | On-demand / git tag | < 10 min | Real hosts, real agents    |
| **Manual runbooks** | Pre-release         | 30+ min  | Real hosts, critical paths |

---

## Pairing Rule

Every critical Op (see CORE-001) must have **both**:

1. **Automated test** — Shell script or Go test, mocked, fast
2. **Manual runbook** — Markdown checklist, real hosts, thorough

| Op       | Automated Test (NixOS)   | Automated Test (macOS) | Manual Runbook       |
| -------- | ------------------------ | ---------------------- | -------------------- |
| `pull`   | `t02-pull-nixos.sh`      | `t03-pull-macos.sh`    | `M01-pull.md`        |
| `switch` | —                        | —                      | `M02-switch.md`      |
| `test`   | `t04-test-nixos.sh`      | `t05-test-macos.sh`    | `M03-test.md`        |
| `reboot` | —                        | —                      | `M04-reboot-totp.md` |
| `do-all` | `t10-pipeline-do-all.sh` | —                      | `M10-full-deploy.md` |

---

## CI Pipelines

### Fast Path (Every Push)

Runs on every push and PR. Must complete in < 2 minutes.

```yaml
# .github/workflows/ci.yml
on: [push, pull_request]

jobs:
  build-and-smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v24

      - name: Build
        run: nix build .#dashboard .#agent

      - name: Smoke tests
        run: go test -short ./...
```

### Comprehensive (On Tag)

Runs on version tags. Can take up to 10 minutes.

```yaml
# .github/workflows/release.yml
on:
  push:
    tags: ["v*"]

jobs:
  full-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v24

      - name: All unit tests
        run: go test ./...

      - name: Integration tests
        run: ./tests/scripts/run-all.sh
```

---

## Directory Structure

```
tests/
├── scripts/                    # Shell-based integration tests
│   ├── run-all.sh              # Runner: executes all, reports summary
│   ├── lib/                    # Shared helpers
│   │   ├── config.sh           # Test host config (gpc0, imac0)
│   │   └── assert.sh           # Assertion helpers
│   ├── t01-agent-connect.sh    # Test agent connection
│   ├── t02-pull-nixos.sh       # Pull on gpc0 (NixOS)
│   ├── t03-pull-macos.sh       # Pull on imac0 (macOS)
│   └── t10-pipeline-do-all.sh  # Full pipeline on test hosts
│
├── manual/                     # Human/AI runbooks
│   ├── README.md               # How to run manual tests
│   ├── M01-pull.md
│   ├── M02-switch.md
│   ├── M03-test.md
│   ├── M04-reboot-totp.md
│   └── M10-full-deploy.md
│
└── integration/                # Go integration tests (existing)
    ├── helpers_test.go
    └── ...
```

---

## Automated Test Format

Shell scripts with clear structure and exit codes.

```bash
#!/usr/bin/env bash
# t02-pull-nixos.sh - Test pull op on NixOS (gpc0)
set -euo pipefail

source "$(dirname "$0")/lib/config.sh"
source "$(dirname "$0")/lib/assert.sh"

HOST="gpc0"  # NixOS test host

# Prerequisites
assert_host_online "$HOST"
assert_no_pending "$HOST"

# Execute
send_command "$HOST" "pull"
wait_for_completion "$HOST" 120  # 2 min timeout

# Assert
assert_status "$HOST" "SUCCESS"
assert_exit_code "$HOST" 0

echo "✓ t02-pull-nixos passed"
```

### Exit Codes

| Code | Meaning     |
| ---- | ----------- |
| 0    | Test passed |
| 1    | Test failed |
| 2    | Setup error |
| 3    | Timeout     |

---

## Manual Runbook Format

Markdown checklists actionable by human or AI.

```markdown
# M02: Switch

## Prerequisites

- [ ] Host hsb1 is online
- [ ] Lock compartment shows "ok"
- [ ] No pending commands

## Steps

1. Open dashboard at https://localhost:8080
2. Locate host `hsb1` in table
3. Click "Switch" button
4. Monitor output tab for progress

## Expected Results

- [ ] Command starts within 5 seconds
- [ ] Output shows `nixos-rebuild switch` progress
- [ ] Completes within 10 minutes
- [ ] Exit code is 0
- [ ] Agent reconnects (if agent was updated)
- [ ] System compartment shows "ok"

## Actual Results

**Date**: \***\*\_\_\_\*\***  
**Tester**: \***\*\_\_\_\*\***  
**Result**: PASS / FAIL

**Notes**:
_Record any observations, errors, or deviations here_
```

---

## When to Run What

| Scenario                 | Smoke | Unit | Integration | Manual |
| ------------------------ | ----- | ---- | ----------- | ------ |
| Regular development      | ✓     | ✓    | —           | —      |
| Before PR merge          | ✓     | ✓    | Optional    | —      |
| Version tag (release)    | ✓     | ✓    | ✓           | —      |
| Before production deploy | ✓     | ✓    | ✓           | ✓      |
| After major refactor     | ✓     | ✓    | ✓           | ✓      |

---

## Test Hosts

Integration tests run against **real hosts** — no mocked agents.

| Host    | OS    | Purpose                 |
| ------- | ----- | ----------------------- |
| `gpc0`  | NixOS | NixOS integration tests |
| `imac0` | macOS | macOS integration tests |

### Why Real Hosts?

- **No mock maintenance** — mocks drift from reality
- **Real confidence** — tests what users experience
- **Simpler** — no mock infrastructure to maintain

### Prerequisites for Integration Tests

- Test hosts online and connected to dashboard
- Hosts in known-good state (lock ok, no pending commands)
- Network access between test runner and dashboard

### What We Test vs Mock

| Component     | Real in Tests? | Notes                          |
| ------------- | -------------- | ------------------------------ |
| Dashboard     | ✓ Real         | Real binary, test DB           |
| Agent (gpc0)  | ✓ Real         | Real agent on real NixOS host  |
| Agent (imac0) | ✓ Real         | Real agent on real macOS host  |
| GitHub API    | Mock           | httptest server (for merge-pr) |
| Git repo      | ✓ Real         | Real nixcfg repo               |
| Nix builds    | ✓ Real         | Real builds, expect 10-15 min  |

---

## Existing Tests Audit

> TODO: Review existing `tests/integration/` tests against this spec.
> Flag tests that don't align with current architecture.

---

## Implementing Backlog Items

> Updated as backlog items are created/completed.

| Backlog Item | Description                    | Status |
| ------------ | ------------------------------ | ------ |
| (pending)    | Create tests/scripts structure | —      |
| (pending)    | Create tests/manual structure  | —      |
| (pending)    | Audit existing tests           | —      |
| (pending)    | Add CI workflow                | —      |

---

## Changelog

| Date       | Change                        |
| ---------- | ----------------------------- |
| 2025-12-27 | Initial testing strategy spec |
