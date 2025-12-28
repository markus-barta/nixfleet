# P4500 - Generation Tracking and Visibility

**Created**: 2025-12-28  
**Completed**: 2025-12-28  
**Priority**: P4500 (🟡 High Priority - Sprint 2)  
**Status**: ✅ Done  
**Effort**: 2-3 hours  
**Depends on**: P3700, P3800

---

## User Story

**As a** fleet administrator  
**I want** to see which generation each host is running  
**So that** I can detect when hosts are on different versions and understand rollback targets

---

## Problem

Currently:

- Generation is hidden (only in tooltips or logs)
- Can't quickly see if hosts are on same version
- Rollback doesn't show target generation
- No visibility into generation history

### Current Problem

```
Host Table:
┌──────────┬─────────┬────────────┬─────────────┐
│ Hostname │ Status  │ Agent      │ Last Seen   │
├──────────┼─────────┼────────────┼─────────────┤
│ gpc0     │ ●●●●    │ 3.0.1      │ 2s ago      │
│ imac0    │ ●●●●    │ 3.0.1      │ 5s ago      │
└──────────┴─────────┴────────────┴─────────────┘

Q: Are they on the same generation?
A: Unknown! Have to click and check tooltip.
```

---

## Solution

**Add generation visibility:**

1. **Generation column** in host table
2. **Generation history** for rollback decisions
3. **Highlight drift** when hosts differ

### After Fix

```
Host Table:
┌──────────┬─────────┬────────────┬──────────┬─────────────┐
│ Hostname │ Status  │ Agent      │ Gen      │ Last Seen   │
├──────────┼─────────┼────────────┼──────────┼─────────────┤
│ gpc0     │ ●●●●●   │ 3.0.1      │ abc123   │ 2s ago      │
│ imac0    │ ●●●●●   │ 3.0.1      │ abc123   │ 5s ago      │
│ hsb0     │ ●●●●●   │ 3.0.1      │ def456 ⚠ │ 8s ago      │
└──────────┴─────────┴────────────┴──────────┴─────────────┘
                                        ↑
                            hsb0 is on different generation!
```

---

## Acceptance Criteria

### Generation Column

- [ ] Add "Gen" column between Agent and Last Seen
- [ ] Show first 6-7 chars of commit hash
- [ ] Tooltip shows: full hash + timestamp + commit message
- [ ] Highlight in yellow if generation differs from majority

### Rollback UI

- [ ] Show available generations when clicking rollback
- [ ] Display: generation number, commit hash, timestamp, "last known working"
- [ ] User sees exactly what they're rolling back to

### Generation History

- [ ] Track last 10 generations per host
- [ ] Show in dropdown: "Gen 48 (current), Gen 47, Gen 46..."
- [ ] Indicate which generation was last successful

---

## Technical Design

### Agent Changes

```go
// internal/agent/agent.go
type GenerationInfo struct {
    Hash      string    // Git commit hash
    Number    int       // NixOS generation number (e.g., 48)
    Timestamp time.Time // When this generation was activated
    Link      string    // Symlink path (e.g., /nix/var/nix/profiles/system-48-link)
}

func (a *Agent) detectGeneration() GenerationInfo {
    // Current generation (already implemented as string hash)
    currentHash := a.generation

    // NEW: Get NixOS generation number
    genNumber := a.getNixOSGenerationNumber()

    // NEW: Get generation timestamp
    timestamp := a.getGenerationTimestamp(genNumber)

    return GenerationInfo{
        Hash:      currentHash,
        Number:    genNumber,
        Timestamp: timestamp,
    }
}

func (a *Agent) getNixOSGenerationNumber() int {
    // NixOS: Read from /run/current-system
    link, _ := os.Readlink("/run/current-system")
    // Parse: /nix/store/...-nixos-system-hostname-48-link → 48

    // macOS: Read from home-manager profile
    link, _ := os.Readlink("~/.local/state/nix/profiles/home-manager")
    // Parse: home-manager-37-link → 37
}

func (a *Agent) listAvailableGenerations() []GenerationInfo {
    // NixOS: List /nix/var/nix/profiles/system-*-link
    // macOS: List home-manager generations
    // Return last 10
}
```

### Dashboard Changes

```go
// internal/templates/dashboard.templ
type Host struct {
    // ... existing fields ...
    Generation     string         // Git commit hash (existing)
    GenNumber      int            // NEW: NixOS generation number
    GenTimestamp   time.Time      // NEW: When generation was activated
    GenHistory     []GenerationInfo  // NEW: Last 10 generations
}

// Highlight generation drift
func detectGenerationDrift(hosts []Host) map[string]bool {
    // Count occurrences of each generation
    counts := make(map[string]int)
    for _, h := range hosts {
        counts[h.Generation]++
    }

    // Find majority generation
    var majority string
    maxCount := 0
    for gen, count := range counts {
        if count > maxCount {
            maxCount = count
            majority = gen
        }
    }

    // Mark hosts that differ
    drift := make(map[string]bool)
    for _, h := range hosts {
        if h.Generation != majority {
            drift[h.ID] = true
        }
    }
    return drift
}
```

### UI Components

```html
<!-- Generation column in table -->
<td class="gen-cell" :class="{ 'gen-drift': host.genDrift }">
  <span class="gen-short" @click="showGenHistory(host)">
    {{ host.generation.substr(0, 7) }}
  </span>
  <span
    v-if="host.genDrift"
    class="drift-icon"
    title="Generation differs from fleet"
    >⚠</span
  >
</td>
```

### Rollback Dialog

```
┌──────────────────────────────────────────────────┐
│ Rollback System - gpc0                           │
├──────────────────────────────────────────────────┤
│ Current Generation:                              │
│   Gen 48 (abc123)                                │
│   Dec 28, 14:32                                  │
│   System broken after switch                     │
│                                                  │
│ Available Generations:                           │
│                                                  │
│ ○ Gen 47 (def456)     ← Last known working       │
│   Dec 27, 18:15                                  │
│   "feat: update nixpkgs"                         │
│                                                  │
│ ○ Gen 46 (ghi789)                                │
│   Dec 26, 12:05                                  │
│   "fix: networking config"                       │
│                                                  │
│ ○ Gen 45 (jkl012)                                │
│   Dec 25, 09:30                                  │
│   "chore: weekly update"                         │
│                                                  │
│     [Cancel]  [Rollback to Selected]             │
└──────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- Add generation tracking to hosts
ALTER TABLE hosts ADD COLUMN gen_number INTEGER;
ALTER TABLE hosts ADD COLUMN gen_timestamp DATETIME;

-- Track generation history
CREATE TABLE IF NOT EXISTS generations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id     TEXT NOT NULL,
    gen_number  INTEGER NOT NULL,
    commit_hash TEXT NOT NULL,
    activated_at DATETIME NOT NULL,
    commit_message TEXT,
    success     INTEGER DEFAULT 1,  -- 0 if switch failed
    FOREIGN KEY (host_id) REFERENCES hosts(id)
);
CREATE INDEX IF NOT EXISTS idx_generations_host ON generations(host_id, gen_number DESC);
```

---

## Generation Drift Detection

### Scenario 1: Normal (All Same)

```
┌──────────┬──────────┐
│ gpc0     │ abc123   │
│ imac0    │ abc123   │  <- All same, no warning
│ hsb0     │ abc123   │
└──────────┴──────────┘
```

### Scenario 2: One Behind

```
┌──────────┬──────────┐
│ gpc0     │ abc123   │
│ imac0    │ abc123   │
│ hsb0     │ def456 ⚠ │  <- Different, show warning
└──────────┴──────────┘

Tooltip: "hsb0 is on different generation (def456). Others are on abc123."
```

### Scenario 3: Split Fleet

```
┌──────────┬──────────┐
│ gpc0     │ abc123   │
│ imac0    │ abc123   │
│ hsb0     │ def456 ⚠ │
│ csb0     │ def456 ⚠ │
└──────────┴──────────┘

Warning: "Fleet is split: 2 hosts on abc123, 2 hosts on def456"
```

---

## Testing Strategy

### Unit Tests

```go
func TestGenerationParsing(t *testing.T) {
    // Parse /nix/store/...-nixos-system-hostname-48-link
    // Extract generation number 48
}

func TestGenerationDriftDetection(t *testing.T) {
    // All same → no drift
    // One different → mark as drift
    // Multiple different → mark all non-majority
}
```

### Integration Tests

- [ ] Agent sends generation number in heartbeat
- [ ] Dashboard displays generation in table
- [ ] Generation drift highlighted correctly
- [ ] Rollback shows generation history

---

## Out of Scope

- Automatic drift resolution (future: P5501)
- Generation diff viewer (future: P5502)
- Generation notes/annotations (future: P5503)
- Cross-host generation sync enforcement (future: P5504)

---

## Related

- **P5600**: Rollback Operations (uses generation history)
- **P5200**: Lock Compartment - Version-Based Tracking
- **P5300**: System Compartment - Inference-Based Status
