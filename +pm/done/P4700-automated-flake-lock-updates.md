# P4700 - Automated flake.lock Updates

**Priority**: P4700 (Medium - Convenience)  
**Status**: ✅ Done  
**Effort**: Medium (~2 hours)  
**Value**: Medium  
**Completed**: 2025-12-27

---

## Problem

When nixfleet is updated, every host needs its flake.lock updated:

1. Push changes to nixfleet repo
2. Go to nixcfg repo
3. Run `nix flake update nixfleet`
4. Commit and push
5. Switch each host

Steps 2-4 are manual and easy to forget.

---

## Solution (Implemented)

Fully automated flow with no manual intervention required:

```
nixfleet: Push to master
      │
      ▼
nixfleet: Docker image builds (GitHub Actions)
      │
      ▼
nixfleet: Triggers repository_dispatch to nixcfg
      │
      ▼
nixcfg: GitHub Action receives event
├── Runs: nix flake update nixfleet
└── Commits directly to main (no PR needed)
      │
      ▼
Hosts: Show "Git outdated" in dashboard
      │
      ▼
You: Click "Pull + Switch" in dashboard
```

---

## Implementation

### 1. nixfleet: Trigger in Docker workflow

Added to `.github/workflows/docker.yml`:

```yaml
# Trigger nixcfg to update its flake.lock with new nixfleet
- name: Trigger nixcfg update
  uses: peter-evans/repository-dispatch@v3
  with:
    token: ${{ secrets.NIXCFG_DISPATCH_TOKEN }}
    repository: markus-barta/nixcfg
    event-type: nixfleet-updated
    client-payload: '{"version": "${{ steps.version.outputs.version }}", "sha": "${{ github.sha }}"}'
```

### 2. nixcfg: Receiver workflow

Template provided in `docs/nixcfg-workflow-template.yml`. Copy to nixcfg as `.github/workflows/update-nixfleet.yml`.

### 3. One-time setup

- Create PAT with `repo` scope
- Add as `NIXCFG_DISPATCH_TOKEN` secret in nixfleet repo
- Copy workflow template to nixcfg

---

## Design Decision: Direct Commit vs PR

Original design suggested creating a PR for human review. Implemented as direct commit because:

1. **Single user** — no need for review workflow
2. **Faster deployment** — hosts update immediately
3. **No merge button clicking** — fully hands-off
4. **Simpler** — less moving parts

If multi-user or team usage is needed later, the workflow can be changed to create PRs.

---

## Acceptance Criteria

- [x] Push to nixfleet master triggers Docker build
- [x] Docker build triggers nixcfg workflow
- [x] nixcfg workflow updates flake.lock automatically
- [x] Commit includes version and SHA reference
- [x] Hosts show "Git outdated" after nixcfg update
- [x] Pull + Switch updates agent to new version

---

## Documentation

- [Release Guide](../../docs/RELEASE.md) — Simple step-by-step guide
- [nixcfg Workflow Template](../../docs/nixcfg-workflow-template.yml) — Copy to nixcfg

---

## Related

- P4600 - GitHub Actions Docker Build (prerequisite, done)
- P7210 - Dashboard Bump Agent Version (obsoleted by this)
