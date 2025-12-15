#!/bin/bash
# T-P4370: Table Columns - Automated Verification
# Verifies that table column features are properly implemented

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NIXFLEET_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
BASE_TEMPL="$NIXFLEET_DIR/v2/internal/templates/base.templ"
DASHBOARD_TEMPL="$NIXFLEET_DIR/v2/internal/templates/dashboard.templ"

echo "=== T-P4370: Table Columns Test ==="
echo ""

PASS=0
FAIL=0

# === TABLE STRUCTURE ===
echo "--- Table Structure ---"
echo ""

echo "Checking host-table class..."
if grep -q "host-table" "$BASE_TEMPL"; then
  echo "  [PASS] host-table CSS present"
  ((PASS++))
else
  echo "  [FAIL] host-table CSS NOT FOUND"
  ((FAIL++))
fi

# === HEARTBEAT RIPPLE ===
echo ""
echo "--- Heartbeat Ripple Animation ---"
echo ""

echo "Checking ripple HTML structure..."
if grep -q "status-ripple" "$DASHBOARD_TEMPL" && grep -q "hb-wave" "$DASHBOARD_TEMPL" && grep -q "hb-core" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Ripple HTML structure present"
  ((PASS++))
else
  echo "  [FAIL] Ripple HTML structure NOT FOUND"
  ((FAIL++))
fi

echo "Checking ripple CSS animation..."
if grep -q "status-ripple" "$BASE_TEMPL" && grep -q "@keyframes" "$BASE_TEMPL"; then
  echo "  [PASS] Ripple CSS animation present"
  ((PASS++))
else
  echo "  [FAIL] Ripple CSS animation NOT FOUND"
  ((FAIL++))
fi

# === OFFLINE OVERLAY ===
echo ""
echo "--- Offline Host Overlay ---"
echo ""

echo "Checking offline host class..."
if grep -q "host-offline" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] host-offline class used in templates"
  ((PASS++))
else
  echo "  [FAIL] host-offline class NOT FOUND in templates"
  ((FAIL++))
fi

echo "Checking offline opacity CSS..."
if grep -q "host-offline" "$BASE_TEMPL" && grep -q "opacity" "$BASE_TEMPL"; then
  echo "  [PASS] Offline opacity CSS present"
  ((PASS++))
else
  echo "  [FAIL] Offline opacity CSS NOT FOUND"
  ((FAIL++))
fi

# === METRICS COLUMN ===
echo ""
echo "--- Metrics Column ---"
echo ""

echo "Checking metrics cell structure..."
if grep -q "metrics-cell" "$DASHBOARD_TEMPL" && grep -q "icon-cpu" "$DASHBOARD_TEMPL" && grep -q "icon-ram" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] Metrics cell with CPU/RAM icons"
  ((PASS++))
else
  echo "  [FAIL] Metrics cell structure NOT FOUND"
  ((FAIL++))
fi

echo "Checking high metrics warning class..."
if grep -q "metricsClass" "$DASHBOARD_TEMPL" && grep -q "high" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] High metrics warning class present"
  ((PASS++))
else
  echo "  [FAIL] High metrics warning class NOT FOUND"
  ((FAIL++))
fi

echo "Checking metrics hover titles..."
if grep -q "title=" "$DASHBOARD_TEMPL" | grep -q "CPU\|RAM"; then
  echo "  [PASS] Metrics hover titles present"
  ((PASS++))
else
  # Alternative check
  if grep -q 'title={ "CPU' "$DASHBOARD_TEMPL"; then
    echo "  [PASS] Metrics hover titles present"
    ((PASS++))
  else
    echo "  [FAIL] Metrics hover titles NOT FOUND"
    ((FAIL++))
  fi
fi

# === OS TYPE ICONS ===
echo ""
echo "--- OS Type Icons ---"
echo ""

echo "Checking osTypeIcon function..."
if grep -q "osTypeIcon" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] osTypeIcon function exists"
  ((PASS++))
else
  echo "  [FAIL] osTypeIcon function NOT FOUND"
  ((FAIL++))
fi

echo "Checking NixOS icon usage..."
if grep -q 'case "nixos"' "$DASHBOARD_TEMPL" && grep -q "icon-nixos" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] NixOS icon mapping present"
  ((PASS++))
else
  echo "  [FAIL] NixOS icon mapping NOT FOUND"
  ((FAIL++))
fi

echo "Checking macOS icon usage..."
if grep -q 'case "macos"' "$DASHBOARD_TEMPL" && grep -q "icon-apple" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] macOS icon mapping present"
  ((PASS++))
else
  echo "  [FAIL] macOS icon mapping NOT FOUND"
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
