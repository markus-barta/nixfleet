package github

import (
	"context"
	"sync"
	"time"
)

// MockClient is a test implementation of the Client interface.
// Use it in unit tests to avoid real GitHub API calls.
type MockClient struct {
	mu sync.Mutex

	// PRs is the list of PRs to return from ListOpenPRs.
	PRs []PullRequest

	// PRByNumber maps PR numbers to PullRequest for GetPR.
	PRByNumber map[int]*PullRequest

	// MergeResults maps PR numbers to MergeResult for MergePR.
	MergeResults map[int]*MergeResult

	// LatestCommits maps "owner/repo/branch" to commit SHA.
	LatestCommits map[string]string

	// Errors to return from each method (set to simulate failures).
	ListOpenPRsError     error
	GetPRError           error
	MergePRError         error
	GetLatestCommitError error

	// Call tracking for assertions.
	ListOpenPRsCalls     []listOpenPRsCall
	GetPRCalls           []getPRCall
	MergePRCalls         []mergePRCall
	GetLatestCommitCalls []getLatestCommitCall
}

type listOpenPRsCall struct {
	Owner, Repo string
}

type getPRCall struct {
	Owner, Repo string
	Number      int
}

type mergePRCall struct {
	Owner, Repo string
	Number      int
	Method      string
}

type getLatestCommitCall struct {
	Owner, Repo, Branch string
}

// NewMockClient creates a new mock client with empty state.
func NewMockClient() *MockClient {
	return &MockClient{
		PRByNumber:    make(map[int]*PullRequest),
		MergeResults:  make(map[int]*MergeResult),
		LatestCommits: make(map[string]string),
	}
}

// ListOpenPRs returns the configured PRs or error.
func (m *MockClient) ListOpenPRs(ctx context.Context, owner, repo string) ([]PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ListOpenPRsCalls = append(m.ListOpenPRsCalls, listOpenPRsCall{Owner: owner, Repo: repo})

	if m.ListOpenPRsError != nil {
		return nil, m.ListOpenPRsError
	}
	return m.PRs, nil
}

// GetPR returns the configured PR or error.
func (m *MockClient) GetPR(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetPRCalls = append(m.GetPRCalls, getPRCall{Owner: owner, Repo: repo, Number: number})

	if m.GetPRError != nil {
		return nil, m.GetPRError
	}
	if pr, ok := m.PRByNumber[number]; ok {
		return pr, nil
	}
	return nil, nil
}

// MergePR returns the configured merge result or error.
func (m *MockClient) MergePR(ctx context.Context, owner, repo string, number int, method string) (*MergeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MergePRCalls = append(m.MergePRCalls, mergePRCall{Owner: owner, Repo: repo, Number: number, Method: method})

	if m.MergePRError != nil {
		return nil, m.MergePRError
	}
	if result, ok := m.MergeResults[number]; ok {
		return result, nil
	}
	// Default success
	return &MergeResult{Merged: true, SHA: "abc123", Message: "Pull Request successfully merged"}, nil
}

// GetLatestCommit returns the configured commit SHA or error.
func (m *MockClient) GetLatestCommit(ctx context.Context, owner, repo, branch string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetLatestCommitCalls = append(m.GetLatestCommitCalls, getLatestCommitCall{Owner: owner, Repo: repo, Branch: branch})

	if m.GetLatestCommitError != nil {
		return "", m.GetLatestCommitError
	}
	key := owner + "/" + repo + "/" + branch
	if sha, ok := m.LatestCommits[key]; ok {
		return sha, nil
	}
	return "mock-sha-" + branch, nil
}

// AddFlakeUpdatePR adds a typical flake.lock update PR to the mock.
func (m *MockClient) AddFlakeUpdatePR(number int, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pr := PullRequest{
		Number:         number,
		Title:          title,
		State:          "open",
		HTMLURL:        "https://github.com/test/repo/pull/" + string(rune('0'+number)),
		Labels:         []Label{{Name: "automated"}},
		CreatedAt:      time.Now().Add(-1 * time.Hour),
		UpdatedAt:      time.Now(),
		MergeableState: "mergeable",
		Head:           GitRef{SHA: "head-sha-" + string(rune('0'+number))},
		Base:           GitRef{Ref: "main"},
	}
	m.PRs = append(m.PRs, pr)
	m.PRByNumber[number] = &pr
}

// Reset clears all state and call history.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PRs = nil
	m.PRByNumber = make(map[int]*PullRequest)
	m.MergeResults = make(map[int]*MergeResult)
	m.LatestCommits = make(map[string]string)
	m.ListOpenPRsError = nil
	m.GetPRError = nil
	m.MergePRError = nil
	m.GetLatestCommitError = nil
	m.ListOpenPRsCalls = nil
	m.GetPRCalls = nil
	m.MergePRCalls = nil
	m.GetLatestCommitCalls = nil
}

// Ensure MockClient implements Client interface.
var _ Client = (*MockClient)(nil)

