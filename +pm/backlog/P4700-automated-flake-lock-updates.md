# P4700 - Automated flake.lock Updates

**Priority**: P4700 (Medium - Convenience)  
**Status**: Backlog  
**Effort**: Medium (~2 hours)  
**Value**: Medium  
**Depends on**: P4600 (GitHub Actions Docker Build)

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

## Solution

GitHub Action in nixcfg that automatically creates a PR when nixfleet changes.

### Workflow

```
nixfleet: Push to master
      │
      ▼
nixfleet: GitHub Action triggers repository_dispatch to nixcfg
      │
      ▼
nixcfg: GitHub Action receives event
├── Runs: nix flake update nixfleet
├── Creates branch: update-nixfleet-{sha}
└── Opens PR: "chore: update nixfleet to {sha}"
      │
      ▼
You: Review and merge PR
      │
      ▼
Hosts: Pull and switch (manual or via dashboard)
```

---

## Implementation

### 1. Trigger in nixfleet repo

`.github/workflows/notify-nixcfg.yml`:

```yaml
name: Notify nixcfg

on:
  push:
    branches: [master]

jobs:
  notify:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger nixcfg update
        uses: peter-evans/repository-dispatch@v2
        with:
          token: ${{ secrets.NIXCFG_PAT }}
          repository: markus-barta/nixcfg
          event-type: nixfleet-updated
          client-payload: '{"sha": "${{ github.sha }}", "ref": "${{ github.ref }}"}'
```

### 2. Receiver in nixcfg repo

`.github/workflows/update-nixfleet.yml`:

```yaml
name: Update nixfleet

on:
  repository_dispatch:
    types: [nixfleet-updated]

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Nix
        uses: cachix/install-nix-action@v24

      - name: Update flake.lock
        run: nix flake update nixfleet

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v5
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          commit-message: "chore: update nixfleet to ${{ github.event.client_payload.sha }}"
          branch: update-nixfleet-${{ github.event.client_payload.sha }}
          title: "chore: update nixfleet"
          body: |
            Automated update of nixfleet input.

            **Commit**: ${{ github.event.client_payload.sha }}

            After merging, run switch on hosts or use NixFleet dashboard.
```

### 3. Create Personal Access Token

Need a PAT with `repo` scope for cross-repo dispatch:

1. GitHub → Settings → Developer settings → Personal access tokens
2. Create token with `repo` scope
3. Add as `NIXCFG_PAT` secret in nixfleet repo

---

## Benefits

| Before                   | After                 |
| ------------------------ | --------------------- |
| Manual flake.lock update | Automatic PR created  |
| Easy to forget           | Never miss an update  |
| No visibility            | PR shows what changed |

---

## Acceptance Criteria

- [ ] Push to nixfleet master triggers nixcfg workflow
- [ ] nixcfg workflow creates PR with updated flake.lock
- [ ] PR includes commit SHA and description
- [ ] Merge PR updates flake.lock for all hosts

---

## Security Notes

- PAT has minimal scope (just repo dispatch)
- PRs require review before merge
- No auto-merge (human in the loop)

---

## Related

- P4600 - GitHub Actions Docker Build (prerequisite)
- P4800 - Webhook-Triggered Deployment
- P4300 - Automated Flake Updates (full automation vision)
