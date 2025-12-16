package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/protocol"
)

// StatusChecker handles update status checks for Lock and System compartments.
type StatusChecker struct {
	a *Agent

	// Cached status
	lockStatus   protocol.StatusCheck
	systemStatus protocol.StatusCheck

	// Last check times
	lastLockCheck   time.Time
	lastSystemCheck time.Time

	// Check intervals
	lockInterval   time.Duration // 5 minutes for lock
	systemInterval time.Duration // 5 minutes for system
}

// NewStatusChecker creates a new status checker.
func NewStatusChecker(a *Agent) *StatusChecker {
	return &StatusChecker{
		a:              a,
		lockInterval:   5 * time.Minute,
		systemInterval: 5 * time.Minute,
	}
}

// GetStatus returns the current update status for Lock and System.
// Git status is computed dashboard-side from GitHub Pages.
func (s *StatusChecker) GetStatus(ctx context.Context) *protocol.UpdateStatus {
	now := time.Now()

	// Check lock status if expired
	if now.Sub(s.lastLockCheck) > s.lockInterval {
		s.a.log.Debug().Msg("running lock status check")
		s.lockStatus = s.checkLockStatus(ctx)
		s.lastLockCheck = now
		s.a.log.Debug().
			Str("status", s.lockStatus.Status).
			Str("message", s.lockStatus.Message).
			Msg("lock status check completed")
	}

	// Check system status if expired
	if now.Sub(s.lastSystemCheck) > s.systemInterval {
		s.a.log.Debug().Msg("running system status check")
		s.systemStatus = s.checkSystemStatus(ctx)
		s.lastSystemCheck = now
		s.a.log.Debug().
			Str("status", s.systemStatus.Status).
			Str("message", s.systemStatus.Message).
			Msg("system status check completed")
	}

	return &protocol.UpdateStatus{
		// Git is computed dashboard-side
		Lock:   s.lockStatus,
		System: s.systemStatus,
	}
}

// ForceRefresh forces an immediate refresh of all status checks.
func (s *StatusChecker) ForceRefresh(ctx context.Context) {
	s.lockStatus = s.checkLockStatus(ctx)
	s.lastLockCheck = time.Now()
	s.systemStatus = s.checkSystemStatus(ctx)
	s.lastSystemCheck = time.Now()
}

// checkLockStatus checks how recently the flake.lock was updated.
func (s *StatusChecker) checkLockStatus(ctx context.Context) protocol.StatusCheck {
	repoDir := s.a.cfg.RepoDir
	if repoDir == "" {
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Repository path not configured",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	lockFile := filepath.Join(repoDir, "flake.lock")
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "flake.lock not found",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Get last commit date that modified flake.lock
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "log", "-1", "--format=%ci", "--", "flake.lock")
	output, err := cmd.Output()
	if err != nil {
		return protocol.StatusCheck{
			Status:    "error",
			Message:   "Failed to check flake.lock: " + err.Error(),
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	dateStr := strings.TrimSpace(string(output))
	if dateStr == "" {
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "flake.lock has no git history",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Parse the date (format: 2025-12-10 14:30:00 +0100)
	lockDate, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
	if err != nil {
		return protocol.StatusCheck{
			Status:    "error",
			Message:   "Failed to parse lock date: " + err.Error(),
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	daysSince := int(time.Since(lockDate).Hours() / 24)

	// Determine status based on age
	var status, message string
	switch {
	case daysSince <= 7:
		status = "ok"
		message = formatDaysAgo(daysSince)
	case daysSince <= 30:
		status = "outdated"
		message = formatDaysAgo(daysSince) + " - consider updating"
	default:
		status = "outdated"
		message = formatDaysAgo(daysSince) + " - needs update"
	}

	return protocol.StatusCheck{
		Status:    status,
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// checkSystemStatus checks if the running system matches what the current flake would build.
func (s *StatusChecker) checkSystemStatus(ctx context.Context) protocol.StatusCheck {
	repoDir := s.a.cfg.RepoDir
	hostname := s.a.cfg.Hostname

	if repoDir == "" {
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Repository path not configured",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second) // Longer timeout for nix operations
	defer cancel()

	if runtime.GOOS == "darwin" {
		return s.checkMacOSSystemStatus(ctx, repoDir, hostname)
	}
	return s.checkNixOSSystemStatus(ctx, repoDir, hostname)
}

// checkNixOSSystemStatus checks NixOS system status by comparing derivations.
func (s *StatusChecker) checkNixOSSystemStatus(ctx context.Context, repoDir, hostname string) protocol.StatusCheck {
	// Get current system derivation (resolve symlink to store path)
	currentPath, err := filepath.EvalSymlinks("/run/current-system")
	if err != nil {
		return protocol.StatusCheck{
			Status:    "error",
			Message:   "Cannot read current system: " + err.Error(),
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Get what the flake would build (dry-run)
	flakeRef := repoDir + "#nixosConfigurations." + hostname + ".config.system.build.toplevel"
	cmd := exec.CommandContext(ctx, "nix", "build", "--dry-run", "--json", flakeRef)
	cmd.Dir = repoDir
	output, err := cmd.Output()

	if err != nil {
		// If nix build fails, it might be because we're not root or other issues
		// Fall back to checking if generation matches the repo
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Cannot evaluate system (try with sudo)",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Parse the JSON output to get the derivation path
	// Output format: [{"drvPath": "...", "outputs": {"out": "/nix/store/..."}}]
	outStr := string(output)
	if strings.Contains(outStr, currentPath) {
		return protocol.StatusCheck{
			Status:    "ok",
			Message:   "System is current",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	return protocol.StatusCheck{
		Status:    "outdated",
		Message:   "System needs rebuild",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// checkMacOSSystemStatus checks macOS/Home Manager system status.
func (s *StatusChecker) checkMacOSSystemStatus(ctx context.Context, repoDir, hostname string) protocol.StatusCheck {
	// For macOS, check Home Manager generation
	// Get current generation path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return protocol.StatusCheck{
			Status:    "error",
			Message:   "Cannot get home directory: " + err.Error(),
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	currentGen := filepath.Join(homeDir, ".local/state/nix/profiles/home-manager")

	// Resolve symlink chain to get final store path
	// home-manager -> home-manager-37-link -> /nix/store/xxx-home-manager-generation
	currentPath, err := filepath.EvalSymlinks(currentGen)
	if err != nil {
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "No Home Manager profile found",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Get what the flake would build (dry-run)
	flakeRef := repoDir + "#homeConfigurations." + hostname + ".activationPackage"
	cmd := exec.CommandContext(ctx, "nix", "build", "--dry-run", "--json", flakeRef)
	cmd.Dir = repoDir
	output, err := cmd.Output()

	if err != nil {
		// Nix evaluation might fail for various reasons
		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Cannot evaluate Home Manager config",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Check if current store path matches what would be built
	// The output contains paths like /nix/store/xxx-home-manager-generation
	outStr := string(output)
	if strings.Contains(outStr, currentPath) {
		return protocol.StatusCheck{
			Status:    "ok",
			Message:   "Home Manager is current",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	return protocol.StatusCheck{
		Status:    "outdated",
		Message:   "Home Manager needs switch",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// formatDaysAgo formats the number of days since last update.
func formatDaysAgo(days int) string {
	switch {
	case days == 0:
		return "Updated today"
	case days == 1:
		return "Updated yesterday"
	default:
		return "Updated " + itoa(days) + " days ago"
	}
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

