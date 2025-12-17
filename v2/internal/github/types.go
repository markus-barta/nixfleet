// Package github provides a client for interacting with the GitHub API.
package github

import "time"

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	State          string    `json:"state"` // "open", "closed"
	HTMLURL        string    `json:"html_url"`
	Labels         []Label   `json:"labels"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	MergeableState string    `json:"mergeable_state"` // "mergeable", "conflicting", "unknown"
	Mergeable      *bool     `json:"mergeable"`
	Merged         bool      `json:"merged"`
	Head           GitRef    `json:"head"`
	Base           GitRef    `json:"base"`
	Body           string    `json:"body"`
	User           User      `json:"user"`
}

// Label represents a GitHub label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// GitRef represents a git reference (branch/commit).
type GitRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// User represents a GitHub user.
type User struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// Commit represents a GitHub commit.
type Commit struct {
	SHA     string     `json:"sha"`
	Message string     `json:"message"`
	Author  CommitUser `json:"author"`
	Date    time.Time  `json:"date"`
}

// CommitUser represents a commit author.
type CommitUser struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

// MergeResult represents the result of a merge operation.
type MergeResult struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

// BranchInfo represents branch information including latest commit.
type BranchInfo struct {
	Name   string `json:"name"`
	Commit Commit `json:"commit"`
}

// IsFlakeLockUpdate checks if this PR is a flake.lock update PR.
// Looks for "automated" label or "flake.lock" in title.
func (pr *PullRequest) IsFlakeLockUpdate() bool {
	// Check labels
	for _, label := range pr.Labels {
		if label.Name == "automated" || label.Name == "dependencies" {
			return true
		}
	}

	// Check title patterns (from nix-community/flake-checker action)
	titlePatterns := []string{
		"flake.lock",
		"Update flake",
		"Bump flake",
	}
	for _, pattern := range titlePatterns {
		if containsIgnoreCase(pr.Title, pattern) {
			return true
		}
	}

	return false
}

// IsMergeable returns true if the PR can be merged.
func (pr *PullRequest) IsMergeable() bool {
	if pr.State != "open" {
		return false
	}
	if pr.Merged {
		return false
	}
	if pr.Mergeable != nil && !*pr.Mergeable {
		return false
	}
	return pr.MergeableState == "mergeable" || pr.MergeableState == ""
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && len(substr) > 0 &&
			(s[0]|32) >= 'a' && (s[0]|32) <= 'z' &&
			containsIgnoreCaseSlow(s, substr)))
}

func containsIgnoreCaseSlow(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

