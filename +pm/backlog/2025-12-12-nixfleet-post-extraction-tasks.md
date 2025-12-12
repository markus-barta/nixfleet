# 2025-12-12 - NixFleet post-extraction tasks

## Status: Open

## Priority: High

## Created: 2025-12-12

---

## Overview

After extracting NixFleet to a separate repository (github:markus-barta/nixfleet), there are manual deployment steps and quality improvements needed to reach production-grade status.

---

## Manual Deployment Steps (Required)

### 1. Dashboard Deployment on csb1

The Docker container needs to be redeployed from the new repository location.

```bash
ssh csb1
cd ~/docker

# Backup current .env
cp nixfleet/.env ~/nixfleet-env.backup

# Remove old and clone new
rm -rf nixfleet
git clone https://github.com/markus-barta/nixfleet.git
cd nixfleet

# Restore secrets
cp ~/nixfleet-env.backup .env

# Build and deploy
docker compose -f docker/docker-compose.csb1.yml up -d --build

# Verify
docker logs -f nixfleet
```

- [ ] Deploy dashboard container from new repo
- [ ] Verify dashboard accessible at fleet.barta.cm
- [ ] Verify agents can connect

### 2. Deploy Agents to All Hosts

After extraction, agents need rebuild to use new module with security hardening.

**NixOS hosts (via hsb1 or gpc0):**

```bash
# For each: hsb0, hsb1, hsb8, csb0, csb1, gpc0
ssh <host> "cd ~/Code/nixcfg && git pull && sudo nixos-rebuild switch --flake .#<hostname>"
```

**macOS hosts (local):**

```bash
# imac0
home-manager switch --flake .#markus@imac0

# mba-imac-work
home-manager switch --flake .#markus@mba-imac-work

# mba-mbp-work
home-manager switch --flake .#mba@mba-mbp-work
```

- [ ] Deploy to hsb0
- [ ] Deploy to hsb1
- [ ] Deploy to hsb8
- [ ] Deploy to csb0
- [ ] Deploy to csb1
- [ ] Deploy to gpc0
- [ ] Deploy to imac0
- [ ] Deploy to mba-imac-work
- [ ] Deploy to mba-mbp-work
- [ ] Verify all hosts appear in dashboard

---

## Quality Improvements (To Reach 9-10/10)

### Module Options (Current: 8/10 → Target: 10/10)

- [ ] Add `lib.mdDoc` to all option descriptions with proper markdown formatting
- [ ] Add `defaultText` to options with computed defaults
- [ ] Add more comprehensive examples in option definitions
- [ ] Add `relatedPackages` where applicable
- [ ] Consider `lib.mkPackageOption` pattern for agent package

### Testing (Current: 5/10 → Target: 9/10)

- [ ] Add NixOS VM test for agent service startup
- [ ] Add NixOS VM test for agent-dashboard communication (mock server)
- [ ] Add Python unit tests for dashboard backend
- [ ] Add integration test for full agent→dashboard flow
- [ ] Add `passthru.tests` to agent package

### CI/CD (Current: 3/10 → Target: 9/10)

- [ ] Add GitHub Actions workflow for `nix flake check`
- [ ] Add GitHub Actions workflow for shellcheck on agent
- [ ] Add GitHub Actions workflow for Python linting (ruff/mypy)
- [ ] Add GitHub Actions workflow for Docker build test
- [ ] Add Cachix integration for faster CI
- [ ] Add automatic release tagging
- [ ] Add PR template with checklist

### Documentation (Current: 8/10 → Target: 10/10)

- [ ] Add generated NixOS options documentation (like nixpkgs)
- [ ] Add CHANGELOG.md with semantic versioning
- [ ] Add CONTRIBUTING.md with development workflow
- [ ] Add architecture diagram (Mermaid or ASCII)
- [ ] Add troubleshooting section to README
- [ ] Document all API endpoints with request/response examples

### Package Quality (Current: 7/10 → Target: 10/10)

- [ ] Create proper derivation for agent (not just writeShellApplication)
- [ ] Add `meta` attributes (description, license, maintainers, homepage)
- [ ] Add `passthru.updateScript` for automated updates
- [ ] Add man page for agent
- [ ] Add shell completions (fish/bash/zsh)

### Security Hardening (Current: 9/10 → Target: 10/10)

- [ ] Add systemd sandboxing options (CapabilityBoundingSet, etc.)
- [ ] Consider separate service account instead of user's account
- [ ] Add rate limiting assertions in module
- [ ] Add warning if HTTP (not HTTPS) URL configured
- [ ] Document security model in README
- [ ] **NixOS module correctness**:
  - token file format: `EnvironmentFile=` expects `KEY=VALUE`; ensure the module supports a token file that contains *just the token* (common for secret files).
  - systemd sandboxing: verify `ProtectSystem=strict` + `ReadWritePaths=[cfg.configRepo]` does not break `nixos-rebuild` (it likely needs additional write paths such as `/nix`), and adjust hardening accordingly.

---

## Quality Score Target

| Criterion       | Current | Target | Gap                          |
| --------------- | ------- | ------ | ---------------------------- |
| Flake structure | 9/10    | 10/10  | Add all systems, more checks |
| Module options  | 8/10    | 10/10  | mdDoc, examples, defaultText |
| Testing         | 5/10    | 9/10   | NixOS tests, Python tests    |
| CI/CD           | 3/10    | 9/10   | GitHub Actions, Cachix       |
| Documentation   | 8/10    | 10/10  | Generated docs, CHANGELOG    |
| Package quality | 7/10    | 10/10  | Proper derivation, meta      |
| Security        | 9/10    | 10/10  | Enhanced sandboxing          |

---

## Acceptance Criteria

- [ ] Dashboard deployed and accessible
- [ ] All hosts connected to dashboard
- [ ] GitHub Actions CI passing
- [ ] NixOS tests passing
- [ ] Documentation complete with generated options
- [ ] All module options have mdDoc descriptions
- [ ] Agent packaged as proper derivation with meta

