# P1000: CRITICAL - Nix Palette Insertion Bug

**Priority**: P1000 (Critical - Data Corruption)
**Status**: Open
**Created**: 2025-12-21
**Component**: `v2/internal/colors/nix.go`

---

## Summary

The `InsertCustomPalette` function in `nix.go` inserts custom palettes at the **wrong location** in `theme-palettes.nix`, breaking NixOS builds for affected hosts.

---

## Impact

- **Severity**: Critical
- **Affected**: Any host that gets a custom color via NixFleet dashboard
- **Result**: `nixos-rebuild switch` fails with `attribute 'custom-<hostname>' missing`
- **Evidence**: hsb8 failed to build after NixFleet set a custom color (commit `4f6e8bb3`)

---

## Root Cause

In `InsertCustomPalette()` (lines 178-192):

```go
// Find the }; before hostPalette
searchArea := content[:hostPaletteIdx]
lastBraceIdx := strings.LastIndex(searchArea, "};")  // BUG: Finds WRONG brace!
```

**Problem**: This finds the **last** `};` before `hostPalette`, which is the closing brace of `statusColors`, NOT `palettes`.

**Result**: Custom palette gets inserted:

- Inside `statusColors` block (wrong location)
- At 4-space indentation (wrong - should be inside `palettes` at 4-space)
- The closing `};` ends up at column 0, prematurely closing the outer block

---

## What Happened

Before (correct structure):

```nix
  palettes = {
    # ... all palettes ...
  };

  statusColors = {
    # ... status colors ...
    rootChar = { ... };
  };

  hostPalette = { ... };
```

After NixFleet insertion (broken):

```nix
  palettes = {
    # ... all palettes ...
  };

  statusColors = {
    # ... status colors ...
    rootChar = { ... };

    # Custom palettes (auto-generated)  <- WRONG: inside statusColors!
    custom-hsb8 = { ... };
};  <- WRONG: closes at column 0!

  hostPalette = { ... };  <- Now outside the main block!
```

---

## Fix Required

The `InsertCustomPalette` function needs to find the correct closing brace for the `palettes` block, not just "the last `};` before hostPalette".

### Option A: Find palettes closing brace by structure

```go
// Find "palettes = {" and count braces to find its matching close
palettesStart := strings.Index(content, "palettes = {")
if palettesStart == -1 {
    return "", fmt.Errorf("could not find palettes block")
}

// Count braces to find matching close
braceCount := 0
palettesEnd := -1
inBlock := false
for i := palettesStart; i < len(content); i++ {
    if content[i] == '{' {
        braceCount++
        inBlock = true
    } else if content[i] == '}' {
        braceCount--
        if inBlock && braceCount == 0 {
            palettesEnd = i
            break
        }
    }
}

// Insert before palettesEnd (the closing brace of palettes)
```

### Option B: Look for specific indentation pattern

```go
// Find "  };" at exactly 2-space indent that comes before hostPalette
// and after the last palette definition
```

### Option C: Use a marker comment

Add a permanent marker comment at the end of palettes:

```nix
    # --- END OF PALETTES (do not remove) ---
  };
```

---

## Testing Required

After fix, verify:

1. New custom palette insertion works correctly
2. Existing custom palette update works correctly
3. Multiple custom palettes work correctly
4. File structure remains valid Nix syntax
5. `nix-instantiate --parse` passes
6. Host can successfully `nixos-rebuild switch`

---

## Workaround

Manual fix in nixcfg: Move any misplaced `custom-*` palettes inside the `palettes = { ... }` block and fix indentation.

---

## Related

- Commit that caused the issue: `4f6e8bb3` (NixFleet auto-commit)
- Fixed in nixcfg: `42da2a67` (manual correction)
- Feature: P2900 (Host Theme Colors)
