# P6500 - Cursor/VS Code Direnv Extension Setup

**Created**: 2025-12-16  
**Priority**: P6500 (Low)  
**Status**: Backlog

---

## Problem

Cursor's Go extension can't find the `go` binary because it doesn't automatically use the nix/devenv shell environment.

**Error**: "Failed to find the 'go' binary in either GOROOT() or PATH(...)"

---

## Current State

- direnv CLI is installed and working
- `.envrc` exists and loads devenv shell correctly in terminal
- But Cursor doesn't pick up the environment

---

## Investigation Needed

1. **direnv extension**: User searched for "direnv" by Martin Kühl but couldn't find it in Cursor's extension marketplace
   - Is it VS Code only, not available in Cursor?
   - Alternative extensions?

2. **Alternatives to explore**:
   - Nix Environment Selector extension
   - Manually configure `go.goroot` in `.vscode/settings.json`
   - Start Cursor from within nix shell (`nix develop --command cursor .`)
   - Use `nix-direnv` with different integration

---

## Workarounds

### Option A: Start Cursor from nix shell

```bash
cd /Users/markus/Code/nixfleet
nix develop --command cursor .
```

### Option B: Configure Go paths manually

Create `.vscode/settings.json`:

```json
{
  "go.goroot": "/nix/store/...-go-1.25.4/share/go",
  "go.toolsEnvVars": {
    "PATH": "/nix/store/...-go-1.25.4/bin:${env:PATH}"
  }
}
```

(Paths would need to be updated after each `nix flake update`)

---

## Acceptance Criteria

- [ ] Find working direnv extension for Cursor (or confirm none exists)
- [ ] Document the recommended setup for nix + Cursor
- [ ] IDE automatically finds Go, gopls, templ without manual config
- [ ] Solution works across nixcfg and nixfleet workspaces

---

## Related

- devenv.nix provides the development environment
- flake.nix devShell also provides Go toolchain
