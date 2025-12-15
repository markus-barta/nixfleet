#!/bin/bash
# T-P4385: Button States & Locking - Automated Verification
# Verifies that button state logic is properly implemented

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NIXFLEET_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DASHBOARD_TEMPL="$NIXFLEET_DIR/v2/internal/templates/dashboard.templ"
BASE_TEMPL="$NIXFLEET_DIR/v2/internal/templates/base.templ"

echo "=== T-P4385: Button States & Locking Test ==="
echo ""

PASS=0
FAIL=0

# Test 1: applyButtonStates function exists
echo "Checking applyButtonStates function..."
if grep -q "function applyButtonStates" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] applyButtonStates function exists"
  ((PASS++))
else
  echo "  [FAIL] applyButtonStates function NOT FOUND"
  ((FAIL++))
fi

# Test 2: Button disable logic exists
echo "Checking button disable logic..."
if grep -q "btn.disabled" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Button disable logic present"
  ((PASS++))
else
  echo "  [FAIL] Button disable logic NOT FOUND"
  ((FAIL++))
fi

# Test 3: pendingCommand tracking
echo "Checking pendingCommand tracking..."
if grep -q "pendingCommand" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] pendingCommand tracking present"
  ((PASS++))
else
  echo "  [FAIL] pendingCommand tracking NOT FOUND"
  ((FAIL++))
fi

# Test 4: CSS disabled styling
echo "Checking CSS disabled styling..."
if grep -q "\.btn:disabled" "$BASE_TEMPL"; then
  echo "  [PASS] .btn:disabled CSS exists"
  ((PASS++))
else
  echo "  [FAIL] .btn:disabled CSS NOT FOUND"
  ((FAIL++))
fi

# Test 5: Stop button special case (stays enabled)
echo "Checking Stop button special case..."
if grep -q "stop.*disabled.*false\|stop.*enabled\|cmd === 'stop'" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Stop button special case logic present"
  ((PASS++))
else
  # Check for the actual pattern used
  if grep -q "else if (cmd === 'stop')" "$DASHBOARD_TEMPL"; then
    echo "  [PASS] Stop button special case logic present (alternative pattern)"
    ((PASS++))
  else
    echo "  [WARN] Stop button special case - checking alternative patterns..."
    if grep -A5 "'stop'" "$DASHBOARD_TEMPL" | grep -q "disabled.*false"; then
      echo "  [PASS] Stop button keeps enabled"
      ((PASS++))
    else
      echo "  [FAIL] Stop button special case NOT FOUND"
      ((FAIL++))
    fi
  fi
fi

# Test 6: setHostBusy function for immediate feedback
echo "Checking immediate feedback (setHostBusy)..."
if grep -q "setHostBusy\|setHostBusy" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] setHostBusy function exists"
  ((PASS++))
else
  echo "  [FAIL] setHostBusy function NOT FOUND"
  ((FAIL++))
fi

# Test 7: CommandButton disabled state handling
echo "Checking CommandButton disabled parameter..."
if grep -q "templ CommandButton.*enabled bool" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] CommandButton has enabled parameter"
  ((PASS++))
else
  echo "  [FAIL] CommandButton enabled parameter NOT FOUND"
  ((FAIL++))
fi

echo ""
echo "==================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==================================="

if [ $FAIL -eq 0 ]; then
  echo "TEST PASSED"
  exit 0
else
  echo "TEST FAILED"
  exit 1
fi
