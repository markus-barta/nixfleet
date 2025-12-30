# P8810: Lock Compartment Always 🔴 — Remote `version.json` Missing `lockHash`

**Priority**: P8810 (High - Compartment correctness / blocks trust)  
**Type**: Bug  
**Status**: Backlog  
**Created**: 2025-12-29  
**Related**: `+pm/spec/CORE-006-compartments.md`, `src/internal/dashboard/version_fetcher.go`

---

## Summary

All hosts show:

- **Lock: `Remote version missing lockHash (version.json)`**

This is reported by the dashboard-side `VersionFetcher.GetLockStatus()` when the remote `version.json` is reachable but lacks the `lockHash` field.

---

## Impact

- **Lock compartment is always 🔴** for the entire fleet (noise, destroys at-a-glance value).
- Any remote-gated inference depending on Lock becomes misleading.

---

## Repro

- Open dashboard and inspect any host’s Lock tooltip/message.
- Observe: `Remote version missing lockHash (version.json)`

---

## Expected

- Remote `version.json` includes a valid `lockHash`, and Lock shows:
  - 🟢 when host lockHash matches remote
  - 🟡 when host lockHash differs
  - 🔴 only on real remote errors (fetch failing / invalid JSON / truly missing)

---

## Likely Root Cause

Confirmed root cause:

- The configured URL is **`https://markus-barta.github.io/nixcfg/version.json`** (from `NIXFLEET_VERSION_URL` on `csb1`).
- The currently served `version.json` **does not include** `lockHash` (only `gitCommit/message/branch/timestamp/repo`).
- In `nixcfg`, there are **two** workflows that publish `version.json`:
  - `nixcfg/.github/workflows/version-pages.yml` **includes** `lockHash`
  - `nixcfg/.github/workflows/update-nixfleet.yml` (job `update-pages`) **does NOT include** `lockHash` and can overwrite Pages with the old/partial schema.

So `update-nixfleet.yml` is effectively “winning” sometimes and publishing a schema that makes the dashboard correctly raise `Remote version missing lockHash`.

---

## Pointers (Code)

- **Dashboard**: `src/internal/dashboard/version_fetcher.go`
  - `RemoteVersion.LockHash` (`json:"lockHash"`)
  - `GetLockStatus()` returns the exact error string when `LockHash == ""`
- **Config**: `src/internal/dashboard/config.go` reads `NIXFLEET_VERSION_URL`
- **Source workflows** (in `nixcfg`):
  - `nixcfg/.github/workflows/version-pages.yml` (has `LOCK_HASH=sha256sum flake.lock` and writes `"lockHash": ...`)
  - `nixcfg/.github/workflows/update-nixfleet.yml` (job `update-pages` currently omits `lockHash`)

---

## Acceptance Criteria

- [ ] Remote `version.json` contains `lockHash`
- [ ] Dashboard Lock compartment no longer shows “Remote version missing lockHash…”
- [ ] Lock dot transitions correctly (🟢/🟡/🔴) across at least 2 hosts with different lock states
