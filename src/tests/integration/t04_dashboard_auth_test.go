package integration

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/internal/dashboard"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// setupTestDashboard creates a test dashboard instance.
func setupTestDashboard(t *testing.T, opts ...func(*testDashboardOpts)) *testDashboard {
	t.Helper()

	o := &testDashboardOpts{
		password:   "testpassword123",
		totpSecret: "",
	}
	for _, opt := range opts {
		opt(o)
	}

	// Generate password hash
	hash, err := bcrypt.GenerateFromPassword([]byte(o.password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// Set environment (ignoring errors in test setup)
	_ = os.Setenv("NIXFLEET_PASSWORD_HASH", string(hash))
	_ = os.Setenv("NIXFLEET_SESSION_SECRET", "test-session-secret-32-bytes-xx")
	_ = os.Setenv("NIXFLEET_AGENT_TOKEN", "test-agent-token")
	if o.totpSecret != "" {
		_ = os.Setenv("NIXFLEET_TOTP_SECRET", o.totpSecret)
	} else {
		_ = os.Unsetenv("NIXFLEET_TOTP_SECRET")
	}
	_ = os.Setenv("NIXFLEET_DB_PATH", ":memory:")
	_ = os.Setenv("NIXFLEET_RATE_LIMIT", "5")
	_ = os.Setenv("NIXFLEET_RATE_WINDOW", "1m")

	// Load config
	cfg, err := dashboard.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create in-memory database
	db, err := dashboard.InitDatabase(":memory:")
	if err != nil {
		t.Fatalf("failed to init database: %v", err)
	}

	// Create server
	log := zerolog.New(io.Discard)
	server := dashboard.New(cfg, db, log)

	// Start test server
	ts := httptest.NewServer(server.Router())

	return &testDashboard{
		t:        t,
		server:   ts,
		db:       db,
		password: o.password,
	}
}

type testDashboardOpts struct {
	password   string
	totpSecret string
}

type testDashboard struct {
	t        *testing.T
	server   *httptest.Server
	db       *sql.DB
	password string
}

func (td *testDashboard) Close() {
	td.server.Close()
	_ = td.db.Close()
}

func (td *testDashboard) URL() string {
	return td.server.URL
}

// newClientWithCookies creates an HTTP client with cookie jar.
func newCookieJar(t *testing.T) *cookiejar.Jar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}
	return jar
}

func newClientWithCookies(t *testing.T) *http.Client {
	return &http.Client{
		Jar: newCookieJar(t),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
}

// newClientWithCookiesFollowRedirects creates an HTTP client that follows redirects.
func newClientWithCookiesFollowRedirects(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

// TestDashboardAuth_LoginSuccess tests successful password-only login.
func TestDashboardAuth_LoginSuccess(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookies(t)

	// POST login
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to dashboard
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %s", location)
	}

	// Should have session cookie
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "nixfleet_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Error("session cookie not set")
	} else {
		if !sessionCookie.HttpOnly {
			t.Error("session cookie should be HttpOnly")
		}
		t.Logf("session cookie set: %s", sessionCookie.Value[:16]+"...")
	}

	// Verify session in database
	var count int
	err = td.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

// TestDashboardAuth_LoginFailed tests failed login with wrong password.
func TestDashboardAuth_LoginFailed(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Use a client that doesn't follow redirects
	client := &http.Client{
		Jar: newCookieJar(t),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// POST login with wrong password
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {"wrongpassword"},
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to /login with error parameter
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 302 redirect, got %d: %s", resp.StatusCode, body)
	}

	// Check redirect location contains error
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected redirect to /login?error=..., got: %s", location)
	}

	// Should not have session cookie
	cookies := resp.Cookies()
	for _, c := range cookies {
		if c.Name == "nixfleet_session" {
			t.Error("session cookie should not be set on failed login")
		}
	}

	// Verify no session in database
	var count int
	err = td.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 sessions, got %d", count)
	}
}

// TestDashboardAuth_RateLimit tests rate limiting on login attempts.
func TestDashboardAuth_RateLimit(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Use a client that doesn't follow redirects
	client := &http.Client{
		Jar: newCookieJar(t),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// Make 5 failed attempts (fills the limit)
	for i := 0; i < 5; i++ {
		resp, err := client.PostForm(td.URL()+"/login", url.Values{
			"password": {"wrongpassword"},
		})
		if err != nil {
			t.Fatalf("login request %d failed: %v", i+1, err)
		}
		_ = resp.Body.Close()
		t.Logf("attempt %d: status %d, location: %s", i+1, resp.StatusCode, resp.Header.Get("Location"))
		if resp.StatusCode != http.StatusFound {
			t.Errorf("attempt %d: expected 302, got %d", i+1, resp.StatusCode)
		}
	}

	// 6th attempt should be rate limited (limit=5, so after 5 attempts we're blocked)
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {"wrongpassword"},
	})
	if err != nil {
		t.Fatalf("6th login request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect with rate limit error message
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 302 redirect, got %d: %s", resp.StatusCode, body)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error=") || !strings.Contains(location, "many") {
		t.Errorf("expected rate limit error in redirect, got: %s", location)
	} else {
		t.Log("rate limiting working correctly")
	}
}

// TestDashboardAuth_CSRF tests CSRF protection on API endpoints.
func TestDashboardAuth_CSRF(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Use client that follows redirects so session cookie is properly set
	client := newClientWithCookiesFollowRedirects(t)

	// Login - will redirect to dashboard
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Should now be on dashboard with CSRF token
	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	if csrfStart == -1 {
		t.Fatalf("CSRF token not found in response: %s", bodyStr)
	}
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	t.Logf("extracted CSRF token: %s", csrfToken[:16]+"...")

	// v3: POST to API without CSRF token should fail
	req, _ := http.NewRequest("POST", td.URL()+"/api/dispatch", strings.NewReader(`{"op":"pull","hosts":["test"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 without CSRF, got %d", resp.StatusCode)
	}

	// v3: POST with CSRF token should work (will fail for other reasons, but not CSRF)
	req, _ = http.NewRequest("POST", td.URL()+"/api/dispatch", strings.NewReader(`{"op":"pull","hosts":["test"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	// Should NOT be 403 (might be 200 with error in results for non-existent host)
	if resp.StatusCode == http.StatusForbidden {
		t.Error("CSRF should have passed with valid token")
	}

	t.Logf("CSRF protection working correctly (status without CSRF: 403, with CSRF: %d)", resp.StatusCode)
}

// TestDashboardAuth_UnauthenticatedAccess tests redirect for unauthenticated access.
func TestDashboardAuth_UnauthenticatedAccess(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	client := newClientWithCookies(t)

	// Try to access dashboard without login
	resp, err := client.Get(td.URL() + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to login
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %s", location)
	}

	t.Log("unauthenticated access correctly redirects to login")
}

// TestDashboardAuth_Logout tests logout functionality.
func TestDashboardAuth_Logout(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Use client that follows redirects
	client := newClientWithCookiesFollowRedirects(t)

	// Login - will redirect to dashboard
	resp, err := client.PostForm(td.URL()+"/login", url.Values{
		"password": {td.password},
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Verify session exists
	var count int
	err = td.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 session before logout, got %d", count)
	}

	// Extract CSRF token from dashboard page
	bodyStr := string(body)
	csrfStart := strings.Index(bodyStr, `data-csrf-token="`)
	if csrfStart == -1 {
		t.Fatalf("CSRF token not found in response: %s", bodyStr)
	}
	csrfStart += len(`data-csrf-token="`)
	csrfEnd := strings.Index(bodyStr[csrfStart:], `"`)
	csrfToken := bodyStr[csrfStart : csrfStart+csrfEnd]

	t.Logf("logout with CSRF token: %s", csrfToken[:16]+"...")

	// Logout (use non-redirect client to check response)
	logoutClient := newClientWithCookies(t)
	// Copy cookies from main client
	serverURL, _ := url.Parse(td.URL())
	logoutClient.Jar.SetCookies(serverURL, client.Jar.Cookies(serverURL))

	resp, err = logoutClient.PostForm(td.URL()+"/logout", url.Values{
		"csrf_token": {csrfToken},
	})
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	_ = resp.Body.Close()

	// Should redirect to login
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", resp.StatusCode)
	}

	// Verify session deleted
	err = td.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 sessions after logout, got %d", count)
	}

	t.Log("logout working correctly")
}

// TestDashboardAuth_SessionExpiry tests that expired sessions are rejected.
func TestDashboardAuth_SessionExpiry(t *testing.T) {
	td := setupTestDashboard(t)
	defer td.Close()

	// Insert an expired session directly
	expiredTime := time.Now().Add(-25 * time.Hour)
	_, err := td.db.Exec(
		`INSERT INTO sessions (id, csrf_token, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		"expired-session-id", "csrf-token", expiredTime, expiredTime.Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	// Create client with the expired session cookie
	jar, _ := cookiejar.New(nil)
	serverURL, _ := url.Parse(td.URL())
	jar.SetCookies(serverURL, []*http.Cookie{
		{Name: "nixfleet_session", Value: "expired-session-id"},
	})
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Try to access dashboard
	resp, err := client.Get(td.URL() + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to login (expired session)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect for expired session, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %s", location)
	}

	t.Log("expired session correctly rejected")
}

