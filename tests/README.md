# NixFleet Integration Tests

> **Test-Driven Development**: Tests define expected behavior. Backlog items make tests pass.

---

## Philosophy

1. **Tests First**: Write test specs before implementation
2. **Human-Readable**: Specs in Markdown for humans and AI
3. **Executable**: Go tests implement the specs
4. **Main Flows First**: Cover critical paths, then edge cases

---

## Structure

```text
tests/
├── README.md                 # This file
└── specs/                    # Human-readable test specifications
    ├── T01-agent-connection.md
    ├── T02-agent-heartbeat.md
    ├── T03-agent-commands.md
    ├── T04-dashboard-auth.md
    ├── T05-dashboard-websocket.md
    ├── T06-dashboard-commands.md
    ├── T07-e2e-deploy-flow.md
    └── T08-e2e-test-flow.md

v2/tests/integration/         # Executable Go tests
├── helpers_test.go           # Mock dashboard, test utilities
├── t01_connection_test.go    # T01 implementation
├── t02_heartbeat_test.go     # T02 implementation
└── t03_commands_test.go      # T03 implementation
```

---

## Test Categories

### Agent Tests (T01-T03)

Test the Go agent in isolation (mock dashboard):

- Connection and reconnection
- Heartbeat behavior
- Command execution and output streaming

### Dashboard Tests (T04-T06)

Test the Go dashboard in isolation (mock agents):

- Authentication flow
- WebSocket handling
- Command dispatch

### End-to-End Tests (T07-T08)

Test agent + dashboard together:

- Full deployment flow (pull → switch → verify)
- Full test flow (trigger → progress → results)

---

## Running Tests

```bash
# With devenv (recommended):
test-agent                              # Run all agent tests
test-agent -run TestAgentConnection     # Run specific test

# Or directly with Go:
cd v2 && go test ./tests/integration/... -v

# Run specific test by name:
cd v2 && go test -run TestAgentHeartbeat_DuringCommand ./tests/integration/...
```

---

## Test Status

| Spec                    | Status     | Tests | Backlog Item |
| ----------------------- | ---------- | ----- | ------------ |
| T01-agent-connection    | 🟢 Passing | 5     | P4000        |
| T02-agent-heartbeat     | 🟢 Passing | 5     | P4000        |
| T03-agent-commands      | 🟢 Passing | 5     | P4000        |
| T04-dashboard-auth      | 🟢 Passing | 7     | P4200        |
| T05-dashboard-websocket | 🟢 Passing | 6     | P4200        |
| T06-dashboard-commands  | 🟢 Passing | 6     | P4200        |
| T07-e2e-deploy-flow     | 🟡 Skipped | 2     | P4200        |
| T08-e2e-test-flow       | 🔴 Pending | 0     | P4200        |

Legend: 🟢 Passing | 🟡 Skipped (needs env) | 🔴 Pending

### Running E2E Tests

E2E tests require environment variables:

```bash
# Against production v1 dashboard
E2E_DASHBOARD_URL=wss://fleet.barta.cm/ws \
E2E_PASSWORD='your-password' \
E2E_HOSTS=mba-mbp-work \
go test -v ./v2/tests/integration/... -run TestE2E

# Against local v2 dashboard
E2E_DASHBOARD_URL=ws://localhost:8000/ws \
E2E_PASSWORD='test-password' \
E2E_HOSTS=mba-mbp-work,hsb1 \
go test -v ./v2/tests/integration/... -run TestE2E
```

See `v2/tests/integration/t07_e2e_deploy_test.go` for details.

---

## Writing New Tests

1. **Create spec in `specs/`** with human-readable scenarios
2. **Create or update Go test** in `integration/`
3. **Link to backlog item** that will make it pass
4. **Update status table** above

### Spec Template

```markdown
# T99 - Feature Name

## Purpose

What this test verifies.

## Prerequisites

- Required setup
- Mock data needed

## Scenarios

### Scenario 1: Happy Path

**Given** initial state
**When** action happens
**Then** expected result

### Scenario 2: Error Case

**Given** error condition
**When** action happens
**Then** error handled gracefully

## Verification Commands

\`\`\`bash

# Commands to manually verify

\`\`\`
```

---

## CI Integration

Tests run on every PR:

```yaml
# .github/workflows/test.yml
- name: Run Integration Tests
  run: go test -v ./tests/integration/...
```

---

## Related

- [PRD](../+pm/PRD.md) - Product requirements (source of truth)
- [Backlog](../+pm/backlog/) - Implementation tasks
