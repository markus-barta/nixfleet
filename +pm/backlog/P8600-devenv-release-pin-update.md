# P8600-devenv-release-pin-update

**Created**: 2025-12-28  
**Updated**: 2025-12-28  
**Priority**: P6500 (✅ Quick Win - Do Anytime)  
**Status**: Backlog

**Note**: Priority maintained - quick maintenance task, do when convenient

## Summary

Update `devenv.yaml` nixpkgs pin from `release-25.05` to next stable when approaching EOL.

## Context

- Pinned to stable release to fix recurring daily LSP/Go path issues (rolling channel invalidated daily)
- `release-25.05` EOL: ~May 2025 (next release: `release-25.11`)
- Check ~1 month before EOL if still in use

## Acceptance Criteria

- [ ] Check nixpkgs EOL status
- [ ] Update `devenv.yaml` to new `release-25.XX`
- [ ] Run `direnv allow && direnv reload`
- [ ] Verify `which go`, `which templ`, `which gopls` work
- [ ] Commit `flake.lock`

## Notes

- This is maintenance, not urgent
- Only do when approaching EOL or when next release drops (May/November)
- GitHub Actions likely handles `flake.lock` updates anyway; this is just the devenv pin
