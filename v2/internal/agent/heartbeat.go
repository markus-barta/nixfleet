package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/protocol"
)

// heartbeatLoop sends periodic heartbeats.
// It continues sending heartbeats even during command execution (T02 critical requirement).
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.ws.IsConnected() && a.IsRegistered() {
				a.sendHeartbeat()
			}
		}
	}
}

// sendHeartbeat sends a single heartbeat message.
func (a *Agent) sendHeartbeat() {
	a.mu.RLock()
	pendingCommand := a.pendingCommand
	commandPID := a.commandPID
	a.mu.RUnlock()

	// Refresh generation (may have changed after switch)
	a.generation = a.detectGeneration()

	payload := protocol.HeartbeatPayload{
		Generation:     a.generation,
		NixpkgsVersion: a.nixpkgsVersion,
		PendingCommand: pendingCommand,
		CommandPID:     commandPID,
		Metrics:        a.readMetrics(),
	}

	if err := a.ws.SendMessage(protocol.TypeHeartbeat, payload); err != nil {
		a.log.Debug().Err(err).Msg("failed to send heartbeat")
		return
	}

	a.log.Debug().
		Str("generation", payload.Generation).
		Interface("pending", pendingCommand).
		Msg("heartbeat sent")
}

// readMetrics reads system metrics from StaSysMo.
func (a *Agent) readMetrics() *protocol.Metrics {
	var metricsDir string
	if runtime.GOOS == "darwin" {
		metricsDir = "/tmp/stasysmo"
	} else {
		metricsDir = "/dev/shm/stasysmo"
	}

	// Check if StaSysMo is available
	if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
		return nil
	}

	cpu := a.readMetricFile(filepath.Join(metricsDir, "cpu"))
	ram := a.readMetricFile(filepath.Join(metricsDir, "ram"))
	swap := a.readMetricFile(filepath.Join(metricsDir, "swap"))
	load := a.readMetricFile(filepath.Join(metricsDir, "load"))

	// If we couldn't read any metrics, return nil
	if cpu == 0 && ram == 0 && swap == 0 && load == 0 {
		return nil
	}

	return &protocol.Metrics{
		CPU:  cpu,
		RAM:  ram,
		Swap: swap,
		Load: load,
	}
}

// readMetricFile reads a single metric value from a file.
func (a *Agent) readMetricFile(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0
	}

	return val
}

// detectSystemInfo caches system information.
func (a *Agent) detectSystemInfo() {
	a.osVersion = a.detectOSVersion()
	a.nixpkgsVersion = a.detectNixpkgsVersion()
	a.generation = a.detectGeneration()

	a.log.Info().
		Str("os_version", a.osVersion).
		Str("nixpkgs_version", a.nixpkgsVersion).
		Str("generation", a.generation).
		Msg("system info detected")
}

// detectHostType returns "nixos" or "macos".
func (a *Agent) detectHostType() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "nixos"
}

// detectOSVersion returns the OS version string.
func (a *Agent) detectOSVersion() string {
	if runtime.GOOS == "darwin" {
		// Try to read macOS version
		data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
		if err == nil {
			// Simple extraction - look for ProductVersion
			if idx := strings.Index(string(data), "<key>ProductVersion</key>"); idx != -1 {
				rest := string(data)[idx:]
				if start := strings.Index(rest, "<string>"); start != -1 {
					rest = rest[start+8:]
					if end := strings.Index(rest, "</string>"); end != -1 {
						return rest[:end]
					}
				}
			}
		}
		return "unknown"
	}

	// NixOS version
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	return "unknown"
}

// detectNixpkgsVersion returns the nixpkgs version.
func (a *Agent) detectNixpkgsVersion() string {
	// First check if version was passed via environment (for macOS/Home Manager)
	if a.cfg.NixpkgsVersion != "" {
		return a.cfg.NixpkgsVersion
	}

	// Try to read from nixos-version (NixOS systems)
	data, err := os.ReadFile("/run/current-system/nixos-version")
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// Fallback: return empty
	return ""
}

// detectGeneration returns the current git commit hash of the deployed config.
func (a *Agent) detectGeneration() string {
	// Read from repo directory
	gitHead := filepath.Join(a.cfg.RepoDir, ".git", "HEAD")
	data, err := os.ReadFile(gitHead)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))

	// If it's a ref, read the ref file
	if strings.HasPrefix(content, "ref: ") {
		refPath := strings.TrimPrefix(content, "ref: ")
		refFile := filepath.Join(a.cfg.RepoDir, ".git", refPath)
		data, err = os.ReadFile(refFile)
		if err != nil {
			return ""
		}
		content = strings.TrimSpace(string(data))
	}

	// Return short hash (7 chars)
	if len(content) >= 7 {
		return content[:7]
	}
	return content
}

