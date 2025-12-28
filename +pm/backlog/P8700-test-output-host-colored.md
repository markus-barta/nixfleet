# P8700 - Host-Colored Test Output Headers

**Created**: 2025-12-28  
**Priority**: P8700 (📋 Quality & Polish)  
**Status**: Backlog  
**Depends on**: None

---

## User Story

**As a** developer  
**I want** test output headers colored by each host's theme color  
**So that** I can visually distinguish test runs by host and improve terminal readability

---

## Overview

When running tests in `tests/T00-*.sh`, `tests/T01-*.sh`, etc., the output headers currently use a **fixed cyan color**:

```bash
header() { echo -e "${CYAN}$1${NC}"; }
```

This produces test suite boxes that all look the same regardless of which host they're testing.

### Current Behavior

```
[0;36m╔════════════════════════════════════════════╗[0m
[0;36m║       hsb1 Test Suite                      ║[0m
[0;36m╚════════════════════════════════════════════╝[0m

[0;36m╔════════════════════════════════════════════╗[0m
[0;36m║       gpc0 Test Suite                      ║[0m
[0;36m╚════════════════════════════════════════════╝[0m
```

**Problem**: Both test headers are the same color (cyan), making it harder to visually scan and distinguish test runs.

---

## Proposed Solution

**Leverage the existing theme color system** that agents already send to the dashboard:

1. Each host has a `theme_color` (e.g., `#769ff0` for hsb1, set via starship/uzumaki)
2. Pass this color to test scripts via **environment variables** or a **config file**
3. **Convert hex color to ANSI terminal color** using existing color utilities
4. Apply the host's theme color to test headers instead of fixed cyan

### Result

```
[Hsb1's blue]╔════════════════════════════════════════════╗[0m
[Hsb1's blue]║       hsb1 Test Suite                      ║[0m
[Hsb1's blue]╚════════════════════════════════════════════╝[0m

[Gpc0's teal]╔════════════════════════════════════════════╗[0m
[Gpc0's teal]║       gpc0 Test Suite                      ║[0m
[Gpc0's teal]╚════════════════════════════════════════════╝[0m
```

---

## Design Decisions

### Color Delivery Method

**Option A: Environment Variable** (Recommended)

- Pass `NIXFLEET_THEME_COLOR=#769ff0` to test scripts
- Minimal test script changes
- Works well with CI/CD

```bash
NIXFLEET_THEME_COLOR="#769ff0" bash tests/T00-heartbeat-metrics.sh
```

**Option B: Config File**

- Read from `$HOME/.config/nixfleet/theme-color`
- More persistent but requires setup

### Hex → ANSI Conversion

Two approaches:

1. **256-color ANSI** (better compatibility)
   - Convert hex → RGB → closest 256-color ANSI code
   - Works in most terminals
   - Example: `#769ff0` → `\033[38;5;147m`

2. **True Color (24-bit)**
   - Use hex directly: `\033[38;2;118;159;240m`
   - Requires modern terminal support
   - Looks better

**Recommendation**: Implement True Color with 256-color fallback

### Fallback

If no theme color is provided:

- Fall back to current cyan (`\033[0;36m`)
- No breaking changes

---

## Implementation Tasks

- [ ] **Color Utility Function**
  - [ ] Create `colors/ansi.go` helper to convert hex → ANSI codes
  - [ ] Support both 24-bit and 256-color fallback
  - [ ] Test with various hex colors

- [ ] **Test Scripts**
  - [ ] Update `tests/T00-heartbeat-metrics.sh` to accept `NIXFLEET_THEME_COLOR`
  - [ ] Update `tests/T01-command-pull.sh`
  - [ ] Update `tests/T02-command-switch.sh`
  - [ ] Update `tests/T03-command-update-agent.sh`
  - [ ] Update `+pm/tests/automated/T-*.sh` scripts

- [ ] **Integration**
  - [ ] Test locally with various colors
  - [ ] Verify fallback behavior when color is missing
  - [ ] Verify ANSI escapes display correctly in logs

- [ ] **Documentation**
  - [ ] Update test README to explain color theming
  - [ ] Document how to pass theme colors

---

## Affected Files

```
src/internal/colors/ansi.go (NEW)
tests/T00-heartbeat-metrics.sh
tests/T01-command-pull.sh
tests/T02-command-switch.sh
tests/T03-command-update-agent.sh
+pm/tests/automated/T-*.sh (all test scripts)
tests/README.md
```

---

## Notes

- ✅ Host theme colors already exist in agent config (starship/uzumaki)
- ✅ Dashboard already handles theme color display
- ✅ Colors are already validated as hex format (`#rrggbb`)
- ✅ This is a **pure UX improvement** with no functional changes

---

## Related

- **P3900** - Test Results Compartment (tests are tracked in dashboard)
- **P2700** - Operation Progress Tracking (test progress shown in UI)
- Dashboard theme color system (`internal/dashboard/handlers.go` lines 1201-1251)
