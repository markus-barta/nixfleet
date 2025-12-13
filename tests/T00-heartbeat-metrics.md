# T00: Heartbeat and Metrics

Test agent registration, heartbeats, and host information display in the dashboard.

## Prerequisites

- NixFleet dashboard running and accessible
- Valid agent token (shared or per-host)
- Network connectivity to dashboard URL

## What This Test Verifies

| Field         | Source              | Displayed In        |
| ------------- | ------------------- | ------------------- |
| Hostname      | Agent registration  | Host column         |
| OS Type       | Agent registration  | OS icon             |
| OS Version    | Agent heartbeat     | OS column (tooltip) |
| Location      | Agent registration  | Location column     |
| Device Type   | Agent registration  | Type column         |
| Theme Color   | Agent registration  | Row styling         |
| Config Hash   | Agent heartbeat     | Config column       |
| Agent Hash    | Agent heartbeat     | Agent column        |
| Uptime        | Agent heartbeat     | Uptime column       |
| Load Averages | Agent heartbeat     | (metrics storage)   |
| Tests Passed  | Agent heartbeat     | Tests column        |
| Tests Total   | Agent heartbeat     | Tests column        |
| Last Seen     | Heartbeat timestamp | Seen column         |

## Manual Test Procedures

### Test 1: Dashboard Health Check

**Steps:**

1. Check dashboard health endpoint:

   ```bash
   curl -s https://fleet.barta.cm/health
   ```

**Expected Results:**

- Returns `{"status": "ok"}` or similar
- HTTP 200 response

**Status:** ⏳ Pending

### Test 2: Agent Registration

**Steps:**

1. Register a test host:

   ```bash
   curl -X POST https://fleet.barta.cm/api/hosts/test-host/register \
     -H "Authorization: Bearer $NIXFLEET_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "hostname": "test-host",
       "host_type": "nixos",
       "location": "home",
       "device_type": "server",
       "theme_color": "#769ff0",
       "criticality": "low",
       "current_generation": "gen123"
     }'
   ```

2. Verify host appears in dashboard

**Expected Results:**

- Returns `{"status": "registered", "host_id": "test-host", ...}`
- Per-host token provided if auto-provisioning enabled
- Host visible in dashboard table

**Status:** ⏳ Pending

### Test 3: Heartbeat with Metrics

**Steps:**

1. Send heartbeat with metrics:

   ```bash
   curl -X POST https://fleet.barta.cm/api/hosts/test-host/heartbeat \
     -H "Authorization: Bearer $PER_HOST_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "uptime_seconds": 86400,
       "load_avg": [1.5, 1.2, 0.9],
       "nixcfg_hash": "abc1234",
       "agent_hash": "def5678",
       "tests_passed": 15,
       "tests_total": 17
     }'
   ```

2. Check dashboard shows updated metrics

**Expected Results:**

- Returns `{"status": "ok"}`
- Dashboard shows:
  - Uptime: "1d 0h"
  - Config: "abc1234" with sync indicator
  - Agent: "def5678" with sync indicator
  - Tests: "15/17"
  - Seen: "just now"

**Status:** ⏳ Pending

### Test 4: Poll for Commands

**Steps:**

1. Poll for pending commands:

   ```bash
   curl https://fleet.barta.cm/api/hosts/test-host/poll \
     -H "Authorization: Bearer $PER_HOST_TOKEN"
   ```

**Expected Results:**

- Returns `{"command": "none"}` or pending command
- HTTP 200 response

**Status:** ⏳ Pending

### Test 5: Host Info Display Verification

**Steps:**

1. Open dashboard in browser
2. Locate test host row
3. Verify all columns display correctly:
   - Host name with theme color
   - OS icon (NixOS snowflake or macOS apple)
   - Location icon (home/work/cloud)
   - Device type icon (server/desktop/laptop/gaming)
   - Uptime in human-readable format
   - Config hash with sync status (✓/↓)
   - Agent hash with sync status (✓/↓)
   - Last seen timestamp
   - Test results (passed/total)

**Expected Results:**

- All fields populated correctly
- Icons match host type
- Sync indicators accurate vs SOT hashes

**Status:** ⏳ Pending

## Summary

- Total Tests: 5
- Passed: 0
- Pending: 5

## API Endpoints Tested

| Endpoint                         | Method | Purpose            |
| -------------------------------- | ------ | ------------------ |
| `/health`                        | GET    | Dashboard health   |
| `/api/hosts/{host_id}/register`  | POST   | Agent registration |
| `/api/hosts/{host_id}/heartbeat` | POST   | Metrics update     |
| `/api/hosts/{host_id}/poll`      | GET    | Command polling    |

## Related

- Automated: [T00-heartbeat-metrics.sh](./T00-heartbeat-metrics.sh)
- Agent: [nixfleet-agent.sh](../agent/nixfleet-agent.sh)
- Dashboard: [main.py](../app/main.py)
