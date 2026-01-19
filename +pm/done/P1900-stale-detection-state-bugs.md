# P1900: Stale Detection State Machine Bugs

**Priority**: P1 (Critical - Correctness)
**Status**: ✅ COMPLETE
**Created**: 2026-01-19
**Completed**: 2026-01-19
**Type**: Bug
**References**: P1110 (Stale Status Detection)
**Commits**: fab5b7c, fc537fc, fd0f7a1

---

## Summary

P1110 stale detection has critical state machine bugs causing false positives and duplicate operation messages.

---

## Critical Issues

### 1. Agent: lastStatusUpdate Never Reset on Success ❌

**Location**: `src/internal/agent/status.go`

**Problem**: When status transitions from "working" to "ok"/"outdated"/"error", `lastStatusUpdate` is NOT reset. This causes false stale detection on rapid state cycles.

**Scenario**:

```
T+0s:   SetSystemWorking() → lastStatusUpdate = T+0s
T+30s:  Switch completes → SetSystemOk() → lastStatusUpdate STILL T+0s ❌
T+60s:  User clicks switch again → SetSystemWorking() → lastStatusUpdate = T+60s
T+90s:  detectStaleStatus() sees elapsed = 90s from FIRST working state
T+301s: FALSE POSITIVE - triggers stale even though current switch is only 241s old
```

**Code**:

```go
// Line 196: Sets lastStatusUpdate when entering "working"
func (s *StatusChecker) SetSystemWorking() {
    s.lastStatusUpdate = time.Now()  // ✅ Set here
}

// Line 118: Success case - MISSING reset!
func (s *StatusChecker) SetSystemOk(message string) {
    s.lastSystemCheck = time.Now()
    // ❌ MISSING: s.lastStatusUpdate = time.Time{} // Reset to zero
}
```

**Impact**: False stale detection on hosts with frequent switch operations.

**Fix**: Reset `lastStatusUpdate` to zero in all terminal state setters:

- `SetSystemOk()`
- `SetSystemOutdated()`
- `SetSystemError()`

---

### 2. Dashboard: Race Condition on updateStatus Mutation ⚠️

**Location**: `src/internal/dashboard/hub.go:1525`

**Problem**: `updateStatus` map is mutated while only holding lock on `h.lastStatusMu`, not the map itself.

**Code**:

```go
func (h *Hub) detectStaleStatus(hostID string, updateStatus map[string]any) {
    if ss, ok := updateStatus["system"].(protocol.StatusCheck); ok {
        if ss.Status == "working" {
            h.lastStatusMu.Lock()  // Lock AFTER reading updateStatus
            // ...
            updateStatus["system"] = protocol.StatusCheck{  // Mutate shared map
                Status: "unknown",
            }
            h.lastStatusMu.Unlock()
        }
    }
}
```

**Impact**: Potential race if multiple goroutines process same host's heartbeat concurrently (unlikely but fragile).

**Fix**: Lock protects wrong thing. Either:

- Option A: Pass `updateStatus` by value (copy)
- Option B: Document that `handleHeartbeat` is single-threaded per host
- Option C: Expand lock scope to cover mutation

---

### 3. Duplicate Operation Messages (Observed) ✅ FIXED

**Symptom**: Multiple "pull started" messages in logs for single action.

**Root Cause**: Duplicate `ChangeCommandStarted` emission in `handlers_ops.go`

**Location**: `src/internal/dashboard/handlers_ops.go:119-122`

**Problem**: HTTP handler emitted `ChangeCommandStarted` AFTER calling `lifecycleManager.ExecuteOp()`, which already broadcasts via `BroadcastCommandState()` → `ApplyChange(ChangeCommandStarted)`.

**Flow (Broken)**:

```
1. User clicks "Pull" → POST /api/ops/execute
2. handlers_ops.go:84 → lifecycleManager.ExecuteOp()
3.   → lifecycle_adapter.go:169 → ApplyChange(ChangeCommandStarted) ✅
4. handlers_ops.go:119 → ApplyChange(ChangeCommandStarted) ❌ DUPLICATE
5. Web UI receives 2x command_started events
6. Logs "pull started" twice
```

**Fix**: Remove duplicate emission from HTTP handler (lifecycle manager is authoritative).

**Impact**: Clean logs, no duplicate "started" messages.

---

## State Machine Analysis

### Agent Side

**Correct Flow**:

```
idle → SetSystemWorking() → working (lastStatusUpdate = now)
working → SetSystemOk() → ok (lastStatusUpdate = zero) ✅
working → detectStaleStatus(>5min) → unknown (lastStatusUpdate = now)
```

**Current (Broken) Flow**:

```
idle → SetSystemWorking() → working (lastStatusUpdate = T1)
working → SetSystemOk() → ok (lastStatusUpdate = T1) ❌ STALE
working → SetSystemWorking() → working (lastStatusUpdate = T2)
detectStaleStatus() → elapsed = T2 - T1 (WRONG, should be T2 - T2)
```

### Dashboard Side

**Tracking Logic** (mostly correct):

```
First "working" → record timestamp
"working" for >5min → mutate to "unknown", update timestamp
Non-"working" → delete timestamp ✅
```

**Issue**: Timestamp never updated during ongoing "working" (this is CORRECT - measures time since start).

---

## Acceptance Criteria

- ✅ Agent resets `lastStatusUpdate` on all terminal states (ok/outdated/error)
- ✅ No false stale detection on rapid state cycles
- ✅ Dashboard mutation is race-free
- ✅ No duplicate operation messages in logs
- ✅ Test coverage for rapid transitions (working → ok → working < 5min)

---

## Implementation Plan

### Phase 1: Fix Agent State Reset

**Files**: `src/internal/agent/status.go`

```go
func (s *StatusChecker) SetSystemOk(message string) {
    s.systemStatus = protocol.StatusCheck{
        Status:    "ok",
        Message:   message,
        CheckedAt: time.Now().UTC().Format(time.RFC3339),
    }
    s.lastSystemCheck = time.Now()
    s.lastStatusUpdate = time.Time{} // Reset tracking
}

func (s *StatusChecker) SetSystemOutdated(message string) {
    // ... existing code ...
    s.lastStatusUpdate = time.Time{} // Reset tracking
}

func (s *StatusChecker) SetSystemError(message string) {
    // ... existing code ...
    s.lastStatusUpdate = time.Time{} // Reset tracking
}
```

### Phase 2: Fix Dashboard Race

**Option A** (safest): Copy before mutate

```go
func (h *Hub) detectStaleStatus(hostID string, updateStatus map[string]any) {
    ss, ok := updateStatus["system"].(protocol.StatusCheck)
    if !ok || ss.Status != "working" {
        // Reset tracking for non-working
        h.lastStatusMu.Lock()
        if h.lastStatusUpdates[hostID] != nil {
            delete(h.lastStatusUpdates[hostID], "system")
        }
        h.lastStatusMu.Unlock()
        return
    }

    h.lastStatusMu.Lock()
    defer h.lastStatusMu.Unlock()

    // ... rest of logic with lock held ...
}
```

### Phase 3: Investigate Duplicate Messages

- Add trace logging to `sendOperationProgress()`
- Check `ApplyChange()` deduplication
- Review WebSocket broadcast paths

---

## Testing

### Unit Tests

```go
func TestStaleDetection_RapidCycles(t *testing.T) {
    checker := NewStatusChecker(agent)

    // First cycle
    checker.SetSystemWorking()
    time.Sleep(100 * time.Millisecond)
    checker.SetSystemOk("done")

    // Second cycle within 5min
    checker.SetSystemWorking()
    status := checker.GetStatus(ctx)

    // Should NOT be stale (lastStatusUpdate was reset)
    assert.Equal(t, "working", status.System.Status)
}
```

### Integration Tests

- Trigger switch → success → switch within 5min
- Verify no stale detection
- Verify no duplicate log messages

---

## Related

- P1110: Stale Status Detection (original implementation)
- CORE-006: Compartment Status Correctness
- P2800: Command State Machine

---

## Notes

**Why 5min threshold?**

- Typical switch: 30s-2min
- Slow hosts: up to 5min
- Beyond 5min = likely stuck/crashed

**Why dual detection (agent + dashboard)?**

- Agent: detects own crashes/restarts
- Dashboard: detects agent disconnects
- Redundancy ensures recovery

**Stale resolution strategy**:

- Resolve to "unknown" (not "error") because we don't know what happened
- User can manually refresh to get current state
- Prevents stuck "working" from blocking future operations
