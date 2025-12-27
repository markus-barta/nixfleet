/**
 * StateSync - Client-side State Sync Protocol implementation (CORE-004)
 *
 * Ensures the browser UI is always live with version-based synchronization.
 * Features:
 * - Full state on connect
 * - Incremental deltas
 * - Automatic drift detection via sync beacons
 * - Self-healing resync
 */
class StateSync {
  constructor(options = {}) {
    this.ws = null;
    this.version = 0;
    this.state = null;
    this.isConnected = false;

    // Callbacks
    this.onStateChange = options.onStateChange || (() => {});
    this.onConnectionChange = options.onConnectionChange || (() => {});
    this.onError = options.onError || console.error;

    // Configuration
    this.wsUrl = options.wsUrl || this._getWebSocketUrl();
    this.reconnectDelay = options.reconnectDelay || 1000;
    this.maxReconnectDelay = options.maxReconnectDelay || 30000;
    this.currentReconnectDelay = this.reconnectDelay;
  }

  /**
   * Connect to the WebSocket server.
   */
  connect() {
    if (this.ws) {
      this.ws.close();
    }

    console.log("[StateSync] Connecting to", this.wsUrl);

    this.ws = new WebSocket(this.wsUrl);

    this.ws.onopen = () => {
      console.log("[StateSync] Connected");
      this.isConnected = true;
      this.currentReconnectDelay = this.reconnectDelay; // Reset backoff
      this.onConnectionChange(true);
    };

    this.ws.onclose = (event) => {
      console.log("[StateSync] Disconnected:", event.code, event.reason);
      this.isConnected = false;
      this.onConnectionChange(false);
      this._scheduleReconnect();
    };

    this.ws.onerror = (error) => {
      console.error("[StateSync] WebSocket error:", error);
      this.onError(error);
    };

    this.ws.onmessage = (event) => {
      this._handleMessage(event);
    };
  }

  /**
   * Disconnect from the WebSocket server.
   */
  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
  }

  /**
   * Request full state from server.
   */
  requestFullState() {
    if (!this.isConnected) {
      console.warn("[StateSync] Not connected, cannot request state");
      return;
    }
    this.ws.send(JSON.stringify({ type: "get_state" }));
  }

  /**
   * Dispatch an op to the server.
   * This is the thin frontend's only job - dispatch ops, let server handle logic.
   */
  dispatchOp(opId, hostIds, options = {}) {
    if (!this.isConnected) {
      console.warn("[StateSync] Not connected, cannot dispatch op");
      return Promise.reject(new Error("Not connected"));
    }

    // Send via REST API (ops are stateful, need proper HTTP response)
    return fetch("/api/dispatch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        op: opId,
        hosts: hostIds,
        force: options.force || false,
        totp: options.totp,
      }),
    }).then((res) => {
      if (!res.ok) {
        return res.json().then((data) => Promise.reject(data));
      }
      return res.json();
    });
  }

  /**
   * Get the current state.
   */
  getState() {
    return this.state;
  }

  /**
   * Get the current version.
   */
  getVersion() {
    return this.version;
  }

  // ═══════════════════════════════════════════════════════════════════════════
  // PRIVATE METHODS
  // ═══════════════════════════════════════════════════════════════════════════

  _handleMessage(event) {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch (e) {
      console.error("[StateSync] Failed to parse message:", e);
      return;
    }

    const { type, version, payload } = msg;

    switch (type) {
      case "init":
      case "full_state":
        // Full state replacement
        console.log(
          `[StateSync] ${type}: version ${this.version} -> ${version}`,
        );
        this.state = payload;
        this.version = version;
        this.onStateChange(this.state, "full");
        break;

      case "delta":
        // Incremental change
        if (version !== this.version + 1) {
          console.warn(
            `[StateSync] Version gap: expected ${this.version + 1}, got ${version}`,
          );
          this.requestFullState();
          return;
        }
        this._applyDelta(payload);
        this.version = version;
        this.onStateChange(this.state, "delta", payload);
        break;

      case "sync":
        // Drift detection
        if (version !== this.version) {
          console.warn(
            `[StateSync] Drift detected: local=${this.version}, server=${version}`,
          );
          this.requestFullState();
        }
        break;

      default:
        // Pass through other message types (legacy compatibility)
        this._handleLegacyMessage(msg);
    }
  }

  _applyDelta(change) {
    if (!this.state) {
      console.warn("[StateSync] No state to apply delta to");
      return;
    }

    switch (change.type) {
      case "host_added":
        this.state.hosts = this.state.hosts || [];
        this.state.hosts.push(change.payload);
        break;

      case "host_updated":
        if (this.state.hosts) {
          const idx = this.state.hosts.findIndex((h) => h.id === change.id);
          if (idx !== -1) {
            Object.assign(this.state.hosts[idx], change.fields);
          }
        }
        break;

      case "host_removed":
        if (this.state.hosts) {
          this.state.hosts = this.state.hosts.filter((h) => h.id !== change.id);
        }
        break;

      case "command_started":
        this.state.commands = this.state.commands || [];
        this.state.commands.push(change.payload);
        break;

      case "command_progress":
        if (this.state.commands) {
          const cmd = this.state.commands.find((c) => c.id === change.id);
          if (cmd) {
            Object.assign(cmd, change.fields);
          }
        }
        break;

      case "command_finished":
        if (this.state.commands) {
          const cmd = this.state.commands.find((c) => c.id === change.id);
          if (cmd) {
            Object.assign(cmd, change.fields);
          }
        }
        break;

      case "event":
        this.state.events = this.state.events || [];
        this.state.events.unshift(change.payload);
        // Keep only last 100 events client-side
        if (this.state.events.length > 100) {
          this.state.events.pop();
        }
        break;

      default:
        console.warn("[StateSync] Unknown change type:", change.type);
    }
  }

  _handleLegacyMessage(msg) {
    // Handle legacy message types for backwards compatibility during transition
    // These will be removed once frontend is fully migrated to v3
    const { type, payload } = msg;

    switch (type) {
      case "host_heartbeat":
      case "host_offline":
      case "command_queued":
      case "command_output":
      case "command_complete":
      case "host_status_update":
      case "state_machine_log":
      case "toast":
        // Forward to legacy handlers via custom event
        window.dispatchEvent(
          new CustomEvent("nixfleet-legacy", { detail: msg }),
        );
        break;

      default:
        console.debug("[StateSync] Unknown message type:", type);
    }
  }

  _scheduleReconnect() {
    console.log(
      `[StateSync] Reconnecting in ${this.currentReconnectDelay}ms...`,
    );
    setTimeout(() => {
      this.connect();
    }, this.currentReconnectDelay);

    // Exponential backoff
    this.currentReconnectDelay = Math.min(
      this.currentReconnectDelay * 2,
      this.maxReconnectDelay,
    );
  }

  _getWebSocketUrl() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws`;
  }
}

// Export for use in modules or attach to window for script tags
if (typeof module !== "undefined" && module.exports) {
  module.exports = StateSync;
} else {
  window.StateSync = StateSync;
}
