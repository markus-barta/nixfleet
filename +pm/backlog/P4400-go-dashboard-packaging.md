# Go Dashboard: Packaging

**Created**: 2025-12-14
**Priority**: P4400 (Critical)
**Status**: Backlog
**Depends on**: P4200 (Dashboard Core), P4300 (Live Logs)

---

## Overview

Package the Go dashboard for Docker and NixOS deployment.

---

## Deliverables

### 1. Nix Package

```nix
# packages/nixfleet-dashboard/default.nix
{ buildGoModule, templ, ... }:

buildGoModule {
  pname = "nixfleet-dashboard";
  version = "2.0.0";

  src = ../../dashboard;

  vendorHash = "sha256-...";

  nativeBuildInputs = [ templ ];

  preBuild = ''
    templ generate
  '';

  ldflags = [
    "-s" "-w"
    "-X main.Version=${version}"
  ];

  # Embed static files
  tags = [ "embed" ];
}
```

### 2. Docker Image

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .

RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate
RUN CGO_ENABLED=1 go build -o nixfleet-dashboard ./cmd/nixfleet-dashboard

FROM alpine:3.19

RUN apk add --no-cache sqlite-libs ca-certificates

COPY --from=builder /app/nixfleet-dashboard /usr/local/bin/
COPY --from=builder /app/static /app/static

EXPOSE 8000
VOLUME /data

ENV NIXFLEET_DATA_DIR=/data

ENTRYPOINT ["/usr/local/bin/nixfleet-dashboard"]
```

### 3. Docker Compose

```yaml
# docker-compose.yml
services:
  dashboard:
    build: .
    ports:
      - "8000:8000"
    volumes:
      - ./data:/data
    environment:
      - NIXFLEET_PASSWORD_HASH=${NIXFLEET_PASSWORD_HASH}
      - NIXFLEET_SESSION_SECRETS=${NIXFLEET_SESSION_SECRETS}
    restart: unless-stopped
```

### 4. NixOS Module (Optional)

For running dashboard natively on NixOS (not Docker):

```nix
services.nixfleet-dashboard = {
  enable = true;
  port = 8000;
  dataDir = "/var/lib/nixfleet";
  environmentFile = "/run/secrets/nixfleet-dashboard.env";
};
```

---

## Static File Embedding

Use Go 1.16+ embed for single binary:

```go
//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS
```

---

## Acceptance Criteria

- [ ] `nix build .#nixfleet-dashboard` produces single binary
- [ ] Binary includes embedded static files
- [ ] Docker build works
- [ ] Docker Compose works
- [ ] NixOS module option (optional)
- [ ] Health check endpoint for Docker

---

## Related

- Depends on: P4200, P4300
- Enables: Deployment to csb1
