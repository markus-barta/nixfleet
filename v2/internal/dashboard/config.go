// Package dashboard implements the NixFleet dashboard server.
package dashboard

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds dashboard configuration from environment variables.
type Config struct {
	// Server
	ListenAddr string
	BaseURL    string

	// Authentication
	PasswordHash   string // bcrypt hash
	SessionSecret  string // for signing cookies
	TOTPSecret     string // optional, for 2FA
	AgentToken     string // token that agents must provide

	// Session
	SessionDuration time.Duration

	// Rate limiting
	RateLimitRequests int           // max attempts
	RateLimitWindow   time.Duration // time window

	// Database
	DatabasePath string

	// Data directory for logs etc
	DataDir string

	// Security
	AllowedOrigins []string // optional, for WebSocket origin validation

	// Update Status (P5000)
	VersionURL      string        // URL to fetch version.json from GitHub Pages
	VersionFetchTTL time.Duration // How often to fetch version (default: 30s)

	// GitHub Integration (P5300)
	GitHubToken   string        // Personal Access Token with repo scope
	GitHubRepo    string        // owner/repo format (e.g., "markus-barta/nixcfg")
	GitHubAPIURL  string        // Optional: override API URL (for testing)
	GitHubPollTTL time.Duration // How often to check for update PRs (default: 1h)

	// Stale command cleanup (PRD FR-2.13)
	// Uses multiplier × heartbeat_interval with a floor (like Kubernetes liveness probes)
	HeartbeatInterval    time.Duration // Reference interval for stale detection (default: 5s)
	StaleMultiplier      int           // Number of missed heartbeats before stale (default: 120)
	StaleMinimum         time.Duration // Floor to prevent aggressive cleanup (default: 5m)
	StaleCleanupInterval time.Duration // How often to run cleanup job (default: 1m)
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	dataDir := getEnv("NIXFLEET_DATA_DIR", "/data")

	cfg := &Config{
		ListenAddr:        getEnv("NIXFLEET_LISTEN", ":8000"),
		BaseURL:           getEnv("NIXFLEET_BASE_URL", "http://localhost:8000"),
		PasswordHash:      os.Getenv("NIXFLEET_PASSWORD_HASH"),
		SessionSecret:     os.Getenv("NIXFLEET_SESSION_SECRET"),
		TOTPSecret:        os.Getenv("NIXFLEET_TOTP_SECRET"), // optional
		AgentToken:        os.Getenv("NIXFLEET_AGENT_TOKEN"),
		SessionDuration:   parseDuration("NIXFLEET_SESSION_DURATION", 24*time.Hour),
		RateLimitRequests: parseInt("NIXFLEET_RATE_LIMIT", 5),
		RateLimitWindow:   parseDuration("NIXFLEET_RATE_WINDOW", 1*time.Minute),
		DatabasePath:      getEnv("NIXFLEET_DB_PATH", dataDir+"/nixfleet.db"),
		DataDir:           dataDir,
		AllowedOrigins:    parseOrigins("NIXFLEET_ALLOWED_ORIGINS"),

		// Update Status (P5000)
		VersionURL:      getEnv("NIXFLEET_VERSION_URL", ""), // e.g., https://user.github.io/nixcfg/version.json
		VersionFetchTTL: parseDuration("NIXFLEET_VERSION_FETCH_TTL", 30*time.Second),

		// GitHub Integration (P5300)
		GitHubToken:   os.Getenv("NIXFLEET_GITHUB_TOKEN"),
		GitHubRepo:    os.Getenv("NIXFLEET_GITHUB_REPO"), // e.g., "markus-barta/nixcfg"
		GitHubAPIURL:  getEnv("NIXFLEET_GITHUB_API_URL", "https://api.github.com"),
		GitHubPollTTL: parseDuration("NIXFLEET_GITHUB_POLL_TTL", 1*time.Hour),

		// Stale command cleanup defaults (PRD FR-2.13)
		HeartbeatInterval:    parseDuration("NIXFLEET_HEARTBEAT_INTERVAL", 5*time.Second),
		StaleMultiplier:      parseInt("NIXFLEET_STALE_MULTIPLIER", 120),
		StaleMinimum:         parseDuration("NIXFLEET_STALE_MINIMUM", 5*time.Minute),
		StaleCleanupInterval: parseDuration("NIXFLEET_STALE_CLEANUP_INTERVAL", 1*time.Minute),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string

	if c.PasswordHash == "" {
		errs = append(errs, "NIXFLEET_PASSWORD_HASH is required")
	}
	if c.SessionSecret == "" {
		errs = append(errs, "NIXFLEET_SESSION_SECRET is required")
	}
	if c.AgentToken == "" {
		errs = append(errs, "NIXFLEET_AGENT_TOKEN is required")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Warnings returns non-fatal configuration warnings.
// Call this after loading config and log the results.
func (c *Config) Warnings() []string {
	var warnings []string

	// GitHub integration requires both token and repo
	if c.GitHubToken != "" && c.GitHubRepo == "" {
		warnings = append(warnings, "NIXFLEET_GITHUB_TOKEN is set but NIXFLEET_GITHUB_REPO is missing; GitHub integration disabled")
	}
	if c.GitHubRepo != "" && c.GitHubToken == "" {
		warnings = append(warnings, "NIXFLEET_GITHUB_REPO is set but NIXFLEET_GITHUB_TOKEN is missing; GitHub integration disabled")
	}

	// Version tracking without URL
	if c.VersionURL == "" {
		warnings = append(warnings, "NIXFLEET_VERSION_URL not set; Git status tracking disabled")
	}

	return warnings
}

// HasTOTP returns true if TOTP is configured.
func (c *Config) HasTOTP() bool {
	return c.TOTPSecret != ""
}

// HasVersionTracking returns true if version URL is configured.
func (c *Config) HasVersionTracking() bool {
	return c.VersionURL != ""
}

// HasGitHubIntegration returns true if GitHub integration is configured.
func (c *Config) HasGitHubIntegration() bool {
	return c.GitHubToken != "" && c.GitHubRepo != ""
}

// GitHubOwnerRepo parses the GitHubRepo into owner and repo parts.
func (c *Config) GitHubOwnerRepo() (owner, repo string) {
	parts := strings.SplitN(c.GitHubRepo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// StaleCommandTimeout calculates the threshold for stale command cleanup.
// Uses multiplier × heartbeat_interval with a floor (like Kubernetes liveness probes).
// Example: 120 × 5s = 10 minutes (with 5m floor, effective = 10 minutes)
func (c *Config) StaleCommandTimeout() time.Duration {
	calculated := c.HeartbeatInterval * time.Duration(c.StaleMultiplier)
	if calculated < c.StaleMinimum {
		return c.StaleMinimum
	}
	return calculated
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func parseDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

func parseInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func parseOrigins(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

