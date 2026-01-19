package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/markus-barta/nixfleet/internal/protocol"
)

// StatusChecker handles update status checks for Lock, System, and Tests compartments.
type StatusChecker struct {
	a *Agent

	// Cached status
	lockStatus   protocol.StatusCheck
	systemStatus protocol.StatusCheck
	testsStatus  protocol.StatusCheck // P3900: Tests compartment

	// Last check times
	lastLockCheck   time.Time
	lastSystemCheck time.Time

	// Check intervals
	lockInterval   time.Duration // 5 minutes for lock
	systemInterval time.Duration // 5 minutes for system

	// Stale state detection (P1110)
	lastStatusUpdate time.Time // When status was last updated
	staleThreshold   time.Duration // How long before status is considered stale
}

// NewStatusChecker creates a new status checker.
func NewStatusChecker(a *Agent) *StatusChecker {
	return &StatusChecker{
		a:              a,
		lockInterval:   5 * time.Minute,
		systemInterval: 5 * time.Minute,
		staleThreshold: 5 * time.Minute, // Status considered stale after 5 minutes
	}
}

// GetStatus returns the current update status for Lock and System.
// Git status is computed dashboard-side from GitHub Pages.
// NOTE: System status is OPT-IN only (too heavy for automatic checks).
// Use RefreshSystem() explicitly when user requests it.
func (s *StatusChecker) GetStatus(ctx context.Context) *protocol.UpdateStatus {
	now := time.Now()

	// Check lock status if expired (lightweight: just git log)
	if now.Sub(s.lastLockCheck) > s.lockInterval {
		s.a.log.Debug().Msg("running lock status check")
		s.lockStatus = s.checkLockStatus(ctx)
		s.lastLockCheck = now
		s.a.log.Debug().
			Str("status", s.lockStatus.Status).
			Str("message", s.lockStatus.Message).
			Msg("lock status check completed")
	}

	// P1110: Detect stale system status (e.g., "working" stuck)
	s.detectAndResolveStaleStatus(now)

	// System status is NOT checked automatically - too expensive!
	// nix build --dry-run does full flake evaluation, which:
	// - Consumes significant CPU/RAM
	// - Can take 30-60+ seconds on small servers
	// - Makes hosts unresponsive
	// User must explicitly click refresh-system to check.

	return &protocol.UpdateStatus{
		// Git is computed dashboard-side
		Lock:   s.lockStatus,
		System: s.systemStatus, // Returns cached/unknown until explicit refresh
		Tests:  s.testsStatus,  // P3900: Tests compartment status
	}
}

// ForceRefresh forces an immediate refresh of lightweight status checks only.
// NOTE: Does NOT refresh System status (too expensive - nix build --dry-run).
// System status must be explicitly requested via RefreshSystem().
func (s *StatusChecker) ForceRefresh(ctx context.Context) {
	s.RefreshLock(ctx)
	// s.RefreshSystem(ctx) - REMOVED: too heavy for automatic refresh
}

// P2800: RefreshLock forces an immediate refresh of just the lock status.
func (s *StatusChecker) RefreshLock(ctx context.Context) {
	s.a.log.Debug().Msg("force-refreshing lock status")
	s.lockStatus = s.checkLockStatus(ctx)
	s.lastLockCheck = time.Now()
}

// P2800: RefreshSystem forces an immediate refresh of just the system status.
func (s *StatusChecker) RefreshSystem(ctx context.Context) {
	s.a.log.Debug().Msg("force-refreshing system status")
	s.systemStatus = s.checkSystemStatus(ctx)
	s.lastSystemCheck = time.Now()
}

// P2800: GetLockStatus returns the current lock status (cached).
func (s *StatusChecker) GetLockStatus() protocol.StatusCheck {
	return s.lockStatus
}

// P2800: GetSystemStatus returns the current system status (cached).
func (s *StatusChecker) GetSystemStatus() protocol.StatusCheck {
	return s.systemStatus
}

// SetSystemOk sets the system status to "ok" without running the expensive check.
// Called after a successful switch (exit 0) since we know the system is current.
func (s *StatusChecker) SetSystemOk(message string) {
	s.systemStatus = protocol.StatusCheck{
		Status:    "ok",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.lastSystemCheck = time.Now()
}

// SetSystemOutdated sets the system status to "outdated" without running the expensive check.
// Called after a successful pull (exit 0) since the system now differs from the flake.
func (s *StatusChecker) SetSystemOutdated(message string) {
	s.systemStatus = protocol.StatusCheck{
		Status:    "outdated",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.lastSystemCheck = time.Now()
}

// P3800: SetSystemError sets the system status to "error" without running the expensive check.
// Called after a failed switch to indicate the system is in an error state.
func (s *StatusChecker) SetSystemError(message string) {
	s.systemStatus = protocol.StatusCheck{
		Status:    "error",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.lastSystemCheck = time.Now()
}

// P3900: Tests compartment status methods

// SetTestsOk sets the tests status to "ok" after all tests pass.
func (s *StatusChecker) SetTestsOk(message string) {
	s.testsStatus = protocol.StatusCheck{
		Status:    "ok",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// SetTestsOutdated sets the tests status to "outdated" (not run yet).
// Called after a successful switch to indicate tests need to be run.
func (s *StatusChecker) SetTestsOutdated(message string) {
	s.testsStatus = protocol.StatusCheck{
		Status:    "outdated",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// SetTestsError sets the tests status to "error" after tests fail.
func (s *StatusChecker) SetTestsError(message string) {
	s.testsStatus = protocol.StatusCheck{
		Status:    "error",
		Message:   message,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// SetTestsWorking sets the tests status to "working" while tests are running.
func (s *StatusChecker) SetTestsWorking() {
	s.testsStatus = protocol.StatusCheck{
		Status:    "working",
		Message:   "Tests running...",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// P1100: SetSystemWorking sets the system status to "working" during switch operations.
func (s *StatusChecker) SetSystemWorking() {
	s.systemStatus = protocol.StatusCheck{
		Status:    "working",
		Message:   "Switch in progress...",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.lastSystemCheck = time.Now()
	s.lastStatusUpdate = time.Now()
}

// P1100: SetLockWorking sets the lock status to "working" during refresh-lock operations.
func (s *StatusChecker) SetLockWorking() {
	s.lockStatus = protocol.StatusCheck{
		Status:    "working",
		Message:   "Refreshing lock...",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.lastLockCheck = time.Now()
}

// GetTestsStatus returns the current tests status.
func (s *StatusChecker) GetTestsStatus() protocol.StatusCheck {
	return s.testsStatus
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
	cmd := exec.CommandContext(ctx, "nix", "build", "--experimental-features", "nix-command flakes", "--dry-run", "--json", flakeRef)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// If nix build fails, it might be because we're not root or other issues
		msg := "Cannot evaluate system (try with sudo)"
		if len(output) > 0 {
			errLines := strings.Split(string(output), "\n")
			if len(errLines) > 0 {
				msg = fmt.Sprintf("Evaluation failed: %s", strings.TrimSpace(errLines[0]))
			}
		} else {
			msg = fmt.Sprintf("Evaluation failed: %v", err)
		}

		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   msg,
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
	cmd := exec.CommandContext(ctx, "nix", "build", "--experimental-features", "nix-command flakes", "--dry-run", "--json", flakeRef)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput() // Capture stderr too

	if err != nil {
		// Nix evaluation might fail for various reasons
		msg := "Cannot evaluate Home Manager config"
		if len(output) > 0 {
			// Extract first line of error for brevity
			errLines := strings.Split(string(output), "\n")
			if len(errLines) > 0 {
				msg = fmt.Sprintf("Evaluation failed: %s", strings.TrimSpace(errLines[0]))
			}
		} else {
			msg = fmt.Sprintf("Evaluation failed: %v", err)
		}

		return protocol.StatusCheck{
			Status:    "unknown",
			Message:   msg,
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

// P1110: detectAndResolveStaleStatus detects and resolves stale status states.
// This handles the case where the agent crashes during a command (e.g., switch)
// and the status remains "working" indefinitely.
func (s *StatusChecker) detectAndResolveStaleStatus(now time.Time) {
	// Only check for stale "working" states
	if s.systemStatus.Status != "working" {
		return
	}

	// Check if status has been "working" for too long
	if s.lastStatusUpdate.IsZero() {
		return
	}

	elapsed := now.Sub(s.lastStatusUpdate)
	if elapsed > s.staleThreshold {
		s.a.log.Warn().
			Str("old_status", s.systemStatus.Status).
			Str("old_message", s.systemStatus.Message).
			Dur("elapsed", elapsed).
			Dur("threshold", s.staleThreshold).
			Msg("P1110: detected stale system status - resolving to unknown")

		// Resolve stale status to "unknown" with explanation
		s.systemStatus = protocol.StatusCheck{
			Status:    "unknown",
			Message:   "Status stale (agent may have restarted during command)",
			CheckedAt: now.UTC().Format(time.RFC3339),
		}
		s.lastSystemCheck = now
		s.lastStatusUpdate = now
	}
}
