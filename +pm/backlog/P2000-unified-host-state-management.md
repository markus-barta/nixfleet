# P2000 - Unified Host State Management

**Created**: 2025-12-18  
**Updated**: 2025-12-19  
**Priority**: P2000 (High - Architecture)  
**Status**: ⚠️ NEEDS REASSESSMENT after P1000  
**Estimated Effort**: 2-3 days  
**Breaking Change**: Yes (complete JS rewrite)  
**Depends on**: P1000 (Reliable Agent Updates)

> **Note**: The current UI is functional. Fix P1000 (agent updates) first, then evaluate if this complete rewrite is still needed or if incremental improvements suffice.

---

## Executive Summary

Replace the current ad-hoc host state management with a **single-source-of-truth architecture**. This is a **hard cut** — no parallel code, no phased migration. The entire client-side state system is rewritten in one PR.

**Core principle**: All host state flows through one store, one render function, one update path.

---

## Current State (What We're Replacing)

### Architecture Diagram: BEFORE

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           WebSocket Messages                             │
│  host_update | command_queued | command_complete | flake_update_pr | ... │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         │                         │                         │
         ▼                         ▼                         ▼
  handleMessage()           handleFlakeUpdatePR()     handleFlakeUpdateJob()
         │                         │                         │
         ▼                         │                         │
    updateHost()                   │                         │
         │                         ▼                         ▼
    ┌────┴────┬────────────────────────────────────────────────┐
    │         │                                                │
    ▼         ▼                                                ▼
updateCell  updateStatus    applyButton    setHostBusy    updateProgress
  Data()   Indicator()       States()          ()          Badge()
    │                            │              │
    ├──────────┐                 │              │
    │          │                 └──────────────┤
    ▼          ▼                                ▼
updateMetrics()  updateStatus              (DUPLICATED
                Compartments()              badge logic)
```

**Problems:**

1. **6 WebSocket message types** touching host state
2. **12 JavaScript functions** with overlapping responsibilities
3. **3 places** where badge logic is duplicated
4. **No canonical state** — DOM is the source of truth
5. **Template/JS duplication** — same logic in Go and JS

### Functions to DELETE (Complete List)

| Function                     | Lines | Responsibility              | Replaced By             |
| ---------------------------- | ----- | --------------------------- | ----------------------- |
| `updateHost()`               | 25    | Orchestrator                | `hostStore.update()`    |
| `updateCellData()`           | 30    | Metrics, gen, last_seen     | `renderHost()`          |
| `updateMetrics()`            | 35    | CPU/RAM display             | `renderHost()`          |
| `updateStatusCompartments()` | 35    | Git/Lock/System indicators  | `renderHost()`          |
| `updateStatusIndicator()`    | 40    | Ripple/dot rendering        | `renderHost()`          |
| `updateProgressBadge()`      | 15    | Command badge               | `renderHost()`          |
| `applyButtonStates()`        | 50    | Button enable + badge (dup) | `renderHost()`          |
| `setHostBusy()`              | 15    | Row + card update (dup)     | `hostStore.update()`    |
| `handleFlakeUpdatePR()`      | 40    | Global banner               | DELETE (no replacement) |
| `handleFlakeUpdateJob()`     | 35    | Global banner progress      | DELETE (no replacement) |
| `getCompartmentClass()`      | 10    | CSS class helper            | `renderHost()` inline   |
| `getIndicatorClass()`        | 10    | CSS class helper            | `renderHost()` inline   |
| `getCompartmentTooltip()`    | 60    | Tooltip builder             | Server-side only        |

**Total lines deleted: ~400**
**Total lines added: ~200** (50% reduction)

### WebSocket Message Types to REMOVE

| Message Type       | Current Use            | Replacement                      |
| ------------------ | ---------------------- | -------------------------------- |
| `host_update`      | All host data          | `host_heartbeat` (minimal)       |
| `flake_update_pr`  | Global banner          | DELETE                           |
| `flake_update_job` | Global deploy progress | DELETE (use `command_*` instead) |

### WebSocket Message Types to KEEP

| Message Type       | Use                          |
| ------------------ | ---------------------------- |
| `host_heartbeat`   | online + metrics + last_seen |
| `host_offline`     | Agent disconnected           |
| `command_queued`   | Show log panel               |
| `command_output`   | Log lines                    |
| `command_complete` | Clear pending state          |

---

## Target State (What We're Building)

### Architecture Diagram: AFTER

```
┌─────────────────────────────────────────────────────────────┐
│                    WebSocket (3 host-related message types) │
│         host_heartbeat | host_offline | command_*           │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                        hostStore                             │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Map<hostId, HostState>                                │  │
│  │  ────────────────────────────────────────────────────  │  │
│  │  {                                                     │  │
│  │    id, hostname, hostType, themeColor,                 │  │
│  │    online, lastSeen, pendingCommand,                   │  │
│  │    metrics: { cpu, ram, swap, load },                  │  │
│  │    updateStatus: { git, lock, system },                │  │
│  │    generation, agentVersion, agentOutdated             │  │
│  │  }                                                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  hydrate()      → Populate from server-rendered data-*      │
│  get(id)        → Read host state                           │
│  update(id, Δ)  → Patch state, trigger render               │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                      renderHost(hostId)                      │
│  ──────────────────────────────────────────────────────────  │
│  SINGLE function that updates ALL host UI:                   │
│                                                              │
│  1. Find elements: tr[data-host-id] + .host-card[data-*]     │
│  2. Derive computed state: isOnline, isBusy, buttonsEnabled  │
│  3. Update status indicator (ripple/dot)                     │
│  4. Update offline class                                     │
│  5. Update metrics display                                   │
│  6. Update compartments (Git/Lock/System)                    │
│  7. Update button states                                     │
│  8. Update progress badge                                    │
│  9. Update last-seen timestamp                               │
└──────────────────────────────────────────────────────────────┘
```

### Data Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Page Load   │     │  WebSocket   │     │   API Call   │
│  (hydrate)   │     │  (realtime)  │     │  (on-demand) │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │                    │                    │
       ▼                    ▼                    ▼
┌──────────────────────────────────────────────────────────┐
│                   hostStore.update()                     │
└────────────────────────────┬─────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────┐
│                     renderHost()                         │
└────────────────────────────┬─────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────┐
│                    DOM Updated                           │
│              (both table row AND card)                   │
└──────────────────────────────────────────────────────────┘
```

---

## API Design

### New Endpoint: Refresh Host Status

```
POST /api/hosts/{hostID}/refresh
Authorization: Session cookie
X-CSRF-Token: required

Response 200:
{
  "host_id": "hsb1",
  "online": true,
  "generation": "abc1234",
  "agent_version": "2.1.0",
  "agent_outdated": false,
  "update_status": {
    "git": { "status": "ok", "message": "Up to date", "checked_at": "..." },
    "lock": { "status": "outdated", "message": "7 days ago", "checked_at": "..." },
    "system": { "status": "ok", "message": "Current", "checked_at": "..." }
  },
  "pending_pr": null | { "number": 42, "title": "...", "url": "...", "mergeable": true }
}
```

### Modified WebSocket Messages

**BEFORE: `host_update` (kitchen sink)**

```json
{
  "type": "host_update",
  "payload": {
    "host_id": "hsb1",
    "online": true,
    "last_seen": "...",
    "generation": "abc1234",
    "nixpkgs_version": "...",
    "pending_command": null,
    "metrics": { "cpu": 25, "ram": 60, ... },
    "update_status": { "git": {...}, "lock": {...}, "system": {...} }
  }
}
```

**AFTER: `host_heartbeat` (minimal)**

```json
{
  "type": "host_heartbeat",
  "payload": {
    "host_id": "hsb1",
    "online": true,
    "last_seen": "2025-12-18T10:30:00Z",
    "metrics": { "cpu": 25, "ram": 60, "swap": 5, "load": 1.2 }
  }
}
```

**AFTER: `host_offline` (explicit disconnect)**

```json
{
  "type": "host_offline",
  "payload": {
    "host_id": "hsb1"
  }
}
```

---

## UI Changes

### 1. Remove Global Flake Update Banner

**DELETE this entire section from `dashboard.templ`:**

```templ
<!-- Flake Update Banner (P4300) -->
if data.PendingPR != nil {
    <div id="flake-update-banner" class="flake-update-banner">
        @FlakeUpdateBanner(data.PendingPR, data.CSRFToken)
    </div>
} else {
    <div id="flake-update-banner" ...></div>
}
```

**MOVE to Bulk Actions dropdown:**

```templ
<div class="dropdown-menu" id="bulk-actions-menu">
    if data.PendingPR != nil && data.PendingPR.Mergeable {
        <button class="dropdown-item" onclick="mergeAndDeploy()">
            <svg class="icon"><use href="#icon-merge"></use></svg>
            Merge & Deploy PR #{ data.PendingPR.Number }
        </button>
        <div class="dropdown-divider"></div>
    }
    <!-- existing bulk actions -->
</div>
```

### 2. Per-Host Refresh Button

**Add to action buttons (appears on hover):**

```templ
<td class="actions-cell">
    <div class="action-buttons">
        @CommandButton(host.ID, "pull", ...)
        @CommandButton(host.ID, "switch", ...)
        @CommandButton(host.ID, "test", ...)
        <button
            class="btn btn-refresh"
            onclick="refreshHost(this.dataset.hostId)"
            data-host-id={ host.ID }
            title="Refresh status"
        >
            <svg class="icon"><use href="#icon-refresh-cw"></use></svg>
        </button>
        @ActionDropdown(host)
    </div>
</td>
```

**CSS (hover visibility):**

```css
.btn-refresh {
  opacity: 0;
  transition: opacity 0.15s;
}
tr:hover .btn-refresh,
.host-card:hover .btn-refresh {
  opacity: 1;
}
.btn-refresh.loading {
  animation: spin 1s linear infinite;
}
```

### 3. Pending PR Indicator on Lock Compartment

When PR pending, Lock compartment gets visual indicator:

```css
.update-compartment.has-pr {
  position: relative;
}
.update-compartment.has-pr::after {
  content: "";
  position: absolute;
  top: -2px;
  right: -2px;
  width: 6px;
  height: 6px;
  background: var(--color-info);
  border-radius: 50%;
}
```

Tooltip shows PR info when hovering Lock compartment.

---

## Complete File Changes

### Files to MODIFY

| File                                     | Type     | Changes                                                                    |
| ---------------------------------------- | -------- | -------------------------------------------------------------------------- |
| `v2/internal/templates/dashboard.templ`  | Template | Remove banner, add refresh btn, add data-\* attrs, rewrite JS (~400 lines) |
| `v2/internal/templates/base.templ`       | Template | Add `#icon-refresh-cw` SVG                                                 |
| `v2/internal/dashboard/handlers.go`      | Go       | Add `handleRefreshHost()`, modify `getHosts()` for data-\* attrs           |
| `v2/internal/dashboard/server.go`        | Go       | Register `POST /api/hosts/{id}/refresh`                                    |
| `v2/internal/dashboard/hub.go`           | Go       | Change `host_update` → `host_heartbeat`, add `host_offline`                |
| `v2/internal/dashboard/flake_updates.go` | Go       | Remove `broadcastPRStatus()`, simplify to bulk-only                        |

### Files UNCHANGED

| File                            | Reason                               |
| ------------------------------- | ------------------------------------ |
| `v2/internal/agent/*`           | Agent protocol unchanged             |
| `v2/internal/protocol/*`        | Message types unchanged (names only) |
| `v2/internal/dashboard/auth.go` | Auth unrelated                       |
| `v2/internal/dashboard/logs.go` | Log handling unrelated               |

---

## JavaScript Specification

### Complete New Script (Replaces Existing)

The new JavaScript has exactly these top-level constructs:

```javascript
// ═══════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════
const HEARTBEAT_INTERVAL = /* from data-* */;
const CSRF_TOKEN = /* from data-* */;

// ═══════════════════════════════════════════════════════════
// HOST STORE (Single Source of Truth)
// ═══════════════════════════════════════════════════════════
const hostStore = {
    _hosts: new Map(),
    hydrate() { /* ... */ },
    get(id) { /* ... */ },
    update(id, patch) { /* ... triggers renderHost */ },
    setOffline(id) { /* ... */ }
};

// ═══════════════════════════════════════════════════════════
// RENDER (Single Render Function)
// ═══════════════════════════════════════════════════════════
function renderHost(hostId) { /* ... */ }

// ═══════════════════════════════════════════════════════════
// WEBSOCKET
// ═══════════════════════════════════════════════════════════
let ws = null;
function connectWebSocket() { /* ... */ }
function handleMessage(msg) {
    switch (msg.type) {
        case 'host_heartbeat': /* hostStore.update() */ break;
        case 'host_offline': /* hostStore.setOffline() */ break;
        case 'command_queued': /* showLogPanel() */ break;
        case 'command_output': /* appendLog() */ break;
        case 'command_complete': /* hostStore.update() */ break;
    }
}

// ═══════════════════════════════════════════════════════════
// ACTIONS (User-initiated)
// ═══════════════════════════════════════════════════════════
function sendCommand(hostId, command) { /* ... */ }
function refreshHost(hostId) { /* fetch + hostStore.update */ }
function mergeAndDeploy() { /* bulk action */ }
function bulkCommand(command) { /* ... */ }

// ═══════════════════════════════════════════════════════════
// LOG PANEL (Alpine.js component - unchanged)
// ═══════════════════════════════════════════════════════════
Alpine.data('logViewer', () => ({ /* ... */ }));

// ═══════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════
function formatLastSeen(isoString) { /* ... */ }
function updateConnectionStatus(connected) { /* ... */ }

// ═══════════════════════════════════════════════════════════
// MODALS & DROPDOWNS (Keep existing, no changes)
// ═══════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════
hostStore.hydrate();
connectWebSocket();
setInterval(updateAllLastSeen, 1000);
```

**Functions NOT in new code (deleted):**

- `updateHost()`
- `updateCellData()`
- `updateMetrics()`
- `updateStatusCompartments()`
- `updateStatusIndicator()`
- `updateProgressBadge()`
- `applyButtonStates()`
- `setHostBusy()`
- `handleFlakeUpdatePR()`
- `handleFlakeUpdateJob()`
- `getCompartmentClass()`
- `getIndicatorClass()`
- `getCompartmentTooltip()`
- `updateAgentBadge()`
- `formatAgentTooltip()`
- `checkForUpdates()`
- `dismissFlakeUpdate()`
- `refreshStatus()` (old version)

---

## Acceptance Criteria

All criteria must pass before merge:

### Functional

- [ ] Host state is managed by `hostStore` — no other source of truth
- [ ] All host UI updates go through `renderHost()` — no exceptions
- [ ] WebSocket only sends `host_heartbeat` for live data (metrics + online)
- [ ] Refresh button appears on hover, triggers API call
- [ ] API `/api/hosts/{id}/refresh` returns complete host status
- [ ] Global flake banner is removed
- [ ] "Merge & Deploy" is in Bulk Actions dropdown (when PR pending)
- [ ] Lock compartment shows dot indicator when PR pending
- [ ] Both table row AND card update correctly
- [ ] Last-seen timer updates every second

### Non-Functional

- [ ] Total JS lines reduced by ≥30%
- [ ] No console errors during normal operation
- [ ] WebSocket reconnection works
- [ ] Page load performance unchanged or improved

### Testing

- [ ] Manual: Verify all hosts show correct status after page load
- [ ] Manual: Verify heartbeat updates metrics in real-time
- [ ] Manual: Verify refresh button updates compartments
- [ ] Manual: Verify command execution updates pending state
- [ ] Manual: Verify offline detection works when agent stops
- [ ] Integration: `t05_dashboard_websocket_test.go` updated and passing

---

## Rollback Plan

If critical issues discovered post-deploy:

1. `git revert` the PR
2. Redeploy previous version
3. No data migration needed (state is ephemeral)

---

## Senior Review: Edge Cases & Refinements

This section documents edge cases and architectural refinements identified during senior-level review.

### 1. New Host Registration

**Problem**: `hostStore.hydrate()` only reads existing DOM. If a new agent connects after page load, the client won't know about it.

**Solution**: Add `host_register` WebSocket message type:

```javascript
case 'host_register':
    // New host connected - add to store
    hostStore.add(msg.payload);
    // Insert DOM elements (or show "refresh page" toast)
    break;
```

**Alternative (simpler)**: Document that page refresh is required for new hosts. This is acceptable for v2.1.0 since new hosts are rare.

### 2. Deep Merge for Nested Objects

**Problem**: `hostStore.update(id, { updateStatus: { git: {...} } })` would lose `lock` and `system` with shallow merge.

**Solution**: Use deep merge for known nested objects:

```javascript
update(id, patch) {
    const current = this._hosts.get(id);
    if (!current) return;

    // Deep merge for specific nested objects
    const next = { ...current, ...patch };
    if (patch.updateStatus && current.updateStatus) {
        next.updateStatus = { ...current.updateStatus, ...patch.updateStatus };
    }
    if (patch.metrics && current.metrics) {
        next.metrics = { ...current.metrics, ...patch.metrics };
    }

    this._hosts.set(id, next);
    renderHost(id);
}
```

### 3. Pending PR Indicator (Global State)

**Problem**: PR status is global (not per-host) but needs to affect Lock compartment display.

**Solution**: Store PR status separately from host state:

```javascript
let globalState = {
  pendingPR: null, // { number, title, url, mergeable }
  wsConnected: false,
};

// On page load, hydrate from data attribute
globalState.pendingPR = JSON.parse(document.body.dataset.pendingPr || "null");

// In renderUpdateStatus(), check global PR state for Lock compartment
if (type === "lock" && globalState.pendingPR) {
  comp.classList.add("has-pr");
}
```

### 4. Error Handling in refreshHost()

**Problem**: API errors should provide user feedback.

**Solution**: Add toast/notification on error:

```javascript
async function refreshHost(hostId) {
    const btn = document.querySelector(`button.btn-refresh[data-host-id="${hostId}"]`);
    if (btn) btn.classList.add('loading');

    try {
        const resp = await fetch(`/api/hosts/${hostId}/refresh`, {...});

        if (resp.status === 404) {
            showToast('Host no longer exists', 'error');
            // Optionally remove from store and DOM
            return;
        }

        if (!resp.ok) {
            showToast('Refresh failed: ' + resp.statusText, 'error');
            return;
        }

        const data = await resp.json();
        hostStore.update(hostId, {...});

    } catch (err) {
        showToast('Network error', 'error');
        console.error('Refresh failed:', err);
    } finally {
        if (btn) btn.classList.remove('loading');
    }
}
```

### 5. handleMessage Defensive Coding

**Problem**: Malformed WebSocket payloads could crash the client.

**Solution**: Add validation:

```javascript
function handleMessage(msg) {
  if (!msg || !msg.type) {
    console.warn("Invalid message:", msg);
    return;
  }

  const payload = msg.payload || {};
  const hostId = payload.host_id;

  switch (msg.type) {
    case "host_heartbeat":
      if (!hostId) return;
      hostStore.update(hostId, {
        online: true,
        lastSeen: payload.last_seen,
        metrics: payload.metrics,
      });
      break;
    // ... etc
  }
}
```

### 6. Element Caching for Performance

**Problem**: `renderHost()` queries DOM on every heartbeat (~10/second for 10 hosts = 100 queries/second).

**Solution**: Cache element references in store:

```javascript
hydrate() {
    document.querySelectorAll('tr[data-host-id]').forEach((row) => {
        const id = row.dataset.hostId;
        const card = document.querySelector(`.host-card[data-host-id="${id}"]`);

        this._hosts.set(id, {
            // ... state
            _elements: { row, card }  // Cache DOM refs
        });
    });
}

function renderHost(hostId) {
    const host = hostStore.get(hostId);
    if (!host) return;

    const { row, card } = host._elements || {};
    [row, card].filter(Boolean).forEach((el) => {
        // ... render
    });
}
```

### 7. Offline Detection Edge Case

**Problem**: If agent disconnects but `host_offline` message is lost (network issue), client shows stale "online" status.

**Solution**: Already mitigated by `lastSeen` display. Consider adding client-side timeout:

```javascript
// Every 30s, check if lastSeen > 60s ago, mark as "stale" (not offline, but uncertain)
setInterval(() => {
  hostStore.all().forEach((host) => {
    if (host.online && host.lastSeen) {
      const age = Date.now() - new Date(host.lastSeen).getTime();
      if (age > 60000) {
        // Add "stale" indicator (yellow border or dimmed)
      }
    }
  });
}, 30000);
```

### 8. Tooltip Generation Strategy

**Problem**: `getCompartmentTooltip()` is deleted but tooltips still needed.

**Solution**: Server-side tooltip generation via `title` attribute:

```templ
<div class="update-compartment" title={ buildCompartmentTooltip(host, "git") }>
```

The JS `renderUpdateStatus()` should preserve the `title` attribute from initial render unless the status changes.

---

## Risk Assessment

| Risk                       | Likelihood | Impact | Mitigation                                                  |
| -------------------------- | ---------- | ------ | ----------------------------------------------------------- |
| JS bugs break dashboard    | Medium     | High   | Tag `v2.0-pre-P2000-refactor`, comprehensive manual testing |
| WebSocket message mismatch | Low        | High   | Test in staging, verify message types in browser DevTools   |
| Performance regression     | Low        | Medium | Profile before/after, element caching                       |
| Missing edge case          | Medium     | Medium | Senior review (this section), integration tests             |

---

## Dependencies

- None (this is foundational)

## Blocks

- Future P4300 work should wait for this to land
- Any new host-related features should use the new architecture

---

## Related Documents

- **P2000-TECH-SPEC.md**: Detailed technical specification (see below)
- **P4300**: Automated Flake Updates (partially deprecated by this)
- **P5010**: Compartment Status Indicator (render logic moves here)
