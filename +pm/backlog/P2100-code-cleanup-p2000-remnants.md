# P2100: Code Cleanup - P2000 Remnants

**Priority**: Low  
**Complexity**: Low  
**Status**: Backlog  
**Created**: 2025-12-20  
**Depends On**: None

---

## Summary

Minor cleanup items remaining after P2000 (Unified Host State Management) was implemented. These are non-critical polish items.

---

## Tasks

### 1. Per-Host Refresh Button

**Status**: CSS/JS ready, HTML not rendered

The `.btn-refresh` styling exists in `base.templ` and `refreshHost()` function exists in JS, but the actual `<button class="btn-refresh">` element is **not rendered** in the `HostRow` template.

**Decision needed**:

- Was this intentional? Compartment clicks now trigger refresh.
- If we want an explicit refresh button, add to `HostRow` template.

**Files**:

- `v2/internal/templates/dashboard.templ` - add button to HostRow if needed
- `v2/internal/templates/base.templ` - CSS already exists (lines 1103-1127)

---

### 2. Remove or Keep `handleFlakeUpdateJob()`

**Status**: Exists but minimal

```javascript
function handleFlakeUpdateJob(job) {
  // Could show a toast or banner for deploy progress
  console.log("Flake update job:", job.state, job.message);
}
```

**Decision needed**:

- Remove entirely (original P2000 said DELETE)
- OR implement proper deploy progress display
- Currently just logs to console

**Files**:

- `v2/internal/templates/dashboard.templ` - lines 1170-1173

---

### 3. Update Integration Tests

**Status**: Tests reference old `host_update` message type

The server now sends `host_heartbeat` but tests still expect `host_update`:

```go
// t05_dashboard_websocket_test.go:315
if msg["type"] == "host_update" {
```

**Action**: Update test to expect `host_heartbeat` message type.

**Files**:

- `v2/tests/integration/t05_dashboard_websocket_test.go` - lines 315, 391

---

## Acceptance Criteria

- [ ] Decision made on refresh button (add or remove dead CSS/JS)
- [ ] Decision made on `handleFlakeUpdateJob` (implement or delete)
- [ ] Integration tests updated to use `host_heartbeat`
- [ ] No dead code remaining
