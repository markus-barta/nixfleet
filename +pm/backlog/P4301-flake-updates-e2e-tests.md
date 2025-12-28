# P4301 - Flake Updates E2E Test Suite

**Created**: 2025-12-17  
**Updated**: 2025-12-28  
**Priority**: P6100 (🟢 Medium Priority - Sprint 3)  
**Status**: Backlog  
**Depends on**: P4300 (Automated Flake Lock Updates) - ✅ Done, P5700 (Merge PR Workflow)

**⚠️ Note**: This spec needs updating after P5700 is implemented. The workflow has changed from "Merge & Deploy" to manual deployment control.

---

## User Story

**As a** developer  
**I want** automated E2E tests for the flake update flow  
**So that** I can refactor and extend P4300 with confidence

---

## Overview

Build comprehensive test infrastructure for P4300 with mock GitHub API and mock agents.

This is split from P4300 to avoid blocking the feature implementation with complex test infrastructure.

---

## Scope

### MockGitHub Server

```go
type MockGitHubAPI struct {
    server     *httptest.Server
    pendingPRs []MockPR
    mergedPRs  []int
}

// Simulates:
// - GET /repos/{owner}/{repo}/pulls
// - PUT /repos/{owner}/{repo}/pulls/{number}/merge
// - GET /repos/{owner}/{repo}/commits/{branch}
```

### MockAgent

```go
type MockAgent struct {
    hostname     string
    ws           *websocket.Conn
    receivedCmds []string
}

// Simulates:
// - WebSocket connection to dashboard
// - Receiving commands (pull, switch)
// - Sending command results
// - Heartbeats
```

### Test Cases

1. **Happy Path**: Merge → Pull all → Switch all → Success
2. **Failure Handling**: Agent fails switch → deployment stops
3. **Partial Deployment**: Offline hosts excluded
4. **Rollback**: Failed switch triggers revert (if enabled)
5. **Canary**: First host deploys, waits, then rest

---

## Acceptance Criteria

- [ ] MockGitHubAPI server with configurable responses
- [ ] MockAgent that can connect to real dashboard
- [ ] `t13_flake_updates_test.go` with all test cases
- [ ] Tests run in CI (with `go test ./tests/integration/...`)
- [ ] Documentation for running tests locally

---

## Notes

- Consider using `httptest` for MockGitHub
- Reuse patterns from existing `MockDashboard` in `helpers_test.go`
- May need dashboard to accept `NIXFLEET_GITHUB_API_URL` override for testing
