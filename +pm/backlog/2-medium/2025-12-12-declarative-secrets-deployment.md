# Declarative Secrets & Deployment Setup

**Created**: 2025-12-12
**Status**: Backlog
**Priority**: Medium

## Problem

Currently, deploying NixFleet requires manual steps to generate and configure secrets:

1. Generate `NIXFLEET_SESSION_SECRETS` (session cookie signing)
2. Generate `NIXFLEET_AGENT_TOKEN_HASH_SECRET` (per-host agent tokens)
3. Generate `NIXFLEET_PASSWORD_HASH` (admin login)
4. Generate `NIXFLEET_API_TOKEN` (shared agent bootstrap token)
5. Optionally: `NIXFLEET_TOTP_SECRET` (2FA)

These are currently done manually via `openssl rand -hex 32` and added to `.env`.

## Goal

Make deployment declarative and reproducible:

### Option A: Nix-based secret generation (preferred for NixOS hosts)

- Use `agenix` or `sops-nix` to manage secrets
- Generate secrets declaratively in NixOS configuration
- Mount secrets into Docker container via volumes

### Option B: Init script approach

- Create `init-secrets.sh` that generates missing secrets
- Run automatically on first deployment
- Store in persistent volume

### Option C: Docker secrets / Compose secrets

- Use Docker Compose secrets feature
- External secret management (Vault, etc.)

## Required Documentation

Update README with:

1. **First-time setup** section explaining all required env vars
2. **Secret generation** commands for each variable
3. **Rotation procedures** for each secret type
4. **Migration guide** when adding new required secrets

## Current Required Env Vars

| Variable                           | Purpose                | Generation                                                                                |
| ---------------------------------- | ---------------------- | ----------------------------------------------------------------------------------------- |
| `NIXFLEET_PASSWORD_HASH`           | Admin login            | `python -c "import bcrypt; print(bcrypt.hashpw(b'password', bcrypt.gensalt()).decode())"` |
| `NIXFLEET_API_TOKEN`               | Shared agent bootstrap | `openssl rand -hex 32`                                                                    |
| `NIXFLEET_SESSION_SECRETS`         | Cookie signing         | `openssl rand -hex 32` (comma-separated for rotation)                                     |
| `NIXFLEET_AGENT_TOKEN_HASH_SECRET` | Per-host token hashing | `openssl rand -hex 32`                                                                    |
| `NIXFLEET_TOTP_SECRET`             | 2FA (optional)         | `python -c "import pyotp; print(pyotp.random_base32())"`                                  |

## Acceptance Criteria

- [ ] New deployment requires zero manual secret generation
- [ ] Existing deployments can upgrade without breaking
- [ ] Secret rotation is documented and tested
- [ ] README includes complete setup instructions
