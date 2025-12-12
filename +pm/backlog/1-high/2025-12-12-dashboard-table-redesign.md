# Dashboard Table Redesign

**Created**: 2025-12-12
**Priority**: High
**Status**: In Progress (broken, needs fix)

## Problem

The initial implementation (commit `42c7ea9`) has broken column alignment - headers don't match data cells. The specification was correct but implementation was rushed.

## Desired Final Structure

### Header Targets Box

Two side-by-side targets for easy comparison:

```
┌─────────────────────────────────┐
│ Config          │ Fleet         │
│ [↻] 7872e6e     │ 42c7ea9       │
└─────────────────────────────────┘
```

- **Config**: Latest nixcfg commit hash (with refresh button)
- **Fleet**: Latest nixfleet build hash (static, from build)
- Both link to GitHub commits

### Table Columns (11 total)

| # | Column | Width | Content |
|---|--------|-------|---------|
| 1 | **Host** | auto | Status dot (●/○) + hostname in theme color |
| 2 | **OS** | narrow | OS icon (/) + version (26.05 / 15.2). Hover shows nixpkgs |
| 3 | **Loc** | icon | Location icon (cloud/home/work) |
| 4 | **Type** | icon | Device type icon (server/desktop/laptop/gaming) |
| 5 | **Metrics** | narrow | CPU/RAM percentages |
| 6 | **Config** | narrow | nixcfg hash (7 chars), ↓ if outdated vs header Config |
| 7 | **Fleet** | narrow | agent hash (7 chars), ↓ if outdated vs header Fleet |
| 8 | **Last Seen** | narrow | Relative time (just now, 5 min ago) |
| 9 | **Tests** | narrow | Test results (6/17 ✓) |
| 10 | **Status** | auto | Combined: status text + last action comment |
| 11 | **Actions** | auto | Pull/Switch/Test buttons + dropdown menu |

### Example Row

```
● csb1 |  26.05 | ☁ | ⚙ | 4/51 | 7872e6e | 42c7ea9 | just now | 8/17 ✓ | Pull successful | [Pull][Switch][Test][⋮]
```

### Status Column Content (Combined)

The Status column replaces old separate Status + Comment columns:

| Condition | Display |
|-----------|---------|
| Testing | `Testing 3/8` (yellow badge) |
| Pending command | `⏳ Pulling...` (blue badge) |
| Offline | `Offline` (gray badge) |
| Online + no comment | `Online` (green badge) |
| Online + has comment | `✓ Pull successful` or `✗ Build failed` |

### Host Column Details

- Status dot at start (before hostname)
  - Green dot with glow = online
  - Gray dot = offline
- Hostname in host's theme color
- NO OS icon (moved to OS column)

### OS Column Details

- OS icon: NixOS snowflake or Apple logo
- Version: First 5 chars (e.g., "26.05", "15.2")
- Hover tooltip: Full version + nixpkgs revision

### Config/Fleet Columns

- 7-character hash in monospace
- Gray background normally
- Yellow background + ↓ arrow if outdated compared to header target
- Click to copy (future enhancement)

## Implementation Notes

1. Ensure `<thead>` columns match `<tbody>` cell count exactly
2. Test with various screen widths
3. Verify SSE updates target correct cells (nth-child indices changed)
4. Update all JavaScript that references old selectors (.comment, .version-cell, etc.)

## Current Issues (from failed attempt)

- [ ] Column headers don't align with data cells
- [ ] Cell content appears in wrong columns
- [ ] SSE updates may target wrong cells (nth-child indices)
- [ ] Some old CSS classes may still be referenced

## Files to Modify

- `app/templates/dashboard.html` - main template with CSS and JS
- `app/main.py` - may need to pass additional data to template

## Rollback Option

If needed, revert to commit `fcaa4e3` (before table redesign).

