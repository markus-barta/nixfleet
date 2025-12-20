# P1010 - Action Bar Component

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1010 (Critical)  
**Status**: ✅ DONE (superseded by Context Bar)  
**Completed**: 2025-12-20  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 5 hours

> **Implementation Note**: This spec was superseded by the **unified Context Bar**. Instead of a header-based Action Bar, all functionality was merged into a single bar below the table that handles both hover previews and selection actions. The state machine and event flow concepts were implemented in the Context Bar component.

---

## Overview

The Action Bar is a fixed header component that previews actions before execution. It serves as the central feedback mechanism for all compartment interactions.

---

## Requirements

### Position & Layout

| Property  | Value         | Notes                                  |
| --------- | ------------- | -------------------------------------- |
| Position  | Header center | Between `.brand` and `.header-actions` |
| Min width | 320px         | Prevents content clipping              |
| Max width | 480px         | Prevents excessive stretching          |
| Margin    | 0 1rem        | Breathing room from adjacent elements  |
| Z-index   | 50            | Above table, below modals              |

### Responsive Behavior

| Breakpoint | Behavior                                    |
| ---------- | ------------------------------------------- |
| > 1024px   | Full width (320-480px), inline in header    |
| 768-1024px | Reduced max-width (300px)                   |
| < 768px    | Hidden (mobile uses different UX per P6800) |

---

## State Machine

### States

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                            ACTION BAR STATE MACHINE                           │
└──────────────────────────────────────────────────────────────────────────────┘

                         ┌─────────────────────────────────────┐
                         │                                     │
                         ▼                                     │
    ┌──────────┐     hover     ┌───────────┐     click     ┌──────────┐
    │   IDLE   │ ────────────► │  PREVIEW  │ ────────────► │ PROGRESS │
    └──────────┘               └───────────┘               └──────────┘
         ▲                           │                           │
         │                           │ leave                     │ complete
         │                           │ (debounced)               │
         │                           ▼                           ▼
         │                     ┌───────────┐               ┌──────────┐
         │                     │   (fade   │               │ COMPLETE │
         │                     │   out)    │               └──────────┘
         │                     └───────────┘                     │
         │                           │                           │ 2s timeout
         │                           │                           │
         └───────────────────────────┴───────────────────────────┘
```

### State Definitions

| State    | Entry Condition                     | Exit Condition             | Content                             |
| -------- | ----------------------------------- | -------------------------- | ----------------------------------- |
| IDLE     | Default, timeout, selection cleared | Hover compartment          | "Hover a status to see actions"     |
| PREVIEW  | Hover maintained 300ms              | Click, leave, or new hover | Action preview + DO NOW button      |
| PROGRESS | DO NOW clicked                      | Command complete           | Spinner + action name + STOP button |
| COMPLETE | Command finished                    | 2s timeout or new hover    | Result icon + message               |

### Transition Timing

| Transition          | Duration | Easing   | Notes                   |
| ------------------- | -------- | -------- | ----------------------- |
| IDLE → PREVIEW      | 300ms    | —        | Debounce, not animation |
| PREVIEW → IDLE      | 300ms    | —        | Debounce on mouse leave |
| Any → PROGRESS      | 0ms      | —        | Immediate on click      |
| PROGRESS → COMPLETE | 0ms      | —        | Immediate on WS message |
| COMPLETE → IDLE     | 2000ms   | —        | Auto-timeout            |
| Content fade in     | 200ms    | ease-out | CSS opacity             |
| Content fade out    | 150ms    | ease-in  | CSS opacity             |

---

## Implementation

### Template (`dashboard.templ`)

Add between `.brand` and `.header-actions` in header:

```go
templ ActionBar() {
    <div
        id="action-bar"
        class="action-bar"
        x-data="actionBar()"
        x-init="init()"
        @action-preview.window="handlePreview($event.detail)"
        @action-clear.window="handleClear()"
        @command-start.window="handleCommandStart($event.detail)"
        @command-complete.window="handleCommandComplete($event.detail)"
    >
        <div
            class="action-bar-content"
            :class="{ 'fade-out': transitioning, 'fade-in': !transitioning }"
        >
            <!-- IDLE State -->
            <template x-if="state === 'idle'">
                <span class="action-bar-idle">Hover a status to see actions</span>
            </template>

            <!-- PREVIEW State -->
            <template x-if="state === 'preview' && action">
                <div class="action-bar-preview">
                    <div class="action-bar-header">
                        <span class="action-icon">
                            <svg class="icon"><use :href="'#icon-' + action.iconRef"></use></svg>
                        </span>
                        <span class="action-name" x-text="action.name"></span>
                        <button
                            class="btn btn-action-bar"
                            @click="execute()"
                            :disabled="!action.hostId"
                            :title="'Execute ' + action.name + ' on ' + action.hostname"
                        >
                            <svg class="icon"><use href="#icon-play"></use></svg>
                            DO NOW
                        </button>
                    </div>
                    <p class="action-bar-description" x-text="action.description"></p>
                    <p class="action-bar-target">
                        Host: <strong x-text="action.hostname"></strong>
                    </p>
                </div>
            </template>

            <!-- PROGRESS State -->
            <template x-if="state === 'progress' && action">
                <div class="action-bar-preview">
                    <div class="action-bar-header">
                        <span class="action-icon spinning">
                            <svg class="icon"><use href="#icon-loader"></use></svg>
                        </span>
                        <span class="action-name" x-text="action.name + '...'"></span>
                        <button
                            class="btn btn-stop"
                            @click="stop()"
                            title="Stop command"
                        >
                            <svg class="icon"><use href="#icon-stop"></use></svg>
                            Stop
                        </button>
                    </div>
                    <p class="action-bar-description" x-text="action.progressText"></p>
                    <p class="action-bar-target">
                        Host: <strong x-text="action.hostname"></strong>
                    </p>
                </div>
            </template>

            <!-- COMPLETE State -->
            <template x-if="state === 'complete' && result">
                <div class="action-bar-preview">
                    <div class="action-bar-header">
                        <span
                            class="action-icon"
                            :class="result.success ? 'success' : 'error'"
                        >
                            <svg class="icon">
                                <use :href="result.success ? '#icon-check' : '#icon-x'"></use>
                            </svg>
                        </span>
                        <span class="action-name" x-text="result.title"></span>
                    </div>
                    <p class="action-bar-description" x-text="result.message"></p>
                    <p class="action-bar-target">
                        Host: <strong x-text="result.hostname"></strong>
                    </p>
                </div>
            </template>
        </div>
    </div>
}
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   ACTION BAR (P1010)
   ═══════════════════════════════════════════════════════════════════════════ */

.action-bar {
  flex: 1;
  min-width: 320px;
  max-width: 480px;
  margin: 0 1rem;
}

@media (max-width: 1024px) {
  .action-bar {
    max-width: 300px;
    margin: 0 0.5rem;
  }
}

@media (max-width: 768px) {
  .action-bar {
    display: none;
  }
}

.action-bar-content {
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
  transition: opacity 200ms ease-out;
  min-height: 64px;
  display: flex;
  align-items: center;
}

.action-bar-content.fade-out {
  opacity: 0;
  transition: opacity 150ms ease-in;
}

.action-bar-content.fade-in {
  opacity: 1;
}

.action-bar-idle {
  color: var(--fg-muted);
  font-style: italic;
  width: 100%;
  text-align: center;
  font-size: 0.875rem;
}

.action-bar-preview {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.action-bar-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.action-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}

.action-icon .icon {
  width: 20px;
  height: 20px;
}

.action-icon.spinning .icon {
  animation: spin 1s linear infinite;
}

.action-icon.success {
  color: var(--green);
}

.action-icon.error {
  color: var(--red);
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.action-name {
  font-weight: 600;
  font-size: 0.9375rem;
  flex: 1;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.action-bar-description {
  font-size: 0.8125rem;
  color: var(--fg-muted);
  margin: 0;
  line-height: 1.4;
}

.action-bar-target {
  font-size: 0.75rem;
  color: var(--fg-dim);
  margin: 0;
}

.action-bar-target strong {
  color: var(--fg-muted);
  font-weight: 500;
}

/* Buttons */
.btn-action-bar {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--green);
  color: var(--bg);
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.8125rem;
  cursor: pointer;
  transition:
    background 150ms ease,
    transform 100ms ease;
  flex-shrink: 0;
}

.btn-action-bar:hover:not(:disabled) {
  background: var(--green-bright);
}

.btn-action-bar:active:not(:disabled) {
  transform: scale(0.97);
}

.btn-action-bar:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-action-bar .icon {
  width: 14px;
  height: 14px;
}

.btn-stop {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--red);
  color: var(--bg);
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.8125rem;
  cursor: pointer;
  transition: background 150ms ease;
  flex-shrink: 0;
}

.btn-stop:hover {
  background: var(--red-bright);
}

.btn-stop .icon {
  width: 14px;
  height: 14px;
}
```

### JavaScript (Alpine.js Component)

```js
// ═══════════════════════════════════════════════════════════════════════════
// ACTION BAR COMPONENT (P1010)
// ═══════════════════════════════════════════════════════════════════════════

Alpine.data("actionBar", () => ({
  // State
  state: "idle", // 'idle' | 'preview' | 'progress' | 'complete'
  action: null, // Current action data
  result: null, // Command result data
  transitioning: false, // For fade animations

  // Timers (tracked for cleanup)
  _debounceTimer: null,
  _completeTimer: null,
  _transitionTimer: null,

  // Command tracking (for matching complete events)
  _activeCommand: null,

  /**
   * Initialize component and set up cleanup
   */
  init() {
    // Cleanup on component destroy
    this.$watch("$el", () => {
      this._cleanup();
    });
  },

  /**
   * Clean up all timers to prevent memory leaks
   */
  _cleanup() {
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
    if (this._completeTimer) clearTimeout(this._completeTimer);
    if (this._transitionTimer) clearTimeout(this._transitionTimer);
    this._debounceTimer = null;
    this._completeTimer = null;
    this._transitionTimer = null;
  },

  /**
   * Handle compartment hover event
   */
  handlePreview(detail) {
    // Ignore during progress (locked to current command)
    if (this.state === "progress") return;

    // Cancel pending timers
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
    if (this._completeTimer) clearTimeout(this._completeTimer);

    // Debounce: wait 300ms before showing preview
    this._debounceTimer = setTimeout(() => {
      this._debounceTimer = null;
      this._showPreview(detail);
    }, 300);
  },

  /**
   * Handle compartment leave event
   */
  handleClear() {
    // Cancel pending preview
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
      this._debounceTimer = null;
    }

    // Only clear from preview state (not progress/complete)
    if (this.state !== "preview") return;

    // Debounce: wait 300ms before clearing
    this._debounceTimer = setTimeout(() => {
      this._debounceTimer = null;
      this._transitionTo("idle");
    }, 300);
  },

  /**
   * Handle command start (from WebSocket)
   */
  handleCommandStart(detail) {
    // Only update if this is our active command
    if (
      this._activeCommand &&
      this._activeCommand.hostId === detail.hostId &&
      this._activeCommand.command === detail.command
    ) {
      if (!this.action) {
        this.action = this._buildAction(
          detail.command,
          detail.hostId,
          detail.hostname,
        );
      }
      this.state = "progress";
    }
  },

  /**
   * Handle command complete (from WebSocket)
   */
  handleCommandComplete(detail) {
    // Only update if this is our active command
    if (this._activeCommand && this._activeCommand.hostId === detail.hostId) {
      this._showComplete(detail);
      this._activeCommand = null;
    }
  },

  /**
   * Execute the previewed action
   */
  execute() {
    if (!this.action || !this.action.hostId) return;

    // Track active command
    this._activeCommand = {
      hostId: this.action.hostId,
      command: this.action.command,
    };

    // Check dependencies (delegates to P1040)
    if (this.action.command === "switch") {
      checkDependenciesAndExecute([this.action.hostId], "switch");
    } else if (this.action.command === "refresh") {
      refreshHost(this.action.hostId);
      // Refresh doesn't go to progress state
      this._activeCommand = null;
      this._transitionTo("idle");
    } else {
      sendCommand(this.action.hostId, this.action.command);
    }

    // Note: State transitions to 'progress' via command-start event
  },

  /**
   * Stop the running command
   */
  stop() {
    if (!this.action || !this.action.hostId) return;
    sendCommand(this.action.hostId, "stop");
  },

  /**
   * Internal: Show preview with fade
   */
  _showPreview(detail) {
    this.action = this._buildAction(
      detail.command,
      detail.hostId,
      detail.hostname,
    );

    if (detail.isInfoOnly) {
      // For info-only compartments, don't change state
      // The compartment click handler shows the toast
      return;
    }

    this.state = "preview";
  },

  /**
   * Internal: Show complete state with auto-timeout
   */
  _showComplete(detail) {
    const success = detail.exit_code === 0;
    const commandName = this._getCommandName(
      detail.command || this._activeCommand?.command,
    );

    this.result = {
      success,
      title: success ? `${commandName} Complete` : `${commandName} Failed`,
      message: success
        ? this._getSuccessMessage(
            detail.command || this._activeCommand?.command,
          )
        : `Exit code: ${detail.exit_code}`,
      hostname: detail.hostname || this.action?.hostname || "Unknown",
    };

    this.state = "complete";
    this.action = null;

    // Auto-return to idle after 2s
    if (this._completeTimer) clearTimeout(this._completeTimer);
    this._completeTimer = setTimeout(() => {
      this._completeTimer = null;
      this._transitionTo("idle");
    }, 2000);
  },

  /**
   * Internal: Transition with fade animation
   */
  _transitionTo(newState) {
    // Skip if already in target state
    if (this.state === newState) return;

    // Clear result when leaving complete state
    if (this.state === "complete" && newState !== "complete") {
      this.result = null;
    }

    // Clear action when going to idle
    if (newState === "idle") {
      this.action = null;
    }

    // Simple state change (no fade needed for most transitions)
    this.state = newState;
  },

  /**
   * Internal: Build action object from details
   */
  _buildAction(command, hostId, hostname) {
    return {
      command,
      hostId,
      hostname,
      name: this._getCommandName(command),
      description: this._getCommandDescription(command),
      progressText: this._getProgressText(command),
      iconRef: this._getCommandIconRef(command),
    };
  },

  // ─── Helper Methods ───────────────────────────────────────────────────────

  _getCommandName(command) {
    const names = {
      pull: "Pull",
      switch: "Switch",
      test: "Test",
      restart: "Restart",
      refresh: "Refresh",
      stop: "Stop",
    };
    return (
      names[command] ||
      command?.charAt(0).toUpperCase() + command?.slice(1) ||
      "Unknown"
    );
  },

  _getCommandDescription(command) {
    const descriptions = {
      pull: "Fetch latest code from GitHub",
      switch: "Apply configuration (nixos-rebuild switch)",
      test: "Test configuration without activating",
      restart: "Restart the NixFleet agent",
      refresh: "Check for status updates",
    };
    return descriptions[command] || "";
  },

  _getProgressText(command) {
    const texts = {
      pull: "Fetching latest code from GitHub...",
      switch: "Applying configuration...",
      test: "Testing configuration...",
      restart: "Restarting agent...",
      refresh: "Checking status...",
    };
    return texts[command] || "Running...";
  },

  _getSuccessMessage(command) {
    const messages = {
      pull: "Successfully fetched latest code",
      switch: "Configuration applied successfully",
      test: "Configuration test passed",
      restart: "Agent restarted successfully",
      refresh: "Status updated",
    };
    return messages[command] || "Completed successfully";
  },

  _getCommandIconRef(command) {
    const icons = {
      pull: "download",
      switch: "refresh",
      test: "flask",
      restart: "refresh",
      refresh: "refresh-cw",
    };
    return icons[command] || "play";
  },
}));
```

### WebSocket Integration

Add to existing WebSocket message handler:

```js
// In handleMessage() switch statement:

case 'command_queued':
    if (!hostId) return;
    hostStore.update(hostId, { pendingCommand: payload.command });
    showLogPanel(hostId);
    // NEW: Dispatch event for Action Bar
    window.dispatchEvent(new CustomEvent('command-start', {
        detail: {
            hostId,
            hostname: hostStore.get(hostId)?.hostname || hostId,
            command: payload.command
        }
    }));
    break;

case 'command_complete':
    if (!hostId) return;
    hostStore.update(hostId, { pendingCommand: null });
    window.dispatchEvent(new CustomEvent('log-complete', { detail: payload }));
    // NEW: Dispatch event for Action Bar
    window.dispatchEvent(new CustomEvent('command-complete', {
        detail: {
            hostId,
            hostname: hostStore.get(hostId)?.hostname || hostId,
            command: payload.command,
            exit_code: payload.exit_code
        }
    }));
    break;
```

---

## New SVG Icons Required

Add to sprite in `base.templ`:

```html
<symbol id="icon-play" viewBox="0 0 24 24">
  <polygon points="5 3 19 12 5 21 5 3" fill="currentColor"></polygon>
</symbol>

<symbol id="icon-loader" viewBox="0 0 24 24">
  <line x1="12" y1="2" x2="12" y2="6"></line>
  <line x1="12" y1="18" x2="12" y2="22"></line>
  <line x1="4.93" y1="4.93" x2="7.76" y2="7.76"></line>
  <line x1="16.24" y1="16.24" x2="19.07" y2="19.07"></line>
  <line x1="2" y1="12" x2="6" y2="12"></line>
  <line x1="18" y1="12" x2="22" y2="12"></line>
  <line x1="4.93" y1="19.07" x2="7.76" y2="16.24"></line>
  <line x1="16.24" y1="7.76" x2="19.07" y2="4.93"></line>
</symbol>

<symbol id="icon-refresh-cw" viewBox="0 0 24 24">
  <polyline points="23 4 23 10 17 10"></polyline>
  <polyline points="1 20 1 14 7 14"></polyline>
  <path
    d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"
  ></path>
</symbol>
```

---

## Edge Cases

### Rapid State Changes

**Problem**: User rapidly hovers/leaves compartments.

**Solution**: Debounce timers prevent flickering. Each new hover cancels previous timer.

### Command Complete While Hovering New Compartment

**Problem**: User starts command, hovers different compartment, command completes.

**Solution**: Complete event only updates if `_activeCommand` matches. Otherwise ignored.

### Memory Leaks

**Problem**: Timers not cleaned up on component unmount.

**Solution**: `_cleanup()` method clears all timers. Called on component destroy.

### Multiple Commands

**Problem**: User clicks DO NOW twice rapidly.

**Solution**: First click clears `action`, second click has nothing to execute.

### Host Goes Offline

**Problem**: Command running, host disconnects.

**Solution**: Backend sends `command_complete` with error. Action Bar shows failure.

---

## Testing

### Unit Tests

```js
describe("ActionBar", () => {
  describe("State Transitions", () => {
    it("starts in idle state", () => {});
    it("transitions to preview after 300ms hover", () => {});
    it("returns to idle after 300ms leave", () => {});
    it("transitions to progress on execute", () => {});
    it("transitions to complete on command finish", () => {});
    it("returns to idle 2s after complete", () => {});
  });

  describe("Timer Management", () => {
    it("cancels debounce timer on new hover", () => {});
    it("cancels complete timer on new hover", () => {});
    it("cleans up all timers on destroy", () => {});
  });

  describe("Command Tracking", () => {
    it("ignores complete for non-matching host", () => {});
    it("ignores complete for non-matching command", () => {});
    it("clears active command on complete", () => {});
  });
});
```

### Manual Test Cases

| #   | Scenario         | Steps                           | Expected                          |
| --- | ---------------- | ------------------------------- | --------------------------------- |
| 1   | Basic preview    | Hover Git compartment for 500ms | Action Bar shows "Pull" preview   |
| 2   | Quick hover      | Hover Git for 100ms, leave      | Action Bar stays idle (debounced) |
| 3   | Execute          | Hover Git → Click DO NOW        | Progress state, then complete     |
| 4   | Stop command     | During progress, click Stop     | Stop command sent                 |
| 5   | Rapid hovers     | Quickly hover 3 compartments    | Only last shows (debounced)       |
| 6   | Complete timeout | Wait for complete → idle        | Returns to idle after 2s          |
| 7   | Keyboard         | Tab to compartment, Enter       | Action Bar updates                |

---

## Acceptance Criteria

- [ ] **Position**: Visible in header, horizontally centered
- [ ] **IDLE**: Shows "Hover a status to see actions" (muted, italic)
- [ ] **PREVIEW**: Shows action icon, name, description, host, DO NOW button
- [ ] **Debounce**: 300ms delay before showing/hiding preview
- [ ] **PROGRESS**: Shows spinner icon, action name with "...", STOP button
- [ ] **COMPLETE Success**: Green checkmark, "Complete" title, success message
- [ ] **COMPLETE Error**: Red X, "Failed" title, exit code
- [ ] **Auto-clear**: Returns to IDLE 2s after complete
- [ ] **Timer cleanup**: No memory leaks after 5 minutes of use
- [ ] **Responsive**: Hidden on mobile (< 768px)
- [ ] **Focus visible**: Buttons show focus outline on keyboard nav
- [ ] **Keyboard**: Works with Tab + Enter navigation

---

## Related

- **P1020**: Clickable Compartments (dispatches hover events)
- **P1030**: Row Selection (not directly related)
- **P1040**: Dependency Dialog (called from execute)
- **P1015**: Selection Bar (separate bulk action UI)
