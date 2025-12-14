package agent

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/markus-barta/nixfleet/v2/internal/protocol"
)

// handleCommand processes an incoming command.
func (a *Agent) handleCommand(command string) {
	a.log.Info().Str("command", command).Msg("received command")

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
		a.ws.SendMessage(protocol.TypeRejected, payload)
		return
	}

	// Handle special commands
	switch command {
	case "stop":
		a.handleStop()
		return
	case "restart":
		a.handleRestart()
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
		cmd, err = a.buildPullCommand()
	case "switch":
		cmd, err = a.buildSwitchCommand()
	case "pull-switch":
		// Pull first
		if err := a.runCommandWithOutput("pull", a.buildPullCommandArgs()); err != nil {
			a.sendStatus("error", command, 1, err.Error())
			return
		}
		// Then switch
		cmd, err = a.buildSwitchCommand()
	case "test":
		cmd, err = a.buildTestCommand()
	case "update":
		cmd, err = a.buildUpdateCommand()
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

	a.sendStatus(status, command, exitCode, message)
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
func (a *Agent) handleStop() {
	a.mu.RLock()
	pid := a.commandPID
	a.mu.RUnlock()

	if pid == nil {
		a.log.Warn().Msg("stop requested but no command running")
		return
	}

	process, err := os.FindProcess(*pid)
	if err != nil {
		a.log.Error().Err(err).Int("pid", *pid).Msg("failed to find process")
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		a.log.Error().Err(err).Int("pid", *pid).Msg("failed to send SIGTERM")
		return
	}

	a.log.Info().Int("pid", *pid).Msg("sent SIGTERM to process")
}

// handleRestart exits the agent (systemd/launchd will restart it).
func (a *Agent) handleRestart() {
	a.log.Info().Msg("restart requested, exiting")
	a.Shutdown()
	os.Exit(0)
}

// Command builders

func (a *Agent) buildPullCommandArgs() []string {
	if a.cfg.RepoURL != "" {
		// Isolated mode: git fetch + reset
		return []string{
			"git", "-C", a.cfg.RepoDir,
			"fetch", "origin", a.cfg.Branch,
		}
	}
	// Legacy mode: git pull
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
	var args []string

	if runtime.GOOS == "darwin" {
		// macOS: home-manager switch
		args = []string{
			"home-manager", "switch",
			"--flake", a.cfg.RepoDir + "#" + a.cfg.Hostname,
		}
	} else {
		// NixOS: sudo nixos-rebuild switch
		args = []string{
			"sudo", "nixos-rebuild", "switch",
			"--flake", a.cfg.RepoDir + "#" + a.cfg.Hostname,
		}
	}

	cmd := exec.CommandContext(a.ctx, args[0], args[1:]...)
	cmd.Dir = a.cfg.RepoDir
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

