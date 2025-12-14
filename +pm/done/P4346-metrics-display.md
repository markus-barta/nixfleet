# P4346 - Display Host Metrics (CPU/RAM/Load)

**Priority**: Medium  
**Status**: Done  
**Effort**: Small  
**References**: PRD FR-1.7, FR-3.7, US-4  
**Completed**: 2025-12-14

## Problem

Agent sends metrics in heartbeat, but dashboard ignores them:

- CPU usage (StaSysMo integration)
- RAM usage
- Swap usage
- Load average

PRD FR-3.7: "Show host metrics (CPU, RAM)" - **Should** priority

## Current State

Agent sends:

```json
{
  "type": "heartbeat",
  "payload": {
    "metrics": {
      "cpu": 45,
      "ram": 72,
      "swap": 5,
      "load": 1.25
    }
  }
}
```

Dashboard receives but doesn't display.

## Solution

### 1. Store Metrics in Database

```sql
ALTER TABLE hosts ADD COLUMN metrics_json TEXT;
ALTER TABLE hosts ADD COLUMN metrics_updated_at TIMESTAMP;
```

### 2. Update Hub to Store Metrics

```go
func (h *Hub) handleHeartbeat(msg protocol.Message, conn *AgentConn) {
    // ... existing code ...

    if payload.Metrics != nil {
        metricsJSON, _ := json.Marshal(payload.Metrics)
        h.db.Exec(
            "UPDATE hosts SET metrics_json = ?, metrics_updated_at = ? WHERE id = ?",
            metricsJSON, time.Now(), conn.HostID,
        )
    }
}
```

### 3. Pass to Template

```go
type Host struct {
    // ... existing fields ...
    Metrics *Metrics
}

type Metrics struct {
    CPU  int     `json:"cpu"`
    RAM  int     `json:"ram"`
    Swap int     `json:"swap"`
    Load float64 `json:"load"`
}
```

### 4. Display in UI

```html
<td class="metrics-cell">
  if host.Metrics != nil {
    <span class={ "metric cpu", templ.KV("high", host.Metrics.CPU >= 80) }>
      <svg class="metric-icon"><use href="#icon-cpu"/></svg>
      { strconv.Itoa(host.Metrics.CPU) }%
    </span>
    <span class={ "metric ram", templ.KV("high", host.Metrics.RAM >= 80) }>
      <svg class="metric-icon"><use href="#icon-ram"/></svg>
      { strconv.Itoa(host.Metrics.RAM) }%
    </span>
  } else {
    <span class="metrics-na">—</span>
  }
</td>
```

### 5. Broadcast via WebSocket

Include metrics in `host_update` message to browsers.

### Requirements

- [x] Add metrics columns to hosts table (metrics_json)
- [x] Store metrics on heartbeat
- [x] Load metrics when rendering dashboard
- [x] Display CPU/RAM with icons (🖥️ 💾)
- [x] Highlight high usage (≥80%) in red
- [x] Show full details (swap, load) on hover (via title tooltip)
- [x] Broadcast metrics updates to browsers (already done via host_update)

## Related

- P4350 (Icons) - CPU/RAM icons
- P4370 (Table Columns) - Metrics column
- T02 (Heartbeat) - Metrics in payload
