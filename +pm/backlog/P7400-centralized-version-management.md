# P7400: Centralized Version Management

**Priority:** High  
**Status:** New  
**Epic:** Developer Experience  
**Created:** 2025-12-25

## Problem Statement

Version numbers are duplicated across **5+ locations**, requiring manual updates in multiple files when bumping versions. This is error-prone and not professional-level development.

### Current Locations (must be synchronized manually)

| File                               | Variable             | Current       |
| ---------------------------------- | -------------------- | ------------- |
| `v2/internal/dashboard/version.go` | `Version = "2.3.0"`  | 2.3.0         |
| `v2/internal/agent/agent.go`       | `Version = "2.3.0"`  | 2.3.0         |
| `packages/nixfleet-agent-v2.nix`   | `version = "2.3.0"`  | 2.3.0         |
| `.github/workflows/docker.yml`     | `VERSION=2.3.0`      | 2.3.0         |
| `justfile`                         | `version := "2.1.0"` | **OUTDATED!** |

### Consequences

- Easy to forget one location → version mismatch bugs
- "Agent: master" bug was caused by this fragmentation
- Version drift between agent and dashboard
- Manual process doesn't scale

## Proposed Solution

### Option A: Single VERSION file + Build-time injection

Create a single `VERSION` file at repo root:

```
2.3.0
```

**Changes:**

1. **Go code**: Read from ldflags (already supported)
2. **Nix package**: Read VERSION file
3. **GitHub Actions**: Read VERSION file
4. **justfile**: Read VERSION file

**Pros:** Simple, works everywhere  
**Cons:** Requires build-time injection for Go

### Option B: Go as source of truth + extraction

Keep version in `v2/internal/version/version.go` and extract it:

```go
package version

const Version = "2.3.0"
```

**Extraction:**

```bash
# In GitHub Actions / Nix / justfile
VERSION=$(grep 'Version = ' v2/internal/version/version.go | cut -d'"' -f2)
```

**Pros:** Single location in code  
**Cons:** Requires grep/parsing

### Option C: Git tags as source of truth

Use git tags for releases, derive version from git:

```bash
VERSION=$(git describe --tags --always)
```

**Pros:** Git is already the release mechanism  
**Cons:** Dev builds show commit hashes, not clean versions

## Recommended: Option A (VERSION file)

Simplest and most portable across all build systems.

## Implementation Plan

### Phase 1: Create VERSION file

```bash
echo "2.3.0" > VERSION
```

### Phase 2: Update Go builds

```go
// v2/internal/version/version.go
package version

// Version is injected at build time via ldflags
// Default for dev builds
var Version = "dev"
```

```bash
# Build with version injection
go build -ldflags "-X github.com/markus-barta/nixfleet/v2/internal/version.Version=$(cat VERSION)" ...
```

### Phase 3: Update Nix package

```nix
{
  version = builtins.readFile ../VERSION;
  # or
  version = lib.fileContents ../VERSION;
}
```

### Phase 4: Update GitHub Actions

```yaml
- name: Read version
  id: version
  run: echo "version=$(cat VERSION)" >> $GITHUB_OUTPUT

- name: Build
  with:
    build-args: |
      VERSION=${{ steps.version.outputs.version }}
```

### Phase 5: Update justfile

```just
version := `cat VERSION`
```

### Phase 6: Add version bump script

```bash
#!/bin/bash
# scripts/bump-version.sh
NEW_VERSION=$1
echo "$NEW_VERSION" > VERSION
git add VERSION
git commit -m "chore: bump version to $NEW_VERSION"
git tag "v$NEW_VERSION"
```

## Acceptance Criteria

- [ ] Single `VERSION` file at repo root
- [ ] All Go code reads version via ldflags
- [ ] Nix packages read from VERSION file
- [ ] GitHub Actions reads from VERSION file
- [ ] justfile reads from VERSION file
- [ ] `just bump-version 2.4.0` updates everything
- [ ] No hardcoded version strings in code (except VERSION file)

## Migration Notes

After implementation, version bump process:

```bash
# Old (error-prone):
# Edit 5 files manually, hope you didn't miss one

# New (single command):
just bump-version 2.4.0
git push && git push --tags
```

## Files to Modify

- `VERSION` (new)
- `v2/internal/dashboard/version.go`
- `v2/internal/agent/agent.go`
- `packages/nixfleet-agent-v2.nix`
- `.github/workflows/docker.yml`
- `justfile`
- `scripts/bump-version.sh` (new)
