// Package config handles agent configuration from environment variables.
package config

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config holds all agent configuration.
type Config struct {
	// Connection
	DashboardURL string // WebSocket URL (ws:// or wss://)
	Token        string // Agent authentication token

	// Repository
	RepoURL string // Git repository URL (for isolated mode)
	RepoDir string // Local repository path
	Branch  string // Git branch (default: main)
	SSHKey  string // SSH key path for git operations

	// Behavior
	HeartbeatInterval time.Duration // How often to send heartbeats
	LogLevel          string        // Logging level (debug, info, warn, error)

	// Derived
	Hostname string // System hostname

	// System info (can be overridden via env)
	NixpkgsVersion string // nixpkgs version from environment
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Branch:            "main",
		HeartbeatInterval: 30 * time.Second,
		LogLevel:          "info",
		Hostname:          getStableHostname(),
	}
}

// getStableHostname returns a stable hostname that doesn't change with network.
// On macOS, os.Hostname() can return network-dependent names like "imac0w.lan"
// which change when switching between ethernet/wifi. We use LocalHostName instead.
func getStableHostname() string {
	// On macOS, use scutil to get the stable LocalHostName
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("scutil", "--get", "LocalHostName").Output(); err == nil {
			if hostname := strings.TrimSpace(string(out)); hostname != "" {
				return hostname
			}
		}
	}

	// Fallback to os.Hostname() with domain suffix stripped
	hostname, _ := os.Hostname()
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}
	return hostname
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	// Required
	cfg.DashboardURL = os.Getenv("NIXFLEET_URL")
	if cfg.DashboardURL == "" {
		return nil, errors.New("NIXFLEET_URL is required")
	}

	cfg.Token = os.Getenv("NIXFLEET_TOKEN")
	if cfg.Token == "" {
		return nil, errors.New("NIXFLEET_TOKEN is required")
	}

	// Repository (one of these patterns is required)
	cfg.RepoURL = os.Getenv("NIXFLEET_REPO_URL")
	cfg.RepoDir = os.Getenv("NIXFLEET_REPO_DIR")
	if cfg.RepoURL == "" && cfg.RepoDir == "" {
		// Check legacy variable
		cfg.RepoDir = os.Getenv("NIXFLEET_NIXCFG")
		if cfg.RepoDir == "" {
			return nil, errors.New("NIXFLEET_REPO_URL or NIXFLEET_REPO_DIR is required")
		}
	}

	// Optional
	if branch := os.Getenv("NIXFLEET_BRANCH"); branch != "" {
		cfg.Branch = branch
	}

	cfg.SSHKey = os.Getenv("NIXFLEET_SSH_KEY")

	if interval := os.Getenv("NIXFLEET_INTERVAL"); interval != "" {
		seconds, err := strconv.Atoi(interval)
		if err != nil {
			return nil, errors.New("NIXFLEET_INTERVAL must be a number (seconds)")
		}
		cfg.HeartbeatInterval = time.Duration(seconds) * time.Second
	}

	if level := os.Getenv("NIXFLEET_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}

	// Override hostname if specified
	if hostname := os.Getenv("NIXFLEET_HOSTNAME"); hostname != "" {
		cfg.Hostname = hostname
	}

	// Override nixpkgs version if specified (for macOS/Home Manager)
	cfg.NixpkgsVersion = os.Getenv("NIXFLEET_NIXPKGS_VERSION")

	return cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.DashboardURL == "" {
		return errors.New("dashboard URL is required")
	}
	if c.Token == "" {
		return errors.New("token is required")
	}
	if c.HeartbeatInterval < time.Second {
		return errors.New("heartbeat interval must be at least 1 second")
	}
	return nil
}

