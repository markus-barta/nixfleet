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
├── specs/                    # Human-readable test specifications
│   ├── T01-agent-connection.md
│   ├── T02-agent-heartbeat.md
│   ├── T03-agent-commands.md
│   ├── T04-dashboard-auth.md
│   ├── T05-dashboard-websocket.md
│   ├── T06-dashboard-commands.md
│   ├── T07-e2e-deploy-flow.md
│   └── T08-e2e-test-flow.md
└── integration/              # Executable Go tests
    ├── agent_test.go
    ├── dashboard_test.go
    └── e2e_test.go
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
# Run all tests
go test ./tests/integration/...

# Run specific test file
go test ./tests/integration/agent_test.go

# Run with verbose output
go test -v ./tests/integration/...

# Run specific test by name
go test -run TestAgentConnection ./tests/integration/...
```

---

## Test Status

| Spec                    | Status             | Backlog Item |
| ----------------------- | ------------------ | ------------ |
| T01-agent-connection    | 🔴 Not Implemented | P4000        |
| T02-agent-heartbeat     | 🔴 Not Implemented | P4000        |
| T03-agent-commands      | 🔴 Not Implemented | P4000        |
| T04-dashboard-auth      | 🔴 Not Implemented | P4200        |
| T05-dashboard-websocket | 🔴 Not Implemented | P4200        |
| T06-dashboard-commands  | 🔴 Not Implemented | P4200        |
| T07-e2e-deploy-flow     | 🔴 Not Implemented | P4200        |
| T08-e2e-test-flow       | 🔴 Not Implemented | P4200        |

Legend: 🟢 Passing | 🟡 Partial | 🔴 Not Implemented

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
