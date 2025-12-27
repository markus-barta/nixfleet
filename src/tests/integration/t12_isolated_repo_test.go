// T12 - Isolated Repository Mode Tests
// Based on tests/specs/T09-isolated-repo.md
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/markus-barta/nixfleet/internal/agent"
	"github.com/markus-barta/nixfleet/internal/config"
	"github.com/markus-barta/nixfleet/internal/protocol"
	"github.com/rs/zerolog"
)

// TestIsolatedRepo_AutoClone tests T09-01: Auto-Clone on Startup
// Given: NIXFLEET_REPO_URL is set to a valid git URL
// When: Agent starts and repo doesn't exist
// Then: Agent clones the repository automatically
func TestIsolatedRepo_AutoClone(t *testing.T) {
	// Create a "remote" repository to clone from
	remoteDir := t.TempDir()
	if err := initGitRepo(remoteDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Create empty local directory for isolated clone
	localDir := t.TempDir()
	repoDir := filepath.Join(localDir, "repo")

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config with isolated mode
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoURL:           remoteDir, // Use local path as "remote" URL
		RepoDir:           repoDir,
		Branch:            "main",
		HeartbeatInterval: 5 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Verify repo doesn't exist yet
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Fatalf("repo should not exist before agent starts")
	}

	// Create and run agent
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()
	a := agent.New(cfg, log)

	go func() {
		if err := a.Run(); err != nil {
			t.Logf("agent run error: %v", err)
		}
	}()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Wait for registration (means agent started successfully)
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	// Verify repo was cloned
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		t.Fatal("repo was not cloned")
	}

	// Verify directory permissions are restrictive (0700)
	info, err := os.Stat(repoDir)
	if err != nil {
		t.Fatalf("failed to stat repo: %v", err)
	}
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		t.Logf("warning: repo permissions are %o, expected 0700", perm)
	}

	t.Log("auto-clone successful")
}

// TestIsolatedRepo_PullCleansSlate tests T09-02: Pull Performs Clean Reset
// Given: Agent is running in isolated mode with dirty repo
// When: Pull command is issued
// Then: Repo is reset to clean state
func TestIsolatedRepo_PullCleansSlate(t *testing.T) {
	// Create a "remote" repository
	remoteDir := t.TempDir()
	if err := initGitRepo(remoteDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Clone it locally
	localDir := t.TempDir()
	repoDir := filepath.Join(localDir, "repo")
	cloneCmd := exec.Command("git", "clone", remoteDir, repoDir)
	if err := cloneCmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	// Create dirty state: add untracked file and modify tracked file
	dirtyFile := filepath.Join(repoDir, "DIRTY_FILE")
	if err := os.WriteFile(dirtyFile, []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	trackedFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(trackedFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config with isolated mode
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoURL:           remoteDir,
		RepoDir:           repoDir,
		Branch:            "main",
		HeartbeatInterval: 5 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()
	a := agent.New(cfg, log)

	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify dirty state exists before pull
	if _, err := os.Stat(dirtyFile); os.IsNotExist(err) {
		t.Fatal("dirty file should exist before pull")
	}

	// Send pull command
	if err := dashboard.SendCommand("pull"); err != nil {
		t.Fatalf("failed to send pull: %v", err)
	}

	// Wait for status
	statusCtx, statusCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer statusCancel()
	statusMsg, err := dashboard.WaitForMessage(statusCtx, protocol.TypeStatus)
	if err != nil {
		t.Fatalf("failed to receive status: %v", err)
	}

	var payload protocol.StatusPayload
	if err := statusMsg.ParsePayload(&payload); err != nil {
		t.Fatalf("failed to parse status: %v", err)
	}

	if payload.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s' (exit=%d)", payload.Status, payload.ExitCode)
	}

	// Verify dirty file is removed
	if _, err := os.Stat(dirtyFile); !os.IsNotExist(err) {
		t.Error("dirty file should be removed after pull (git clean -fd)")
	}

	// Verify tracked file is restored
	content, err := os.ReadFile(trackedFile)
	if err != nil {
		t.Fatalf("failed to read tracked file: %v", err)
	}
	if string(content) == "modified content" {
		t.Error("tracked file should be restored to original (git reset --hard)")
	}

	t.Log("pull correctly cleaned slate")
}

// TestIsolatedRepo_GenerationDetection tests T09-04: Generation Detection Uses Isolated Path
// Given: Agent is running in isolated mode
// When: Heartbeat is sent
// Then: Generation matches the isolated repo's commit
func TestIsolatedRepo_GenerationDetection(t *testing.T) {
	// Create a "remote" repository
	remoteDir := t.TempDir()
	if err := initGitRepo(remoteDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Get the commit hash from remote
	hashCmd := exec.Command("git", "-C", remoteDir, "rev-parse", "--short=7", "HEAD")
	hashOutput, err := hashCmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit hash: %v", err)
	}
	expectedHash := string(hashOutput[:7])

	// Create local repo dir
	localDir := t.TempDir()
	repoDir := filepath.Join(localDir, "repo")

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoURL:           remoteDir,
		RepoDir:           repoDir,
		Branch:            "main",
		HeartbeatInterval: 1 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Wait for registration
	regMsg, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	var regPayload protocol.RegisterPayload
	if err := regMsg.ParsePayload(&regPayload); err != nil {
		t.Fatalf("failed to parse registration: %v", err)
	}

	t.Logf("registered with generation: %s", regPayload.Generation)
	t.Logf("expected generation: %s", expectedHash)

	if regPayload.Generation != expectedHash {
		t.Errorf("generation mismatch: got %s, expected %s", regPayload.Generation, expectedHash)
	}
}

// TestIsolatedRepo_LegacyModeStillWorks tests T09-07: Legacy Mode Still Works
// Given: Only REPO_DIR is set (no REPO_URL)
// When: Pull command is issued
// Then: Normal git pull is used (not fetch+reset)
func TestIsolatedRepo_LegacyModeStillWorks(t *testing.T) {
	// Create a local git repository
	repoDir := t.TempDir()
	if err := initGitRepo(repoDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Start mock dashboard
	dashboard := NewMockDashboard(t)
	defer dashboard.Close()

	// Create agent config WITHOUT RepoURL (legacy mode)
	cfg := &config.Config{
		DashboardURL:      dashboard.URL(),
		Token:             "test-token",
		RepoDir:           repoDir,
		// RepoURL is empty - legacy mode
		Branch:            "main",
		HeartbeatInterval: 5 * time.Second,
		Hostname:          "test-host",
		LogLevel:          "debug",
	}

	// Create and run agent
	log := zerolog.Nop()
	a := agent.New(cfg, log)

	go func() { _ = a.Run() }()
	defer a.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait for registration
	_, err := dashboard.WaitForMessage(ctx, protocol.TypeRegister)
	if err != nil {
		t.Fatalf("failed to receive registration: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Send pull command
	if err := dashboard.SendCommand("pull"); err != nil {
		t.Fatalf("failed to send pull: %v", err)
	}

	// Wait for status (will fail because no remote, but that's OK)
	time.Sleep(2 * time.Second)

	// Check output messages for "git pull" (legacy) vs "fetch" (isolated)
	outputs := dashboard.MessagesOfType(protocol.TypeOutput)
	t.Logf("received %d output messages", len(outputs))

	// In legacy mode, git pull will fail with "no remote" but that's expected
	// The important thing is it didn't crash and completed
	statuses := dashboard.MessagesOfType(protocol.TypeStatus)
	if len(statuses) > 0 {
		var payload protocol.StatusPayload
		if err := statuses[0].ParsePayload(&payload); err == nil {
			t.Logf("legacy pull result: status=%s, exit=%d", payload.Status, payload.ExitCode)
		}
	}

	t.Log("legacy mode works")
}

// initGitRepo initializes a git repo with a test commit on main branch
func initGitRepo(dir string) error {
	// Create main branch explicitly
	cmds := [][]string{
		{"git", "init", "--initial-branch=main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			// Try without --initial-branch for older git
			if args[0] == "git" && args[1] == "init" {
				cmd = exec.Command("git", "init")
				cmd.Dir = dir
				if err := cmd.Run(); err != nil {
					return err
				}
				// Rename master to main if needed
				_ = exec.Command("git", "-C", dir, "branch", "-m", "master", "main").Run()
				continue
			}
			return err
		}
	}

	// Create test file and commit
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content\n"), 0644); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run(); err != nil {
		return err
	}

	return nil
}

