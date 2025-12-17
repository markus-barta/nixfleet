package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client defines the interface for GitHub API operations.
// This interface allows for easy mocking in tests.
type Client interface {
	// ListOpenPRs returns all open pull requests for a repository.
	ListOpenPRs(ctx context.Context, owner, repo string) ([]PullRequest, error)

	// GetPR returns a specific pull request by number.
	GetPR(ctx context.Context, owner, repo string, number int) (*PullRequest, error)

	// MergePR merges a pull request.
	// method can be "merge", "squash", or "rebase".
	MergePR(ctx context.Context, owner, repo string, number int, method string) (*MergeResult, error)

	// GetLatestCommit returns the SHA of the latest commit on a branch.
	GetLatestCommit(ctx context.Context, owner, repo, branch string) (string, error)
}

// HTTPClient is the real GitHub API client using HTTP.
type HTTPClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// ClientConfig holds configuration for the GitHub client.
type ClientConfig struct {
	Token   string // GitHub Personal Access Token
	BaseURL string // API base URL (default: https://api.github.com)
	Timeout time.Duration
}

// NewClient creates a new GitHub API client.
func NewClient(cfg ClientConfig) *HTTPClient {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPClient{
		token:   cfg.Token,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ListOpenPRs returns all open pull requests for a repository.
func (c *HTTPClient) ListOpenPRs(ctx context.Context, owner, repo string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100", c.baseURL, owner, repo)

	var prs []PullRequest
	if err := c.get(ctx, url, &prs); err != nil {
		return nil, fmt.Errorf("list open PRs: %w", err)
	}

	return prs, nil
}

// GetPR returns a specific pull request by number.
func (c *HTTPClient) GetPR(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, repo, number)

	var pr PullRequest
	if err := c.get(ctx, url, &pr); err != nil {
		return nil, fmt.Errorf("get PR #%d: %w", number, err)
	}

	return &pr, nil
}

// MergePR merges a pull request.
func (c *HTTPClient) MergePR(ctx context.Context, owner, repo string, number int, method string) (*MergeResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", c.baseURL, owner, repo, number)

	if method == "" {
		method = "merge"
	}

	body := fmt.Sprintf(`{"merge_method": "%s"}`, method)

	var result MergeResult
	if err := c.put(ctx, url, body, &result); err != nil {
		return nil, fmt.Errorf("merge PR #%d: %w", number, err)
	}

	return &result, nil
}

// GetLatestCommit returns the SHA of the latest commit on a branch.
func (c *HTTPClient) GetLatestCommit(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s", c.baseURL, owner, repo, branch)

	var info BranchInfo
	if err := c.get(ctx, url, &info); err != nil {
		return "", fmt.Errorf("get branch %s: %w", branch, err)
	}

	return info.Commit.SHA, nil
}

// get performs a GET request and unmarshals the response.
func (c *HTTPClient) get(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, result)
}

// put performs a PUT request and unmarshals the response.
func (c *HTTPClient) put(ctx context.Context, url, body string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.doRequest(req, result)
}

// doRequest executes an HTTP request with authentication.
func (c *HTTPClient) doRequest(req *http.Request, result interface{}) error {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response from %s: %w", req.URL.Path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s (status %d): %s", req.URL.Path, resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("parse response from %s: %w", req.URL.Path, err)
		}
	}

	return nil
}

// Ensure HTTPClient implements Client interface.
var _ Client = (*HTTPClient)(nil)

