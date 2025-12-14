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

// HasTOTP returns true if TOTP is configured.
func (c *Config) HasTOTP() bool {
	return c.TOTPSecret != ""
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

