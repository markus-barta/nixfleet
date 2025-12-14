# Quality Improvements

**Created**: 2025-12-14
**Priority**: P6100 (Low)
**Status**: Backlog
**Depends on**: P4000-P4400 (Core rewrite)

---

## Overview

Polish items to reach production-grade quality.

---

## Testing

- [ ] Unit tests for agent commands
- [ ] Unit tests for dashboard handlers
- [ ] Integration test: agent ↔ dashboard WebSocket
- [ ] End-to-end test with test host

## CI/CD

- [ ] GitHub Actions: `go build`, `go test`
- [ ] GitHub Actions: `nix flake check`
- [ ] Cachix for faster CI

## Documentation

- [ ] CHANGELOG.md with semantic versioning
- [ ] API documentation (OpenAPI spec)
- [ ] Architecture diagram
- [ ] CONTRIBUTING.md

## Code Quality

- [ ] golangci-lint in CI
- [ ] Structured logging (zerolog)
- [ ] Error wrapping with context

---

## Related

- Post-MVP polish
