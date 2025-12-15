#!/bin/bash
# Run all automated verification tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "============================================"
echo "NixFleet v2 - Automated Verification Tests"
echo "============================================"
echo ""

TOTAL_PASS=0
TOTAL_FAIL=0
TESTS_RUN=0

for test in "$SCRIPT_DIR"/T-*.sh; do
  if [ -f "$test" ]; then
    echo ""
    echo "Running: $(basename "$test")"
    echo "--------------------------------------------"

    if bash "$test"; then
      ((TOTAL_PASS++))
    else
      ((TOTAL_FAIL++))
    fi
    ((TESTS_RUN++))

    echo ""
  fi
done

echo "============================================"
echo "SUMMARY: $TESTS_RUN tests, $TOTAL_PASS passed, $TOTAL_FAIL failed"
echo "============================================"

if [ $TOTAL_FAIL -eq 0 ]; then
  echo "ALL TESTS PASSED"
  exit 0
else
  echo "SOME TESTS FAILED"
  exit 1
fi
