package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ensureRepoExists ensures the repository is cloned and ready.
// For isolated mode (RepoURL set), it will auto-clone if needed.
func (a *Agent) ensureRepoExists() error {
	// If not in isolated mode (no RepoURL), just verify directory exists
	if a.cfg.RepoURL == "" {
		if _, err := os.Stat(a.cfg.RepoDir); os.IsNotExist(err) {
			return err
		}
		return nil
	}

	// Isolated mode: ensure repo exists
	gitDir := filepath.Join(a.cfg.RepoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		a.log.Info().
			Str("url", a.cfg.RepoURL).
			Str("path", a.cfg.RepoDir).
			Msg("cloning repository (isolated mode)")

		return a.cloneRepo()
	}

	// Verify it's a valid git repo
	if !a.isValidRepo() {
		a.log.Warn().Msg("repository appears corrupt, re-cloning")
		if err := os.RemoveAll(a.cfg.RepoDir); err != nil {
			return err
		}
		return a.cloneRepo()
	}

	a.log.Debug().Str("path", a.cfg.RepoDir).Msg("repository exists and is valid")
	return nil
}

// cloneRepo clones the repository from RepoURL.
func (a *Agent) cloneRepo() error {
	// Ensure parent directory exists with proper permissions
	parentDir := filepath.Dir(a.cfg.RepoDir)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return err
	}

	args := []string{
		"clone",
		"--branch", a.cfg.Branch,
		"--single-branch",
		a.cfg.RepoURL,
		a.cfg.RepoDir,
	}

	cmd := exec.Command("git", args...)

	// Set SSH key if configured
	if a.cfg.SSHKey != "" {
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+a.cfg.SSHKey+" -o StrictHostKeyChecking=no",
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		a.log.Error().
			Err(err).
			Str("output", string(output)).
			Msg("failed to clone repository")
		return err
	}

	// Set directory permissions to 0700 (owner only)
	if err := os.Chmod(a.cfg.RepoDir, 0700); err != nil {
		a.log.Warn().Err(err).Msg("failed to set repo permissions")
	}

	a.log.Info().
		Str("path", a.cfg.RepoDir).
		Str("branch", a.cfg.Branch).
		Msg("repository cloned successfully")
	return nil
}

// isValidRepo checks if the repo directory is a valid git repository.
func (a *Agent) isValidRepo() bool {
	cmd := exec.Command("git", "-C", a.cfg.RepoDir, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// resetToRemote does a hard reset to the remote branch (clean slate).
func (a *Agent) resetToRemote() error {
	args := []string{
		"-C", a.cfg.RepoDir,
		"reset", "--hard", "origin/" + a.cfg.Branch,
	}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		a.log.Error().
			Err(err).
			Str("output", string(output)).
			Msg("failed to reset to remote")
		return err
	}
	return nil
}

// cleanUntracked removes untracked files from the repo.
func (a *Agent) cleanUntracked() error {
	args := []string{
		"-C", a.cfg.RepoDir,
		"clean", "-fd",
	}
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

// getDefaultRepoDir returns the platform-specific default isolated repo path.
// These paths match what the Nix modules configure.
func getDefaultRepoDir() string {
	if runtime.GOOS == "darwin" {
		// macOS: ~/.local/state/nixfleet-agent/repo (matches home-manager module)
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "state", "nixfleet-agent", "repo")
	}
	// NixOS: /var/lib/nixfleet-agent/repo (matches nixos module StateDirectory)
	return "/var/lib/nixfleet-agent/repo"
}

