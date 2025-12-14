# P4348 - Version Injection from Build

**Priority**: Low  
**Status**: Done  
**Effort**: Small  
**References**: `v2/internal/dashboard/version.go`

## Problem

Version was hardcoded in handlers.go.

## Solution

Created `version.go` with package-level variables that can be set via ldflags at build time.

### Requirements

- [x] Create version.go with package variables (Version, GitCommit, BuildTime)
- [x] Add VersionInfo() helper function
- [x] Update handlers.go to use VersionInfo()
- [x] Update Dockerfile with build args and ldflags
- [x] Add justfile recipes (build-dashboard, build-agent, build)
- [x] Display version in footer (uses VersionInfo())

### Build Commands

```bash
# Via justfile
just build-dashboard
just build-agent
just build

# Via Docker
docker build --build-arg VERSION=2.0.0 --build-arg GIT_COMMIT=$(git rev-parse HEAD) -t nixfleet:v2 .
```

## Related

- P4360 (Footer) - Displays version
