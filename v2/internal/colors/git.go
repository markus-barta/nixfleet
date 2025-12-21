package colors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// NixcfgRepo manages operations on the nixcfg repository for color changes.
type NixcfgRepo struct {
	repoPath    string
	repoURL     string // e.g., https://github.com/markus-barta/nixcfg.git
	token       string // GitHub token for authentication
	commitMode  string // "push" or "pr"
	log         zerolog.Logger
	mu          sync.Mutex
}

// NewNixcfgRepo creates a new nixcfg repository manager.
func NewNixcfgRepo(repoPath, repoURL, token, commitMode string, log zerolog.Logger) *NixcfgRepo {
	// Convert owner/repo to full URL if needed
	if !strings.HasPrefix(repoURL, "http") && !strings.HasPrefix(repoURL, "git@") {
		// Assume it's owner/repo format
		repoURL = fmt.Sprintf("https://github.com/%s.git", repoURL)
	}

	return &NixcfgRepo{
		repoPath:   repoPath,
		repoURL:    repoURL,
		token:      token,
		commitMode: commitMode,
		log:        log.With().Str("component", "nixcfg-repo").Logger(),
	}
}

// EnsureCloned makes sure the repo is cloned and up to date.
func (r *NixcfgRepo) EnsureCloned() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if repo directory exists
	if _, err := os.Stat(filepath.Join(r.repoPath, ".git")); os.IsNotExist(err) {
		// Clone the repo
		r.log.Info().Str("path", r.repoPath).Msg("cloning nixcfg repository")
		if err := r.clone(); err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}
	} else {
		// Pull latest changes
		r.log.Debug().Msg("pulling latest changes")
		if err := r.pull(); err != nil {
			return fmt.Errorf("failed to pull: %w", err)
		}
	}

	return nil
}

// clone clones the repository.
func (r *NixcfgRepo) clone() error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(r.repoPath), 0755); err != nil {
		return err
	}

	// Build clone URL with token
	cloneURL := r.repoURL
	if r.token != "" && strings.HasPrefix(cloneURL, "https://") {
		// Insert token into URL: https://TOKEN@github.com/...
		cloneURL = strings.Replace(cloneURL, "https://", fmt.Sprintf("https://%s@", r.token), 1)
	}

	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, r.repoPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// pull fetches and merges the latest changes.
func (r *NixcfgRepo) pull() error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = r.repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If pull fails due to local changes, try to reset
		r.log.Warn().Str("output", string(output)).Msg("pull failed, attempting reset")
		if resetErr := r.reset(); resetErr != nil {
			return fmt.Errorf("git pull failed: %w, reset also failed: %v", err, resetErr)
		}
		// Retry pull
		cmd = exec.Command("git", "pull", "--ff-only")
		cmd.Dir = r.repoPath
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git pull failed after reset: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}

// reset resets any local changes.
func (r *NixcfgRepo) reset() error {
	cmd := exec.Command("git", "reset", "--hard", "HEAD")
	cmd.Dir = r.repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// ThemePalettesPath returns the path to theme-palettes.nix.
func (r *NixcfgRepo) ThemePalettesPath() string {
	return filepath.Join(r.repoPath, "modules", "uzumaki", "theme", "theme-palettes.nix")
}

// ReadThemePalettes reads the current theme-palettes.nix content.
func (r *NixcfgRepo) ReadThemePalettes() (string, error) {
	data, err := os.ReadFile(r.ThemePalettesPath())
	if err != nil {
		return "", fmt.Errorf("failed to read theme-palettes.nix: %w", err)
	}
	return string(data), nil
}

// WriteThemePalettes writes new content to theme-palettes.nix.
func (r *NixcfgRepo) WriteThemePalettes(content string) error {
	if err := os.WriteFile(r.ThemePalettesPath(), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write theme-palettes.nix: %w", err)
	}
	return nil
}

// CommitAndPush commits the changes and pushes to the remote.
func (r *NixcfgRepo) CommitAndPush(hostname, paletteName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stage the file
	cmd := exec.Command("git", "add", r.ThemePalettesPath())
	cmd.Dir = r.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\nOutput: %s", err, string(output))
	}

	// Commit
	commitMsg := fmt.Sprintf("theme(%s): change color to %s", hostname, paletteName)
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = r.repoPath
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=NixFleet",
		"GIT_AUTHOR_EMAIL=nixfleet@localhost",
		"GIT_COMMITTER_NAME=NixFleet",
		"GIT_COMMITTER_EMAIL=nixfleet@localhost",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\nOutput: %s", err, string(output))
	}

	// Push
	if r.commitMode == "push" {
		// Build push URL with token
		pushURL := r.repoURL
		if r.token != "" && strings.HasPrefix(pushURL, "https://") {
			pushURL = strings.Replace(pushURL, "https://", fmt.Sprintf("https://%s@", r.token), 1)
		}

		cmd = exec.Command("git", "push", pushURL, "HEAD:main")
		cmd.Dir = r.repoPath
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %w\nOutput: %s", err, string(output))
		}

		r.log.Info().
			Str("hostname", hostname).
			Str("palette", paletteName).
			Str("commit", commitMsg).
			Msg("pushed color change to nixcfg")
	} else {
		// PR mode - would create a branch and PR via GitHub API
		// For now, just log that we need this
		r.log.Warn().
			Str("hostname", hostname).
			Str("palette", paletteName).
			Msg("PR mode not yet implemented; commit created but not pushed")
		return fmt.Errorf("PR mode not yet implemented")
	}

	return nil
}

// UpdateHostColor updates a host's color in theme-palettes.nix and commits.
func (r *NixcfgRepo) UpdateHostColor(hostname string, paletteName string, customPalette *Palette) error {
	// Ensure repo is up to date
	if err := r.EnsureCloned(); err != nil {
		return err
	}

	// Read current content
	content, err := r.ReadThemePalettes()
	if err != nil {
		return err
	}

	// If custom palette, insert/update the palette definition
	if customPalette != nil {
		content, err = UpdateOrInsertCustomPalette(content, hostname, customPalette)
		if err != nil {
			return fmt.Errorf("failed to insert custom palette: %w", err)
		}
	}

	// Update hostPalette entry
	content, err = UpdateHostPalette(content, hostname, paletteName)
	if err != nil {
		return fmt.Errorf("failed to update hostPalette: %w", err)
	}

	// Write back
	if err := r.WriteThemePalettes(content); err != nil {
		return err
	}

	// Commit and push
	return r.CommitAndPush(hostname, paletteName)
}

