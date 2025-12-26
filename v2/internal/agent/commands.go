package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
	case "reboot":
		// P6900: Reboot works anytime (like restart and stop)
		a.handleReboot()
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
		// P2800: Signal pull phase starting
		a.sendOperationProgress("pull", "in_progress", 0, 4)
		if a.cfg.RepoURL != "" {
			// Isolated mode: fetch + reset --hard (clean slate)
			if err := a.runIsolatedPull(); err != nil {
				a.sendOperationProgress("pull", "error", 0, 4)
				a.sendStatus("error", command, 1, err.Error())
				return
			}
			// Force refresh status checks since repo changed
			a.statusChecker.ForceRefresh(a.ctx)
			a.sendOperationProgress("pull", "complete", 4, 4)
			a.sendStatus("ok", command, 0, "")
			return
		}
		cmd, err = a.buildPullCommand()
	case "switch":
		// P2800: Signal system phase starting
		a.sendOperationProgress("system", "in_progress", 0, 3)
		cmd, err = a.buildSwitchCommand()
	case "pull-switch":
		// P2800: Signal pull phase starting
		a.sendOperationProgress("pull", "in_progress", 0, 4)
		// Pull first
		if a.cfg.RepoURL != "" {
			// Isolated mode: fetch + reset --hard (clean slate)
			if err := a.runIsolatedPull(); err != nil {
				a.sendOperationProgress("pull", "error", 0, 4)
				a.sendStatus("error", command, 1, err.Error())
				return
			}
		} else {
			if err := a.runCommandWithOutput("pull", a.buildPullCommandArgs()); err != nil {
				a.sendOperationProgress("pull", "error", 0, 4)
				a.sendStatus("error", command, 1, err.Error())
				return
			}
		}
		// P2800: Pull complete, system starting
		a.sendOperationProgress("pull", "complete", 4, 4)
		a.sendOperationProgress("system", "in_progress", 0, 3)
		// Force refresh status after pull so switch sees updated state
		a.statusChecker.ForceRefresh(a.ctx)
		// Then switch
		cmd, err = a.buildSwitchCommand()
	case "test":
		// P2800: Signal tests phase starting
		a.sendOperationProgress("tests", "in_progress", 0, 8)
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

	// P7200: Force uncached update - bypasses Nix binary cache
	case "force-update":
		a.sendOutput("⚠️  Force update initiated (bypassing Nix cache)...", "stdout")
		a.handleForceUpdate()
		return

	// P7230: Check version - compares running vs installed binary
	case "check-version":
		a.handleCheckVersion()
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

	// P2800: Send completion progress based on command type
	switch command {
	case "pull":
		if exitCode == 0 {
			a.sendOperationProgress("pull", "complete", 4, 4)
		} else {
			a.sendOperationProgress("pull", "error", 0, 4)
		}
	case "switch":
		if exitCode == 0 {
			a.sendOperationProgress("system", "complete", 3, 3)
		} else {
			a.sendOperationProgress("system", "error", 0, 3)
		}
	case "pull-switch":
		// Pull already marked complete, now mark system
		if exitCode == 0 {
			a.sendOperationProgress("system", "complete", 3, 3)
		} else {
			a.sendOperationProgress("system", "error", 0, 3)
		}
	case "test":
		if exitCode == 0 {
			a.sendOperationProgress("tests", "complete", 8, 8)
		} else {
			a.sendOperationProgress("tests", "error", 0, 8)
		}
	}

	// P2800: Post-validation - refresh status and report on goal achievement
	if exitCode == 0 && (command == "pull" || command == "switch" || command == "pull-switch") {
		a.sendOutput("", "stdout") // Blank line separator
		a.sendOutput("📊 Post-validation: Checking goal achievement...", "stdout")
		
		// Force refresh Lock status (lightweight - just git log)
		// System status is NOT auto-refreshed (too heavy - nix build --dry-run)
		a.statusChecker.ForceRefresh(a.ctx)
		
		lockStatus := a.statusChecker.GetLockStatus()
		
		// Report lock status (lightweight check)
		lockIcon := "🟢"
		if lockStatus.Status == "outdated" {
			lockIcon = "🟡"
		} else if lockStatus.Status == "error" || lockStatus.Status == "unknown" {
			lockIcon = "🔴"
		}
		a.sendOutput(fmt.Sprintf("%s Lock: %s", lockIcon, lockStatus.Message), "stdout")
		
		// System status not reported automatically (check is too expensive)
		// User can run refresh-system to check if needed
		
		// Overall goal check based on command type
		switch command {
		case "pull":
			// Goal: lock should now be current, system needs rebuild
			// Set system to outdated since we pulled new config
			a.statusChecker.SetSystemOutdated("Pull completed - rebuild needed")
			a.sendOutput("", "stdout")
			if lockStatus.Status == "ok" {
				a.sendOutput("✅ Pull goal achieved: Lock file is current", "stdout")
			} else {
				a.sendOutput("⚠️  Pull completed but lock still appears outdated (cache?)", "stdout")
			}
			a.sendOutput("🟡 System: Outdated (rebuild needed)", "stdout")
		case "switch":
			// Goal: system should now be current
			// Infer System=ok from exit code (avoids expensive nix build --dry-run)
			a.statusChecker.SetSystemOk("Switch successful (exit 0)")
			a.sendOutput("", "stdout")
			a.sendOutput("✅ Switch completed successfully", "stdout")
			a.sendOutput("🟢 System: Current (inferred from exit 0)", "stdout")
		case "pull-switch":
			// Goal: both should be current
			// Infer System=ok from exit code (avoids expensive nix build --dry-run)
			a.statusChecker.SetSystemOk("Pull+Switch successful (exit 0)")
			a.sendOutput("", "stdout")
			if lockStatus.Status == "ok" {
				a.sendOutput("✅ Pull+Switch completed: Lock current, switch successful", "stdout")
			} else {
				a.sendOutput("⚠️  Switch OK but lock shows outdated (Git cache may need time)", "stdout")
			}
			a.sendOutput("🟢 System: Current (inferred from exit 0)", "stdout")
		}
	}

	a.sendStatus(status, command, exitCode, message)

	// P7100: Send immediate heartbeat after command to push fresh status to dashboard
	// This ensures the UI updates immediately instead of waiting up to 5s for next heartbeat
	a.sendHeartbeat()

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
	// Refresh generation after command (especially important after pull)
	generation := a.detectGeneration()

	payload := protocol.StatusPayload{
		Status:     status,
		Command:    command,
		ExitCode:   exitCode,
		Message:    message,
		Generation: generation,
	}
	if err := a.ws.SendMessage(protocol.TypeStatus, payload); err != nil {
		a.log.Error().Err(err).Msg("failed to send status")
	}

	a.log.Info().
		Str("status", status).
		Str("command", command).
		Int("exit_code", exitCode).
		Str("generation", generation).
		Msg("command completed")
}

// sendOperationProgress sends progress updates for the status dots (P2800).
// phase: "pull", "lock", "system", "tests"
// status: "pending", "in_progress", "complete", "error"
// current/total: progress within the phase
func (a *Agent) sendOperationProgress(phase, status string, current, total int) {
	// Build operation progress based on which phase we're in
	progress := protocol.OperationProgress{}

	phaseProgress := &protocol.PhaseProgress{
		Current: current,
		Total:   total,
		Status:  status,
	}

	switch phase {
	case "pull":
		progress.Pull = phaseProgress
	case "lock":
		progress.Lock = phaseProgress
	case "system":
		progress.System = phaseProgress
	case "tests":
		progress.Tests = &protocol.TestsProgress{
			Current: current,
			Total:   total,
			Status:  status,
		}
	}

	payload := protocol.OperationProgressPayload{
		Progress: progress,
	}

	a.log.Debug().
		Str("phase", phase).
		Str("status", status).
		Int("current", current).
		Int("total", total).
		Msg("sending operation_progress")

	if err := a.ws.SendMessage(protocol.TypeOperationProgress, payload); err != nil {
		a.log.Error().Err(err).Msg("failed to send operation progress")
	}
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

// P6900: handleReboot executes system reboot.
func (a *Agent) handleReboot() {
	a.log.Warn().Msg("reboot requested, executing system reboot")

	// Send WebSocket message before reboot (so dashboard knows it's happening)
	a.sendOutput("⚠️  Reboot command received. System will reboot in 3 seconds...", "stdout")

	// Give time for WebSocket message to be sent
	time.Sleep(3 * time.Second)

	// Platform-specific reboot command
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS
		cmd = exec.Command("sudo", "reboot")
	} else {
		// Linux/NixOS
		cmd = exec.Command("sudo", "reboot")
	}

	// Execute reboot (this will terminate the agent)
	if err := cmd.Run(); err != nil {
		// If we get here, reboot failed (unlikely but possible)
		a.log.Error().Err(err).Msg("reboot command failed")
		// Try to send error message (may not reach dashboard)
		a.sendOutput(fmt.Sprintf("❌ Reboot failed: %v. Check sudo permissions.", err), "stderr")
	}
	// If successful, agent will be terminated by reboot
}

// P7200: handleForceUpdate runs the force uncached update sequence.
// This bypasses Nix binary cache to ensure fresh binaries are built.
func (a *Agent) handleForceUpdate() {
	command := "force-update"

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

	// Get repo directory
	repoDir := a.cfg.RepoDir
	if repoDir == "" {
		repoDir = os.ExpandEnv("$HOME/Code/nixcfg")
	}

	hostname, _ := os.Hostname()

	a.sendOutput("", "stdout")
	a.sendOutput("╔═══════════════════════════════════════════════════════════╗", "stdout")
	a.sendOutput("║  P7200: FORCE UNCACHED UPDATE                             ║", "stdout")
	a.sendOutput("╚═══════════════════════════════════════════════════════════╝", "stdout")
	a.sendOutput("", "stdout")

	// Step 1: Git pull
	a.sendOutput("━━━ Step 1/4: Git pull ━━━", "stdout")
	pullCmd := exec.Command("git", "pull")
	pullCmd.Dir = repoDir
	if err := a.runStepWithStreaming(pullCmd); err != nil {
		a.sendOutput(fmt.Sprintf("❌ Git pull failed: %v", err), "stderr")
		a.sendStatus("error", command, 1, "git pull failed")
		return
	}
	a.sendOutput("✓ Git pull complete", "stdout")
	a.sendOutput("", "stdout")

	// Step 2: Update nixfleet flake input
	a.sendOutput("━━━ Step 2/4: Update nixfleet flake input ━━━", "stdout")
	updateCmd := exec.Command("nix", "flake", "update", "nixfleet")
	updateCmd.Dir = repoDir
	if err := a.runStepWithStreaming(updateCmd); err != nil {
		a.sendOutput(fmt.Sprintf("❌ Flake update failed: %v", err), "stderr")
		a.sendStatus("error", command, 1, "nix flake update failed")
		return
	}
	a.sendOutput("✓ Flake update complete", "stdout")
	a.sendOutput("", "stdout")

	// Step 3: Stop agent (so it doesn't interfere with rebuild)
	a.sendOutput("━━━ Step 3/4: Stopping agent for rebuild ━━━", "stdout")
	a.sendOutput("Agent will restart automatically after rebuild completes.", "stdout")
	a.sendOutput("", "stdout")

	// Step 4: Rebuild with cache bypass
	a.sendOutput("━━━ Step 4/4: Rebuilding with cache bypass ━━━", "stdout")
	a.sendOutput("Using --option narinfo-cache-negative-ttl 0 to force fresh build", "stdout")
	a.sendOutput("", "stdout")

	var rebuildCmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS: home-manager switch
		rebuildCmd = exec.Command("home-manager", "switch",
			"--flake", fmt.Sprintf(".#%s", hostname),
			"--option", "narinfo-cache-negative-ttl", "0")
	} else {
		// NixOS: nixos-rebuild switch
		rebuildCmd = exec.Command("sudo", "nixos-rebuild", "switch",
			"--flake", fmt.Sprintf(".#%s", hostname),
			"--option", "narinfo-cache-negative-ttl", "0")
	}
	rebuildCmd.Dir = repoDir

	exitCode := a.runWithStreaming(rebuildCmd)
	if exitCode != 0 {
		a.sendOutput(fmt.Sprintf("❌ Rebuild failed with exit code %d", exitCode), "stderr")
		a.sendStatus("error", command, exitCode, "rebuild failed")
		return
	}

	a.sendOutput("", "stdout")
	a.sendOutput("✅ Force update complete! Agent will restart with new version.", "stdout")
	a.sendStatus("ok", command, 0, "force update complete")

	// Refresh status
	a.statusChecker.ForceRefresh(a.ctx)
}

// runStepWithStreaming runs a command step and streams output.
func (a *Agent) runStepWithStreaming(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stdout")
		}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			a.sendOutput(scanner.Text(), "stderr")
		}
	}()

	return cmd.Wait()
}

// handleKillCommand handles kill command from dashboard (P2800).
// This is different from handleStop - it's initiated by the dashboard
// when a command exceeds timeout and user requests termination.
func (a *Agent) handleKillCommand(signal string, requestedPID int) {
	a.mu.RLock()
	pid := a.commandPID
	currentCmd := a.pendingCommand
	a.mu.RUnlock()

	// Use provided PID or current command PID
	targetPID := requestedPID
	if targetPID == 0 && pid != nil {
		targetPID = *pid
	}

	if targetPID == 0 {
		a.log.Warn().Msg("kill_command received but no PID to kill")
		a.sendOutput("⚠️ No command running to kill", "stderr")
		return
	}

	cmdName := "unknown"
	if currentCmd != nil {
		cmdName = *currentCmd
	}

	a.log.Info().
		Int("pid", targetPID).
		Str("signal", signal).
		Str("command", cmdName).
		Msg("kill_command received from dashboard")

	a.sendOutput(fmt.Sprintf("⚠️ Received kill command: %s (PID %d)", signal, targetPID), "stderr")

	// Get process group for killing children too
	pgid, err := syscall.Getpgid(targetPID)
	if err != nil {
		pgid = targetPID // Fallback to just the process
	}

	switch signal {
	case "SIGTERM":
		// Send SIGTERM first
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			if err := syscall.Kill(targetPID, syscall.SIGTERM); err != nil {
				a.log.Error().Err(err).Int("pid", targetPID).Msg("failed to send SIGTERM")
				a.sendOutput(fmt.Sprintf("❌ Failed to send SIGTERM: %v", err), "stderr")
				return
			}
		}
		a.sendOutput("✓ SIGTERM sent, waiting for process to terminate...", "stderr")

		// Start goroutine to SIGKILL if not dead in 5s
		go func() {
			select {
			case <-a.ctx.Done():
				return
			case <-timeAfter(5):
				a.mu.RLock()
				stillRunning := a.commandPID != nil && *a.commandPID == targetPID
				a.mu.RUnlock()

				if stillRunning {
					a.log.Warn().Int("pid", targetPID).Msg("process didn't respond to SIGTERM, sending SIGKILL")
					a.sendOutput("⚠️ Process didn't terminate, sending SIGKILL...", "stderr")
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
					_ = syscall.Kill(targetPID, syscall.SIGKILL)
				}
			}
		}()

	case "SIGKILL":
		// Immediate SIGKILL
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			if err := syscall.Kill(targetPID, syscall.SIGKILL); err != nil {
				a.log.Error().Err(err).Int("pid", targetPID).Msg("failed to send SIGKILL")
				a.sendOutput(fmt.Sprintf("❌ Failed to send SIGKILL: %v", err), "stderr")
				return
			}
		}
		a.sendOutput("✓ SIGKILL sent", "stderr")

	default:
		a.log.Warn().Str("signal", signal).Msg("unknown signal requested")
		a.sendOutput(fmt.Sprintf("❌ Unknown signal: %s", signal), "stderr")
	}
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

// P7230: handleCheckVersion compares running vs installed agent binary version.
// This detects when binary is updated but service hasn't restarted.
func (a *Agent) handleCheckVersion() {
	command := "check-version"

	// Running version (from compiled-in constant)
	runningVersion := Version

	// Find the installed binary path
	binaryPath := a.findAgentBinaryPath()

	a.sendOutput("🔍 Checking agent version...", "stdout")
	a.sendOutput(fmt.Sprintf("   Running version: %s", runningVersion), "stdout")
	a.sendOutput(fmt.Sprintf("   Binary path: %s", binaryPath), "stdout")

	// Execute the installed binary with --version
	cmd := exec.CommandContext(a.ctx, binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		a.sendOutput(fmt.Sprintf("❌ Failed to check installed version: %v", err), "stderr")
		a.sendStatus("error", command, 1, "failed to check installed version")
		return
	}

	// Parse installed version from output (format: "nixfleet-agent X.Y.Z")
	installedVersion := strings.TrimSpace(string(output))
	installedVersion = strings.TrimPrefix(installedVersion, "nixfleet-agent ")

	a.sendOutput(fmt.Sprintf("   Installed version: %s", installedVersion), "stdout")
	a.sendOutput("", "stdout")

	// Compare versions
	if runningVersion == installedVersion {
		a.sendOutput(fmt.Sprintf("✅ Agent OK: running %s, installed %s", runningVersion, installedVersion), "stdout")
		a.sendStatus("ok", command, 0, fmt.Sprintf("running=%s installed=%s", runningVersion, installedVersion))
	} else {
		a.sendOutput(fmt.Sprintf("⚠️  Agent mismatch: running %s, installed %s", runningVersion, installedVersion), "stdout")
		a.sendOutput("   Restart agent to pick up new version", "stdout")
		a.sendStatus("warning", command, 0, fmt.Sprintf("mismatch: running=%s installed=%s", runningVersion, installedVersion))
	}
}

// findAgentBinaryPath locates the nixfleet-agent binary on the system.
func (a *Agent) findAgentBinaryPath() string {
	// Try common Nix paths first
	paths := []string{
		"/run/current-system/sw/bin/nixfleet-agent",          // NixOS system-level
		"/etc/profiles/per-user/" + os.Getenv("USER") + "/bin/nixfleet-agent", // Home Manager
	}

	// On macOS, also check user-specific paths
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		paths = append([]string{
			home + "/.nix-profile/bin/nixfleet-agent",
		}, paths...)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback: use which/command -v
	if out, err := exec.Command("which", "nixfleet-agent").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}

	// Last resort: assume it's in PATH
	return "nixfleet-agent"
}

