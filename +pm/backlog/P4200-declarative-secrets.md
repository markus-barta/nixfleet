# P4200 - Declarative Secrets Deployment

**Created**: 2025-12-14  
**Updated**: 2025-12-15  
**Priority**: P4200 (Medium)  
**Status**: Backlog  
**Depends on**: P4400 (Dashboard Packaging)

---

## Overview

Make NixFleet deployment declarative with automatic secret generation.

---

## Current State

Manual steps required:

1. Generate `NIXFLEET_SESSION_SECRETS`
2. Generate `NIXFLEET_PASSWORD_HASH`
3. Generate `NIXFLEET_TOTP_SECRET` (optional)
4. Create `.env` file

---

## Solution

### Option A: Init Script

```bash
#!/bin/bash
# init-secrets.sh

if [[ ! -f /data/.secrets-initialized ]]; then
    export NIXFLEET_SESSION_SECRETS=$(openssl rand -hex 32)
    # ... generate others
    touch /data/.secrets-initialized
fi
```

Run on first container start.

### Option B: NixOS + agenix

```nix
age.secrets.nixfleet-env.file = ./secrets/nixfleet-env.age;

services.nixfleet-dashboard = {
  enable = true;
  environmentFile = config.age.secrets.nixfleet-env.path;
};
```

---

## Documentation

Update README with:

1. First-time setup steps
2. Secret generation commands
3. Rotation procedures

---

## Acceptance Criteria

- [ ] New deployment requires minimal manual steps
- [ ] Secrets documented in README
- [ ] Rotation procedure documented
- [ ] Works with Docker and native NixOS

---

## Related

- Infrastructure concern
- Can be done after core rewrite
