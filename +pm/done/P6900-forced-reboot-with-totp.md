# P6900 - Forced Reboot with TOTP Authentication

**Created**: 2025-01-XX  
**Priority**: P6900 (Medium)  
**Status**: Backlog  
**Depends on**: P4200 (Go Dashboard Core - TOTP auth already implemented)

---

## Overview

Add per-host forced reboot capability (`sudo reboot`) protected by TOTP authentication as an additional security measure. Uses the same TOTP mechanism as login. Reboot is a **special command** (like "restart" and "stop") that bypasses the command state machine and works even when the agent is busy.

---

## Requirements

### FR-1: Reboot Command UI

- Add "Reboot Host" option to per-host dropdown menu (ellipsis menu)
- Position after "Restart Agent" option
- Icon: Power/Reboot icon (distinct from agent restart icon)
- Only visible when host is online
- Disabled state: Show tooltip "Host offline" when host is offline

### FR-2: TOTP Verification Modal

- Clicking "Reboot Host" opens a modal dialog
- Modal requires TOTP code entry (same UI as login TOTP field)
- TOTP field:
  - 6-digit numeric input
  - `autocomplete="one-time-code"`
  - `inputmode="numeric"`
  - `pattern="[0-9]*"`
  - `maxlength="6"`
  - Letter spacing for readability (0.2em)
  - Auto-focus on open
- Modal shows:
  - Hostname being rebooted (prominently displayed)
  - Warning message: "⚠️ This will immediately reboot the host. This action cannot be undone."
  - TOTP input field with label
  - Cancel and "Reboot" buttons
- "Reboot" button:
  - Disabled until valid TOTP entered (6 digits)
  - Shows loading state during API call
  - Disabled during request to prevent double-submission
- Modal can be closed via:
  - Cancel button
  - Click outside modal (backdrop)
  - ESC key

### FR-3: Backend TOTP Validation

- New endpoint: `POST /api/hosts/{hostID}/reboot`
- Protected by `requireAuth` middleware (session required)
- Protected by `requireCSRF` middleware (CSRF token required)
- Validates TOTP code using existing `auth.CheckTOTP()` method
- **TOTP is mandatory**: Returns 403 if TOTP not configured system-wide
- Returns 400 if request body invalid (missing TOTP field)
- Returns 401 if TOTP invalid or missing
- Returns 404 if host not found
- Returns 409 if host offline
- Returns 429 if rate limit exceeded
- Returns 200 with JSON response if reboot command sent successfully
- Response format: `{"status": "queued", "message": "Reboot command sent"}`

### FR-4: Agent Reboot Command

- Add "reboot" command handler in agent (special command, works even when busy)
- **Platform-specific execution**:
  - Linux/NixOS: `sudo reboot`
  - macOS: `sudo reboot` (or `sudo shutdown -r now` as fallback)
- Works even when agent is busy (like "restart" and "stop" commands)
- **Bypasses command state machine** (no pre/post-validation)
- Logs reboot request with timestamp and hostname
- Sends WebSocket message before executing (so dashboard knows reboot is happening)
- 3-second countdown before execution (allows WebSocket message to be sent)
- Agent connection will be lost (expected behavior)
- Error handling: If sudo fails, log error (but connection may be lost before message sent)

### FR-5: Security Considerations

- **TOTP Required**: Even authenticated users must provide TOTP for reboot
- **Rate Limiting**: Max 5 reboot attempts per host per hour (persistent, database-backed)
- **Rate limit scope**: Applies to all attempts (successful and failed)
- **Rate limit window**: Rolling 1-hour window per host
- **Audit logging**: Log all reboot attempts (successful and failed) with:
  - Host ID
  - User session ID
  - IP address
  - Timestamp
  - TOTP validation result
  - Rate limit status
  - Command send result
- **Cannot be queued**: Requires immediate execution (host must be online)
- **Pre-validation**: Check host is online before showing TOTP modal (UI-level)
- **CSRF protection**: All requests require valid CSRF token
- **Session validation**: Session must be valid and non-expired

---

## Technical Design

### Backend Handler

```go
// handleReboot processes reboot request with TOTP verification
func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request) {
    hostID := chi.URLParam(r, "hostID")
    session := sessionFromContext(r.Context())
    if session == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req struct {
        TOTP string `json:"totp"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid request body",
        })
        return
    }

    // TOTP is mandatory for reboot
    if !s.cfg.HasTOTP() {
        s.log.Warn().Msg("reboot rejected: TOTP not configured")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "TOTP must be configured to use reboot feature",
        })
        return
    }

    // Validate TOTP
    if req.TOTP == "" {
        s.log.Warn().Str("host", hostID).Msg("reboot rejected: missing TOTP")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "TOTP code required",
        })
        return
    }

    if !s.auth.CheckTOTP(req.TOTP) {
        s.log.Warn().
            Str("host", hostID).
            Str("session", session.ID).
            Str("ip", r.RemoteAddr).
            Msg("reboot rejected: invalid TOTP")

        // Audit log failed attempt
        s.auditLogReboot(hostID, session.ID, r.RemoteAddr, false, "invalid_totp")

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Invalid TOTP code",
        })
        return
    }

    // Check rate limit (per-host, rolling 1-hour window)
    if s.isRebootRateLimited(hostID) {
        s.log.Warn().
            Str("host", hostID).
            Str("session", session.ID).
            Str("ip", r.RemoteAddr).
            Msg("reboot rejected: rate limit exceeded")

        // Audit log rate limit
        s.auditLogReboot(hostID, session.ID, r.RemoteAddr, false, "rate_limit")

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Too many reboot attempts. Please wait before trying again.",
        })
        return
    }

    // Verify host exists and is online
    host, err := s.getHostByID(hostID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Host not found",
        })
        return
    }

    agent := s.hub.GetAgent(hostID)
    if agent == nil {
        s.log.Warn().Str("host", hostID).Msg("reboot rejected: host offline")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusConflict)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Host is offline",
        })
        return
    }

    // Record attempt in rate limiting table
    if err := s.recordRebootAttempt(hostID, true); err != nil {
        s.log.Error().Err(err).Str("host", hostID).Msg("failed to record reboot attempt")
        // Continue anyway - rate limiting failure shouldn't block reboot
    }

    // Send reboot command to agent
    cmdMsg, err := json.Marshal(map[string]any{
        "type": "command",
        "payload": map[string]any{
            "command": "reboot",
        },
    })
    if err != nil {
        s.log.Error().Err(err).Msg("failed to marshal reboot command")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Internal server error",
        })
        return
    }

    select {
    case agent.send <- cmdMsg:
        s.log.Info().
            Str("host", hostID).
            Str("hostname", host.Hostname).
            Str("session", session.ID).
            Str("ip", r.RemoteAddr).
            Msg("reboot command sent")

        // Audit log successful attempt
        s.auditLogReboot(hostID, session.ID, r.RemoteAddr, true, "success")

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{
            "status":  "queued",
            "message": "Reboot command sent to host",
        })
    default:
        s.log.Warn().Str("host", hostID).Msg("reboot failed: agent send buffer full")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Agent not responsive",
        })
    }
}

// isRebootRateLimited checks if host has exceeded rate limit (5 per hour)
func (s *Server) isRebootRateLimited(hostID string) bool {
    cutoff := time.Now().Add(-1 * time.Hour).Unix()

    var count int
    err := s.db.QueryRow(
        `SELECT COUNT(*) FROM reboot_attempts WHERE host_id = ? AND attempted_at > ?`,
        hostID, cutoff,
    ).Scan(&count)

    if err != nil {
        s.log.Error().Err(err).Str("host", hostID).Msg("failed to check rate limit")
        // Fail open: if we can't check rate limit, allow the request
        return false
    }

    return count >= 5
}

// recordRebootAttempt records a reboot attempt in the database
func (s *Server) recordRebootAttempt(hostID string, success bool) error {
    _, err := s.db.Exec(
        `INSERT INTO reboot_attempts (host_id, attempted_at, success) VALUES (?, ?, ?)`,
        hostID, time.Now().Unix(), success,
    )
    return err
}

// auditLogReboot logs reboot attempt to audit log
func (s *Server) auditLogReboot(hostID, sessionID, ip string, success bool, reason string) {
    details := fmt.Sprintf(`{"reason": "%s", "ip": "%s"}`, reason, ip)
    _, err := s.db.Exec(
        `INSERT INTO audit_log (action, host_id, user_session, timestamp, success, details) VALUES (?, ?, ?, ?, ?, ?)`,
        "reboot", hostID, sessionID, time.Now().Unix(), success, details,
    )
    if err != nil {
        s.log.Error().Err(err).Msg("failed to write audit log")
    }
}
```

### Agent Handler

```go
// In handleCommand function, add before busy check:
case "reboot":
    // Reboot works anytime (like restart and stop)
    a.handleReboot()
    return

// ...

// handleReboot executes system reboot
func (a *Agent) handleReboot() {
    a.log.Warn().Msg("reboot requested, executing system reboot")

    // Send WebSocket message before reboot (so dashboard knows it's happening)
    a.ws.SendMessage(protocol.TypeOutput, protocol.OutputPayload{
        Line: "⚠️  Reboot command received. System will reboot in 3 seconds...",
        Stream: "stdout",
    })

    // Give time for WebSocket message to be sent
    time.Sleep(3 * time.Second)

    // Platform-specific reboot command
    var cmd *exec.Cmd
    if runtime.GOOS == "darwin" {
        // macOS: try reboot first, fallback to shutdown -r
        cmd = exec.Command("sudo", "reboot")
    } else {
        // Linux/NixOS
        cmd = exec.Command("sudo", "reboot")
    }

    // Execute reboot (this will terminate the agent)
    if err := cmd.Run(); err != nil {
        // If we get here, reboot failed (unlikely but possible)
        a.log.Error().Err(err).Msg("reboot command failed")
        // Try to send error message (may not reach dashboard)
        a.ws.SendMessage(protocol.TypeOutput, protocol.OutputPayload{
            Line: fmt.Sprintf("❌ Reboot failed: %v. Check sudo permissions.", err),
            Stream: "stderr",
        })
    }
    // If successful, agent will be terminated by reboot
}
```

### Frontend Modal

```javascript
function showRebootModal(hostId, hostname) {
  // Check host is online before showing modal
  const host = hostStore.get(hostId);
  if (!host || host.status !== "online") {
    showToast("Host is offline", "error");
    return;
  }

  const modal = document.createElement("div");
  modal.className = "modal";
  modal.id = "reboot-modal";
  modal.innerHTML = `
        <div class="modal-backdrop" onclick="closeRebootModal()"></div>
        <div class="modal-content">
            <h2>Reboot Host: ${hostname}</h2>
            <p class="warning">⚠️ This will immediately reboot the host. This action cannot be undone.</p>
            <div style="margin: 1.5rem 0;">
                <label for="reboot-totp" style="display: block; margin-bottom: 0.5rem;">
                    TOTP Code
                </label>
                <input 
                    type="text" 
                    id="reboot-totp"
                    autocomplete="one-time-code"
                    inputmode="numeric"
                    pattern="[0-9]*"
                    maxlength="6"
                    placeholder="000000"
                    style="width: 100%; padding: 0.75rem; letter-spacing: 0.2em; text-align: center; font-size: 1.2rem;"
                />
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="closeRebootModal()">Cancel</button>
                <button id="reboot-confirm" class="btn btn-danger" onclick="confirmReboot('${hostId}')" disabled>
                    <span id="reboot-text">Reboot</span>
                    <span id="reboot-spinner" style="display: none;">Sending...</span>
                </button>
            </div>
        </div>
    `;

  // Enable button when 6 digits entered
  const totpInput = document.getElementById("reboot-totp");
  const confirmBtn = document.getElementById("reboot-confirm");

  totpInput.addEventListener("input", (e) => {
    const value = e.target.value.replace(/\D/g, ""); // Remove non-digits
    e.target.value = value;
    confirmBtn.disabled = value.length !== 6;
  });

  // Close on ESC key
  document.addEventListener("keydown", function escHandler(e) {
    if (e.key === "Escape" && document.getElementById("reboot-modal")) {
      closeRebootModal();
      document.removeEventListener("keydown", escHandler);
    }
  });

  // Auto-focus TOTP input
  document.body.appendChild(modal);
  setTimeout(() => totpInput.focus(), 100);
}

function closeRebootModal() {
  const modal = document.getElementById("reboot-modal");
  if (modal) {
    modal.remove();
  }
}

async function confirmReboot(hostId) {
  const totpInput = document.getElementById("reboot-totp");
  const confirmBtn = document.getElementById("reboot-confirm");
  const rebootText = document.getElementById("reboot-text");
  const rebootSpinner = document.getElementById("reboot-spinner");
  const totp = totpInput.value;

  // Disable button and show loading state
  confirmBtn.disabled = true;
  rebootText.style.display = "none";
  rebootSpinner.style.display = "inline";

  try {
    const resp = await fetch(`/api/hosts/${hostId}/reboot`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": CSRF_TOKEN,
      },
      body: JSON.stringify({ totp }),
    });

    const data = await resp.json();

    if (resp.status === 401) {
      showToast("Invalid TOTP code", "error");
      totpInput.focus();
      totpInput.select();
      confirmBtn.disabled = false;
      rebootText.style.display = "inline";
      rebootSpinner.style.display = "none";
      return;
    }

    if (resp.status === 429) {
      showToast(
        data.error || "Too many reboot attempts. Please wait.",
        "error",
      );
      closeRebootModal();
      return;
    }

    if (!resp.ok) {
      showToast(data.error || `Reboot failed: ${resp.statusText}`, "error");
      confirmBtn.disabled = false;
      rebootText.style.display = "inline";
      rebootSpinner.style.display = "none";
      return;
    }

    // Success
    closeRebootModal();
    showToast(
      `Reboot command sent to ${hostStore.get(hostId)?.hostname || hostId}`,
      "info",
    );
  } catch (err) {
    showToast(`Network error: ${err.message}`, "error");
    confirmBtn.disabled = false;
    rebootText.style.display = "inline";
    rebootSpinner.style.display = "none";
  }
}
```

---

## Database

### Rate Limiting Table

```sql
-- Tracks reboot attempts for rate limiting (rolling 1-hour window)
CREATE TABLE IF NOT EXISTS reboot_attempts (
    host_id TEXT NOT NULL,
    attempted_at INTEGER NOT NULL,  -- Unix timestamp
    success INTEGER NOT NULL DEFAULT 0,  -- 0 = failed, 1 = successful
    PRIMARY KEY (host_id, attempted_at)
);

CREATE INDEX idx_reboot_attempts_host_time ON reboot_attempts(host_id, attempted_at);

-- Cleanup old entries (older than 24 hours) - run periodically
DELETE FROM reboot_attempts WHERE attempted_at < (strftime('%s', 'now') - 86400);
```

### Audit Logging

Uses existing audit log table (if exists) or create:

```sql
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,           -- "reboot"
    host_id TEXT,                   -- Host that was rebooted
    user_session TEXT,              -- Session ID of user who initiated
    timestamp INTEGER NOT NULL,      -- Unix timestamp
    success INTEGER NOT NULL DEFAULT 0,  -- 0 = failed, 1 = successful
    details TEXT                    -- JSON: {"reason": "...", "ip": "..."}
);

CREATE INDEX idx_audit_log_action_time ON audit_log(action, timestamp);
CREATE INDEX idx_audit_log_host_time ON audit_log(host_id, timestamp);
```

---

## Integration with P2800 Command State Machine

**Reboot bypasses the command state machine** because:

1. It's a **special command** (like "restart" and "stop") that works even when busy
2. It doesn't have pre/post-validation requirements
3. It terminates the agent connection (expected behavior)
4. It doesn't need state tracking (reboot is immediate and terminal)

**No changes needed to P2800** - reboot is handled separately, similar to how "restart" and "stop" are handled.

---

## Acceptance Criteria

- [ ] "Reboot Host" option in per-host dropdown menu (only when online)
- [ ] TOTP modal opens on click with proper styling
- [ ] TOTP input validates format (6 digits, numeric only)
- [ ] "Reboot" button disabled until valid TOTP entered
- [ ] Modal can be closed via Cancel, backdrop click, or ESC key
- [ ] TOTP validation works (same as login, uses `auth.CheckTOTP()`)
- [ ] Backend returns appropriate HTTP status codes and JSON error messages
- [ ] Rate limiting prevents abuse (5 per hour per host, rolling window)
- [ ] Rate limiting persists across dashboard restarts (database-backed)
- [ ] Reboot command sent to agent via WebSocket
- [ ] Agent executes platform-specific reboot command (`sudo reboot`)
- [ ] Agent sends WebSocket message before rebooting (3-second countdown)
- [ ] Reboot works even when agent is busy (special command)
- [ ] Audit logging records all attempts with session ID, IP, timestamp
- [ ] Error handling for all failure modes:
  - [ ] Invalid TOTP (401)
  - [ ] TOTP not configured (403)
  - [ ] Host offline (409)
  - [ ] Rate limit exceeded (429)
  - [ ] Agent not responsive (503)
- [ ] UI shows appropriate feedback (toast notifications, loading states)
- [ ] Works on both Linux (systemd) and macOS (launchd) agents
- [ ] CSRF protection enforced
- [ ] Session validation enforced

---

## Security Notes

- **TOTP Required**: Even authenticated users must provide TOTP for reboot (mandatory, not optional)
- **Rate Limiting**: Prevents accidental or malicious repeated reboots (5 per host per hour)
- **Rate Limit Scope**: Applies to all attempts (successful and failed) to prevent abuse
- **Audit Trail**: All reboot attempts logged with user session, IP address, and timestamp
- **Online Only**: Cannot reboot offline hosts (prevents queuing abuse)
- **Immediate Execution**: No delay or confirmation beyond TOTP (user already confirmed via modal)
- **CSRF Protection**: All requests require valid CSRF token
- **Session Validation**: Session must be valid and non-expired
- **Database Persistence**: Rate limiting persists across restarts (prevents reset-based bypass)

---

## Edge Cases & Error Handling

### Backend Edge Cases

1. **TOTP not configured**: Return 403 with clear message
2. **Invalid TOTP format**: Validate before checking (empty string = 401)
3. **Rate limit check fails**: Fail open (allow request if DB error)
4. **Host goes offline between check and send**: Return 409
5. **Agent send buffer full**: Return 503
6. **Database errors**: Log but don't block reboot (fail gracefully)

### Agent Edge Cases

1. **Sudo permissions**: If sudo fails, log error (may not reach dashboard)
2. **Platform detection**: Use `runtime.GOOS` to determine command
3. **WebSocket message**: Send before sleep to ensure delivery
4. **Reboot command fails**: Log error (rare but possible)

### Frontend Edge Cases

1. **Host goes offline**: Disable menu item, show tooltip
2. **Network error**: Show error toast, re-enable button
3. **Invalid TOTP**: Clear input, focus, allow retry
4. **Rate limit**: Show clear message, close modal
5. **Double-click prevention**: Disable button during request

---

## Related

- **P4200** - Go Dashboard Core (provides TOTP authentication infrastructure)
- **P6200** - Security Hardening (complements security measures)
- **P2800** - Command State Machine (reboot bypasses state machine, like "restart")
- Existing "restart" command (restarts agent, not host - different use case)
- Existing "stop" command (stops current command - different use case)

---

## Implementation Order

1. **Phase 1: Database Schema**
   - Create `reboot_attempts` table
   - Create/update `audit_log` table
   - Add cleanup job for old rate limit entries

2. **Phase 2: Backend**
   - Add `/api/hosts/{hostID}/reboot` endpoint
   - Implement TOTP validation
   - Implement rate limiting logic (database-backed)
   - Implement audit logging
   - Add route to router with CSRF protection
   - Error handling and JSON responses

3. **Phase 3: Agent**
   - Add "reboot" case to `handleCommand` (before busy check)
   - Implement `handleReboot()` function
   - Platform-specific reboot command
   - WebSocket message before reboot
   - Error handling

4. **Phase 4: Frontend**
   - Add "Reboot Host" menu item (with online check)
   - Implement TOTP modal component
   - Implement `showRebootModal()` and `confirmReboot()` functions
   - Error handling and user feedback
   - Loading states and button management

5. **Phase 5: Testing**
   - Unit tests for rate limiting logic
   - Unit tests for TOTP validation
   - Integration tests for endpoint
   - E2E tests for full flow
   - Test on Linux (systemd) agent
   - Test on macOS (launchd) agent
   - Test rate limiting (5 attempts)
   - Test audit logging
   - Test error cases (offline, invalid TOTP, rate limit)

---

## Testing Checklist

### Backend Tests

- [ ] TOTP validation (valid code)
- [ ] TOTP validation (invalid code)
- [ ] TOTP validation (missing code)
- [ ] TOTP not configured (returns 403)
- [ ] Rate limiting (5th attempt succeeds, 6th fails)
- [ ] Rate limiting (rolling window - old attempts don't count)
- [ ] Host offline (returns 409)
- [ ] Host not found (returns 404)
- [ ] Agent send buffer full (returns 503)
- [ ] CSRF token validation
- [ ] Session validation
- [ ] Audit logging (successful attempt)
- [ ] Audit logging (failed attempt)

### Agent Tests

- [ ] Reboot command received
- [ ] WebSocket message sent before reboot
- [ ] Platform-specific command (Linux)
- [ ] Platform-specific command (macOS)
- [ ] Reboot works when agent is busy
- [ ] Reboot bypasses command state machine

### Frontend Tests

- [ ] Modal opens on click
- [ ] TOTP input validation (6 digits)
- [ ] Button disabled until valid TOTP
- [ ] Modal closes on Cancel
- [ ] Modal closes on ESC key
- [ ] Modal closes on backdrop click
- [ ] Loading state during request
- [ ] Error handling (invalid TOTP)
- [ ] Error handling (rate limit)
- [ ] Error handling (network error)
- [ ] Success toast notification
- [ ] Menu item disabled when host offline

### Integration Tests

- [ ] Full flow: Click → Modal → TOTP → Reboot → Agent receives
- [ ] Rate limiting across multiple requests
- [ ] Audit log entries created
- [ ] Database cleanup of old rate limit entries
