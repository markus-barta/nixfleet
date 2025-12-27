package integration

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestRemoveHost_Success tests successful host removal.
func TestRemoveHost_Success(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Add a test host directly to database
	_, err := td.db.Exec(`INSERT INTO hosts (id, hostname, host_type, status) VALUES (?, ?, ?, ?)`,
		"remove-test", "remove-test", "nixos", "offline")
	if err != nil {
		t.Fatalf("failed to insert test host: %v", err)
	}

	client := newClientWithCookiesFollowRedirects(t)

	// Login and get CSRF token
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	// Delete the host
	req, _ := http.NewRequest("DELETE", td.URL()+"/api/hosts/remove-test", nil)
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify host is gone
	var count int
	_ = td.db.QueryRow("SELECT COUNT(*) FROM hosts WHERE id = ?", "remove-test").Scan(&count)
	if count != 0 {
		t.Errorf("expected host to be deleted, but found %d", count)
	}

	t.Log("Remove Host API working correctly")
}

// TestRemoveHost_NotFound tests removing non-existent host.
func TestRemoveHost_NotFound(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookiesFollowRedirects(t)

	// Login and get CSRF token
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	// Try to delete non-existent host
	req, _ := http.NewRequest("DELETE", td.URL()+"/api/hosts/nonexistent-host", nil)
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	t.Log("Remove Host returns 404 for non-existent host")
}

// TestRemoveHost_WithoutCSRF tests that CSRF is required.
func TestRemoveHost_WithoutCSRF(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Add a test host
	_, _ = td.db.Exec(`INSERT INTO hosts (id, hostname, host_type, status) VALUES (?, ?, ?, ?)`,
		"csrf-test", "csrf-test", "nixos", "offline")

	client := newClientWithCookiesFollowRedirects(t)

	// Login
	_, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Try to delete without CSRF
	req, _ := http.NewRequest("DELETE", td.URL()+"/api/hosts/csrf-test", nil)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}

	// Verify host still exists
	var count int
	_ = td.db.QueryRow("SELECT COUNT(*) FROM hosts WHERE id = ?", "csrf-test").Scan(&count)
	if count != 1 {
		t.Errorf("expected host to still exist, but found %d", count)
	}

	t.Log("CSRF protection working for Remove Host")
}

