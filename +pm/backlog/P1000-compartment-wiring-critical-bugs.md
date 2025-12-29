# P1000: Compartment Wiring Critical Bugs

**Priority**: P1000 (Critical - blocking deployment)  
**Type**: Bug  
**Status**: Superseded by P1100  
**Epic**: P5000 Compartment System

> ⚠️ **SUPERSEDED**: This item has been expanded into [P1100 - Compartment State Machine Overhaul](./P1100-compartment-state-machine-overhaul.md) which includes a complete state machine specification and all identified issues.

## Summary

Multiple compartments are broken despite appearing complete in code. The entire compartment data pipeline has wiring issues from source through display.

## Issues Identified

### Issue 1: Lock Compartment - No lockHash in version.json

**Symptom**: Lock compartment shows "no lockHash in version.json"

**Root Cause Analysis**:

- `version.json` on GitHub Pages confirmed to be **missing `lockHash` field**
- Current content:
  ```json
  {
    "gitCommit": "176e721a5e037373ce342e8ee76ca88705026412",
    "message": "chore: bump nixfleet to v3.1.1 (213f261)\n",
    "branch": "main",
    "timestamp": "2025-12-28T18:05:20Z",
    "repo": "markus-barta/nixcfg"
  }
  ```
- Workflow files (`version-pages.yml`, `update-nixfleet.yml`) have the code to generate lockHash
- **Hypothesis**: Either:
  1. Shell variable expansion in heredoc not working in GitHub Actions
  2. Workflow didn't re-run after lockHash was added
  3. Caching issue with GitHub Pages

**Questions for User**:

- When was the last time `version-pages.yml` ran? Was it before or after lockHash was added?
- Can we manually trigger the workflow to verify it works?

---

### Issue 2: Lock Compartment - "Switch already running" when no command pending

**Symptom**: Clicking Lock compartment says "switch is already running" when no command is actually running

**Root Cause Analysis**:

- Error comes from `lifecycle.go:155` or `registry.go` validation
- `host.HasPendingCommand()` returns true when it shouldn't
- Database `pending_command` column likely not being cleared after command completes
- Possible scenarios:
  1. Agent completed command but status update didn't clear `pending_command`
  2. Agent disconnected mid-command and `pending_command` never got cleared
  3. StateManager not syncing `pending_command = null` properly

**Questions for User**:

- Which host is this happening on? (So we can check database)
- Did any commands recently fail or get interrupted?
- Can you check the dashboard database for that host's `pending_command` value?

---

### Issue 3: Git/Gen Compartment - "Waiting for heartbeat"

**Symptom**: Shows "waiting for heartbeat" despite heartbeats clearly arriving

**Root Cause Analysis**:

- Message source unclear (not found in codebase - user may be paraphrasing)
- Possible scenarios:
  1. `generation` field in heartbeat payload is empty
  2. `host.Generation` not being populated on initial state load
  3. Database `generation` column is null
  4. VersionFetcher returning "unknown" status due to empty generation

**Questions for User**:

- What is the exact hover/click message text?
- Is this for all hosts or specific hosts?
- Are the hosts' Git compartments colored (green/yellow) or gray?

---

### Issue 4: Tests Compartment - "Tests not configured"

**Symptom**: Tests compartment says "not configured" but tests exist and run

**Root Cause Analysis**:

- `testsContextDescription()` returns "Tests not configured" when `check.Status` is empty/unknown
- Data pipeline issue:
  1. Agent: `statusChecker.SetTestsOk/Error/Outdated()` methods exist
  2. Agent: `GetStatus()` includes `Tests: s.testsStatus`
  3. **BUT**: `testsStatus` starts as zero value (empty Status field)
  4. Agent never initializes testsStatus until after first test run
  5. Hub now broadcasts `tests` in updateStatus (fixed in last commit)
- **Likely issue**: Tests status is never initialized on agent startup
  - Agent boots → testsStatus is `{Status:"", Message:""}`
  - Heartbeat sends this → Dashboard shows "not configured"
  - Only after running `test` command does status change

**Questions for User**:

- Have you run a `test` command on any host since the last agent restart?
- If you run `test` on a host, does the compartment update correctly afterward?

---

## Investigation Plan

1. **Check GitHub Actions**: Manually trigger `version-pages.yml` and verify lockHash appears
2. **Check Database**: Query `SELECT hostname, pending_command, generation FROM hosts` to see state
3. **Check Agent Logs**: Look for heartbeat payload to verify what's being sent
4. **Check Browser Console**: Look for WebSocket messages to verify what dashboard receives

## Files Involved

- `nixcfg/.github/workflows/version-pages.yml` - lockHash generation
- `nixcfg/.github/workflows/update-nixfleet.yml` - lockHash generation
- `src/internal/dashboard/hub.go` - heartbeat processing
- `src/internal/dashboard/state_provider.go` - initial state loading
- `src/internal/agent/status.go` - testsStatus initialization
- `src/internal/agent/heartbeat.go` - heartbeat payload construction
- `src/internal/ops/lifecycle.go` - pending command tracking

## Dependencies

- None (blocking other work)

## Notes

This appears to be a systemic issue with the data pipeline rather than individual component bugs. The code exists but data isn't flowing correctly from source to display.
