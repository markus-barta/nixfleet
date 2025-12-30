# P8820: Stale “switch” Pending Command on `mba-imac-work` Blocks Tests

**Priority**: P8820 (High - Ops correctness / blocks actions)  
**Type**: Bug  
**Status**: Backlog  
**Created**: 2025-12-29  
**Related**: `src/internal/ops/lifecycle.go`, `src/internal/dashboard/hub.go`, `src/internal/dashboard/state_provider.go`

---

## Summary

Host `mba-imac-work` shows a stale **pending command = `switch`** even though nothing is actually running, which prevents running **tests** (and other ops) because the system treats the host as “busy”.

---

## Impact

- Host appears **busy** indefinitely.
- **Tests cannot be run** (and other actions are disabled) because ops are gated on `pending_command == NULL`.

---

## Repro

- Go to dashboard → host `mba-imac-work`
- Observe host shows **busy / pending `switch`** (or actions are disabled)
- Attempt to run **test** → not available / blocked

---

## Expected

- `pending_command` clears reliably when:
  - command completes (success or failure), OR
  - command is rejected, OR
  - dashboard restarts and loses in-memory lifecycle state but host is actually idle

---

## Notes / Likely Root Cause

Confirmed likely root cause (high confidence):

- `ops.LifecycleManager` keys active commands by **`host.GetID()`** (see `src/internal/ops/lifecycle.go`: `hostID := host.GetID()`).
- Dashboard ops dispatch uses `ops.NewHostAdapter(host)` where `HostAdapter.GetID()` returns **`templates.Host.ID`** (DB primary key).
- Agent messages (heartbeat/status/command_complete/register) identify the host by **hostname**.
- In `src/internal/dashboard/hub.go`, the Hub currently calls lifecycle methods with **hostname**:
  - `HandleCommandComplete(hostID, ...)`
  - `HandleHeartbeat(hostID, ...)`
  - `HandleAgentReconnect(payload.Hostname, ...)`
  - `HandleCommandRejected(msg.client.clientID, ...)`

That mismatch means completions/reconnects can be treated as “untracked command”, so lifecycle never clears the active command and the DB `pending_command` stays stuck → tests blocked.

Other contributing failure modes (still possible):

- **Lifecycle state lost** (dashboard restart) while DB still has `pending_command`
- Agent-side completion/rejection message lost / not reconciled
- `switch` special-case (await reconnect) not resolving cleanly

The UI’s op availability is computed server-side:

- `src/internal/dashboard/state_provider.go`: if `pending_command` is set → **no ops** (including tests)

---

## Diagnostics to Capture

- **DB**: `hosts.pending_command` for `mba-imac-work`
- **Lifecycle**: whether LifecycleManager tracks an active command for this host
- **Agent connection**: whether agent is connected and what heartbeat says about “pending”

Key debug question:

- Do lifecycle calls use **DB id** consistently (start/complete/reconnect/heartbeat), or do some use hostname? If mixed, stale pending is expected.

---

## Acceptance Criteria

- [ ] Stale `pending_command` no longer persists for online idle hosts
- [ ] `mba-imac-work` can run **tests** after the stale state resolves
- [ ] Regression check: real in-flight commands still block ops (no false clears)
