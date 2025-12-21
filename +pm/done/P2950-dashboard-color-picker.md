# P2950 - Dashboard Color Picker (Phase 2)

**Created**: 2025-12-21  
**Completed**: 2025-12-21  
**Priority**: P2950 (Medium - Feature)  
**Status**: ✅ Done  
**Effort**: High (8-16h) - Actual: ~4h

---

## Summary

Allow changing host colors via NixFleet dashboard UI, with changes persisted to nixcfg's `theme-palettes.nix`.

---

## What Was Implemented

### 1. Color Picker UI (Frontend)

- Modal with 11 preset palette swatches (matching theme-palettes.nix)
- Native HTML5 `<input type="color">` for custom colors
- Hex input field with validation
- Live gradient preview (7 segments: lightest → darkest)
- Access via host row dropdown menu → "Change Color"

### 2. Color Picker API

- `POST /api/hosts/{hostID}/theme-color` - Set host color
- Request: `{ "color": "#ff6b6b", "palette": "yellow" }`
- Response includes nixcfg commit status

### 3. Gradient Generation (`v2/internal/colors/gradient.go`)

- HSL color manipulation (RGB ↔ HSL conversion)
- `GeneratePalette()` - Creates full palette from primary color
- `PresetPalettes` - List of built-in palettes

### 4. Nix File Generation (`v2/internal/colors/nix.go`)

- `UpdateHostPalette()` - Updates hostPalette entry
- `GenerateCustomPalette()` - Generates Nix code for custom palette
- `InsertCustomPalette()` - Adds new palette to theme-palettes.nix

### 5. Git Operations (`v2/internal/colors/git.go`)

- `NixcfgRepo` - Manages nixcfg repository clone
- Clone/pull with GitHub token authentication
- Commit with descriptive message: `theme(hostname): change color to palette`
- Push to main (ColorCommitMode = "push")

### 6. Configuration

New environment variables:

- `NIXFLEET_NIXCFG_PATH` - Local path for nixcfg clone (default: `/data/nixcfg`)
- `NIXFLEET_COLOR_COMMIT_MODE` - "push" or "pr" (default: "push")

Uses existing GitHub integration:

- `NIXFLEET_GITHUB_TOKEN` - For authentication
- `NIXFLEET_GITHUB_REPO` - owner/repo format

---

## User Flow

```
1. User opens NixFleet dashboard
2. Clicks host row → "⋮" menu → "Change Color"
3. Modal opens with current color selected
4. User picks preset OR enters custom hex color
5. Preview shows live gradient
6. User clicks "Apply Color"
7. Dashboard updates row gradient immediately (visual feedback)
8. Backend: Updates database + commits to nixcfg
9. Next rebuild: starship, zellij, eza use new color
```

---

## Files Created/Modified

| File                                    | Purpose                                    |
| --------------------------------------- | ------------------------------------------ |
| `v2/internal/colors/gradient.go`        | HSL manipulation, gradient generation      |
| `v2/internal/colors/nix.go`             | Nix file generation for theme-palettes.nix |
| `v2/internal/colors/git.go`             | Git clone/commit/push operations           |
| `v2/internal/dashboard/config.go`       | Added NixcfgRepoPath, ColorCommitMode      |
| `v2/internal/dashboard/server.go`       | Wired up NixcfgRepo                        |
| `v2/internal/dashboard/handlers.go`     | Added handleSetThemeColor                  |
| `v2/internal/templates/dashboard.templ` | Color picker modal, JavaScript             |
| `v2/internal/templates/base.templ`      | CSS styles, palette icon                   |

---

## Acceptance Criteria

- [x] Dashboard shows color picker UI per host
- [x] Palette presets available (11 colors matching theme-palettes.nix)
- [x] Custom hex color input with validation
- [x] Preview shows gradient before applying
- [x] `ColorCommitMode` setting controls PR vs direct push (code-only, default: push)
- [x] Custom colors auto-generate full gradient palette
- [x] Changes commit to nixcfg with descriptive message
- [x] Immediate visual feedback in dashboard (row gradient updates)
- [ ] Validation with `nix eval` before commit (TODO: future enhancement)
- [ ] PR mode implementation (TODO: future enhancement)

---

## Future Enhancements

1. **PR Mode**: Create branch and GitHub PR instead of direct push
2. **Nix Validation**: Run `nix eval` before commit to verify syntax
3. **Settings UI**: Expose ColorCommitMode in dashboard settings (P6400)
4. **Bulk Color Change**: Apply same color to multiple hosts

---

## Related Tasks

- **P7200** (nixcfg): Phase 1 ✅ done - Wired theme colors to agent
- **P2900** (nixfleet): ✅ done - Dashboard theme color display
- **P6400** (nixfleet): Settings page (future: expose ColorCommitMode)
