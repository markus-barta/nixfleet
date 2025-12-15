package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
)

// TestGitStatus_VersionFetcher tests the version fetcher logic.
func TestGitStatus_VersionFetcher(t *testing.T) {
	// Create a mock GitHub Pages server
	mockVersion := map[string]any{
		"gitCommit": "abc1234567890def",
		"message":   "feat: test commit",
		"branch":    "main",
		"timestamp": "2025-12-15T12:00:00Z",
		"repo":      "test/nixcfg",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockVersion)
	}))
	defer server.Close()

	// Create fetcher with mock URL
	fetcher := dashboard.NewVersionFetcher(server.URL, 100*time.Millisecond)

	// Initially should have no data
	if fetcher.HasData() {
		t.Error("fetcher should not have data before start")
	}

	// Start fetcher (synchronous first fetch)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher.Start(ctx)

	// Wait a bit for fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Should have data now
	if !fetcher.HasData() {
		t.Error("fetcher should have data after start")
	}

	// Get latest
	latest := fetcher.GetLatest()
	if latest == nil {
		t.Fatal("GetLatest returned nil")
	}
	if latest.GitCommit != "abc1234567890def" {
		t.Errorf("expected commit abc1234567890def, got %s", latest.GitCommit)
	}

	// Test status comparison - matching generation
	status, msg, _ := fetcher.GetGitStatus("abc1234567890def")
	if status != "ok" {
		t.Errorf("expected status ok, got %s: %s", status, msg)
	}

	// Test status comparison - matching short prefix
	status, msg, _ = fetcher.GetGitStatus("abc1234")
	if status != "ok" {
		t.Errorf("expected status ok for short prefix, got %s: %s", status, msg)
	}

	// Test status comparison - outdated
	status, msg, _ = fetcher.GetGitStatus("xyz9876543210abc")
	if status != "outdated" {
		t.Errorf("expected status outdated, got %s", status)
	}
	if !strings.Contains(msg, "Behind remote") {
		t.Errorf("expected 'Behind remote' in message, got: %s", msg)
	}

	// Test status comparison - empty generation
	status, msg, _ = fetcher.GetGitStatus("")
	if status != "unknown" {
		t.Errorf("expected status unknown for empty generation, got %s", status)
	}

	t.Log("VersionFetcher working correctly")
}

// TestGitStatus_DisplayInDashboard tests that git status is shown in dashboard.
func TestGitStatus_DisplayInDashboard(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Add a host with a generation
	_, err := td.db.Exec(`INSERT INTO hosts (id, hostname, host_type, status, generation) VALUES (?, ?, ?, ?, ?)`,
		"test-host", "test-host", "nixos", "online", "abc1234567890def")
	if err != nil {
		t.Fatalf("failed to insert test host: %v", err)
	}

	client := newClientWithCookiesFollowRedirects(t)

	// Login
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	bodyStr := string(body)

	// Should contain the update status container
	if !strings.Contains(bodyStr, "update-status") {
		t.Error("dashboard should contain update-status class")
	}

	// Should contain the host
	if !strings.Contains(bodyStr, "test-host") {
		t.Error("dashboard should contain test-host")
	}

	// Check for update compartment (either needs-update, ok, or unknown)
	if !strings.Contains(bodyStr, "update-compartment") {
		t.Error("dashboard should contain update-compartment elements")
	}

	t.Log("Git status display working in dashboard")
}

