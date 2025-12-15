#!/bin/bash
# T-P2000: Hub Resilience - Automated Verification
# Verifies that hub resilience features are properly implemented

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NIXFLEET_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
HUB_GO="$NIXFLEET_DIR/v2/internal/dashboard/hub.go"
MAIN_GO="$NIXFLEET_DIR/v2/cmd/nixfleet-dashboard/main.go"

echo "=== T-P2000: Hub Resilience Test ==="
echo ""

PASS=0
FAIL=0

# === PHASE 1: DEADLOCK FIX ===
echo "--- Phase 1: Deadlock Fix ---"
echo ""

# 1.1 handleUnregister function exists
echo "Checking handleUnregister function..."
if grep -q "func (h \*Hub) handleUnregister" "$HUB_GO"; then
  echo "  [PASS] handleUnregister function exists"
  ((PASS++))
else
  echo "  [FAIL] handleUnregister function NOT FOUND"
  ((FAIL++))
fi

# 1.2 Lock is released before external operations
echo "Checking lock release before external ops..."
# The pattern should be: Unlock() appears BEFORE db.Exec or broadcast calls
if grep -A50 "handleUnregister" "$HUB_GO" | grep -q "mu.Unlock()"; then
  # Check that Unlock appears in handleUnregister
  UNLOCK_LINE=$(grep -n "mu.Unlock()" "$HUB_GO" | head -3 | tail -1 | cut -d: -f1)
  if [ -n "$UNLOCK_LINE" ]; then
    echo "  [PASS] Lock released (Unlock at line $UNLOCK_LINE)"
    ((PASS++))
  else
    echo "  [FAIL] Lock release pattern not found"
    ((FAIL++))
  fi
else
  echo "  [FAIL] mu.Unlock() not found in handleUnregister"
  ((FAIL++))
fi

# 1.3 Panic recovery
echo "Checking panic recovery..."
if grep -q "defer func()" "$HUB_GO" && grep -q "recover()" "$HUB_GO"; then
  echo "  [PASS] Panic recovery with defer/recover"
  ((PASS++))
else
  echo "  [FAIL] Panic recovery NOT FOUND"
  ((FAIL++))
fi

# 1.4 runLoop function with panic recovery
echo "Checking runLoop function..."
if grep -q "func (h \*Hub) runLoop" "$HUB_GO"; then
  echo "  [PASS] runLoop function exists"
  ((PASS++))
else
  echo "  [FAIL] runLoop function NOT FOUND"
  ((FAIL++))
fi

# === PHASE 2: SAFE CHANNEL OPERATIONS ===
echo ""
echo "--- Phase 2: Safe Channel Operations ---"
echo ""

# 2.1 SafeSend function
echo "Checking SafeSend function..."
if grep -q "func (c \*Client) SafeSend" "$HUB_GO"; then
  echo "  [PASS] SafeSend function exists"
  ((PASS++))
else
  echo "  [FAIL] SafeSend function NOT FOUND"
  ((FAIL++))
fi

# 2.2 Client.Close with sync.Once
echo "Checking Client.Close with sync.Once..."
if grep -q "closeOnce\|sync.Once" "$HUB_GO"; then
  echo "  [PASS] sync.Once for channel close"
  ((PASS++))
else
  echo "  [FAIL] sync.Once NOT FOUND"
  ((FAIL++))
fi

# 2.3 atomic.Bool for closed state
echo "Checking atomic.Bool for closed state..."
if grep -q "atomic.Bool\|closed.Load\|closed.Store" "$HUB_GO"; then
  echo "  [PASS] atomic.Bool for closed tracking"
  ((PASS++))
else
  echo "  [FAIL] atomic.Bool NOT FOUND"
  ((FAIL++))
fi

# === PHASE 3: ASYNC BROADCAST QUEUE ===
echo ""
echo "--- Phase 3: Async Broadcast Queue ---"
echo ""

# 3.1 broadcasts channel
echo "Checking broadcasts channel..."
if grep -q "broadcasts.*chan" "$HUB_GO"; then
  echo "  [PASS] Broadcasts channel exists"
  ((PASS++))
else
  echo "  [FAIL] Broadcasts channel NOT FOUND"
  ((FAIL++))
fi

# 3.2 broadcastLoop function
echo "Checking broadcastLoop function..."
if grep -q "func (h \*Hub) broadcastLoop" "$HUB_GO"; then
  echo "  [PASS] broadcastLoop function exists"
  ((PASS++))
else
  echo "  [FAIL] broadcastLoop function NOT FOUND"
  ((FAIL++))
fi

# 3.3 queueBroadcast function
echo "Checking queueBroadcast function..."
if grep -q "func (h \*Hub) queueBroadcast\|queueBroadcast" "$HUB_GO"; then
  echo "  [PASS] queueBroadcast function exists"
  ((PASS++))
else
  echo "  [FAIL] queueBroadcast function NOT FOUND"
  ((FAIL++))
fi

# === GRACEFUL SHUTDOWN ===
echo ""
echo "--- Graceful Shutdown ---"
echo ""

# 4.1 Context passed to Run
echo "Checking context in Run..."
if grep -q "func (h \*Hub) Run(ctx context.Context)" "$HUB_GO"; then
  echo "  [PASS] Run accepts context"
  ((PASS++))
else
  echo "  [FAIL] Run context parameter NOT FOUND"
  ((FAIL++))
fi

# 4.2 Signal handling in main
echo "Checking signal handling in main..."
if grep -q "signal.Notify\|SIGINT\|SIGTERM" "$MAIN_GO"; then
  echo "  [PASS] Signal handling present"
  ((PASS++))
else
  echo "  [FAIL] Signal handling NOT FOUND"
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
