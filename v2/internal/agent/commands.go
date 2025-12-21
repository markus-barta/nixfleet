package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/protocol"
)

// timeAfter returns a channel that receives after n seconds.
// Helper to avoid importing time in multiple places.
func timeAfter(seconds int) <-chan time.Time {
	return time.After(time.Duration(seconds) * time.Second)
}

// handleCommand processes an incoming command.
func (a *Agent) handleCommand(command string) {
	a.log.Info().Str("command", command).Msg("received command")

	// Special commands that work even when busy
	switch command {
	case "stop":
		// Stop MUST work when busy - that's the whole point!
		a.handleStop()
		return
	case "restart":
		// Restart also works anytime
		a.handleRestart()
		return
	}

	// Check if already busy (T02: reject concurrent commands)
	if a.IsBusy() {
		a.mu.RLock()
		currentCmd := ""
		currentPID := 0
		if a.pendingCommand != nil {
			currentCmd = *a.pendingCommand
		}
		if a.commandPID != nil {
			currentPID = *a.commandPID
		}
		a.mu.RUnlock()

		a.log.Warn().
			Str("requested", command).
			Str("current", currentCmd).
			Msg("command rejected: already busy")

		payload := protocol.CommandRejectedPayload{
			Reason:         "command already running",
			CurrentCommand: currentCmd,
			CurrentPID:     currentPID,
		}
		if err := a.ws.SendMessage(protocol.TypeRejected, payload); err != nil {
			a.log.Debug().Err(err).Msg("failed to send rejection")
		}
		return
	}

	// Execute command in goroutine
	go a.executeCommand(command)
}

// executeCommand runs a command and streams output.
func (a *Agent) executeCommand(command string) {
	// Set busy state
	a.mu.Lock()
	a.pendingCommand = &command
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.pendingCommand = nil
		a.commandPID = nil
		a.mu.Unlock()
	}()

	var cmd *exec.Cmd
	var err error

	switch command {
	case "pull":
		if a.cfg.RepoURL != "" {
			// Isolated mode: fetch + reset --hard (clean slate)
			if err := a.runIsolatedPull(); err != nil {
				a.sendStatus("error", command, 1, err.Error())
				return
			}
			// Force refresh status checks since repo changed
			a.statusChecker.ForceRefresh(a.ctx)
			a.sendStatus("ok", command, 0, "")
			return
		}
		cmd, err = a.buildPullCommand()
	case "switch":
		cmd, err = a.buildSwitchCommand()
	case "pull-switch":
		// Pull first
		if a.cfg.RepoURL != "" {
			// Isolated mode: fetch + reset --hard (clean slate)
			if err := a.runIsolatedPull(); err != nil {
				a.sendStatus("error", command, 1, err.Error())
				return
			}
		} else {
			if err := a.runCommandWithOutput("pull", a.buildPullCommandArgs()); err != nil {
				a.sendStatus("error", command, 1, err.Error())
				return
			}
		}
		// Force refresh status after pull so switch sees updated state
		a.statusChecker.ForceRefresh(a.ctx)
		// Then switch
		cmd, err = a.buildSwitchCommand()
	case "test":
		cmd, err = a.buildTestCommand()
	case "update":
		cmd, err = a.buildUpdateCommand()

	// P2800: Refresh commands - force update of specific status checks
	case "refresh-git":
		// Git status is checked by dashboard, not agent
		a.sendOutput("Git status is checked by dashboard (GitHub API)", "stdout")
		a.sendStatus("ok", command, 0, "git status check is dashboard-side")
		return
	case "refresh-lock":
		a.sendOutput("Refreshing lock file status...", "stdout")
		a.statusChecker.RefreshLock(a.ctx)
		lockStatus := a.statusChecker.GetLockStatus()
		a.sendOutput(fmt.Sprintf("✓ Lock status: %s (%s)", lockStatus.Status, lockStatus.Message), "stdout")
		a.sendStatus("ok", command, 0, lockStatus.Status)
		return
	case "refresh-system":
		a.sendOutput("Refreshing system status (this may take 30-60s)...", "stdout")
		a.statusChecker.RefreshSystem(a.ctx)
		sysStatus := a.statusChecker.GetSystemStatus()
		a.sendOutput(fmt.Sprintf("✓ System status: %s (%s)", sysStatus.Status, sysStatus.Message), "stdout")
		a.sendStatus("ok", command, 0, sysStatus.Status)
		return
	case "refresh-all":
		a.sendOutput("Refreshing all status checks...", "stdout")
		a.statusChecker.ForceRefresh(a.ctx)
		lockStatus := a.statusChecker.GetLockStatus()
		sysStatus := a.statusChecker.GetSystemStatus()
		a.sendOutput(fmt.Sprintf("✓ Lock: %s (%s)", lockStatus.Status, lockStatus.Message), "stdout")
		a.sendOutput(fmt.Sprintf("✓ System: %s (%s)", sysStatus.Status, sysStatus.Message), "stdout")
		a.sendStatus("ok", command, 0, "all refreshed")
		return

	default:
		a.log.Error().Str("command", command).Msg("unknown command")
		a.sendStatus("error", command, 1, "unknown command")
		return
	}

	if err != nil {
		a.log.Error().Err(err).Str("command", command).Msg("failed to build command")
		a.sendStatus("error", command, 1, err.Error())
		return
	}

	// Run with output streaming
	exitCode := a.runWithStreaming(cmd)

	status := "ok"
	message := ""
	if exitCode != 0 {
		status = "error"
		message = "command failed"
	}

	// P2800: Post-validation - refresh status and report on goal achievement
	if exitCode == 0 && (command == "pull" || command == "switch" || command == "pull-switch") {
		a.sendOutput("", "stdout") // Blank line separator
		a.sendOutput("📊 Post-validation: Checking if goal was achieved...", "stdout")
		
		// Force refresh to get updated state
		a.statusChecker.ForceRefresh(a.ctx)
		
		lockStatus := a.statusChecker.GetLockStatus()
		sysStatus := a.statusChecker.GetSystemStatus()
		
		// Report lock status
		lockIcon := "🟢"
		if lockStatus.Status == "outdated" {
			lockIcon = "🟡"
		} else if lockStatus.Status == "error" || lockStatus.Status == "unknown" {
			lockIcon = "🔴"
		}
		a.sendOutput(fmt.Sprintf("%s Lock: %s", lockIcon, lockStatus.Message), "stdout")
		
		// Report system status
		sysIcon := "🟢"
		if sysStatus.Status == "outdated" {
			sysIcon = "🟡"
		} else if sysStatus.Status == "error" || sysStatus.Status == "unknown" {
			sysIcon = "🔴"
		}
		a.sendOutput(fmt.Sprintf("%s System: %s", sysIcon, sysStatus.Message), "stdout")
		
		// Overall goal check based on command type
		switch command {
		case "pull":
			// Goal: system should now be outdated (needs rebuild) or already current
			a.sendOutput("", "stdout")
			if lockStatus.Status == "ok" {
				a.sendOutput("✅ Pull goal achieved: Lock file is current", "stdout")
			} else {
				a.sendOutput("⚠️  Pull completed but lock still appears outdated (cache?)", "stdout")
			}
		case "switch":
			// Goal: system should now be current
			a.sendOutput("", "stdout")
			if sysStatus.Status == "ok" {
				a.sendOutput("✅ Switch goal achieved: System is current", "stdout")
			} else {
				a.sendOutput("⚠️  Switch completed but system still shows outdated (may need agent restart)", "stdout")
			}
		case "pull-switch":
			// Goal: both should be current
			a.sendOutput("", "stdout")
			if lockStatus.Status == "ok" && sysStatus.Status == "ok" {
				a.sendOutput("✅ Pull+Switch goal achieved: Fully up to date", "stdout")
			} else if sysStatus.Status == "ok" {
				a.sendOutput("⚠️  System current but lock shows outdated (GitHub cache?)", "stdout")
			} else {
				a.sendOutput("⚠️  Commands completed but goals not fully achieved", "stdout")
			}
		}
	}

	a.sendStatus(status, command, exitCode, message)

	// Auto-restart after successful switch to pick up new binary
	// Only on NixOS - macOS is handled by home-manager's launchctl bootout/bootstrap
	if exitCode == 0 && (command == "switch" || command == "pull-switch") && runtime.GOOS != "darwin" {
		a.log.Info().Msg("switch completed successfully, restarting to pick up new binary")
		// Give time for the status message to be sent
		time.Sleep(500 * time.Millisecond)
		a.Shutdown()
		os.Exit(101) // Triggers RestartForceExitStatus in systemd
	}
}

// runWithStreaming runs a command and streams stdout/stderr.
func (a *Agent) runWithStreaming(cmd *exec.Cmd) int {
	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.log.Error().Err(err).Msg("failed to create stdout pipe")
		return 1
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.log.Error().Err(err).Msg("failed to create stderr pipe")
		return 1
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		a.log.Error().Err(err).Msg("failed to start command")
		return 1
	}

	// Store PID
	a.mu.Lock()
	pid := cmd.Process.Pid
	a.commandPID = &pid
	a.mu.Unlock()

	a.log.Debug().Int("pid", pid).Msg("command started")

	// Stream output
	done := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stdout")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stderr")
		}
		close(done)
	}()

	// Wait for output to finish
	<-done

	// Wait for command to complete
	err = cmd.Wait()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// runCommandWithOutput is a helper for running commands with output capture.
func (a *Agent) runCommandWithOutput(name string, args []string) error {
	cmd := exec.CommandContext(a.ctx, args[0], args[1:]...)
	cmd.Dir = a.cfg.RepoDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stdout")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stderr")
		}
	}()

	return cmd.Wait()
}

// sendOutput sends a line of command output.
func (a *Agent) sendOutput(line, stream string) {
	payload := protocol.OutputPayload{
		Line:   line,
		Stream: stream,
	}
	if err := a.ws.SendMessage(protocol.TypeOutput, payload); err != nil {
		a.log.Debug().Err(err).Msg("failed to send output")
	}
}

// sendStatus sends command completion status.
func (a *Agent) sendStatus(status, command string, exitCode int, message string) {
	payload := protocol.StatusPayload{
		Status:   status,
		Command:  command,
		ExitCode: exitCode,
		Message:  message,
	}
	if err := a.ws.SendMessage(protocol.TypeStatus, payload); err != nil {
		a.log.Error().Err(err).Msg("failed to send status")
	}

	a.log.Info().
		Str("status", status).
		Str("command", command).
		Int("exit_code", exitCode).
		Msg("command completed")
}

// handleStop kills the currently running command.
// Sends SIGTERM to process group, with SIGKILL fallback after 3 seconds.
func (a *Agent) handleStop() {
	a.mu.RLock()
	pid := a.commandPID
	currentCmd := a.pendingCommand
	a.mu.RUnlock()

	if pid == nil {
		a.log.Warn().Msg("stop requested but no command running")
		// Still send status so UI updates
		a.sendStatus("error", "stop", 1, "no command running")
		return
	}

	processID := *pid
	cmdName := "unknown"
	if currentCmd != nil {
		cmdName = *currentCmd
	}

	a.log.Info().Int("pid", processID).Str("command", cmdName).Msg("stopping command")

	// Send SIGTERM to process group (negative PID kills all children too)
	// On macOS/Linux, this kills the entire process tree
	pgid, err := syscall.Getpgid(processID)
	if err != nil {
		// Fallback to just the process
		pgid = processID
	}

	// Try graceful termination first (SIGTERM to process group)
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		// If process group kill fails, try direct kill
		if err := syscall.Kill(processID, syscall.SIGTERM); err != nil {
			a.log.Error().Err(err).Int("pid", processID).Msg("failed to send SIGTERM")
			a.sendStatus("error", "stop", 1, "failed to terminate: "+err.Error())
			return
		}
	}

	a.log.Info().Int("pid", processID).Int("pgid", pgid).Msg("sent SIGTERM to process group")

	// Start goroutine to SIGKILL if process doesn't die in 3 seconds
	go func() {
		select {
		case <-a.ctx.Done():
			return
		case <-timeAfter(3):
			a.mu.RLock()
			stillRunning := a.commandPID != nil && *a.commandPID == processID
			a.mu.RUnlock()

			if stillRunning {
				a.log.Warn().Int("pid", processID).Msg("process didn't respond to SIGTERM, sending SIGKILL")
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
				_ = syscall.Kill(processID, syscall.SIGKILL)
			}
		}
	}()

	// Send status - the actual command completion will send its own status
	// when it detects it was killed (exit code will be non-zero)
	a.sendStatus("stopped", cmdName, 130, "terminated by user")
}

// handleRestart exits the agent (systemd/launchd will restart it).
func (a *Agent) handleRestart() {
	a.log.Info().Msg("restart requested, exiting")
	a.Shutdown()
	os.Exit(0)
}

// Command builders

// runIsolatedPull performs a clean-slate pull for isolated mode:
// git fetch + git reset --hard + git clean -fd
func (a *Agent) runIsolatedPull() error {
	a.log.Info().Msg("running isolated pull (fetch + reset --hard)")

	// Step 1: git fetch origin <branch>
	fetchArgs := []string{"git", "-C", a.cfg.RepoDir, "fetch", "origin", a.cfg.Branch}
	fetchCmd := exec.CommandContext(a.ctx, fetchArgs[0], fetchArgs[1:]...)

	// Set SSH key if configured
	if a.cfg.SSHKey != "" {
		fetchCmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+a.cfg.SSHKey+" -o StrictHostKeyChecking=no",
		)
	}

	if err := a.runCmdWithStreaming(fetchCmd, "fetch"); err != nil {
		return err
	}

	// Step 2: git reset --hard origin/<branch>
	resetArgs := []string{"git", "-C", a.cfg.RepoDir, "reset", "--hard", "origin/" + a.cfg.Branch}
	resetCmd := exec.CommandContext(a.ctx, resetArgs[0], resetArgs[1:]...)
	if err := a.runCmdWithStreaming(resetCmd, "reset"); err != nil {
		return err
	}

	// Step 3: git clean -fd (remove untracked files)
	cleanArgs := []string{"git", "-C", a.cfg.RepoDir, "clean", "-fd"}
	cleanCmd := exec.CommandContext(a.ctx, cleanArgs[0], cleanArgs[1:]...)
	if err := a.runCmdWithStreaming(cleanCmd, "clean"); err != nil {
		// Clean is optional, just log warning
		a.log.Warn().Err(err).Msg("git clean failed (non-fatal)")
	}

	a.log.Info().Msg("isolated pull completed successfully")
	return nil
}

// runCmdWithStreaming runs a command and streams output to the dashboard.
func (a *Agent) runCmdWithStreaming(cmd *exec.Cmd, label string) error {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stdout")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stderr")
		}
	}()

	return cmd.Wait()
}

func (a *Agent) buildPullCommandArgs() []string {
	// Legacy mode: git pull (only used when RepoURL is not set)
	return []string{"git", "-C", a.cfg.RepoDir, "pull"}
}

func (a *Agent) buildPullCommand() (*exec.Cmd, error) {
	args := a.buildPullCommandArgs()
	cmd := exec.CommandContext(a.ctx, args[0], args[1:]...)
	cmd.Dir = a.cfg.RepoDir

	// Set SSH key if configured
	if a.cfg.SSHKey != "" {
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+a.cfg.SSHKey+" -o StrictHostKeyChecking=no",
		)
	}

	return cmd, nil
}

func (a *Agent) buildSwitchCommand() (*exec.Cmd, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "darwin" {
		// macOS: home-manager switch
		//
		// CRITICAL: home-manager switch calls "launchctl bootout" which kills THIS agent.
		// If the switch process is in our process group, it dies too.
		//
		// Solution: Run in a NEW SESSION (Setsid) so it:
		// 1. Gets its own process group (survives when launchd kills our group)
		// 2. Becomes session leader (survives when we die)
		// 3. Still runs as our child (we can still stream stdout/stderr)
		//
		// The switch will continue even after launchd kills us, and the new agent
		// will reconnect once the switch completes.
		//
		flakeRef := a.cfg.RepoDir + "#" + a.cfg.Hostname
		cmd = exec.Command("home-manager", "switch", "--flake", flakeRef)
		cmd.Dir = a.cfg.RepoDir

		// Create new session - this is the key to surviving agent death
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true, // Create new session, become session leader
		}

		a.log.Info().
			Str("flake", flakeRef).
			Msg("starting switch with new session (survives agent restart)")

	} else {
		// NixOS: sudo nixos-rebuild switch
		// This runs as root in a separate session, so it survives agent restart
		args := []string{
			"sudo", "nixos-rebuild", "switch",
			"--flake", a.cfg.RepoDir + "#" + a.cfg.Hostname,
		}
		cmd = exec.CommandContext(a.ctx, args[0], args[1:]...)
		cmd.Dir = a.cfg.RepoDir
	}

	return cmd, nil
}

func (a *Agent) buildTestCommand() (*exec.Cmd, error) {
	// Run test suite from hosts/<hostname>/tests/
	testDir := a.cfg.RepoDir + "/hosts/" + a.cfg.Hostname + "/tests"

	// Find and run test scripts
	cmd := exec.CommandContext(a.ctx, "sh", "-c",
		"for f in "+testDir+"/*.sh; do [ -x \"$f\" ] && \"$f\"; done",
	)
	cmd.Dir = a.cfg.RepoDir
	return cmd, nil
}

func (a *Agent) buildUpdateCommand() (*exec.Cmd, error) {
	// nix flake update then switch
	args := []string{"nix", "flake", "update"}
	cmd := exec.CommandContext(a.ctx, args[0], args[1:]...)
	cmd.Dir = a.cfg.RepoDir
	return cmd, nil
}

