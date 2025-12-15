package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// RemoteVersion represents the version.json from GitHub Pages.
type RemoteVersion struct {
	GitCommit string `json:"gitCommit"`
	Message   string `json:"message"`
	Branch    string `json:"branch"`
	Timestamp string `json:"timestamp"`
	Repo      string `json:"repo"`
}

// VersionFetcher periodically fetches version info from a remote URL.
type VersionFetcher struct {
	url       string
	ttl       time.Duration
	client    *http.Client
	mu        sync.RWMutex
	cached    *RemoteVersion
	fetchedAt time.Time
	lastError error
}

// NewVersionFetcher creates a new version fetcher.
func NewVersionFetcher(url string, ttl time.Duration) *VersionFetcher {
	return &VersionFetcher{
		url: url,
		ttl: ttl,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins the background fetch loop.
func (vf *VersionFetcher) Start(ctx context.Context) {
	// Fetch immediately on start
	vf.fetch()

	go func() {
		ticker := time.NewTicker(vf.ttl)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				vf.fetch()
			}
		}
	}()
}

func (vf *VersionFetcher) fetch() {
	resp, err := vf.client.Get(vf.url)
	if err != nil {
		vf.mu.Lock()
		vf.lastError = err
		vf.mu.Unlock()
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var version RemoteVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		vf.mu.Lock()
		vf.lastError = err
		vf.mu.Unlock()
		return
	}

	vf.mu.Lock()
	vf.cached = &version
	vf.fetchedAt = time.Now()
	vf.lastError = nil
	vf.mu.Unlock()
}

// GetLatest returns the cached latest version, or nil if not available.
func (vf *VersionFetcher) GetLatest() *RemoteVersion {
	vf.mu.RLock()
	defer vf.mu.RUnlock()
	return vf.cached
}

// GetGitStatus compares the given generation with the latest.
// Returns: status ("ok", "outdated", "unknown"), message string, checkedAt timestamp
func (vf *VersionFetcher) GetGitStatus(agentGeneration string) (status, message, checkedAt string) {
	vf.mu.RLock()
	defer vf.mu.RUnlock()

	checkedAt = time.Now().UTC().Format(time.RFC3339)

	if vf.cached == nil {
		return "unknown", "Version tracking not available", checkedAt
	}

	latestCommit := vf.cached.GitCommit
	if latestCommit == "" {
		return "unknown", "No commit in version.json", checkedAt
	}

	if agentGeneration == "" {
		return "unknown", "Agent has not reported generation", checkedAt
	}

	// Compare short hashes (7 chars) for display, but full for comparison
	latestShort := latestCommit
	if len(latestShort) > 7 {
		latestShort = latestShort[:7]
	}
	agentShort := agentGeneration
	if len(agentShort) > 7 {
		agentShort = agentShort[:7]
	}

	// Check if agent is up to date
	// Match either full hash or prefix
	if agentGeneration == latestCommit ||
		(len(agentGeneration) >= 7 && len(latestCommit) >= 7 && agentGeneration[:7] == latestCommit[:7]) {
		return "ok", "Up to date with remote", checkedAt
	}

	message = "Behind remote (" + agentShort + " → " + latestShort + ")"
	if vf.cached.Message != "" {
		message += ": " + vf.cached.Message
	}
	return "outdated", message, checkedAt
}

// HasData returns true if we have fetched version data.
func (vf *VersionFetcher) HasData() bool {
	vf.mu.RLock()
	defer vf.mu.RUnlock()
	return vf.cached != nil
}

