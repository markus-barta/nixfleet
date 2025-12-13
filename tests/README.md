# NixFleet Test Suite

Integration tests for the NixFleet dashboard and agent.

## Quick Stats

- **Total Tests**: 4 integration tests
- **Unit Tests**: 1 (security)

## Test List

| Test ID | Feature           | 🤖 Auto Last Run | Notes                                      |
| ------- | ----------------- | ---------------- | ------------------------------------------ |
| T00     | Heartbeat/Metrics | ✅ 2025-12-13    | Registration, heartbeat, host info display |
| T01     | Pull Command      | ✅ 2025-12-13    | Git pull via dashboard command             |
| T02     | Switch Command    | ✅ 2025-12-13    | NixOS/home-manager switch via dashboard    |
| T03     | Update Agent      | ✅ 2025-12-13    | Agent self-update via dashboard            |

## Test Types

### 1. Integration Tests (Shell scripts)

**Location:** `tests/T*.sh` with matching `T*.md` documentation

Shell scripts that test the full agent ↔ dashboard flow:

- Agent registration and heartbeats
- Command dispatch and execution
- Status reporting and metrics

```bash
# Run individual test
./tests/T00-heartbeat-metrics.sh

# Run all integration tests
for t in tests/T*.sh; do bash "$t"; done
```

### 2. Unit Tests (Python)

**Location:** `tests/test_*.py`

Python tests using FastAPI TestClient for API endpoint testing:

- Security headers (CSP, CSRF)
- Authentication flows
- Input validation

```bash
# Run Python tests
cd tests && python -m pytest test_security.py -v
```

## Prerequisites

Before running integration tests, ensure:

1. **Dashboard running** at the configured URL
2. **Agent token** available (for authenticated tests)
3. **Git repository** accessible (for pull/switch tests)

### Environment Variables

```bash
export NIXFLEET_TEST_URL="https://fleet.barta.cm"      # Dashboard URL
export NIXFLEET_TEST_TOKEN="your-agent-token"          # Shared agent token
export NIXFLEET_TEST_HOST="test-host"                  # Test hostname
export NIXFLEET_NIXCFG="/path/to/nixcfg"               # nixcfg repo path
```

## Test Naming Convention

```
T##-descriptive-name.sh   # Automated test
T##-descriptive-name.md   # Manual procedures + documentation
```

## Writing Tests

### Shell Test Template

```bash
#!/usr/bin/env bash
# Test T##: Feature Name
# Description: What this test verifies

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

pass() { echo -e "${GREEN}✅${NC} $1"; ((PASSED++)); }
fail() { echo -e "${RED}❌${NC} $1"; ((FAILED++)); }
info() { echo -e "${YELLOW}ℹ️${NC} $1"; }

# ════════════════════════════════════════════════════════════════════════════════
echo "=== T##: Feature Name ==="
echo ""

# Test 1: Description
if [[ some_condition ]]; then
    pass "Test description"
else
    fail "Test description: reason"
fi

# ════════════════════════════════════════════════════════════════════════════════
# SUMMARY
# ════════════════════════════════════════════════════════════════════════════════
echo ""
echo "=== Summary ==="
echo -e "Passed: ${GREEN}${PASSED}${NC}"
echo -e "Failed: ${RED}${FAILED}${NC}"

[[ $FAILED -gt 0 ]] && exit 1
exit 0
```

### Documentation Template (.md)

```markdown
# T##: Feature Name

Test the Feature Name functionality.

## Prerequisites

- List requirements

## Manual Test Procedures

### Test 1: Description

**Steps:**

1. Step one
2. Step two

**Expected Results:**

- Expected output

**Status:** ⏳ Pending

## Summary

- Total Tests: X
- Passed: 0
- Pending: X

## Related

- Automated: [T##-feature-name.sh](./T##-feature-name.sh)
```

## Related Documentation

- [Agent Script](../agent/nixfleet-agent.sh) - Agent implementation
- [Dashboard API](../app/main.py) - API endpoints
- [NixOS Module](../modules/nixos.nix) - NixOS integration
- [Home Manager Module](../modules/home-manager.nix) - macOS integration

---

**Last Updated**: December 13, 2025  
**Maintainer**: Markus Barta
