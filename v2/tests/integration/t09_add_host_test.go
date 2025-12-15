package integration

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestAddHost_Success tests successful manual host addition.
func TestAddHost_Success(t *testing.T) {
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

	// Extract CSRF token
	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	if csrfStart == -1 {
		t.Fatal("CSRF token not found")
	}
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	// Add a host
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts", strings.NewReader(`{
		"hostname": "test-host",
		"host_type": "nixos",
		"location": "home",
		"device_type": "desktop",
		"theme_color": "#ff0000"
	}`))
	req.Header.Set("Content-Type", "application/json")
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

	// Verify host exists in database
	var count int
	err = td.db.QueryRow("SELECT COUNT(*) FROM hosts WHERE hostname = ?", "test-host").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 host, got %d", count)
	}

	// Verify host attributes
	var hostType, location, deviceType, themeColor string
	err = td.db.QueryRow("SELECT host_type, location, device_type, theme_color FROM hosts WHERE hostname = ?", "test-host").
		Scan(&hostType, &location, &deviceType, &themeColor)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if hostType != "nixos" {
		t.Errorf("expected host_type nixos, got %s", hostType)
	}
	if location != "home" {
		t.Errorf("expected location home, got %s", location)
	}
	if deviceType != "desktop" {
		t.Errorf("expected device_type desktop, got %s", deviceType)
	}
	if themeColor != "#ff0000" {
		t.Errorf("expected theme_color #ff0000, got %s", themeColor)
	}

	t.Log("Add Host API working correctly")
}

// TestAddHost_WithoutCSRF tests that adding host requires CSRF token.
func TestAddHost_WithoutCSRF(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookiesFollowRedirects(t)

	// Login
	_, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Try to add host without CSRF token
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts", strings.NewReader(`{
		"hostname": "test-host"
	}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", resp.StatusCode)
	}

	t.Log("CSRF protection working for Add Host")
}

// TestAddHost_DuplicateUpdates tests that adding duplicate host updates it.
func TestAddHost_DuplicateUpdates(t *testing.T) {
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

	// Add host first time
	req, _ := http.NewRequest("POST", td.URL()+"/api/hosts", strings.NewReader(`{
		"hostname": "dup-host",
		"theme_color": "#111111"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp, _ = client.Do(req)
	_ = resp.Body.Close()

	// Add same host again with different color
	req, _ = http.NewRequest("POST", td.URL()+"/api/hosts", strings.NewReader(`{
		"hostname": "dup-host",
		"theme_color": "#222222"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Should still have only 1 host, but with updated color
	var count int
	var themeColor string
	_ = td.db.QueryRow("SELECT COUNT(*) FROM hosts WHERE hostname = ?", "dup-host").Scan(&count)
	_ = td.db.QueryRow("SELECT theme_color FROM hosts WHERE hostname = ?", "dup-host").Scan(&themeColor)

	if count != 1 {
		t.Errorf("expected 1 host, got %d", count)
	}
	if themeColor != "#222222" {
		t.Errorf("expected updated color #222222, got %s", themeColor)
	}

	t.Log("duplicate host correctly updates existing record")
}

