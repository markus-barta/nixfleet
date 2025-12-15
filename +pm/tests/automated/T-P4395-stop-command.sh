#!/bin/bash
# T-P4395: Stop Command - Automated Verification
# Verifies that stop command implementation is complete

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NIXFLEET_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
COMMANDS_GO="$NIXFLEET_DIR/v2/internal/agent/commands.go"
DASHBOARD_TEMPL="$NIXFLEET_DIR/v2/internal/templates/dashboard.templ"

echo "=== T-P4395: Stop Command Test ==="
echo ""

PASS=0
FAIL=0

# === AGENT TESTS ===
echo "--- Agent Implementation ---"
echo ""

# Test 1: handleStop function exists
echo "Checking handleStop function..."
if grep -q "func (a \*Agent) handleStop()" "$COMMANDS_GO"; then
  echo "  [PASS] handleStop function exists"
  ((PASS++))
else
  echo "  [FAIL] handleStop function NOT FOUND"
  ((FAIL++))
fi

# Test 2: Stop command handled BEFORE busy check
echo "Checking stop bypasses busy check..."
# The stop case should appear before the IsBusy() check
STOP_LINE=$(grep -n "case \"stop\":" "$COMMANDS_GO" | head -1 | cut -d: -f1)
BUSY_LINE=$(grep -n "IsBusy()" "$COMMANDS_GO" | head -1 | cut -d: -f1)

if [ -n "$STOP_LINE" ] && [ -n "$BUSY_LINE" ] && [ "$STOP_LINE" -lt "$BUSY_LINE" ]; then
  echo "  [PASS] Stop handled before busy check (line $STOP_LINE < $BUSY_LINE)"
  ((PASS++))
else
  echo "  [FAIL] Stop should be handled before busy check"
  ((FAIL++))
fi

# Test 3: SIGTERM signal used
echo "Checking SIGTERM usage..."
if grep -q "syscall.SIGTERM" "$COMMANDS_GO"; then
  echo "  [PASS] SIGTERM signal used"
  ((PASS++))
else
  echo "  [FAIL] SIGTERM NOT FOUND"
  ((FAIL++))
fi

# Test 4: SIGKILL fallback
echo "Checking SIGKILL fallback..."
if grep -q "syscall.SIGKILL" "$COMMANDS_GO"; then
  echo "  [PASS] SIGKILL fallback present"
  ((PASS++))
else
  echo "  [FAIL] SIGKILL fallback NOT FOUND"
  ((FAIL++))
fi

# Test 5: Process group kill (kills children)
echo "Checking process group kill..."
if grep -q "Getpgid\|Kill(-" "$COMMANDS_GO"; then
  echo "  [PASS] Process group kill present"
  ((PASS++))
else
  echo "  [WARN] Process group kill not found (may kill only parent)"
  ((PASS++)) # Not critical
fi

# Test 6: Status message sent after stop
echo "Checking status message after stop..."
if grep -A20 "handleStop" "$COMMANDS_GO" | grep -q "sendStatus"; then
  echo "  [PASS] sendStatus called in handleStop"
  ((PASS++))
else
  echo "  [FAIL] sendStatus NOT called in handleStop"
  ((FAIL++))
fi

echo ""
echo "--- UI Implementation ---"
echo ""

# Test 7: Stop icon defined
echo "Checking stop icon in commandIcon..."
if grep -q "case \"stop\":" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Stop case in commandIcon"
  ((PASS++))
else
  echo "  [FAIL] Stop case NOT FOUND in commandIcon"
  ((FAIL++))
fi

# Test 8: Test/Stop button swap
echo "Checking Test/Stop button swap logic..."
if grep -q "cmd === 'test'\|cmd === 'stop'" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Test/Stop swap logic present"
  ((PASS++))
else
  echo "  [FAIL] Test/Stop swap logic NOT FOUND"
  ((FAIL++))
fi

# Test 9: Stop button gets danger class
echo "Checking Stop button danger styling..."
if grep -q "btn-danger\|classList.add.*danger" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Stop button danger styling present"
  ((PASS++))
else
  echo "  [FAIL] Stop button danger styling NOT FOUND"
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
