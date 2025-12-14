# P4348 - Version Injection from Build

**Priority**: Low  
**Status**: Pending  
**Effort**: Small  
**References**: `v2/internal/dashboard/handlers.go:135`

## Problem

Version is hardcoded:

```go
Version: "2.0.0", // TODO: inject from build
```

Should show actual version from git tag or build system.

## Solution

### 1. Define Version Variable

```go
// version.go
package dashboard

var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)
```

### 2. Inject at Build Time

```bash
go build -ldflags "-X github.com/.../dashboard.Version=2.0.1 \
                   -X github.com/.../dashboard.GitCommit=$(git rev-parse --short HEAD) \
                   -X github.com/.../dashboard.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

### 3. Update Dockerfile

```dockerfile
ARG VERSION=dev
ARG GIT_COMMIT=unknown

RUN go build -ldflags "-s -w \
    -X github.com/.../dashboard.Version=${VERSION} \
    -X github.com/.../dashboard.GitCommit=${GIT_COMMIT}"
```

### 4. Update Nix Package

```nix
buildGoModule rec {
  pname = "nixfleet-dashboard";
  version = "2.0.0";

  ldflags = [
    "-X github.com/.../dashboard.Version=${version}"
    "-X github.com/.../dashboard.GitCommit=${src.rev or "unknown"}"
  ];
}
```

### 5. Display in Footer

```go
// In template data
Version: fmt.Sprintf("%s (%s)", dashboard.Version, dashboard.GitCommit[:7])
```

### Requirements

- [ ] Create version.go with package variables
- [ ] Update Makefile/build script with ldflags
- [ ] Update Dockerfile to pass build args
- [ ] Update Nix package with ldflags
- [ ] Display version in footer
- [ ] Show commit hash on hover

## Related

- P4360 (Footer) - Displays version
