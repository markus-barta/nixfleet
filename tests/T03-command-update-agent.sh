#!/usr/bin/env bash
# Test T03: Update Agent Command
# Description: Test agent self-update via dashboard (flake update + switch)

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

AGENT_SCRIPT="${REPO_ROOT}/agent/nixfleet-agent.sh"

echo ""
header "╔══════════════════════════════════════════════════════════════════════════════╗"
header "║                    T03: Update Agent Command Tests                           ║"
header "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""
info "Testing agent self-update functionality"
echo ""

# ════════════════════════════════════════════════════════════════════════════════
# Test 1: Agent has do_update function
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 1: Agent has do_update function ---"

if grep -q "^do_update()" "$AGENT_SCRIPT" 2>/dev/null; then
    pass "do_update() function defined in agent"
else
    fail "do_update() function not found in agent"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 2: Update function updates nixfleet flake input
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 2: Update function updates nixfleet flake input ---"

if grep -A20 "^do_update()" "$AGENT_SCRIPT" | grep -q "nix flake update nixfleet"; then
    pass "do_update() updates nixfleet flake input"
else
    fail "do_update() doesn't update nixfleet input"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 3: Update function commits changes
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 3: Update function commits changes ---"

if grep -A30 "^do_update()" "$AGENT_SCRIPT" | grep -q "git commit"; then
    pass "do_update() commits flake.lock changes"
else
    fail "do_update() doesn't commit changes"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 4: Update function pushes to remote
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 4: Update function pushes to remote ---"

if grep -A40 "^do_update()" "$AGENT_SCRIPT" | grep -q "git push"; then
    pass "do_update() pushes to remote"
else
    fail "do_update() doesn't push to remote"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 5: Update function calls switch
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 5: Update function calls switch ---"

if grep -A50 "^do_update()" "$AGENT_SCRIPT" | grep -q "do_switch"; then
    pass "do_update() calls do_switch after update"
else
    fail "do_update() doesn't call switch"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 6: Update function reports status
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 6: Update function reports status ---"

if grep -A60 "^do_update()" "$AGENT_SCRIPT" | grep -q "report_status"; then
    pass "do_update() reports status to dashboard"
else
    fail "do_update() doesn't report status"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 7: Update command is handled in main loop
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 7: Update command handled in main loop ---"

if grep -q 'update)' "$AGENT_SCRIPT"; then
    pass "Main loop handles 'update' command"
else
    fail "Main loop doesn't handle 'update' command"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 8: Dashboard supports update command
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 8: Dashboard supports update command ---"

DASHBOARD_MAIN="${REPO_ROOT}/app/main.py"
if grep -q '"update"' "$DASHBOARD_MAIN" 2>/dev/null; then
    pass "Dashboard main.py includes 'update' command"
else
    fail "Dashboard doesn't support 'update' command"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 9: UI has Update Agent button
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 9: UI has Update Agent button ---"

DASHBOARD_HTML="${REPO_ROOT}/app/templates/dashboard.html"
if grep -q "Update Agent" "$DASHBOARD_HTML" 2>/dev/null; then
    pass "Dashboard UI has 'Update Agent' button"
else
    fail "Dashboard UI missing 'Update Agent' button"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 10: Verify nixfleet flake input exists in nixcfg
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 10: nixfleet flake input in nixcfg ---"

NIXCFG_PATH="${NIXFLEET_NIXCFG:-$HOME/Code/nixcfg}"
if [[ -f "$NIXCFG_PATH/flake.nix" ]]; then
    if grep -q "nixfleet" "$NIXCFG_PATH/flake.nix"; then
        pass "nixcfg flake.nix has nixfleet input"
    else
        fail "nixcfg flake.nix missing nixfleet input"
    fi
else
    info "Skipping: $NIXCFG_PATH/flake.nix not found"
    pass "Skipped: nixcfg not available"
fi

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

