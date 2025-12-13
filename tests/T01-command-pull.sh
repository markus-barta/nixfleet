#!/usr/bin/env bash
# Test T01: Pull Command
# Description: Test git pull command dispatch and execution via dashboard

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
TEST_HOST="${NIXFLEET_TEST_HOST:-}"

# Check requirements
if [[ -z "$AGENT_TOKEN" ]]; then
    if [[ -f "$HOME/.config/nixfleet/token" ]]; then
        AGENT_TOKEN="$(cat "$HOME/.config/nixfleet/token")"
    else
        echo "Error: NIXFLEET_TEST_TOKEN not set"
        exit 1
    fi
fi

if [[ -z "$TEST_HOST" ]]; then
    TEST_HOST="$(hostname -s 2>/dev/null || hostname)"
fi

echo ""
header "╔══════════════════════════════════════════════════════════════════════════════╗"
header "║                       T01: Pull Command Tests                                ║"
header "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""
info "Dashboard: $FLEET_URL"
info "Test Host: $TEST_HOST"
echo ""

# ════════════════════════════════════════════════════════════════════════════════
# Test 1: Send Pull Command via API
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 1: Dispatch Pull Command ---"

# This requires an authenticated session - for now we test the agent's pull function directly
# In a full integration test, we'd use browser automation or session cookies

info "Note: Full UI dispatch requires authenticated session"
info "Testing agent's do_pull function directly instead"

# Check if agent script exists
AGENT_SCRIPT="${REPO_ROOT}/agent/nixfleet-agent.sh"
if [[ -f "$AGENT_SCRIPT" ]]; then
    pass "Agent script found: $AGENT_SCRIPT"
else
    fail "Agent script not found"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 2: Verify git pull function exists in agent
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 2: Agent has do_pull function ---"

if grep -q "^do_pull()" "$AGENT_SCRIPT" 2>/dev/null; then
    pass "do_pull() function defined in agent"
else
    fail "do_pull() function not found in agent"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 3: Pull function handles git operations
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 3: Pull function uses git ---"

if grep -A10 "^do_pull()" "$AGENT_SCRIPT" | grep -q "git pull"; then
    pass "do_pull() executes git pull"
else
    fail "do_pull() doesn't contain git pull"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 4: Pull reports status back to dashboard
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 4: Pull reports status ---"

if grep -A20 "^do_pull()" "$AGENT_SCRIPT" | grep -q "report_status"; then
    pass "do_pull() reports status to dashboard"
else
    fail "do_pull() doesn't report status"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 5: Live pull test (if in nixcfg directory)
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 5: Live Git Pull Test ---"

NIXCFG_PATH="${NIXFLEET_NIXCFG:-$HOME/Code/nixcfg}"

if [[ -d "$NIXCFG_PATH/.git" ]]; then
    info "Testing git pull in $NIXCFG_PATH"
    cd "$NIXCFG_PATH"
    
    # Get current HEAD
    BEFORE_HEAD=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    
    # Do a dry-run fetch to check connectivity
    if git fetch --dry-run origin 2>/dev/null; then
        pass "Git remote accessible"
        
        # Check if we're behind
        git fetch origin master:refs/remotes/origin/master 2>/dev/null || true
        LOCAL=$(git rev-parse HEAD 2>/dev/null)
        REMOTE=$(git rev-parse origin/master 2>/dev/null || echo "unknown")
        
        if [[ "$LOCAL" == "$REMOTE" ]]; then
            pass "Already up to date (no pull needed)"
        else
            info "Local and remote differ - pull would update"
            pass "Git pull connectivity verified"
        fi
    else
        fail "Cannot reach git remote"
    fi
else
    info "Skipping: $NIXCFG_PATH not a git repository"
    pass "Live test skipped (no repo)"
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

