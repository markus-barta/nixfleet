#!/bin/bash
# T-P4400: Test stale command cleanup for offline hosts
# Verifies PRD FR-2.13: Clear stale pending_command for offline hosts

set -e

PASS=0
FAIL=0

echo "=== T-P4400: Stale Command Cleanup Tests ==="
echo ""

# Test 1: cleanupStaleCommands function exists
echo "[TEST] cleanupStaleCommands function exists"
if grep -q "func (h \*Hub) cleanupStaleCommands()" v2/internal/dashboard/hub.go; then
  echo "  [PASS] cleanupStaleCommands function found"
  ((PASS++))
else
  echo "  [FAIL] cleanupStaleCommands function NOT FOUND"
  ((FAIL++))
fi

# Test 2: staleCommandCleanupLoop exists and is started
echo "[TEST] staleCommandCleanupLoop exists and is started"
if grep -q "func (h \*Hub) staleCommandCleanupLoop" v2/internal/dashboard/hub.go; then
  echo "  [PASS] staleCommandCleanupLoop function found"
  ((PASS++))
else
  echo "  [FAIL] staleCommandCleanupLoop function NOT FOUND"
  ((FAIL++))
fi

if grep -q "go h.staleCommandCleanupLoop(ctx)" v2/internal/dashboard/hub.go; then
  echo "  [PASS] staleCommandCleanupLoop is started in Run()"
  ((PASS++))
else
  echo "  [FAIL] staleCommandCleanupLoop NOT started in Run()"
  ((FAIL++))
fi

# Test 3: Config has stale detection fields
echo "[TEST] Config has stale detection fields"
if grep -q "StaleMultiplier" v2/internal/dashboard/config.go; then
  echo "  [PASS] StaleMultiplier config field exists"
  ((PASS++))
else
  echo "  [FAIL] StaleMultiplier config field NOT FOUND"
  ((FAIL++))
fi

if grep -q "StaleMinimum" v2/internal/dashboard/config.go; then
  echo "  [PASS] StaleMinimum config field exists"
  ((PASS++))
else
  echo "  [FAIL] StaleMinimum config field NOT FOUND"
  ((FAIL++))
fi

if grep -q "StaleCommandTimeout" v2/internal/dashboard/config.go; then
  echo "  [PASS] StaleCommandTimeout method exists"
  ((PASS++))
else
  echo "  [FAIL] StaleCommandTimeout method NOT FOUND"
  ((FAIL++))
fi

# Test 4: Cleanup targets ONLY offline hosts
echo "[TEST] Cleanup targets ONLY offline hosts"
if grep -q "status = 'offline'" v2/internal/dashboard/hub.go; then
  echo "  [PASS] Cleanup SQL filters by offline status"
  ((PASS++))
else
  echo "  [FAIL] Cleanup SQL does NOT filter by offline status"
  ((FAIL++))
fi

# Test 5: Cleanup broadcasts to browsers
echo "[TEST] Cleanup broadcasts updates to browsers"
if grep -A 70 "func (h \*Hub) cleanupStaleCommands" v2/internal/dashboard/hub.go | grep -q "queueBroadcast"; then
  echo "  [PASS] Cleanup broadcasts host_update to browsers"
  ((PASS++))
else
  echo "  [FAIL] Cleanup does NOT broadcast updates"
  ((FAIL++))
fi

# Test 6: Panic recovery in cleanup loop
echo "[TEST] Cleanup loop has panic recovery"
if grep -A 5 "staleCommandCleanupLoop" v2/internal/dashboard/hub.go | grep -q "recover()"; then
  echo "  [PASS] Cleanup loop has panic recovery"
  ((PASS++))
else
  echo "  [FAIL] Cleanup loop does NOT have panic recovery"
  ((FAIL++))
fi

# Test 7: Environment variable parsing
echo "[TEST] Environment variables are parsed"
if grep -q "NIXFLEET_STALE_MULTIPLIER" v2/internal/dashboard/config.go; then
  echo "  [PASS] NIXFLEET_STALE_MULTIPLIER env var parsed"
  ((PASS++))
else
  echo "  [FAIL] NIXFLEET_STALE_MULTIPLIER env var NOT parsed"
  ((FAIL++))
fi

if grep -q "NIXFLEET_STALE_MINIMUM" v2/internal/dashboard/config.go; then
  echo "  [PASS] NIXFLEET_STALE_MINIMUM env var parsed"
  ((PASS++))
else
  echo "  [FAIL] NIXFLEET_STALE_MINIMUM env var NOT parsed"
  ((FAIL++))
fi

# Summary
echo ""
echo "=== Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo ""

if [ $FAIL -eq 0 ]; then
  echo "✅ All tests passed!"
  exit 0
else
  echo "❌ Some tests failed!"
  exit 1
fi
