# P1020 - Clickable Compartments

**Created**: 2025-12-19  
**Updated**: 2025-12-20  
**Priority**: P1020 (Critical)  
**Status**: ✅ DONE  
**Completed**: 2025-12-20  
**Parent**: [P1000](./P1000-update-ux-overhaul.md)  
**Estimated Effort**: 4 hours  
**Depends on**: P1010 (Action Bar), P1040 (Dependency Dialog)

> **Completion Note**: All compartments are now `<button>` elements with proper hover/click handlers. Events dispatch to the Context Bar (unified from P1010+P1015). Toast notifications work for info/error states. Rate limiting implemented with 500ms cooldown.

---

## Overview

Transform the Update column's status compartments (Git, Lock, System) from passive indicators into interactive buttons that trigger actions and update the Action Bar on hover.

---

## Requirements

### Compartment Action Matrix

| Compartment | Status   | Indicator | Click Action                      | Preview Text          |
| ----------- | -------- | --------- | --------------------------------- | --------------------- |
| Git         | ok       | 🟢        | `refreshHost()`                   | "Check for updates"   |
| Git         | outdated | 🟡        | `sendCommand('pull')`             | "Fetch latest code"   |
| Git         | error    | 🔴        | `showToast(message)`              | —                     |
| Lock        | ok       | 🟢        | `refreshHost()`                   | "Check dependencies"  |
| Lock        | outdated | 🟡        | `showToast(info)`                 | — (info only)         |
| Lock        | error    | 🔴        | `sendCommand('switch')`           | "Update agent"        |
| System      | ok       | 🟢        | `refreshHost()`                   | "Check system"        |
| System      | outdated | 🟡        | `sendCommand('switch')` via P1040 | "Apply configuration" |
| System      | error    | 🔴        | `showToast(message)`              | —                     |

### Hover Behavior

| Property                | Value                        |
| ----------------------- | ---------------------------- |
| Cursor                  | `pointer` (except info-only) |
| Scale on hover          | 1.1×                         |
| Debounce before preview | 300ms                        |
| Event dispatched        | `action-preview` to window   |

### Click Behavior

| Property        | Value                |
| --------------- | -------------------- |
| Visual feedback | scale(0.95) briefly  |
| Rate limit      | 500ms between clicks |
| Error handling  | Toast on failure     |

---

## Implementation

### Template Changes (`dashboard.templ`)

Replace `<span>` compartments in `UpdateStatusCell` with `<button>`:

```go
// UpdateStatusCell renders the update status indicator (3 compartments)
templ UpdateStatusCell(host Host) {
    <div
        class="update-status"
        data-host-id={ host.ID }
    >
        <!-- Git compartment -->
        <button
            type="button"
            class={ compartmentButtonClass(host.UpdateStatus, "git") }
            title={ gitTooltip(host) }
            data-host-id={ host.ID }
            data-hostname={ host.Hostname }
            data-compartment="git"
            data-status={ getCompartmentStatus(host.UpdateStatus, "git") }
            data-action={ getCompartmentActionType(host.UpdateStatus, "git") }
            onmouseenter="handleCompartmentHover(this)"
            onmouseleave="handleCompartmentLeave()"
            onclick="handleCompartmentClick(this)"
        >
            <svg class="update-icon"><use href="#icon-git-branch"></use></svg>
            <span class={ compartmentIndicatorClass(host.UpdateStatus, "git") }></span>
        </button>

        <!-- Lock compartment -->
        <button
            type="button"
            class={ lockCompartmentButtonClass(host) }
            title={ lockTooltip(host) }
            data-host-id={ host.ID }
            data-hostname={ host.Hostname }
            data-compartment="lock"
            data-status={ getLockStatus(host) }
            data-action={ getLockActionType(host) }
            onmouseenter="handleCompartmentHover(this)"
            onmouseleave="handleCompartmentLeave()"
            onclick="handleCompartmentClick(this)"
        >
            <svg class="update-icon"><use href="#icon-lock"></use></svg>
            <span class={ lockIndicatorClass(host) }></span>
        </button>

        <!-- System compartment -->
        <button
            type="button"
            class={ compartmentButtonClass(host.UpdateStatus, "system") }
            title={ systemTooltip(host) }
            data-host-id={ host.ID }
            data-hostname={ host.Hostname }
            data-compartment="system"
            data-status={ getCompartmentStatus(host.UpdateStatus, "system") }
            data-action={ getCompartmentActionType(host.UpdateStatus, "system") }
            onmouseenter="handleCompartmentHover(this)"
            onmouseleave="handleCompartmentLeave()"
            onclick="handleCompartmentClick(this)"
        >
            if host.HostType == "macos" {
                <svg class="update-icon"><use href="#icon-home"></use></svg>
            } else {
                <svg class="update-icon"><use href="#icon-nixos"></use></svg>
            }
            <span class={ compartmentIndicatorClass(host.UpdateStatus, "system") }></span>
        </button>
    </div>
}
```

### Go Helper Functions (`dashboard.templ`)

```go
// compartmentButtonClass returns CSS classes for compartment button
func compartmentButtonClass(status *UpdateStatus, compartment string) string {
    base := "compartment-btn"

    if status == nil {
        return base + " unknown"
    }

    var check StatusCheck
    switch compartment {
    case "git":
        check = status.Git
    case "system":
        check = status.System
    }

    if check.Status == "" || check.Status == "unknown" {
        return base + " unknown"
    }

    return base
}

// lockCompartmentButtonClass returns CSS classes for Lock button
func lockCompartmentButtonClass(host Host) string {
    base := "compartment-btn"

    if host.UpdateStatus == nil {
        return base + " unknown"
    }

    if host.UpdateStatus.Lock.Status == "" || host.UpdateStatus.Lock.Status == "unknown" {
        return base + " unknown"
    }

    // Lock outdated is info-only (no action)
    if host.UpdateStatus.Lock.Status == "outdated" && !host.AgentOutdated {
        return base + " info-only"
    }

    return base
}

// getCompartmentStatus returns status string for data attribute
func getCompartmentStatus(status *UpdateStatus, compartment string) string {
    if status == nil {
        return "unknown"
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

    if check.Status == "" {
        return "unknown"
    }
    return check.Status
}

// getLockStatus returns status for Lock (considers agent outdated)
func getLockStatus(host Host) string {
    if host.AgentOutdated {
        return "error"
    }
    if host.UpdateStatus == nil {
        return "unknown"
    }
    return host.UpdateStatus.Lock.Status
}

// getCompartmentActionType returns action type for routing
func getCompartmentActionType(status *UpdateStatus, compartment string) string {
    if status == nil {
        return "refresh"
    }

    var check StatusCheck
    switch compartment {
    case "git":
        check = status.Git
        switch check.Status {
        case "ok":
            return "refresh"
        case "outdated":
            return "pull"
        case "error":
            return "error"
        default:
            return "refresh"
        }
    case "system":
        check = status.System
        switch check.Status {
        case "ok":
            return "refresh"
        case "outdated":
            return "switch"
        case "error":
            return "error"
        default:
            return "refresh"
        }
    }

    return "refresh"
}

// getLockActionType returns action type for Lock compartment
func getLockActionType(host Host) string {
    if host.AgentOutdated {
        return "switch"
    }
    if host.UpdateStatus == nil {
        return "refresh"
    }
    switch host.UpdateStatus.Lock.Status {
    case "ok":
        return "refresh"
    case "outdated":
        return "info" // No action, just info
    case "error":
        return "error"
    default:
        return "refresh"
    }
}
```

### CSS (`styles.css`)

```css
/* ═══════════════════════════════════════════════════════════════════════════
   CLICKABLE COMPARTMENTS (P1020)
   ═══════════════════════════════════════════════════════════════════════════ */

.compartment-btn {
  /* Reset button defaults */
  appearance: none;
  background: none;
  border: none;
  padding: 0;
  margin: 0;
  font: inherit;
  color: inherit;

  /* Layout */
  display: inline-flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 4px;
  border-radius: 4px;

  /* Interaction */
  cursor: pointer;
  transition:
    transform 100ms ease,
    background 100ms ease;
}

.compartment-btn:hover {
  transform: scale(1.1);
  background: var(--bg-highlight);
}

.compartment-btn:active {
  transform: scale(0.95);
}

.compartment-btn:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

/* Unknown status: slightly dimmed */
.compartment-btn.unknown {
  opacity: 0.6;
}

/* Info-only: different cursor, no scale */
.compartment-btn.info-only {
  cursor: help;
}

.compartment-btn.info-only:hover {
  transform: none;
  background: none;
}

.compartment-btn.info-only:active {
  transform: none;
}

/* Rate-limiting: disable during cooldown */
.compartment-btn.rate-limited {
  pointer-events: none;
  opacity: 0.7;
}
```

### JavaScript

```js
// ═══════════════════════════════════════════════════════════════════════════
// CLICKABLE COMPARTMENTS (P1020)
// ═══════════════════════════════════════════════════════════════════════════

// Rate limiting state
const compartmentCooldowns = new Map();
const COOLDOWN_MS = 500;

/**
 * Handle compartment hover - dispatch event to Action Bar
 */
function handleCompartmentHover(btn) {
  const hostId = btn.dataset.hostId;
  const hostname = btn.dataset.hostname;
  const compartment = btn.dataset.compartment;
  const action = btn.dataset.action;

  // Don't show preview for error/info actions
  if (action === "error" || action === "info") {
    return;
  }

  // Map action type to command
  const command = action === "refresh" ? "refresh" : action;

  window.dispatchEvent(
    new CustomEvent("action-preview", {
      detail: {
        hostId,
        hostname,
        command,
        compartment,
      },
    }),
  );
}

/**
 * Handle compartment leave - clear Action Bar preview
 */
function handleCompartmentLeave() {
  window.dispatchEvent(new CustomEvent("action-clear"));
}

/**
 * Handle compartment click - execute action
 */
function handleCompartmentClick(btn) {
  const hostId = btn.dataset.hostId;
  const hostname = btn.dataset.hostname;
  const compartment = btn.dataset.compartment;
  const action = btn.dataset.action;
  const status = btn.dataset.status;

  // Rate limiting check
  const cooldownKey = `${hostId}-${compartment}`;
  if (compartmentCooldowns.has(cooldownKey)) {
    return;
  }

  // Set cooldown
  compartmentCooldowns.set(cooldownKey, true);
  btn.classList.add("rate-limited");
  setTimeout(() => {
    compartmentCooldowns.delete(cooldownKey);
    btn.classList.remove("rate-limited");
  }, COOLDOWN_MS);

  // Execute based on action type
  switch (action) {
    case "refresh":
      refreshHost(hostId);
      break;

    case "pull":
      sendCommand(hostId, "pull");
      break;

    case "switch":
      // Check dependencies (delegates to P1040)
      checkDependenciesAndExecute([hostId], "switch");
      break;

    case "info":
      // Lock outdated - show informational toast
      showToast(
        "Dependencies outdated. Update flake.lock via GitHub PR.",
        "info",
      );
      break;

    case "error":
      // Show error message from tooltip or generic
      const message = btn.title || "Status check failed";
      showToast(message, "error");
      break;

    default:
      refreshHost(hostId);
  }
}

/**
 * Show toast notification
 */
function showToast(message, type = "info") {
  // Remove existing toast
  const existing = document.querySelector(".toast");
  if (existing) {
    existing.remove();
  }

  // Create toast element
  const toast = document.createElement("div");
  toast.className = `toast toast-${type}`;
  toast.textContent = message;

  // Add to document
  document.body.appendChild(toast);

  // Trigger animation
  requestAnimationFrame(() => {
    toast.classList.add("show");
  });

  // Auto-remove
  setTimeout(() => {
    toast.classList.remove("show");
    setTimeout(() => toast.remove(), 300);
  }, 3000);
}
```

### Toast CSS

```css
/* ═══════════════════════════════════════════════════════════════════════════
   TOAST NOTIFICATIONS (P1020)
   ═══════════════════════════════════════════════════════════════════════════ */

.toast {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%) translateY(20px);
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 20px;
  font-size: 0.875rem;
  opacity: 0;
  transition:
    transform 300ms ease,
    opacity 300ms ease;
  z-index: 2000;
  max-width: 90%;
  text-align: center;
}

.toast.show {
  transform: translateX(-50%) translateY(0);
  opacity: 1;
}

.toast-info {
  border-color: var(--blue);
}

.toast-error {
  border-color: var(--red);
  color: var(--red);
}

.toast-success {
  border-color: var(--green);
  color: var(--green);
}
```

---

## Touch Device Handling

For touch devices (no hover):

| Scenario   | Behavior                             |
| ---------- | ------------------------------------ |
| Single tap | Execute action directly (no preview) |
| Long press | Future: could show tooltip           |

Detection is via `@media (hover: hover)` in CSS, but JavaScript behavior is the same — action executes on click regardless of device.

---

## Edge Cases

### Rapid Clicking

**Problem**: User clicks same compartment rapidly.

**Solution**: 500ms rate limit per compartment per host. Button shows `rate-limited` class during cooldown.

### Compartment Status Changes During Hover

**Problem**: Status updates via WebSocket while hovering.

**Solution**: Action Bar preview remains until mouse leaves. Click uses current data-\* attributes which are updated by `renderUpdateStatus()`.

### Host Goes Offline

**Problem**: User hovers/clicks compartment on offline host.

**Solution**: Row has `host-offline` class. Compartments still clickable but commands will fail gracefully on backend.

---

## Testing

### Manual Test Cases

| #   | Scenario      | Steps                             | Expected                          |
| --- | ------------- | --------------------------------- | --------------------------------- |
| 1   | Hover preview | Hover Git yellow for 500ms        | Action Bar shows "Pull" preview   |
| 2   | Quick hover   | Hover Git for 100ms, leave        | No preview (debounced)            |
| 3   | Git pull      | Click Git yellow                  | Pull command executes             |
| 4   | Git refresh   | Click Git green                   | refreshHost() called              |
| 5   | Lock info     | Click Lock yellow                 | Toast: "Dependencies outdated..." |
| 6   | Lock switch   | Click Lock red (agent outdated)   | Switch command executes           |
| 7   | System switch | Click System yellow               | P1040 dependency check            |
| 8   | Error toast   | Click error compartment           | Toast with error message          |
| 9   | Rate limit    | Click same compartment twice fast | Second click ignored              |
| 10  | Keyboard      | Tab to compartment, Enter         | Action executes                   |

---

## Acceptance Criteria

- [ ] All 3 compartments are `<button>` elements
- [ ] Hover shows pointer cursor and scale(1.1)
- [ ] Hover dispatches `action-preview` event (after 300ms debounce in P1010)
- [ ] Leave dispatches `action-clear` event
- [ ] Click executes action per matrix
- [ ] Git green → refresh
- [ ] Git yellow → pull
- [ ] Lock yellow → info toast (no command)
- [ ] Lock red (agent) → switch
- [ ] System yellow → switch (via P1040)
- [ ] Error compartments → error toast
- [ ] 500ms rate limit prevents rapid clicks

---

## Related

- **P1010**: Action Bar (receives hover events)
- **P1040**: Dependency Dialog (called for switch actions)
- **P1050**: Remove Action Buttons (compartments replace them)
