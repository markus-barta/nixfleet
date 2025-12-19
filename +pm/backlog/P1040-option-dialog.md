# P1040 - Dependency Warning Dialog

**Created**: 2025-12-19  
**Updated**: 2025-12-19  
**Priority**: P1040 (Critical)  
**Status**: Ready for Development  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 3 hours  
**Depends on**: None (utility component)

---

## Overview

Modal dialog that warns users when attempting actions with unmet dependencies. Primarily used when clicking Switch while Git is outdated. Supports both single-host and multi-host scenarios.

---

## Requirements

### Trigger Conditions

| Action | Condition                           | Show Dialog?         |
| ------ | ----------------------------------- | -------------------- |
| Switch | Git = outdated (on any target host) | Yes                  |
| Switch | Git = ok (all hosts)                | No, execute directly |
| Pull   | Any                                 | No (always allowed)  |
| Test   | Any                                 | No (always allowed)  |

### Single-Host Dialog

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  ⚠ Git is behind on hsb1                                        │
│  ───────────────────────────────────────────────────────────    │
│                                                                 │
│  Running Switch without Pull may deploy old code.               │
│                                                                 │
│  [Cancel]      [Pull Only]      [Switch Anyway]   [Pull + Switch] │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Multi-Host Dialog

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  ⚠ 2 of 3 hosts need Pull first                                 │
│  ───────────────────────────────────────────────────────────    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ hsb0        Git is behind                               │    │
│  │ gpc0        Git is behind                               │    │
│  │ hsb1        OK (will switch)                            │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  [Cancel]            [Switch All Anyway]        [Pull + Switch All] │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Button Actions

| Button        | Single-Host             | Multi-Host                                       |
| ------------- | ----------------------- | ------------------------------------------------ |
| Cancel        | Close dialog            | Close dialog                                     |
| Pull Only     | Pull host               | —                                                |
| Switch Anyway | Switch without pull     | Switch all without pull                          |
| Pull + Switch | Pull, wait, then switch | Pull+switch hosts that need it, switch-only rest |

---

## Implementation

### Template (`dashboard.templ`)

Add before closing `</body>` or with other modals:

```go
templ DependencyDialog() {
    <div
        id="dependency-dialog"
        class="modal-overlay"
        x-data="dependencyDialog()"
        x-show="show"
        x-cloak
        @keydown.escape.window="handleEscape()"
        @click.self="close()"
    >
        <div class="modal dialog-modal" @click.stop>
            <div class="dialog-header">
                <svg class="dialog-icon warning"><use href="#icon-alert-triangle"></use></svg>
                <h3 class="dialog-title" x-text="title"></h3>
            </div>

            <div class="dialog-body">
                <p class="dialog-message" x-text="message"></p>

                <!-- Host list for multi-host -->
                <template x-if="hosts.length > 1">
                    <ul class="dialog-host-list">
                        <template x-for="host in hosts" :key="host.id">
                            <li :class="{ 'needs-action': host.needsPull }">
                                <span class="host-name" x-text="host.hostname"></span>
                                <span class="host-status" x-text="host.needsPull ? 'Git is behind' : 'OK (will switch)'"></span>
                            </li>
                        </template>
                    </ul>
                </template>
            </div>

            <div class="dialog-footer">
                <button type="button" class="btn btn-cancel" @click="close()">
                    Cancel
                </button>

                <div class="dialog-actions">
                    <!-- Single-host buttons -->
                    <template x-if="hosts.length === 1">
                        <div class="dialog-action-group">
                            <button type="button" class="btn" @click="choose('pull')">
                                Pull Only
                            </button>
                            <button type="button" class="btn" @click="choose('switch')">
                                Switch Anyway
                            </button>
                            <button type="button" class="btn btn-primary" @click="choose('pull-switch')">
                                Pull + Switch
                            </button>
                        </div>
                    </template>

                    <!-- Multi-host buttons -->
                    <template x-if="hosts.length > 1">
                        <div class="dialog-action-group">
                            <button type="button" class="btn" @click="choose('switch-all')">
                                Switch All Anyway
                            </button>
                            <button type="button" class="btn btn-primary" @click="choose('pull-switch-all')">
                                Pull + Switch All
                            </button>
                        </div>
                    </template>
                </div>
            </div>

            <!-- Progress overlay (shown during chain execution) -->
            <div class="dialog-progress" x-show="executing" x-cloak>
                <div class="progress-content">
                    <svg class="icon spinning"><use href="#icon-loader"></use></svg>
                    <span x-text="progressText"></span>
                    <button type="button" class="btn btn-cancel-small" @click="cancelExecution()">
                        Cancel
                    </button>
                </div>
            </div>
        </div>
    </div>
}
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   DEPENDENCY DIALOG (P1040)
   ═══════════════════════════════════════════════════════════════════════════ */

.dialog-modal {
  max-width: 500px;
  width: 90%;
}

.dialog-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.dialog-icon {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}

.dialog-icon.warning {
  color: var(--yellow);
}

.dialog-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0;
}

.dialog-body {
  margin-bottom: 20px;
}

.dialog-message {
  color: var(--fg-muted);
  margin: 0 0 16px 0;
  line-height: 1.5;
}

/* Host List */
.dialog-host-list {
  list-style: none;
  padding: 0;
  margin: 0;
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
  max-height: 200px;
  overflow-y: auto;
}

.dialog-host-list li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
}

.dialog-host-list li:last-child {
  border-bottom: none;
}

.dialog-host-list li.needs-action {
  background: rgba(var(--yellow-rgb), 0.1);
}

.dialog-host-list .host-name {
  font-weight: 500;
}

.dialog-host-list .host-status {
  font-size: 0.8125rem;
  color: var(--fg-muted);
}

/* Footer */
.dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.dialog-action-group {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 480px) {
  .dialog-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .dialog-action-group {
    justify-content: flex-end;
  }

  .btn-cancel {
    order: 1;
  }
}

.btn-primary {
  background: var(--blue);
  color: var(--bg);
}

.btn-primary:hover {
  background: var(--blue-bright);
}

/* Progress Overlay */
.dialog-progress {
  position: absolute;
  inset: 0;
  background: rgba(var(--bg-rgb), 0.9);
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: inherit;
}

.progress-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  text-align: center;
}

.progress-content .icon {
  width: 32px;
  height: 32px;
  color: var(--blue);
}

.progress-content .icon.spinning {
  animation: spin 1s linear infinite;
}

.btn-cancel-small {
  font-size: 0.8125rem;
  padding: 4px 12px;
}
```

### JavaScript (Alpine.js Component)

```js
// ═══════════════════════════════════════════════════════════════════════════
// DEPENDENCY DIALOG (P1040)
// ═══════════════════════════════════════════════════════════════════════════

Alpine.data("dependencyDialog", () => ({
  show: false,
  title: "",
  message: "",
  hosts: [], // [{ id, hostname, needsPull }]
  executing: false,
  progressText: "",
  cancelled: false,
  _completionHandlers: [], // Cleanup array

  init() {
    // Listen for dialog open requests
    window.addEventListener("show-dependency-dialog", (e) =>
      this.open(e.detail),
    );
  },

  /**
   * Open dialog with given configuration
   */
  open(detail) {
    this.title = detail.title;
    this.message = detail.message;
    this.hosts = detail.hosts;
    this.executing = false;
    this.progressText = "";
    this.cancelled = false;
    this.show = true;
  },

  /**
   * Close dialog and cleanup
   */
  close() {
    if (this.executing) return; // Can't close during execution
    this.show = false;
    this._cleanup();
  },

  /**
   * Handle Escape key
   */
  handleEscape() {
    if (this.executing) {
      this.cancelExecution();
    } else {
      this.close();
    }
  },

  /**
   * Handle button choice
   */
  async choose(action) {
    switch (action) {
      case "pull":
        sendCommand(this.hosts[0].id, "pull");
        this.show = false;
        break;

      case "switch":
        sendCommand(this.hosts[0].id, "switch");
        this.show = false;
        break;

      case "pull-switch":
        await this.executePullThenSwitch([this.hosts[0]]);
        break;

      case "switch-all":
        this.hosts.forEach((h) => sendCommand(h.id, "switch"));
        this.show = false;
        break;

      case "pull-switch-all":
        await this.executePullThenSwitch(this.hosts);
        break;
    }
  },

  /**
   * Execute Pull then Switch for given hosts
   */
  async executePullThenSwitch(hosts) {
    this.executing = true;
    this.cancelled = false;

    const hostsToPull = hosts.filter((h) => h.needsPull);
    const hostsToSwitchOnly = hosts.filter((h) => !h.needsPull);

    try {
      // First: Pull all hosts that need it
      if (hostsToPull.length > 0) {
        this.progressText = `Pulling ${hostsToPull.length} host${hostsToPull.length > 1 ? "s" : ""}...`;

        await Promise.all(
          hostsToPull.map((h) => {
            if (this.cancelled) throw new Error("Cancelled");
            return this._sendCommandAndWait(h.id, "pull");
          }),
        );
      }

      if (this.cancelled) {
        this._showCancelled();
        return;
      }

      // Then: Switch all hosts
      const allToSwitch = [...hostsToPull, ...hostsToSwitchOnly];
      this.progressText = `Switching ${allToSwitch.length} host${allToSwitch.length > 1 ? "s" : ""}...`;

      allToSwitch.forEach((h) => {
        if (!this.cancelled) {
          sendCommand(h.id, "switch");
        }
      });

      // Close dialog after initiating switches
      this.show = false;
      this._cleanup();
    } catch (err) {
      if (err.message === "Cancelled") {
        this._showCancelled();
      } else {
        console.error("Pull+Switch failed:", err);
        this.progressText = `Error: ${err.message}`;
        setTimeout(() => {
          this.executing = false;
        }, 2000);
      }
    }
  },

  /**
   * Cancel ongoing execution
   */
  cancelExecution() {
    this.cancelled = true;
    this.progressText = "Cancelling...";
  },

  /**
   * Show cancelled state and close
   */
  _showCancelled() {
    this.progressText = "Cancelled";
    setTimeout(() => {
      this.show = false;
      this.executing = false;
      this._cleanup();
    }, 1000);
  },

  /**
   * Send command and wait for completion
   */
  _sendCommandAndWait(hostId, command) {
    return new Promise((resolve, reject) => {
      const handler = (e) => {
        const detail = e.detail;
        if (detail.host_id === hostId) {
          window.removeEventListener("command-complete", handler);
          // Remove from cleanup array
          const idx = this._completionHandlers.indexOf(handler);
          if (idx !== -1) this._completionHandlers.splice(idx, 1);

          if (detail.exit_code === 0) {
            resolve();
          } else {
            reject(
              new Error(
                `${command} failed on ${hostId} (exit ${detail.exit_code})`,
              ),
            );
          }
        }
      };

      window.addEventListener("command-complete", handler);
      this._completionHandlers.push(handler);

      sendCommand(hostId, command);
    });
  },

  /**
   * Cleanup event handlers
   */
  _cleanup() {
    this._completionHandlers.forEach((handler) => {
      window.removeEventListener("command-complete", handler);
    });
    this._completionHandlers = [];
  },
}));

// ═══════════════════════════════════════════════════════════════════════════
// DEPENDENCY CHECK HELPER (P1040)
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Check dependencies and execute or show dialog
 * @param {string[]} hostIds - Host IDs to operate on
 * @param {string} command - Command to execute
 */
function checkDependenciesAndExecute(hostIds, command) {
  // Only Switch has dependency check
  if (command !== "switch") {
    hostIds.forEach((id) => sendCommand(id, command));
    return;
  }

  // Analyze hosts
  const hostsNeedingPull = [];
  const hostsReady = [];

  hostIds.forEach((id) => {
    const host = hostStore.get(id);
    if (!host) return;

    const gitStatus = host.updateStatus?.git?.status;

    if (gitStatus === "outdated") {
      hostsNeedingPull.push({
        id: host.id,
        hostname: host.hostname,
        needsPull: true,
      });
    } else {
      hostsReady.push({
        id: host.id,
        hostname: host.hostname,
        needsPull: false,
      });
    }
  });

  // If no hosts need pull, execute directly
  if (hostsNeedingPull.length === 0) {
    hostIds.forEach((id) => sendCommand(id, "switch"));
    return;
  }

  // Show dialog
  const allHosts = [...hostsNeedingPull, ...hostsReady];
  const isSingle = allHosts.length === 1;

  window.dispatchEvent(
    new CustomEvent("show-dependency-dialog", {
      detail: {
        title: isSingle
          ? `Git is behind on ${allHosts[0].hostname}`
          : `${hostsNeedingPull.length} of ${allHosts.length} hosts need Pull first`,
        message: "Running Switch without Pull may deploy old code.",
        hosts: allHosts,
      },
    }),
  );
}
```

### New SVG Icon

Add to sprite:

```html
<symbol id="icon-alert-triangle" viewBox="0 0 24 24">
  <path
    d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
  ></path>
  <line
    x1="12"
    y1="9"
    x2="12"
    y2="13"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
  ></line>
  <line
    x1="12"
    y1="17"
    x2="12.01"
    y2="17"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
  ></line>
</symbol>
```

---

## Edge Cases

### Host Goes Offline During Chain

| Phase         | Behavior                                |
| ------------- | --------------------------------------- |
| During Pull   | Promise rejects, shows error, can retry |
| During Switch | Already dispatched, backend handles     |

### Cancel During Pull

| Scenario                    | Behavior                                                      |
| --------------------------- | ------------------------------------------------------------- |
| User clicks Cancel          | `cancelled` flag set, pending Promises ignored, dialog closes |
| Pull completes after cancel | Ignored (dialog already closed)                               |

### All Hosts Ready (No Dialog)

If all hosts have `git: ok`, dialog is never shown and switches execute immediately.

---

## Testing

### Manual Test Cases

| #   | Scenario                  | Steps                             | Expected                               |
| --- | ------------------------- | --------------------------------- | -------------------------------------- |
| 1   | Single host, Git outdated | Click System yellow               | Dialog with 4 buttons                  |
| 2   | Pull Only                 | In dialog, click Pull Only        | Pull executes, dialog closes           |
| 3   | Switch Anyway             | In dialog, click Switch Anyway    | Switch executes, dialog closes         |
| 4   | Pull + Switch             | In dialog, click Pull + Switch    | Shows progress, pull runs, then switch |
| 5   | Multi-host                | Select 3 (2 outdated), Switch All | Dialog shows host list                 |
| 6   | Cancel during pull        | Pull + Switch, click Cancel       | Shows "Cancelled", dialog closes       |
| 7   | Escape key                | With dialog open, press Escape    | Dialog closes                          |
| 8   | Click outside             | Click backdrop                    | Dialog closes                          |
| 9   | No dialog needed          | All hosts Git ok, Switch All      | No dialog, switches execute            |
| 10  | Pull fails                | Pull + Switch, pull returns error | Shows error message                    |

---

## Acceptance Criteria

- [ ] Dialog appears when Switch with Git outdated
- [ ] Single-host: Shows 4 buttons
- [ ] Multi-host: Shows host list
- [ ] Multi-host: Shows 3 buttons
- [ ] Host list scrollable if many hosts
- [ ] Pull Only executes pull, closes
- [ ] Switch Anyway executes switch, closes
- [ ] Pull + Switch shows progress, executes sequentially
- [ ] Cancel during chain stops pending commands
- [ ] Escape closes (or cancels if executing)
- [ ] Click outside closes
- [ ] Error handling shows message
- [ ] No dialog if all hosts Git ok

---

## Related

- **P1020**: Clickable Compartments (calls `checkDependenciesAndExecute`)
- **P1015**: Selection Bar (calls `checkDependenciesAndExecute`)
