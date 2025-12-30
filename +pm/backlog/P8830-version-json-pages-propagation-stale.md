# P8830: `version.json` on GitHub Pages stays stale after workflow fix (lockHash not visible)

**Priority**: P8830 (Medium - blocks verifying P8810 fix rollout)  
**Type**: Bug / Ops / Release plumbing  
**Status**: Backlog  
**Created**: 2025-12-30  
**Related**: `+pm/backlog/P8810-lockhash-missing-in-version-json.md`, `nixcfg/.github/workflows/update-nixfleet.yml`, `nixcfg/.github/workflows/version-pages.yml`

---

## Summary

After landing the `lockHash` publishing fix in `nixcfg/.github/workflows/update-nixfleet.yml`, the expected field still does not appear in the served GitHub Pages file:

- `https://markus-barta.github.io/nixcfg/version.json`

This may be GitHub Pages propagation delay, caching, or the Pages publish job not running/targeting the correct artifact.

---

## Impact

- Dashboard remains noisy (Lock compartment stays 🔴 with “Remote version missing lockHash…”) until Pages serves the updated schema.
- Makes it hard to verify the `P8810` fix actually deployed correctly.

---

## Repro

- Merge/push changes that add `lockHash` to the `version.json` generator.
- Open `https://markus-barta.github.io/nixcfg/version.json` (optionally with a cache-busting querystring).
- Observe: JSON still does **not** include `"lockHash": ...`.

---

## Expected

- GitHub Pages serves `version.json` that includes:
  - `gitCommit`
  - `lockHash`
  - other metadata (`timestamp`, `repo`, etc.)

---

## Notes / Likely Causes

- Pages build not triggered / failing / pointing to wrong build output.
- Competing workflows still overwrite `_site/version.json` with a partial schema.
- CDN/browser caching (check response headers; may ignore querystring caching semantics).
- Pages source/branch configured differently than expected (e.g. `/docs` vs artifact deploy action).

---

## Diagnostics to Capture

- GitHub Actions runs:
  - `nixcfg/.github/workflows/version-pages.yml` (status, logs, artifacts)
  - `nixcfg/.github/workflows/update-nixfleet.yml` (status, logs, artifacts)
- GitHub Pages settings for the repo (source branch / deploy action).
- `curl -i` headers for `version.json` (cache-control / etag / last-modified).
- Confirm the deployed artifact actually contains `"lockHash"` before publish.

---

## Acceptance Criteria

- [ ] `version.json` served via Pages includes `lockHash`
- [ ] Dashboard no longer shows “Remote version missing lockHash (version.json)” for all hosts
- [ ] Document the expected owner/workflow for Pages publishing (single source of truth)
