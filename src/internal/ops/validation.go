package ops

import (
	"fmt"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// TIMEOUT CONFIGURATION
// Migrated from command_state.go (P2800)
// ═══════════════════════════════════════════════════════════════════════════

// TimeoutConfig holds timeout thresholds for ops.
type TimeoutConfig struct {
	WarningTimeout   time.Duration
	HardTimeout      time.Duration
	ReconnectTimeout time.Duration
}

// DefaultTimeouts returns the default timeout configuration for each op.
var DefaultTimeouts = map[string]TimeoutConfig{
	"pull": {
		WarningTimeout:   2 * time.Minute,
		HardTimeout:      5 * time.Minute,
		ReconnectTimeout: 0, // N/A for pull
	},
	"switch": {
		WarningTimeout:   10 * time.Minute,
		HardTimeout:      30 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	},
	"pull-switch": {
		WarningTimeout:   12 * time.Minute,
		HardTimeout:      35 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	},
	"test": {
		WarningTimeout:   5 * time.Minute,
		HardTimeout:      10 * time.Minute,
		ReconnectTimeout: 0, // N/A for test
	},
}

// GetTimeoutConfig returns timeout config for an op.
func GetTimeoutConfig(opID string) TimeoutConfig {
	if cfg, ok := DefaultTimeouts[opID]; ok {
		return cfg
	}
	// Default fallback
	return TimeoutConfig{
		WarningTimeout:   5 * time.Minute,
		HardTimeout:      15 * time.Minute,
		ReconnectTimeout: 90 * time.Second,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// BINARY FRESHNESS (P2810 - 3-Layer Detection)
// ═══════════════════════════════════════════════════════════════════════════

// AgentFreshness tracks 3-layer binary verification.
type AgentFreshness struct {
	SourceCommit string `json:"source_commit"` // Layer 1: Git commit (ldflags)
	StorePath    string `json:"store_path"`    // Layer 2: Nix store path
	BinaryHash   string `json:"binary_hash"`   // Layer 3: SHA256 of binary
}

// FreshnessVerdict is the result of binary freshness comparison.
type FreshnessVerdict string

const (
	FreshnessUnknown    FreshnessVerdict = "unknown"
	FreshnessFresh      FreshnessVerdict = "fresh"      // Binary changed
	FreshnessSuspicious FreshnessVerdict = "suspicious" // Commit changed, binary didn't
	FreshnessStale      FreshnessVerdict = "stale"      // Nothing changed
)

// CompareFreshness compares before/after freshness to detect stale binaries.
func CompareFreshness(before, after AgentFreshness) (FreshnessVerdict, string) {
	commitChanged := before.SourceCommit != after.SourceCommit && before.SourceCommit != "" && after.SourceCommit != ""
	pathChanged := before.StorePath != after.StorePath && before.StorePath != "" && after.StorePath != ""
	hashChanged := before.BinaryHash != after.BinaryHash && before.BinaryHash != "" && after.BinaryHash != ""

	// If we don't have enough data, can't determine
	if before.StorePath == "" || before.BinaryHash == "" || after.StorePath == "" || after.BinaryHash == "" {
		return FreshnessUnknown, "Insufficient data for binary freshness verification"
	}

	// Any path or hash change = definitely fresh
	if pathChanged || hashChanged {
		return FreshnessFresh, "Agent binary updated successfully"
	}

	// Commit changed but binary didn't = suspicious (cache issue?)
	if commitChanged {
		return FreshnessSuspicious, "Source commit changed but binary unchanged (Nix cache issue?)"
	}

	// Nothing changed = stale
	return FreshnessStale, "Agent running old binary - run nix-collect-garbage -d and switch again"
}

// ═══════════════════════════════════════════════════════════════════════════
// PRE-CONDITION VALIDATORS
// These are used by Op.Validate functions.
// ═══════════════════════════════════════════════════════════════════════════

// ValidateCanExecute checks if ANY op can run on this host.
func ValidateCanExecute(host Host) *ValidationError {
	if !host.IsOnline() {
		return &ValidationError{Code: "host_offline", Message: "Host is offline"}
	}
	if host.HasPendingCommand() {
		return &ValidationError{
			Code:    "command_pending",
			Message: fmt.Sprintf("Command '%s' already running", host.GetPendingCommand()),
		}
	}
	return nil
}

// ValidatePull checks if Pull is meaningful for this host.
func ValidatePull(host Host) *ValidationError {
	if err := ValidateCanExecute(host); err != nil {
		return err
	}

	gitStatus := host.GetGitStatus()
	if gitStatus == "error" {
		return &ValidationError{Code: "remote_unavailable", Message: "Cannot verify remote desired state (Git status error)"}
	}
	if gitStatus == "ok" {
		return &ValidationError{Code: "already_current", Message: "Git already up to date"}
	}
	return nil // outdated or unknown is valid
}

// ValidateSwitch checks if Switch is meaningful for this host.
func ValidateSwitch(host Host) *ValidationError {
	if err := ValidateCanExecute(host); err != nil {
		return err
	}

	// Check if git is current (prerequisite for meaningful switch)
	gitStatus := host.GetGitStatus()
	if gitStatus == "error" {
		return &ValidationError{Code: "remote_unavailable", Message: "Cannot verify remote desired state (Git status error)"}
	}
	if gitStatus == "outdated" {
		return &ValidationError{Code: "git_outdated", Message: "Pull required before switch (git outdated)"}
	}

	systemStatus := host.GetSystemStatus()
	if systemStatus == "ok" && !host.IsAgentOutdated() {
		return &ValidationError{Code: "already_current", Message: "System already up to date"}
	}
	return nil // outdated or unknown is valid
}

// ValidateTest checks if Test is meaningful for this host.
func ValidateTest(host Host) *ValidationError {
	return ValidateCanExecute(host)
}

// ValidatePullSwitch checks if Pull+Switch sequence is meaningful.
func ValidatePullSwitch(host Host) *ValidationError {
	if err := ValidateCanExecute(host); err != nil {
		return err
	}

	// At least one of git or system should need update
	gitStatus := host.GetGitStatus()
	systemStatus := host.GetSystemStatus()

	if gitStatus == "error" {
		return &ValidationError{Code: "remote_unavailable", Message: "Cannot verify remote desired state (Git status error)"}
	}

	if gitStatus == "ok" && systemStatus == "ok" && !host.IsAgentOutdated() {
		return &ValidationError{Code: "already_current", Message: "Both git and system already up to date"}
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// POST-CONDITION VALIDATORS
// These are used by Op.PostCheck functions.
// ═══════════════════════════════════════════════════════════════════════════

// HostSnapshot captures host state before an op for post-validation comparison.
type HostSnapshot struct {
	HostID        string
	Generation    string
	AgentVersion  string
	AgentOutdated bool
	GitStatus     string
	LockStatus    string
	SystemStatus  string
	Freshness     *AgentFreshness
}

// CaptureSnapshot captures the current state of a host.
func CaptureSnapshot(host Host) HostSnapshot {
	return HostSnapshot{
		HostID:        host.GetID(),
		Generation:    host.GetGeneration(),
		AgentVersion:  host.GetAgentVersion(),
		AgentOutdated: host.IsAgentOutdated(),
		GitStatus:     host.GetGitStatus(),
		LockStatus:    host.GetLockStatus(),
		SystemStatus:  host.GetSystemStatus(),
	}
}

// PostCheckPull checks if Pull achieved its goal.
func PostCheckPull(before, after HostSnapshot, exitCode int) *ValidationError {
	if exitCode != 0 {
		return &ValidationError{
			Code:    "exit_nonzero",
			Message: fmt.Sprintf("Pull failed with exit code %d", exitCode),
		}
	}

	// Goal: Git compartment should now be ok
	if after.GitStatus == "ok" {
		return nil // Success
	}

	// Check if generation changed (partial success)
	if before.Generation != after.Generation {
		return nil // Some progress made
	}

	return &ValidationError{
		Code:    "goal_not_achieved",
		Message: "Pull completed (exit 0) but git compartment still outdated",
	}
}

// PostCheckSwitch checks if Switch achieved its goal.
func PostCheckSwitch(before, after HostSnapshot, exitCode int) *ValidationError {
	if exitCode != 0 {
		return &ValidationError{
			Code:    "exit_nonzero",
			Message: fmt.Sprintf("Switch failed with exit code %d", exitCode),
		}
	}

	// Goal: System compartment should now be ok
	if after.SystemStatus == "ok" {
		return nil // Success
	}

	// Check if we're waiting for agent restart
	if after.AgentOutdated && before.AgentVersion != after.AgentVersion {
		return nil // Pending restart, not an error
	}

	return &ValidationError{
		Code:    "goal_not_achieved",
		Message: "Switch completed (exit 0) but system compartment still outdated",
	}
}

// PostCheckTest checks if Test passed.
func PostCheckTest(exitCode int) *ValidationError {
	if exitCode != 0 {
		return &ValidationError{
			Code:    "test_failed",
			Message: fmt.Sprintf("Test failed with exit code %d", exitCode),
		}
	}
	return nil
}

// PostCheckPullSwitch checks if Pull+Switch achieved combined goal.
func PostCheckPullSwitch(before, after HostSnapshot, exitCode int) *ValidationError {
	if exitCode != 0 {
		return &ValidationError{
			Code:    "exit_nonzero",
			Message: fmt.Sprintf("Pull+Switch failed with exit code %d", exitCode),
		}
	}

	gitOK := after.GitStatus == "ok"
	systemOK := after.SystemStatus == "ok"

	if gitOK && systemOK {
		return nil // Full success
	}

	if gitOK && !systemOK {
		return &ValidationError{
			Code:    "partial_git_only",
			Message: "Pull succeeded but switch did not update system",
		}
	}

	if !gitOK && systemOK {
		return nil // System updated, git might be stale data
	}

	return &ValidationError{
		Code:    "goal_not_achieved",
		Message: "Pull+Switch completed but neither git nor system updated",
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════

// ShortHash returns the first 7 characters of a hash.
func ShortHash(hash string) string {
	if len(hash) >= 7 {
		return hash[:7]
	}
	return hash
}

// ShortPath truncates a path for display.
func ShortPath(path string) string {
	if len(path) > 40 {
		return "..." + path[len(path)-37:]
	}
	return path
}

