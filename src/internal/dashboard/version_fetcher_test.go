package dashboard

import (
	"testing"
	"time"
)

// P3700: Tests for version-based Lock compartment tracking

func TestGetLockStatus_Match(t *testing.T) {
	vf := &VersionFetcher{
		ttl: 5 * time.Second,
		cached: &RemoteVersion{
			GitCommit: "abc123",
			LockHash:  "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		},
	}

	status, message, _ := vf.GetLockStatus("deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678")

	if status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status)
	}
	if message != "flake.lock is current" {
		t.Errorf("Expected 'flake.lock is current', got '%s'", message)
	}
}

func TestGetLockStatus_Outdated(t *testing.T) {
	vf := &VersionFetcher{
		ttl: 5 * time.Second,
		cached: &RemoteVersion{
			GitCommit: "abc123",
			LockHash:  "aabb1122334455667788990011223344556677889900112233445566778899aa",
		},
	}

	status, message, _ := vf.GetLockStatus("ccdd5566778899001122334455667788990011223344556677889900aabbccdd")

	if status != "outdated" {
		t.Errorf("Expected status 'outdated', got '%s'", status)
	}
	// Should show short hashes (8 chars each)
	if message == "" {
		t.Error("Expected non-empty message for outdated status")
	}
	// Agent short hash should be first 8 chars: "ccdd5566"
	if !contains(message, "ccdd5566") {
		t.Errorf("Expected message to contain agent short hash 'ccdd5566', got '%s'", message)
	}
	// Latest short hash should be first 8 chars: "aabb1122"
	if !contains(message, "aabb1122") {
		t.Errorf("Expected message to contain latest short hash 'aabb1122', got '%s'", message)
	}
}

func TestGetLockStatus_NoCache(t *testing.T) {
	vf := &VersionFetcher{
		ttl:    5 * time.Second,
		cached: nil,
	}

	status, message, _ := vf.GetLockStatus("somehash")

	if status != "unknown" {
		t.Errorf("Expected status 'unknown', got '%s'", status)
	}
	if message != "Version tracking not available" {
		t.Errorf("Expected 'Version tracking not available', got '%s'", message)
	}
}

func TestGetLockStatus_NoLockHashInCache(t *testing.T) {
	vf := &VersionFetcher{
		ttl: 5 * time.Second,
		cached: &RemoteVersion{
			GitCommit: "abc123",
			LockHash:  "", // Empty - old version.json without lockHash
		},
	}

	status, message, _ := vf.GetLockStatus("somehash")

	if status != "unknown" {
		t.Errorf("Expected status 'unknown', got '%s'", status)
	}
	if message != "No lockHash in version.json" {
		t.Errorf("Expected 'No lockHash in version.json', got '%s'", message)
	}
}

func TestGetLockStatus_NoAgentHash(t *testing.T) {
	vf := &VersionFetcher{
		ttl: 5 * time.Second,
		cached: &RemoteVersion{
			GitCommit: "abc123",
			LockHash:  "latesthash1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		},
	}

	status, message, _ := vf.GetLockStatus("")

	if status != "unknown" {
		t.Errorf("Expected status 'unknown', got '%s'", status)
	}
	if message != "Agent has not reported lock hash" {
		t.Errorf("Expected 'Agent has not reported lock hash', got '%s'", message)
	}
}

func TestGetLatestLockHash(t *testing.T) {
	vf := &VersionFetcher{
		ttl: 5 * time.Second,
		cached: &RemoteVersion{
			GitCommit: "abc123",
			LockHash:  "expectedhash123",
		},
	}

	hash := vf.GetLatestLockHash()
	if hash != "expectedhash123" {
		t.Errorf("Expected 'expectedhash123', got '%s'", hash)
	}
}

func TestGetLatestLockHash_NoCache(t *testing.T) {
	vf := &VersionFetcher{
		ttl:    5 * time.Second,
		cached: nil,
	}

	hash := vf.GetLatestLockHash()
	if hash != "" {
		t.Errorf("Expected empty string, got '%s'", hash)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

