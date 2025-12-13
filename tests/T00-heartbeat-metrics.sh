#!/usr/bin/env bash
# Test T00: Heartbeat and Metrics
# Description: Test agent registration, heartbeats, and host info display

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASSED=0
FAILED=0

pass() { echo -e "${GREEN}✅${NC} $1"; PASSED=$((PASSED + 1)); }
fail() { echo -e "${RED}❌${NC} $1"; FAILED=$((FAILED + 1)); }
info() { echo -e "${YELLOW}ℹ️${NC} $1"; }
header() { echo -e "${CYAN}$1${NC}"; }

# ════════════════════════════════════════════════════════════════════════════════
# Configuration
# ════════════════════════════════════════════════════════════════════════════════

FLEET_URL="${NIXFLEET_TEST_URL:-https://fleet.barta.cm}"
AGENT_TOKEN="${NIXFLEET_TEST_TOKEN:-}"
TEST_HOST="integration-test-$$"

# Check for token
if [[ -z "$AGENT_TOKEN" ]]; then
    # Try to read from standard locations
    if [[ -f "$HOME/.config/nixfleet/token" ]]; then
        AGENT_TOKEN="$(cat "$HOME/.config/nixfleet/token")"
    else
        echo "Error: NIXFLEET_TEST_TOKEN not set and no token file found"
        echo "Set: export NIXFLEET_TEST_TOKEN='your-token'"
        exit 1
    fi
fi

echo ""
header "╔══════════════════════════════════════════════════════════════════════════════╗"
header "║                    T00: Heartbeat and Metrics Tests                          ║"
header "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""
info "Dashboard: $FLEET_URL"
info "Test Host: $TEST_HOST"
echo ""

# ════════════════════════════════════════════════════════════════════════════════
# Test 1: Dashboard Health Check
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 1: Dashboard Health Check ---"

HEALTH_RESPONSE=$(curl -sf "${FLEET_URL}/health" 2>/dev/null || echo "FAILED")
if [[ "$HEALTH_RESPONSE" == *"healthy"* ]] || [[ "$HEALTH_RESPONSE" == *"ok"* ]]; then
    pass "Dashboard health endpoint responding"
else
    fail "Dashboard health check failed: $HEALTH_RESPONSE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 2: Agent Registration
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 2: Agent Registration ---"

REG_RESPONSE=$(curl -sf -X POST "${FLEET_URL}/api/hosts/${TEST_HOST}/register" \
    -H "Authorization: Bearer ${AGENT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "hostname": "'"${TEST_HOST}"'",
        "host_type": "nixos",
        "location": "cloud",
        "device_type": "server",
        "theme_color": "#769ff0",
        "criticality": "low",
        "current_generation": "test-gen-123",
        "os_version": "NixOS 26.05",
        "nixcfg_hash": "abc1234",
        "agent_hash": "def5678",
        "uptime_seconds": 86400,
        "load_avg": [1.5, 1.2, 0.9]
    }' 2>/dev/null || echo '{"error": "request failed"}')

if [[ "$REG_RESPONSE" == *'"status":"registered"'* ]]; then
    pass "Agent registration successful"
    # Extract per-host token if provided
    PER_HOST_TOKEN=$(echo "$REG_RESPONSE" | grep -o '"agent_token":"[^"]*"' | cut -d'"' -f4 || echo "")
    if [[ -n "$PER_HOST_TOKEN" ]]; then
        info "Per-host token received (${#PER_HOST_TOKEN} chars)"
        AGENT_TOKEN="$PER_HOST_TOKEN"
    fi
else
    fail "Agent registration failed: $REG_RESPONSE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 3: Heartbeat via Poll (metrics sent with registration)
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 3: Repeated Poll (Heartbeat) ---"

# In NixFleet, heartbeats are done via poll endpoint
# Metrics are sent during registration, poll keeps the host alive
POLL2_RESPONSE=$(curl -sf "${FLEET_URL}/api/hosts/${TEST_HOST}/poll" \
    -H "Authorization: Bearer ${AGENT_TOKEN}" \
    2>/dev/null || echo '{"error": "request failed"}')

if [[ "$POLL2_RESPONSE" == *'"command"'* ]]; then
    pass "Second poll successful (heartbeat)"
else
    fail "Second poll failed: $POLL2_RESPONSE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 4: Poll for Commands
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 4: Poll for Commands ---"

POLL_RESPONSE=$(curl -sf "${FLEET_URL}/api/hosts/${TEST_HOST}/poll" \
    -H "Authorization: Bearer ${AGENT_TOKEN}" \
    2>/dev/null || echo '{"error": "request failed"}')

if [[ "$POLL_RESPONSE" == *'"command"'* ]]; then
    COMMAND=$(echo "$POLL_RESPONSE" | grep -o '"command":"[^"]*"' | cut -d'"' -f4 || echo "none")
    pass "Poll successful (command: $COMMAND)"
else
    fail "Poll failed: $POLL_RESPONSE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 5: Re-registration with Updated Metrics
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 5: Re-registration with Updated Metrics ---"

# Re-register with updated metrics (this is how agents update their info)
REREG_RESPONSE=$(curl -sf -X POST "${FLEET_URL}/api/hosts/${TEST_HOST}/register" \
    -H "Authorization: Bearer ${AGENT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{
        "hostname": "'"${TEST_HOST}"'",
        "host_type": "nixos",
        "location": "cloud",
        "device_type": "server",
        "theme_color": "#769ff0",
        "criticality": "low",
        "current_generation": "test-gen-456",
        "os_version": "NixOS 26.05",
        "nixcfg_hash": "fc2b013",
        "agent_hash": "e1af76b",
        "uptime_seconds": 90000,
        "load_avg": [2.0, 1.5, 1.0],
        "tests_passed": 17,
        "tests_total": 17
    }' 2>/dev/null || echo '{"error": "request failed"}')

if [[ "$REREG_RESPONSE" == *'"status":"registered"'* ]]; then
    pass "Re-registration with updated metrics successful"
else
    fail "Re-registration failed: $REREG_RESPONSE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Cleanup: Remove test host
# ════════════════════════════════════════════════════════════════════════════════

header "--- Cleanup ---"
info "Test host '$TEST_HOST' left in dashboard (manual removal if needed)"

# ════════════════════════════════════════════════════════════════════════════════
# SUMMARY
# ════════════════════════════════════════════════════════════════════════════════

echo ""
header "═══════════════════════════════════════════════════════════════════════════════"
echo -e "Passed: ${GREEN}${PASSED}${NC}"
echo -e "Failed: ${RED}${FAILED}${NC}"
header "═══════════════════════════════════════════════════════════════════════════════"

[[ $FAILED -gt 0 ]] && exit 1
exit 0

