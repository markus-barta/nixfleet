# P4600 - GitHub Actions: Docker Build & Registry

**Priority**: P4600 (High - Infrastructure)  
**Status**: Backlog  
**Effort**: Low (~1 hour)  
**Value**: High

---

## Problem

Currently, the dashboard is built on csb1 every time we deploy:

```bash
docker compose build nixfleet  # Takes 1-2 minutes
docker compose up -d nixfleet
```

Issues:

- Build happens on production server (uses resources)
- No versioned images (can't rollback easily)
- Build time adds to deploy latency

---

## Solution

Use GitHub Actions to build Docker image on every push, store in GitHub Container Registry (ghcr.io).

### Workflow

```
Push to master
      │
      ▼
GitHub Actions
├── Build Docker image
├── Tag with version + git SHA
└── Push to ghcr.io/markus-barta/nixfleet
      │
      ▼
csb1 deployment
└── docker compose pull && up -d  # Just pull, no build
```

---

## Implementation

### 1. Create GitHub Actions Workflow

`.github/workflows/docker.yml`:

```yaml
name: Docker Build

on:
  push:
    branches: [master]
  release:
    types: [published]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=sha,prefix=
            type=ref,event=branch
            type=semver,pattern={{version}}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: v2/Dockerfile
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=${{ github.ref_name }}
            GIT_COMMIT=${{ github.sha }}
```

### 2. Update docker-compose.yml on csb1

Change from build to image:

```yaml
services:
  nixfleet:
    image: ghcr.io/markus-barta/nixfleet:master
    # Remove build: section
```

### 3. New Deploy Command

```bash
# On csb1
docker compose pull nixfleet
docker compose up -d nixfleet
```

---

## Benefits

| Before                        | After                         |
| ----------------------------- | ----------------------------- |
| Build on server (1-2 min)     | Pull pre-built image (10 sec) |
| No version history            | Tagged images for rollback    |
| Server CPU spike during build | Zero build load on server     |
| Manual version injection      | Automatic from git            |

---

## Acceptance Criteria

- [ ] GitHub Actions workflow builds on every push to master
- [ ] Images pushed to ghcr.io with SHA and branch tags
- [ ] csb1 docker-compose.yml uses image instead of build
- [ ] Deploy takes <30 seconds (pull only)
- [ ] Can rollback to previous image by tag

---

## Rollback Example

```bash
# Something broke? Rollback to previous SHA
docker compose down
docker compose pull nixfleet:abc1234  # Previous working SHA
docker compose up -d
```

---

## Related

- P4700 - Automated flake.lock Updates
- P4800 - Webhook-Triggered Deployment
