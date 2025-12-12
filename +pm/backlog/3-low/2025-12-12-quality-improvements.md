# Quality Improvements (Nice-to-Have)

**Created**: 2025-12-12  
**Priority**: Low  
**Status**: Backlog

---

## Overview

Long-term quality improvements to reach "production-grade" NixFleet. None of these are blocking current functionality.

---

## Module Options (8/10 → 10/10)

- [ ] Add `lib.mdDoc` to all option descriptions
- [ ] Add `defaultText` to options with computed defaults
- [ ] Add more comprehensive examples in option definitions

## Testing (5/10 → 9/10)

- [ ] Add NixOS VM test for agent service startup
- [ ] Add Python unit tests for dashboard backend
- [ ] Add integration test for full agent→dashboard flow

## CI/CD (3/10 → 9/10)

- [ ] Add GitHub Actions workflow for `nix flake check`
- [ ] Add GitHub Actions workflow for shellcheck on agent
- [ ] Add GitHub Actions workflow for Python linting
- [ ] Add Cachix integration for faster CI

## Documentation (8/10 → 10/10)

- [ ] Add CHANGELOG.md with semantic versioning
- [ ] Add CONTRIBUTING.md with development workflow
- [ ] Add architecture diagram
- [ ] Document all API endpoints

## Package Quality (7/10 → 10/10)

- [ ] Add `meta` attributes (description, license, maintainers)
- [ ] Add shell completions (fish/bash/zsh)

---

## Notes

These are polish items. The system works well as-is. Tackle when time permits.

Split from: `2025-12-12-nixfleet-post-extraction-tasks.md`

