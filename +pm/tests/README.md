# NixFleet v2 Tests

This directory contains manual test procedures and automated test scripts for verifying NixFleet features.

## Test Types

### Manual Tests (`manual/`)

Step-by-step instructions for humans or AI to verify features. Each test file includes:

- **Preconditions**: What must be true before testing
- **Steps**: Exact actions to perform
- **Expected Results**: What should happen
- **Pass/Fail Criteria**: How to determine success

### Automated Tests (`automated/`)

Shell scripts that can be run to verify features programmatically. These complement the Go integration tests in `v2/tests/integration/`.

## Running Tests

### Manual Tests

Read the test file and follow the steps. Record pass/fail status.

### Automated Tests

```bash
cd +pm/tests/automated
./run-all.sh           # Run all tests
./T-P4350-icons.sh     # Run specific test
```

## Test Coverage

| Feature ID | Feature Name    | Manual | Automated | Status |
| ---------- | --------------- | ------ | --------- | ------ |
| P4350      | SVG Icon System | ✓      | ✓         | Pass   |
| P4385      | Button States   | ✓      | ✓         | Pass   |
| P4395      | Stop Command    | ✓      | ✓         | Pass   |
| P2000      | Hub Resilience  | ✓      | ✓         | Pass   |
