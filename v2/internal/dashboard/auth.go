package dashboard

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// Session represents a user session.
type Session struct {
	ID        string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// RateLimiter tracks login attempts.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request from the given IP is allowed.
// Returns true if under limit, false if rate limited.
func (r *RateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Filter old attempts
	var recent []time.Time
	for _, t := range r.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	// Check if already at limit BEFORE recording this attempt
	if len(recent) >= r.limit {
		r.attempts[ip] = recent
		return false
	}

	// Record this attempt
	r.attempts[ip] = append(recent, now)
	return true
}

// Reset clears attempts for an IP (on successful login).
func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

// AuthService handles authentication.
type AuthService struct {
	cfg         *Config
	db          *sql.DB
	rateLimiter *RateLimiter
}

// NewAuthService creates a new auth service.
func NewAuthService(cfg *Config, db *sql.DB) *AuthService {
	return &AuthService{
		cfg:         cfg,
		db:          db,
		rateLimiter: NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow),
	}
}

// CheckPassword verifies the password against the hash.
func (a *AuthService) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.cfg.PasswordHash), []byte(password))
	return err == nil
}

// CheckTOTP verifies the TOTP code.
func (a *AuthService) CheckTOTP(code string) bool {
	if !a.cfg.HasTOTP() {
		return true // TOTP not required
	}
	return totp.Validate(code, a.cfg.TOTPSecret)
}

// CreateSession creates a new session and stores it in the database.
func (a *AuthService) CreateSession() (*Session, error) {
	sessionID, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}
	csrfToken, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		CSRFToken: csrfToken,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(a.cfg.SessionDuration),
	}

	_, err = a.db.Exec(
		`INSERT INTO sessions (id, csrf_token, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		session.ID, session.CSRFToken, session.CreatedAt, session.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession retrieves a session from the database.
func (a *AuthService) GetSession(sessionID string) (*Session, error) {
	session := &Session{}
	err := a.db.QueryRow(
		`SELECT id, csrf_token, created_at, expires_at FROM sessions WHERE id = ?`,
		sessionID,
	).Scan(&session.ID, &session.CSRFToken, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		return nil, err
	}

	// Check expiry
	if time.Now().After(session.ExpiresAt) {
		_ = a.DeleteSession(sessionID)
		return nil, sql.ErrNoRows
	}

	return session, nil
}

// DeleteSession removes a session from the database.
func (a *AuthService) DeleteSession(sessionID string) error {
	_, err := a.db.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// ValidateCSRF checks if the CSRF token matches the session.
func (a *AuthService) ValidateCSRF(session *Session, token string) bool {
	return subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) == 1
}

// IsRateLimited checks if the IP is rate limited.
func (a *AuthService) IsRateLimited(ip string) bool {
	return !a.rateLimiter.Allow(ip)
}

// ResetRateLimit clears rate limit for an IP.
func (a *AuthService) ResetRateLimit(ip string) {
	a.rateLimiter.Reset(ip)
}

// SetSessionCookie sets the session cookie on the response.
func (a *AuthService) SetSessionCookie(w http.ResponseWriter, session *Session) {
	// Note: Secure should be true in production (HTTPS only)
	// In development/testing it's set to false to work with HTTP
	http.SetCookie(w, &http.Cookie{
		Name:     "nixfleet_session",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // TODO: make configurable for production
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

// ClearSessionCookie clears the session cookie.
func (a *AuthService) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "nixfleet_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// GetSessionFromRequest extracts the session from the request cookie.
func (a *AuthService) GetSessionFromRequest(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie("nixfleet_session")
	if err != nil {
		return nil, err
	}
	return a.GetSession(cookie.Value)
}

// ValidateAgentToken checks if the agent token is valid.
func (a *AuthService) ValidateAgentToken(token string) bool {
	return subtle.ConstantTimeCompare([]byte(a.cfg.AgentToken), []byte(token)) == 1
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateCSRFToken generates a new CSRF token as a hex string.
func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

