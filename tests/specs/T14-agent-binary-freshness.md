# T14 - Agent Binary Freshness Detection (P2810)

**Backlog**: P2810 (Agent Binary Freshness Detection)
**Priority**: Must Have
**Status**: 🟡 Pending

---

## Purpose

Verify that the system correctly detects when the agent binary is outdated, even after a successful switch operation. This prevents the silent failure where switch succeeds but old agent binary continues running.

---

## Prerequisites

- Agent built with SourceCommit ldflag
- Dashboard knows expected nixfleet source commit
- Host state tracking with AgentOutdated flag

---

## Test Structure

### Unit Tests (Commit Comparison)

Test commit comparison logic in isolation.

### Integration Tests (Agent Reporting)

Test agent reports SourceCommit in heartbeat.

### Integration Tests (Dashboard Detection)

Test dashboard detects outdated agent binary.

### E2E Tests (Full Detection Flow)

Test complete flow from switch to detection.

---

## Scenarios

### Scenario 1: Agent Reports Source Commit

**Given** agent built with SourceCommit ldflag
**When** agent sends heartbeat
**Then** SourceCommit included in payload

**Test Cases**:

- ✅ SourceCommit set at build time → included in RegisterPayload
- ✅ SourceCommit set at build time → included in HeartbeatPayload
- ✅ SourceCommit "unknown" if not set → still included

**Verification**:

```go
// In agent registration
payload := protocol.RegisterPayload{
    // ... other fields ...
    SourceCommit: agent.SourceCommit, // From build-time ldflag
}

// In heartbeat
payload := protocol.HeartbeatPayload{
    // ... other fields ...
    SourceCommit: agent.SourceCommit,
}
```

### Scenario 2: Dashboard Stores Expected Commit

**Given** dashboard knows nixfleet flake input
**When** dashboard starts
**Then** expected source commit stored

**Test Cases**:

- ✅ Dashboard reads NIXFLEET_SOURCE_COMMIT env var
- ✅ Dashboard computes from flake.lock if env not set
- ✅ Dashboard stores expected commit per host

**Verification**:

- Expected commit available via `dashboard.GetExpectedSourceCommit()`
- Stored in host state or config

### Scenario 3: Dashboard Compares Commits

**Given** agent reports SourceCommit and dashboard knows expected
**When** heartbeat received
**Then** dashboard compares and sets AgentOutdated flag

**Test Cases**:

- ✅ Commits match → AgentOutdated = false
- ❌ Commits differ → AgentOutdated = true
- ✅ Agent commit "unknown" → AgentOutdated = false (graceful)
- ✅ Expected commit "unknown" → AgentOutdated = false (graceful)

**Verification**:

```go
func (d *Dashboard) checkAgentFreshness(host *Host, agentCommit string) {
    expected := d.GetExpectedSourceCommit()
    if expected != "" && agentCommit != "" && expected != agentCommit {
        host.AgentOutdated = true
    } else {
        host.AgentOutdated = false
    }
}
```

### Scenario 4: Post-Switch Validation Detects Unchanged Binary

**Given** switch completes successfully
**When** post-validation runs
**Then** unchanged agent binary detected

**Test Cases**:

- ✅ Agent version changed → normal success
- ⚠️ Agent version unchanged but expected commit updated → warning
- ⚠️ Agent version unchanged, expected commit updated, store path unchanged → warning

**Verification**:

```go
func ValidateSwitchResult(before, after HostSnapshot, exitCode int) ValidationResult {
    // ... existing checks ...

    // P2810: Check if agent binary was updated
    if before.AgentVersion == after.AgentVersion && expectedCommitUpdated {
        return ValidationResult{
            Valid: true,
            Code: "agent_binary_not_updated",
            Message: "Switch completed but agent binary not rebuilt (may be cached)",
        }
    }
}
```

### Scenario 5: UI Shows Agent Outdated Indicator

**Given** AgentOutdated = true
**When** dashboard renders host
**Then** agent outdated indicator shown

**Test Cases**:

- ✅ AgentOutdated flag set → red "A" badge shown
- ✅ Tooltip: "Agent binary outdated - run switch again or garbage collect"
- ✅ AgentOutdated flag cleared → badge removed

**Verification**:

- UI component checks `host.AgentOutdated`
- Badge displayed in host row
- Tooltip provides actionable guidance

### Scenario 6: E2E - Stale Binary Detection

**Given** host with outdated agent binary
**When** switch completes
**Then** system detects and warns

**Test Cases**:

1. **Setup**: Agent running old binary (commit abc123)
2. **Action**: User updates nixcfg flake.lock to new nixfleet (commit def456)
3. **Action**: User triggers switch
4. **Result**: Switch completes (exit 0)
5. **Detection**: Post-validation detects agent version unchanged
6. **Warning**: UI shows "Agent binary not updated - may be cached"

**Verification**:

- Pre-switch: AgentOutdated = false (not yet detected)
- Post-switch: AgentOutdated = true (detected)
- Log entry: "Agent binary unchanged after switch"
- UI warning shown

### Scenario 7: E2E - Fresh Binary After Switch

**Given** host with outdated agent binary
**When** switch completes and agent restarts
**Then** system detects fresh binary

**Test Cases**:

1. **Setup**: Agent running old binary (commit abc123)
2. **Action**: User triggers switch
3. **Result**: Switch completes, agent restarts
4. **Detection**: Next heartbeat shows new SourceCommit
5. **Result**: AgentOutdated = false

**Verification**:

- Pre-switch: AgentOutdated = true
- Post-switch: AgentOutdated = false (after restart)
- Log entry: "Agent updated to commit def456"

### Scenario 8: Edge Case - Unknown Commits

**Given** agent or dashboard doesn't know commit
**When** comparison runs
**Then** graceful handling

**Test Cases**:

- ✅ Agent commit "unknown" → no error, AgentOutdated = false
- ✅ Expected commit "unknown" → no error, AgentOutdated = false
- ✅ Both unknown → no error, AgentOutdated = false

**Verification**:

- No panic on unknown commits
- Graceful fallback to version comparison if available

### Scenario 9: Edge Case - Commit Comparison Edge Cases

**Given** various commit scenarios
**When** comparison runs
**Then** correct detection

**Test Cases**:

- ✅ Empty strings → treated as unknown
- ✅ Partial commits (7 chars) → compared correctly
- ✅ Full commits (40 chars) → compared correctly
- ✅ Case sensitivity → case-sensitive comparison

---

## Implementation Verification

### Agent Build Verification

**Verify** agent package sets SourceCommit ldflag:

```nix
# packages/nixfleet-agent-v2.nix
ldflags = [
  "-s" "-w"
  "-X main.Version=${version}"
  "-X main.SourceCommit=${src.rev or "unknown"}"  # P2810
];
```

**Test**:

```bash
# Build agent
nix build .#packages.x86_64-linux.nixfleet-agent

# Verify binary has SourceCommit
strings result/bin/nixfleet-agent | grep -i "sourcecommit"
```

### Dashboard Config Verification

**Verify** dashboard knows expected commit:

```go
// Option 1: Environment variable
expectedCommit := os.Getenv("NIXFLEET_SOURCE_COMMIT")

// Option 2: Compute from flake.lock
expectedCommit := computeFromFlakeLock()
```

---

## Acceptance Criteria

- [ ] Agent reports SourceCommit in RegisterPayload
- [ ] Agent reports SourceCommit in HeartbeatPayload
- [ ] Dashboard stores expected source commit
- [ ] Dashboard compares agent commit to expected
- [ ] Dashboard sets AgentOutdated flag correctly
- [ ] Post-switch validation detects unchanged binary
- [ ] UI shows agent outdated indicator
- [ ] E2E tests pass for stale binary detection
- [ ] E2E tests pass for fresh binary detection
- [ ] Edge cases handled gracefully

---

## Related Issues

- **Nix Binary Cache**: Sometimes cache.nixos.org serves stale binaries
- **Workaround**: `nix build --no-substitute` or `nix-collect-garbage -d`
- **Detection**: This test suite ensures we detect when this happens

---

## Verification Commands

```bash
# Run all P2810 tests
cd v2 && go test -v ./tests/integration/... -run "T14"

# Run specific test
cd v2 && go test -v ./tests/integration/... -run "TestT14_AgentReportsSourceCommit"

# Verify agent binary has SourceCommit
nix build .#packages.x86_64-linux.nixfleet-agent
strings result/bin/nixfleet-agent | grep SourceCommit
```
