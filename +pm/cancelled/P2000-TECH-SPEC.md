# P2000 Technical Specification

**Parent**: P2000-unified-host-state-management.md  
**Status**: ❌ CANCELLED - Premature optimization  
**Closed**: 2025-12-19  
**Reason**: 1100+ lines of implementation detail for a rewrite that may not happen. Current UI is functional. Fix P1000 (agent updates) first, then reassess if this rewrite is needed.

---

## Table of Contents

1. [Current Code Inventory](#1-current-code-inventory)
2. [Deletion Checklist](#2-deletion-checklist)
3. [New Code Specification](#3-new-code-specification)
4. [Go Backend Changes](#4-go-backend-changes)
5. [Template Changes](#5-template-changes)
6. [CSS Changes](#6-css-changes)
7. [Test Updates](#7-test-updates)

---

## 1. Current Code Inventory

### JavaScript Functions in dashboard.templ (Lines 304-1316)

| Function Name                | Line Start | Line End | Purpose                       | Action  |
| ---------------------------- | ---------- | -------- | ----------------------------- | ------- |
| `connectWebSocket()`         | 310        | 338      | WS connection                 | KEEP    |
| `updateConnectionStatus()`   | 340        | 349      | Footer status indicator       | KEEP    |
| `scheduleReconnect()`        | 351        | 356      | Reconnection backoff          | KEEP    |
| `handleMessage()`            | 358        | 379      | Message router                | REWRITE |
| `handleFlakeUpdatePR()`      | 382        | 427      | Global banner                 | DELETE  |
| `handleFlakeUpdateJob()`     | 430        | 465      | Deploy progress banner        | DELETE  |
| `mergeAndDeploy()`           | 467        | 488      | Trigger deploy                | MOVE    |
| `dismissFlakeUpdate()`       | 490        | 493      | Hide banner                   | DELETE  |
| `updateHost()`               | 495        | 521      | Orchestrator                  | DELETE  |
| `updateCellData()`           | 523        | 553      | Update multiple cells         | DELETE  |
| `updateStatusCompartments()` | 556        | 584      | Git/Lock/System               | DELETE  |
| `updateAgentBadge()`         | 587        | 606      | Agent badge on Lock           | DELETE  |
| `formatAgentTooltip()`       | 608        | 617      | Tooltip text                  | DELETE  |
| `getCompartmentClass()`      | 619        | 628      | CSS class helper              | DELETE  |
| `getIndicatorClass()`        | 630        | 638      | CSS class helper              | DELETE  |
| `getCompartmentTooltip()`    | 640        | 703      | Tooltip builder               | DELETE  |
| `updateMetrics()`            | 705        | 738      | CPU/RAM display               | DELETE  |
| `getMetricClass()`           | 740        | 745      | CSS class helper              | INLINE  |
| `formatLastSeen()`           | 750        | 782      | Relative time                 | KEEP    |
| `updateProgressBadge()`      | 784        | 800      | Command badge                 | DELETE  |
| `updateStatusIndicator()`    | 802        | 843      | Ripple/dot                    | DELETE  |
| `triggerHeartbeat()`         | 845        | 852      | Animation trigger             | KEEP    |
| `appendLog()`                | 854        | 857      | Dispatch to Alpine            | KEEP    |
| `showLogPanel()`             | 859        | 863      | Show log panel                | KEEP    |
| `markCommandComplete()`      | 865        | 869      | Clear busy state              | REWRITE |
| `sendCommand()`              | 872        | 898      | Send command to host          | KEEP    |
| `setHostBusy()`              | 900        | 914      | Update row + card             | DELETE  |
| `applyButtonStates()`        | 916        | 966      | Button enable/disable + badge | DELETE  |
| `deleteHost()`               | 969        | 992      | Delete host                   | KEEP    |
| `Alpine.data('logViewer')`   | 995        | 1084     | Log viewer component          | KEEP    |
| `initLastSeenCells()`        | 1087       | 1096     | Format on load                | DELETE  |
| `startLastSeenUpdater()`     | 1099       | 1110     | 1s interval                   | REWRITE |
| `toggleDropdown()`           | 1113       | 1124     | Dropdown toggle               | KEEP    |
| `toggleBulkMenu()`           | 1143       | 1148     | Bulk menu toggle              | KEEP    |
| `closeBulkMenu()`            | 1150       | 1153     | Close bulk menu               | KEEP    |
| `unlockActions()`            | 1161       | 1166     | Unlock stuck host             | REWRITE |
| `downloadLogs()`             | 1168       | 1170     | Download logs                 | KEEP    |
| `confirmRemoveHost()`        | 1175       | 1180     | Show modal                    | KEEP    |
| `doRemoveHost()`             | 1182       | 1198     | Remove host                   | KEEP    |
| `closeModal()`               | 1200       | 1203     | Close modal                   | KEEP    |
| `openAddHostModal()`         | 1205       | 1207     | Open add modal                | KEEP    |
| `doAddHost()`                | 1209       | 1231     | Add host                      | KEEP    |
| `bulkCommand()`              | 1244       | 1264     | Bulk action                   | KEEP    |
| `refreshStatus()`            | 1267       | 1281     | OLD refresh (stub)            | REWRITE |
| `checkForUpdates()`          | 1284       | 1310     | Check GitHub                  | DELETE  |

### Summary

- **DELETE**: 18 functions (~400 lines)
- **REWRITE**: 5 functions
- **KEEP**: 17 functions
- **NEW**: 3 functions (hostStore, renderHost, refreshHost)

---

## 2. Deletion Checklist

Use this checklist during implementation:

### JavaScript Deletions

```
[ ] handleFlakeUpdatePR()       - lines 382-427
[ ] handleFlakeUpdateJob()      - lines 430-465
[ ] dismissFlakeUpdate()        - lines 490-493
[ ] updateHost()                - lines 495-521
[ ] updateCellData()            - lines 523-553
[ ] updateStatusCompartments()  - lines 556-584
[ ] updateAgentBadge()          - lines 587-606
[ ] formatAgentTooltip()        - lines 608-617
[ ] getCompartmentClass()       - lines 619-628
[ ] getIndicatorClass()         - lines 630-638
[ ] getCompartmentTooltip()     - lines 640-703
[ ] updateMetrics()             - lines 705-738
[ ] updateProgressBadge()       - lines 784-800
[ ] updateStatusIndicator()     - lines 802-843
[ ] setHostBusy()               - lines 900-914
[ ] applyButtonStates()         - lines 916-966
[ ] initLastSeenCells()         - lines 1087-1096
[ ] checkForUpdates()           - lines 1284-1310
```

### Template Deletions

```
[ ] FlakeUpdateBanner component - lines 1345-1371
[ ] flake-update-banner div     - lines 150-157
[ ] Hidden flake-update-banner  - line 156-157
```

### Go Deletions

```
[ ] FlakeUpdateService.broadcastPRStatus() - flake_updates.go
[ ] FlakeUpdateService.broadcastJobStatus() - flake_updates.go (keep internal use)
[ ] Hub handleRegister browser PR send - hub.go lines 226-236
```

---

## 3. New Code Specification

### 3.1 hostStore Object

```javascript
/**
 * Single source of truth for all host state.
 * All updates flow through this store.
 *
 * Design decisions:
 * - Deep merge for nested objects (metrics, updateStatus)
 * - Element caching for O(1) DOM access
 * - Defensive null checks throughout
 */
const hostStore = {
  _hosts: new Map(),

  /**
   * Initialize store from server-rendered data-* attributes.
   * Called once on page load.
   * Caches DOM element references for performance.
   */
  hydrate() {
    document.querySelectorAll("tr[data-host-id]").forEach((row) => {
      const id = row.dataset.hostId;
      if (this._hosts.has(id)) return;

      // Cache element references for O(1) access during render
      const card = document.querySelector(`.host-card[data-host-id="${id}"]`);

      this._hosts.set(id, {
        // Identity
        id: id,
        hostname: row.dataset.hostname || id,
        hostType: row.dataset.hostType || "nixos",
        themeColor: row.dataset.themeColor || "#7aa2f7",

        // State
        online: !row.classList.contains("host-offline"),
        lastSeen:
          row.querySelector('[data-cell="last-seen"]')?.dataset.timestamp ||
          null,
        pendingCommand: row.dataset.pendingCommand || null,

        // Data
        metrics: this._parseMetrics(row),
        updateStatus: this._parseUpdateStatus(row),
        generation: row.dataset.generation || null,
        agentVersion: row.dataset.agentVersion || null,
        agentOutdated: row.dataset.agentOutdated === "true",

        // Cached DOM references (not serializable, prefix with _)
        _elements: { row, card },
      });
    });
    console.log(`hostStore: hydrated ${this._hosts.size} hosts`);
  },

  _parseMetrics(row) {
    const cell = row.querySelector('[data-cell="metrics"]');
    if (!cell) return null;
    const cpu = cell.querySelector('[data-metric="cpu"]');
    const ram = cell.querySelector('[data-metric="ram"]');
    if (!cpu && !ram) return null;
    return {
      cpu: parseFloat(cpu?.dataset.value) || 0,
      ram: parseFloat(ram?.dataset.value) || 0,
      swap: parseFloat(ram?.dataset.swap) || 0,
      load: parseFloat(ram?.dataset.load) || 0,
    };
  },

  _parseUpdateStatus(row) {
    const container = row.querySelector(".update-status");
    if (!container) return null;
    try {
      return {
        git: JSON.parse(container.dataset.git || "null"),
        lock: JSON.parse(container.dataset.lock || "null"),
        system: JSON.parse(container.dataset.system || "null"),
        repoUrl: container.dataset.repoUrl || "",
        repoDir: container.dataset.repoDir || "",
      };
    } catch (e) {
      console.warn("Failed to parse updateStatus:", e);
      return null;
    }
  },

  /**
   * Get host state by ID.
   * @returns {object|undefined} Host state or undefined if not found
   */
  get(id) {
    return this._hosts.get(id);
  },

  /**
   * Get all hosts as array.
   * @returns {object[]} Array of host states
   */
  all() {
    return Array.from(this._hosts.values());
  },

  /**
   * Update host state and trigger render.
   * Uses deep merge for nested objects (metrics, updateStatus).
   *
   * @param {string} id - Host ID
   * @param {object} patch - Partial state to merge
   */
  update(id, patch) {
    const current = this._hosts.get(id);
    if (!current) {
      console.warn(`hostStore: unknown host ${id}`);
      return;
    }

    // Start with shallow merge
    const next = { ...current, ...patch };

    // Deep merge for known nested objects
    if (patch.metrics && current.metrics) {
      next.metrics = { ...current.metrics, ...patch.metrics };
    }
    if (patch.updateStatus && current.updateStatus) {
      next.updateStatus = { ...current.updateStatus, ...patch.updateStatus };
    }

    // Preserve cached elements
    next._elements = current._elements;

    this._hosts.set(id, next);
    renderHost(id);
  },

  /**
   * Mark host as offline.
   * Clears pending command since it can't complete.
   */
  setOffline(id) {
    this.update(id, { online: false, pendingCommand: null });
  },
};

/**
 * Global state for non-host-specific data.
 * Separated from hostStore for clarity.
 */
const globalState = {
  pendingPR: null, // { number, title, url, mergeable } or null
  wsConnected: false,
};

// Hydrate global state from body data attributes
(function hydrateGlobalState() {
  const prData = document.body.dataset.pendingPr;
  if (prData) {
    try {
      globalState.pendingPR = JSON.parse(prData);
    } catch (e) {
      console.warn("Failed to parse pendingPR:", e);
    }
  }
})();
```

### 3.2 renderHost Function

```javascript
/**
 * Single render function for all host UI updates.
 * Updates both table row and mobile card.
 *
 * Design: Uses cached element references from hostStore for O(1) DOM access.
 * This avoids 100+ querySelector calls per second with 10 hosts.
 *
 * @param {string} hostId - Host ID to render
 */
function renderHost(hostId) {
  const host = hostStore.get(hostId);
  if (!host) return;

  // Derived state (computed once, used multiple times)
  const isOnline = host.online;
  const isBusy = !!host.pendingCommand;
  const buttonsEnabled = isOnline && !isBusy;

  // Use cached element references (set during hydrate)
  const { row, card } = host._elements || {};

  [row, card].filter(Boolean).forEach((el) => {
    // 1. Offline class
    el.classList.toggle("host-offline", !isOnline);

    // 2. Status indicator (ripple/dot)
    renderStatusIndicator(el, isOnline, isBusy);

    // 3. Progress badge
    renderProgressBadge(el, host.pendingCommand);

    // 4. Metrics
    if (host.metrics) {
      renderMetrics(el, host.metrics);
    }

    // 5. Update status compartments
    if (host.updateStatus) {
      renderUpdateStatus(el, host);
    }

    // 6. Button states
    el.querySelectorAll("button[data-command]").forEach((btn) => {
      const cmd = btn.dataset.command;
      // Stop button always enabled during command
      if (cmd === "stop") {
        btn.disabled = !isBusy;
        btn.style.display = isBusy ? "" : "none";
      } else if (cmd === "test") {
        // Test becomes Stop when busy
        btn.style.display = isBusy ? "none" : "";
        btn.disabled = !buttonsEnabled;
      } else {
        btn.disabled = !buttonsEnabled;
      }
    });

    // 7. Last seen
    const lastSeenCell = el.querySelector('[data-cell="last-seen"]');
    if (lastSeenCell && host.lastSeen) {
      lastSeenCell.dataset.timestamp = host.lastSeen;
      const result = formatLastSeen(host.lastSeen);
      lastSeenCell.textContent = result.text;
      lastSeenCell.className = result.className;
    }
  });
}

// Helper: Render status indicator
function renderStatusIndicator(el, isOnline, isBusy) {
  const wrapper =
    el.querySelector(".status-wrapper") ||
    el.querySelector(".status-with-badge");
  if (!wrapper) return;

  const existing = wrapper.querySelector(".status-ripple, .status-dot");

  let html;
  if (isOnline && !isBusy) {
    // Online idle - ripple
    if (existing?.classList.contains("status-ripple")) {
      triggerHeartbeat(existing);
      return;
    }
    html =
      '<span class="status-ripple"><span class="hb-wave"></span><span class="hb-wave"></span><span class="hb-wave"></span><span class="hb-core"></span></span>';
  } else if (isBusy) {
    // Running - pulsing yellow
    html = '<span class="status-dot status-running"></span>';
  } else {
    // Offline - static red
    html = '<span class="status-dot status-offline"></span>';
  }

  if (existing) {
    const temp = document.createElement("div");
    temp.innerHTML = html;
    existing.replaceWith(temp.firstChild);
    if (isOnline && !isBusy) {
      triggerHeartbeat(wrapper.querySelector(".status-ripple"));
    }
  }
}

// Helper: Render progress badge
function renderProgressBadge(el, pendingCommand) {
  const wrapper =
    el.querySelector(".status-wrapper") ||
    el.querySelector(".status-with-badge");
  if (!wrapper) return;

  let badge = wrapper.querySelector(".progress-badge-mini");
  if (pendingCommand) {
    if (!badge) {
      badge = document.createElement("span");
      badge.className = "progress-badge-mini";
      wrapper.appendChild(badge);
    }
    badge.textContent = pendingCommand;
  } else if (badge) {
    badge.remove();
  }
}

// Helper: Render metrics
function renderMetrics(el, metrics) {
  const cell = el.querySelector('[data-cell="metrics"]');
  if (!cell) return;

  const cpuEl = cell.querySelector('[data-metric="cpu"]');
  const ramEl = cell.querySelector('[data-metric="ram"]');

  if (cpuEl) {
    const val = cpuEl.querySelector(".metric-val");
    if (val) val.textContent = Math.round(metrics.cpu) + "%";
    cpuEl.classList.toggle("metric-high", metrics.cpu >= 80);
    cpuEl.dataset.value = metrics.cpu;
  }

  if (ramEl) {
    const val = ramEl.querySelector(".metric-val");
    if (val) val.textContent = Math.round(metrics.ram) + "%";
    ramEl.classList.toggle("metric-high", metrics.ram >= 80);
    ramEl.dataset.value = metrics.ram;
    ramEl.dataset.swap = metrics.swap;
    ramEl.dataset.load = metrics.load;
    ramEl.title = `RAM: ${Math.round(metrics.ram)}%, Swap: ${Math.round(metrics.swap)}%, Load: ${metrics.load.toFixed(2)}`;
  }

  // Replace "—" placeholder if needed
  const naSpan = cell.querySelector(".metrics-na");
  if (naSpan) {
    cell.innerHTML = `
      <span class="metric" data-metric="cpu" data-value="${metrics.cpu}">
        <svg class="metric-icon"><use href="#icon-cpu"></use></svg>
        <span class="metric-val">${Math.round(metrics.cpu)}%</span>
      </span>
      <span class="metric" data-metric="ram" data-value="${metrics.ram}">
        <svg class="metric-icon"><use href="#icon-ram"></use></svg>
        <span class="metric-val">${Math.round(metrics.ram)}%</span>
      </span>
    `;
  }
}

// Helper: Render update status compartments
function renderUpdateStatus(el, host) {
  const container = el.querySelector(".update-status");
  if (!container) return;

  const status = host.updateStatus;
  if (!status) return;

  const compartments = container.querySelectorAll(".update-compartment");

  ["git", "lock", "system"].forEach((type, i) => {
    const comp = compartments[i];
    if (!comp) return;

    const check = status[type];
    if (!check) return;

    // Build class list
    comp.className = "update-compartment";
    switch (check.status) {
      case "ok":
        break; // no additional class
      case "outdated":
        comp.classList.add("needs-update");
        break;
      case "error":
        comp.classList.add("error");
        break;
      default:
        comp.classList.add("unknown");
    }

    // Update indicator dot
    const indicator = comp.querySelector(".compartment-indicator");
    if (indicator) {
      indicator.className = "compartment-indicator";
      indicator.classList.add(
        `compartment-indicator--${check.status || "unknown"}`,
      );
    }

    // Lock-specific: Agent badge and PR indicator
    if (type === "lock") {
      // Agent outdated badge
      let agentBadge = comp.querySelector(".agent-badge");
      if (host.agentOutdated && !agentBadge) {
        agentBadge = document.createElement("span");
        agentBadge.className = "agent-badge";
        agentBadge.textContent = "A";
        comp.appendChild(agentBadge);
      } else if (!host.agentOutdated && agentBadge) {
        agentBadge.remove();
      }

      // Pending PR indicator (uses globalState)
      comp.classList.toggle("has-pr", !!globalState.pendingPR);
    }
  });
}
```

### 3.3 refreshHost Function

```javascript
/**
 * Fetch fresh status for a single host.
 * Called when user clicks refresh button.
 *
 * Error handling:
 * - 404: Host deleted - could remove from UI or show message
 * - 500: Server error - show toast, keep stale data
 * - Network error: Show toast, keep stale data
 */
async function refreshHost(hostId) {
  // Find button using cached elements if possible, fallback to querySelector
  const host = hostStore.get(hostId);
  let btn = host?._elements?.row?.querySelector(".btn-refresh");
  if (!btn) {
    btn = document.querySelector(
      `button.btn-refresh[data-host-id="${hostId}"]`,
    );
  }
  if (btn) btn.classList.add("loading");

  try {
    const resp = await fetch(`/api/hosts/${hostId}/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": CSRF_TOKEN,
      },
    });

    if (resp.status === 404) {
      console.warn(`Host ${hostId} not found`);
      // Optionally: remove from store and DOM, or show toast
      return;
    }

    if (!resp.ok) {
      console.error(`Refresh failed: ${resp.status} ${resp.statusText}`);
      // Could show toast: showToast('Refresh failed', 'error');
      return;
    }

    const data = await resp.json();

    // Update store with fresh data
    hostStore.update(hostId, {
      online: data.online,
      generation: data.generation,
      agentVersion: data.agent_version,
      agentOutdated: data.agent_outdated,
      updateStatus: data.update_status,
    });

    // Update global PR state if included
    if (data.pending_pr !== undefined) {
      globalState.pendingPR = data.pending_pr;
    }
  } catch (err) {
    console.error("Refresh failed:", err);
    // Could show toast: showToast('Network error', 'error');
  } finally {
    if (btn) btn.classList.remove("loading");
  }
}
```

### 3.4 Simplified handleMessage

```javascript
/**
 * Route incoming WebSocket messages to appropriate handlers.
 * Includes defensive validation to prevent crashes from malformed data.
 */
function handleMessage(msg) {
  // Defensive: validate message structure
  if (!msg || typeof msg !== "object") {
    console.warn("Invalid WS message (not object):", msg);
    return;
  }
  if (!msg.type) {
    console.warn("Invalid WS message (no type):", msg);
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

    case "host_offline":
      if (!hostId) return;
      hostStore.setOffline(hostId);
      break;

    case "command_queued":
      if (!hostId) return;
      hostStore.update(hostId, {
        pendingCommand: payload.command,
      });
      showLogPanel(hostId);
      break;

    case "command_output":
      appendLog(payload);
      break;

    case "command_complete":
      if (!hostId) return;
      hostStore.update(hostId, {
        pendingCommand: null,
      });
      window.dispatchEvent(
        new CustomEvent("log-complete", { detail: payload }),
      );
      break;

    default:
      // Unknown message type - log but don't crash
      console.debug("Unknown WS message type:", msg.type);
  }
}
```

---

## 4. Go Backend Changes

### 4.1 handlers.go - Add handleRefreshHost

```go
// handleRefreshHost fetches fresh status for a single host.
// POST /api/hosts/{hostID}/refresh
func (s *Server) handleRefreshHost(w http.ResponseWriter, r *http.Request) {
    hostID := chi.URLParam(r, "hostID")

    // Query host from database
    var h struct {
        Hostname       string
        Generation     *string
        AgentVersion   *string
        LockStatusJSON *string
        SystemStatusJSON *string
        RepoURL        *string
        RepoDir        *string
        Status         string
    }

    err := s.db.QueryRow(`
        SELECT hostname, generation, agent_version, lock_status_json,
               system_status_json, repo_url, repo_dir, status
        FROM hosts WHERE id = ?
    `, hostID).Scan(
        &h.Hostname, &h.Generation, &h.AgentVersion,
        &h.LockStatusJSON, &h.SystemStatusJSON,
        &h.RepoURL, &h.RepoDir, &h.Status,
    )
    if err != nil {
        http.Error(w, "Host not found", http.StatusNotFound)
        return
    }

    // Compute Git status (dashboard-side)
    var gitStatus, gitMsg, gitChecked string
    generation := ""
    if h.Generation != nil {
        generation = *h.Generation
    }
    if s.versionFetcher != nil {
        gitStatus, gitMsg, gitChecked = s.versionFetcher.GetGitStatus(generation)
    } else {
        gitStatus, gitMsg, gitChecked = "unknown", "Version tracking not configured", ""
    }

    // Parse Lock and System from DB
    var lockStatus, systemStatus map[string]any
    if h.LockStatusJSON != nil {
        _ = json.Unmarshal([]byte(*h.LockStatusJSON), &lockStatus)
    }
    if h.SystemStatusJSON != nil {
        _ = json.Unmarshal([]byte(*h.SystemStatusJSON), &systemStatus)
    }

    // Check agent version
    agentVersion := ""
    if h.AgentVersion != nil {
        agentVersion = *h.AgentVersion
    }
    agentOutdated := agentVersion != "" && agentVersion != VersionInfo()

    // Check for pending PR
    var pendingPR any
    if s.flakeUpdates != nil {
        pendingPR = s.flakeUpdates.GetPendingPR()
    }

    resp := map[string]any{
        "host_id":        hostID,
        "online":         h.Status == "online" || h.Status == "running",
        "generation":     generation,
        "agent_version":  agentVersion,
        "agent_outdated": agentOutdated,
        "update_status": map[string]any{
            "git":    map[string]any{"status": gitStatus, "message": gitMsg, "checked_at": gitChecked},
            "lock":   lockStatus,
            "system": systemStatus,
        },
        "pending_pr": pendingPR,
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}
```

### 4.2 server.go - Register Route

```go
// In setupRoutes(), add:
r.With(s.authMiddleware).Post("/api/hosts/{hostID}/refresh", s.handleRefreshHost)
```

### 4.3 hub.go - Change Message Types

**In handleHeartbeat(), change:**

```go
// BEFORE:
h.BroadcastToBrowsers(map[string]any{
    "type": "host_update",
    "payload": map[string]any{
        "host_id":         hostID,
        "online":          true,
        "last_seen":       time.Now().Format(time.RFC3339),
        "generation":      payload.Generation,
        "nixpkgs_version": payload.NixpkgsVersion,
        "pending_command": payload.PendingCommand,
        "metrics":         payload.Metrics,
        "update_status":   updateStatus,
    },
})

// AFTER:
h.BroadcastToBrowsers(map[string]any{
    "type": "host_heartbeat",
    "payload": map[string]any{
        "host_id":   hostID,
        "online":    true,
        "last_seen": time.Now().Format(time.RFC3339),
        "metrics":   payload.Metrics,
    },
})
```

**In handleUnregister(), change:**

```go
// BEFORE:
h.queueBroadcast(map[string]any{
    "type": "host_update",
    "payload": map[string]any{
        "host_id": hostID,
        "online":  false,
        "status":  "offline",
    },
})

// AFTER:
h.queueBroadcast(map[string]any{
    "type": "host_offline",
    "payload": map[string]any{
        "host_id": hostID,
    },
})
```

### 4.4 flake_updates.go - Remove Broadcasts

**DELETE these methods:**

```go
// DELETE: broadcastPRStatus (called on PR detection)
func (s *FlakeUpdateService) broadcastPRStatus() {
    // ... entire function
}
```

**KEEP broadcastJobStatus for internal use only (progress during deploy).**

---

## 5. Template Changes

### 5.1 dashboard.templ - Data Attributes

Add data attributes to `HostRow` for hydration:

```templ
templ HostRow(host Host, csrfToken string) {
    <tr
        data-host-id={ host.ID }
        data-hostname={ host.Hostname }
        data-host-type={ host.HostType }
        data-theme-color={ host.ThemeColor }
        data-generation={ host.Generation }
        data-agent-version={ host.AgentVersion }
        data-agent-outdated={ strconv.FormatBool(host.AgentOutdated) }
        data-pending-command={ host.PendingCommand }
        class={ templ.KV("host-offline", !host.Online) }
    >
```

Add data attributes to `.update-status` for compartment hydration:

```templ
templ UpdateStatusCell(host Host) {
    <div
        class="update-status"
        data-host-id={ host.ID }
        data-git={ updateStatusJSON(host.UpdateStatus, "git") }
        data-lock={ updateStatusJSON(host.UpdateStatus, "lock") }
        data-system={ updateStatusJSON(host.UpdateStatus, "system") }
        data-repo-url={ host.RepoURL }
        data-repo-dir={ host.RepoDir }
    >
```

Add helper function:

```go
func updateStatusJSON(status *UpdateStatus, compartment string) string {
    if status == nil {
        return "null"
    }
    var check StatusCheck
    switch compartment {
    case "git":
        check = status.Git
    case "lock":
        check = status.Lock
    case "system":
        check = status.System
    }
    data, _ := json.Marshal(check)
    return string(data)
}
```

### 5.2 dashboard.templ - Refresh Button

Add to action buttons:

```templ
<td class="actions-cell">
    <div class="action-buttons">
        @CommandButton(host.ID, "pull", "", "btn", host.Online && host.PendingCommand == "")
        @CommandButton(host.ID, "switch", "", "btn", host.Online && host.PendingCommand == "")
        @CommandButton(host.ID, "test", "", "btn", host.Online && host.PendingCommand == "")
        <button
            class="btn btn-refresh"
            data-host-id={ host.ID }
            onclick="refreshHost(this.dataset.hostId)"
            title="Refresh status"
        >
            <svg class="icon"><use href="#icon-refresh-cw"></use></svg>
        </button>
        @ActionDropdown(host)
    </div>
</td>
```

### 5.3 dashboard.templ - Remove Banner

DELETE these lines:

```templ
<!-- DELETE THIS ENTIRE BLOCK -->
<!-- Flake Update Banner (P4300) -->
if data.PendingPR != nil {
    <div id="flake-update-banner" class="flake-update-banner">
        @FlakeUpdateBanner(data.PendingPR, data.CSRFToken)
    </div>
} else {
    <div id="flake-update-banner" class="flake-update-banner" style="display: none;"></div>
}
```

DELETE `FlakeUpdateBanner` templ component entirely.

### 5.4 dashboard.templ - Bulk Actions with PR

Modify bulk actions dropdown:

```templ
<div class="dropdown-menu" id="bulk-actions-menu">
    if data.PendingPR != nil && data.PendingPR.Mergeable {
        <button class="dropdown-item" onclick="mergeAndDeploy()">
            <svg class="icon"><use href="#icon-merge"></use></svg>
            Merge & Deploy PR #{ strconv.Itoa(data.PendingPR.Number) }
        </button>
        <div class="dropdown-divider"></div>
    }
    <button class="dropdown-item" onclick="bulkCommand('update'); closeBulkMenu()">
        <!-- ... existing ... -->
```

---

## 6. CSS Changes

### Add to base.templ styles:

```css
/* Refresh button - visible on hover */
.btn-refresh {
  opacity: 0;
  transition: opacity 0.15s ease;
  padding: 0.25rem;
  background: transparent;
  border: none;
  cursor: pointer;
}

tr:hover .btn-refresh,
.host-card:hover .btn-refresh {
  opacity: 0.6;
}

tr:hover .btn-refresh:hover,
.host-card:hover .btn-refresh:hover {
  opacity: 1;
}

.btn-refresh.loading .icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* Stop button visibility */
button[data-command="stop"] {
  display: none;
}

tr[data-pending-command]:not([data-pending-command=""])
  button[data-command="stop"] {
  display: inline-flex;
}

tr[data-pending-command]:not([data-pending-command=""])
  button[data-command="test"] {
  display: none;
}
```

---

## 7. Test Updates

### Update t05_dashboard_websocket_test.go

Change expected message types:

```go
// BEFORE:
expectedTypes := []string{"host_update"}

// AFTER:
expectedTypes := []string{"host_heartbeat", "host_offline"}
```

### Add t13_refresh_endpoint_test.go

```go
func TestRefreshHostEndpoint(t *testing.T) {
    // Setup test server and database...

    t.Run("Success", func(t *testing.T) {
        resp, err := http.Post(baseURL+"/api/hosts/testhost/refresh", ...)
        require.NoError(t, err)
        require.Equal(t, 200, resp.StatusCode)

        var result map[string]any
        json.NewDecoder(resp.Body).Decode(&result)

        assert.Equal(t, "testhost", result["host_id"])
        assert.Contains(t, result, "update_status")
        assert.Contains(t, result, "agent_outdated")
    })

    t.Run("NotFound", func(t *testing.T) {
        resp, err := http.Post(baseURL+"/api/hosts/nonexistent/refresh", ...)
        require.NoError(t, err)
        require.Equal(t, 404, resp.StatusCode)
    })

    t.Run("RequiresCSRF", func(t *testing.T) {
        req, _ := http.NewRequest("POST", baseURL+"/api/hosts/testhost/refresh", nil)
        // No X-CSRF-Token header
        resp, err := client.Do(req)
        require.NoError(t, err)
        require.Equal(t, 403, resp.StatusCode)
    })

    t.Run("RequiresAuth", func(t *testing.T) {
        // No session cookie
        resp, err := http.Post(baseURL+"/api/hosts/testhost/refresh", ...)
        require.NoError(t, err)
        require.Equal(t, 401, resp.StatusCode)
    })
}
```

---

## Verification Checklist

After implementation, verify in this order:

### Phase 1: Store & Hydration

```
[ ] hostStore.hydrate() populates all hosts on page load
[ ] hostStore has correct count: console.log(hostStore.all().length)
[ ] Each host has _elements.row and _elements.card cached
[ ] globalState.pendingPR is populated from body data attribute
```

### Phase 2: Real-time Updates

```
[ ] WebSocket receives host_heartbeat (not host_update)
[ ] WebSocket receives host_offline on disconnect
[ ] Metrics update in real-time (CPU/RAM values change)
[ ] Last seen updates every second
[ ] renderHost() updates both row and card simultaneously
```

### Phase 3: On-Demand Refresh

```
[ ] Refresh button appears on hover (opacity transition)
[ ] Refresh button shows spinner during fetch
[ ] Refresh button updates compartments correctly
[ ] Refresh 404 doesn't crash (console warning only)
[ ] Refresh network error doesn't crash
```

### Phase 4: UI Changes

```
[ ] No global flake banner appears
[ ] "Merge & Deploy" appears in Bulk Actions when PR pending
[ ] Lock compartment has .has-pr class when PR pending
[ ] Lock compartment tooltip mentions PR when pending
[ ] Agent badge appears/disappears correctly
```

### Phase 5: Code Cleanup

```
[ ] All old functions are deleted (grep for each function name)
[ ] No duplicate logic (search for classList.add, classList.toggle)
[ ] JS file size reduced (target: -30% or more)
```

### Phase 6: Quality

```
[ ] No console errors during normal operation
[ ] No console errors during WebSocket reconnect
[ ] Tests pass: go test ./v2/tests/...
[ ] Manual test: all hosts show correct status after page load
[ ] Manual test: command execution shows log panel and badge
[ ] Manual test: stop agent → host goes offline within 60s
```
