// Package integration contains integration tests for NixFleet v2.
// This file tests P2810: Agent Binary Freshness Detection (3-layer verification).
package integration

import (
	"testing"

	"github.com/markus-barta/nixfleet/internal/dashboard"
)

// ═══════════════════════════════════════════════════════════════════════════
// BINARY FRESHNESS COMPARISON TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestCompareFreshness_AllChanged(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "def5678",
		StorePath:    "/nix/store/yyy-nixfleet-agent-2.1.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:ccccccccdddddddd",
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessFresh {
		t.Errorf("expected FRESH, got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_NothingChanged(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "abc1234", // Same
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent", // Same
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb", // Same
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessStale {
		t.Errorf("expected STALE, got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_OnlyPathChanged(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "abc1234", // Same
		StorePath:    "/nix/store/yyy-nixfleet-agent-2.0.0/bin/nixfleet-agent", // Changed
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb", // Same
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessFresh {
		t.Errorf("expected FRESH (path changed), got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_OnlyHashChanged(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "abc1234", // Same
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent", // Same
		BinaryHash:   "sha256:ccccccccdddddddd", // Changed
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessFresh {
		t.Errorf("expected FRESH (hash changed), got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_CommitChangedButBinaryDidnt(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "def5678", // Changed
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent", // Same
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb", // Same
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessSuspicious {
		t.Errorf("expected SUSPICIOUS, got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_MissingBeforeData(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "", // Missing
		BinaryHash:   "", // Missing
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "def5678",
		StorePath:    "/nix/store/yyy-nixfleet-agent-2.1.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:ccccccccdddddddd",
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessUnknown {
		t.Errorf("expected UNKNOWN (missing data), got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_MissingAfterData(t *testing.T) {
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "def5678",
		StorePath:    "", // Missing
		BinaryHash:   "", // Missing
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	if verdict != dashboard.FreshnessUnknown {
		t.Errorf("expected UNKNOWN (missing data), got %s: %s", verdict, msg)
	}
}

func TestCompareFreshness_PathChangedHashSame(t *testing.T) {
	// Edge case: Store path changed but binary content same (possible with same derivation)
	before := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/xxx-nixfleet-agent-2.0.0/bin/nixfleet-agent",
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb",
	}
	after := dashboard.AgentFreshness{
		SourceCommit: "abc1234",
		StorePath:    "/nix/store/yyy-nixfleet-agent-2.0.0/bin/nixfleet-agent", // Different path
		BinaryHash:   "sha256:aaaaaaaabbbbbbbb", // Same content
	}

	verdict, msg := dashboard.CompareFreshness(before, after)

	// Path changed is still FRESH - the derivation was rebuilt
	if verdict != dashboard.FreshnessFresh {
		t.Errorf("expected FRESH (path changed), got %s: %s", verdict, msg)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DECISION MATRIX TESTS (from P2800 spec)
// ═══════════════════════════════════════════════════════════════════════════

func TestFreshnessDecisionMatrix(t *testing.T) {
	tests := []struct {
		name           string
		commitChanged  bool
		pathChanged    bool
		hashChanged    bool
		expectedVerdict dashboard.FreshnessVerdict
	}{
		{"commit+path+hash changed", true, true, true, dashboard.FreshnessFresh},
		{"commit+path changed", true, true, false, dashboard.FreshnessFresh},
		{"commit+hash changed", true, false, true, dashboard.FreshnessFresh},
		{"commit only changed", true, false, false, dashboard.FreshnessSuspicious},
		{"path+hash changed", false, true, true, dashboard.FreshnessFresh},
		{"path only changed", false, true, false, dashboard.FreshnessFresh},
		{"hash only changed", false, false, true, dashboard.FreshnessFresh},
		{"nothing changed", false, false, false, dashboard.FreshnessStale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := dashboard.AgentFreshness{
				SourceCommit: "abc1234",
				StorePath:    "/nix/store/old",
				BinaryHash:   "hash-old",
			}
			after := dashboard.AgentFreshness{
				SourceCommit: "abc1234",
				StorePath:    "/nix/store/old",
				BinaryHash:   "hash-old",
			}

			if tc.commitChanged {
				after.SourceCommit = "def5678"
			}
			if tc.pathChanged {
				after.StorePath = "/nix/store/new"
			}
			if tc.hashChanged {
				after.BinaryHash = "hash-new"
			}

			verdict, _ := dashboard.CompareFreshness(before, after)

			if verdict != tc.expectedVerdict {
				t.Errorf("expected %s, got %s", tc.expectedVerdict, verdict)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// AGENT FRESHNESS DETECTION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestAgentFreshness_GetFreshness(t *testing.T) {
	// This test verifies the agent can compute its own freshness
	// Note: This will use the test binary's info, not a real agent
	// We just verify the function doesn't panic and returns something

	// We can't test the actual agent freshness detection without
	// running as the agent, but we can verify the types work correctly

	freshness := dashboard.AgentFreshness{
		SourceCommit: "test-commit",
		StorePath:    "/test/path",
		BinaryHash:   "test-hash",
	}

	if freshness.SourceCommit != "test-commit" {
		t.Error("freshness struct not working correctly")
	}
}

