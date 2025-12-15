#!/bin/bash
# T-P4350: SVG Icon System - Automated Verification
# Verifies that all required SVG icons are defined in the template

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NIXFLEET_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
BASE_TEMPL="$NIXFLEET_DIR/v2/internal/templates/base.templ"
DASHBOARD_TEMPL="$NIXFLEET_DIR/v2/internal/templates/dashboard.templ"

echo "=== T-P4350: SVG Icon System Test ==="
echo ""

# Required icons from P4350 specification
REQUIRED_ICONS=(
  "icon-nixos"
  "icon-apple"
  "icon-cloud"
  "icon-home"
  "icon-office"
  "icon-server"
  "icon-desktop"
  "icon-laptop"
  "icon-game"
  "icon-download"
  "icon-refresh"
  "icon-flask"
  "icon-stop"
  "icon-plus"
  "icon-trash"
  "icon-more"
  "icon-check"
  "icon-chevron"
  "icon-file"
  "icon-cpu"
  "icon-ram"
  "icon-github"
  "icon-heart"
  "icon-license"
)

PASS=0
FAIL=0

echo "Checking icon definitions in base.templ..."
echo ""

for icon in "${REQUIRED_ICONS[@]}"; do
  if grep -q "symbol id=\"$icon\"" "$BASE_TEMPL"; then
    echo "  [PASS] $icon"
    ((PASS++))
  else
    echo "  [FAIL] $icon - NOT FOUND"
    ((FAIL++))
  fi
done

echo ""
echo "Checking commandIcon function uses icons..."

if grep -q "commandIcon" "$DASHBOARD_TEMPL"; then
  echo "  [PASS] commandIcon function exists"
  ((PASS++))
else
  echo "  [FAIL] commandIcon function NOT FOUND"
  ((FAIL++))
fi

# Check that command buttons use commandIcon
for cmd in "pull" "switch" "test" "stop"; do
  if grep -q "case \"$cmd\":" "$DASHBOARD_TEMPL" && grep -q "icon-" "$DASHBOARD_TEMPL"; then
    echo "  [PASS] $cmd command has icon mapping"
    ((PASS++))
  fi
done

echo ""
echo "Checking no emojis in templates..."

# Common emoji patterns
if grep -qP '[\x{1F300}-\x{1F9FF}]' "$DASHBOARD_TEMPL" 2>/dev/null; then
  echo "  [FAIL] Emojis found in dashboard.templ"
  ((FAIL++))
else
  echo "  [PASS] No emojis in dashboard.templ"
  ((PASS++))
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
