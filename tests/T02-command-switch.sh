#!/usr/bin/env bash
# Test T02: Switch Command
# Description: Test nixos-rebuild/home-manager switch command via dashboard

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

pass() {
  echo -e "${GREEN}✅${NC} $1"
  PASSED=$((PASSED + 1))
}
fail() {
  echo -e "${RED}❌${NC} $1"
  FAILED=$((FAILED + 1))
}
info() { echo -e "${YELLOW}ℹ️${NC} $1"; }
header() { echo -e "${CYAN}$1${NC}"; }

# ════════════════════════════════════════════════════════════════════════════════
# Configuration
# ════════════════════════════════════════════════════════════════════════════════

AGENT_SCRIPT="${REPO_ROOT}/agent/nixfleet-agent.sh"

echo ""
header "╔══════════════════════════════════════════════════════════════════════════════╗"
header "║                      T02: Switch Command Tests                               ║"
header "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""
info "Testing agent switch functionality"
echo ""

# ════════════════════════════════════════════════════════════════════════════════
# Test 1: Agent has do_switch function
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 1: Agent has do_switch function ---"

if grep -q "^do_switch()" "$AGENT_SCRIPT" 2>/dev/null; then
  pass "do_switch() function defined in agent"
else
  fail "do_switch() function not found in agent"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 2: Switch detects OS type
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 2: Switch detects OS type ---"

if grep -A30 "^do_switch()" "$AGENT_SCRIPT" | grep -q "HOST_TYPE"; then
  pass "do_switch() checks HOST_TYPE"
else
  fail "do_switch() doesn't check HOST_TYPE"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 3: Switch handles NixOS
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 3: Switch handles NixOS ---"

if grep -A40 "^do_switch()" "$AGENT_SCRIPT" | grep -q "nixos-rebuild"; then
  pass "do_switch() uses nixos-rebuild for NixOS"
else
  fail "do_switch() doesn't handle NixOS"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 4: Switch handles macOS
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 4: Switch handles macOS ---"

if grep -A40 "^do_switch()" "$AGENT_SCRIPT" | grep -q "home-manager"; then
  pass "do_switch() uses home-manager for macOS"
else
  fail "do_switch() doesn't handle macOS"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 5: Switch reports status
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 5: Switch reports status ---"

if grep -A50 "^do_switch()" "$AGENT_SCRIPT" | grep -q "report_status"; then
  pass "do_switch() reports status to dashboard"
else
  fail "do_switch() doesn't report status"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 6: Current system detection
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 6: Current system detection ---"

OS_NAME="$(uname -s)"
if [[ "$OS_NAME" == "Darwin" ]]; then
  info "Running on macOS - would use home-manager"
  if command -v home-manager >/dev/null 2>&1; then
    pass "home-manager available: $(home-manager --version 2>/dev/null | head -1)"
  else
    info "home-manager not in PATH (may be available via nix)"
    pass "Skipped: home-manager check"
  fi
elif [[ "$OS_NAME" == "Linux" ]]; then
  if [[ -f /etc/NIXOS ]]; then
    info "Running on NixOS - would use nixos-rebuild"
    if command -v nixos-rebuild >/dev/null 2>&1; then
      pass "nixos-rebuild available"
    else
      fail "nixos-rebuild not found on NixOS"
    fi
  else
    info "Running on Linux (non-NixOS)"
    pass "Skipped: not NixOS"
  fi
else
  info "Unknown OS: $OS_NAME"
  pass "Skipped: unknown OS"
fi

# ════════════════════════════════════════════════════════════════════════════════
# Test 7: Flake path handling
# ════════════════════════════════════════════════════════════════════════════════

header "--- Test 7: Flake path handling ---"

if grep -A50 "^do_switch()" "$AGENT_SCRIPT" | grep -q "flake"; then
  pass "do_switch() uses flake-based configuration"
else
  fail "do_switch() doesn't use flakes"
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
