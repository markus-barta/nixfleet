# Agent Resilience & Autohealing

**Created**: 2025-12-13  
**Priority**: Medium  
**Status**: Backlog

## Summary

Make the NixFleet agent more resilient by detecting and recovering from stuck states, particularly duplicate processes that prevent proper operation.

## Problem

The agent can become stuck when duplicate processes are running:

- Multiple agent instances compete for the same resources
- Agent may fail to respond to commands or heartbeat properly
- No automatic detection or recovery mechanism exists
- Manual intervention required (kill processes, restart service)

**Root causes**:

- Service restart without proper cleanup of old processes
- Process crashes leaving zombie/stuck processes
- Manual agent execution while service is running
- Race conditions during service startup

## Solution

Implement multi-layered resilience mechanisms:

1. **Process Locking**: Ensure only one agent instance runs at a time
2. **Duplicate Detection**: Detect and handle duplicate processes
3. **Autohealing**: Automatically recover from stuck states
4. **Health Monitoring**: Self-monitoring and status reporting
5. **Child Process Timeouts**: Add "smart" timeouts to all spawned child processes to prevent hanging (i.e. expected timeouts for common commands)

## Technical Analysis

### 1. Process Locking

**Approach**: Use file-based locking (flock) to ensure single instance.

**Implementation**:

```bash
# At start of main()
LOCK_FILE="/tmp/nixfleet-agent-${HOST_ID}.lock"
exec 200>"$LOCK_FILE"

if ! flock -n 200; then
  log_error "Another agent instance is already running (lock: $LOCK_FILE)"
  log_info "Attempting to kill stale processes..."
  kill_stale_processes
  # Retry lock after cleanup
  if ! flock -n 200; then
    log_error "Failed to acquire lock after cleanup - exiting"
    exit 1
  fi
fi

# Lock acquired - ensure cleanup on exit
trap 'flock -u 200; rm -f "$LOCK_FILE"' EXIT
```

**Lock file location**:

- NixOS: `/var/lib/nixfleet-agent/lock` (if StateDirectory configured) or `/tmp/nixfleet-agent-${HOST_ID}.lock`
- macOS: `~/.local/state/nixfleet-agent/lock` or `/tmp/nixfleet-agent-${HOST_ID}.lock`

### 2. Duplicate Process Detection

**Approach**: Check for other agent processes before starting.

**Implementation**:

```bash
detect_duplicate_processes() {
  local script_name="nixfleet-agent.sh"
  local current_pid=$$
  local pids
  
  # Find all processes matching agent script
  if [[ "$(uname)" == "Darwin" ]]; then
    pids=$(pgrep -f "$script_name" 2>/dev/null || true)
  else
    pids=$(pgrep -f "$script_name" 2>/dev/null || true)
  fi
  
  # Filter out current process
  local duplicates=()
  for pid in $pids; do
    if [[ "$pid" != "$current_pid" ]]; then
      duplicates+=("$pid")
    fi
  done
  
  if [[ ${#duplicates[@]} -gt 0 ]]; then
    log_warn "Found ${#duplicates[@]} duplicate agent process(es): ${duplicates[*]}"
    return 0  # Duplicates found
  fi
  
  return 1  # No duplicates
}

kill_stale_processes() {
  local killed=0
  
  if detect_duplicate_processes; then
    for pid in "${duplicates[@]}"; do
      log_info "Killing stale agent process: $pid"
      kill "$pid" 2>/dev/null && ((killed++)) || true
    done
    
    # Wait briefly for processes to exit
    sleep 2
    
    # Force kill if still running
    for pid in "${duplicates[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        log_warn "Force killing stale process: $pid"
        kill -9 "$pid" 2>/dev/null || true
      fi
    done
    
    log_info "Cleaned up $killed stale process(es)"
  fi
}
```

### 3. Health Monitoring & Autohealing

**Approach**: Self-monitoring with automatic recovery.

**Implementation**:

```bash
# Track last successful heartbeat
LAST_HEARTBEAT_FILE="/tmp/nixfleet-agent-last-heartbeat-${HOST_ID}"
LAST_HEARTBEAT_TIMEOUT=300  # 5 minutes

check_health() {
  local now=$(date +%s)
  local last_heartbeat=0
  
  if [[ -f "$LAST_HEARTBEAT_FILE" ]]; then
    last_heartbeat=$(cat "$LAST_HEARTBEAT_FILE" 2>/dev/null || echo 0)
  fi
  
  local age=$((now - last_heartbeat))
  
  if [[ $age -gt $LAST_HEARTBEAT_TIMEOUT ]]; then
    log_warn "No successful heartbeat in ${age}s (threshold: ${LAST_HEARTBEAT_TIMEOUT}s)"
    log_info "Agent may be stuck - checking for issues..."
    
    # Check for duplicate processes
    if detect_duplicate_processes; then
      log_warn "Found duplicates during health check - cleaning up"
      kill_stale_processes
    fi
    
    # Report health issue to dashboard
    report_status "warning" "$(get_generation)" "Health check: stale heartbeat (${age}s old)"
    
    return 1  # Unhealthy
  fi
  
  return 0  # Healthy
}

# Update heartbeat timestamp on successful heartbeat
update_heartbeat_timestamp() {
  date +%s > "$LAST_HEARTBEAT_FILE" 2>/dev/null || true
}
```

**Integration**: Call `check_health()` periodically in main loop, update timestamp after successful heartbeat.

### 4. Child Process Timeouts

**Approach**: Add timeouts to all child processes spawned by the agent to prevent hanging operations.

**Requirements**:

- Short-running processes (git pull, git commands, file operations): Add appropriate short timeouts (30s-2min)
- Long-running processes (nixos-rebuild, home-manager switch): Use ~10-minute timeout as last resort
- Timeout should kill the process and report failure if exceeded
- Timeout values should be configurable via environment variables

**Implementation**:

```bash
# Timeout configuration
readonly GIT_TIMEOUT="${NIXFLEET_GIT_TIMEOUT:-120}"      # 2 minutes for git operations
readonly SWITCH_TIMEOUT="${NIXFLEET_SWITCH_TIMEOUT:-600}" # 10 minutes for rebuilds (last resort)
readonly TEST_TIMEOUT="${NIXFLEET_TEST_TIMEOUT:-300}"     # 5 minutes per test script

# Generic timeout wrapper
run_with_timeout() {
  local timeout_seconds="$1"
  shift
  local cmd=("$@")
  local pid
  
  # Run command in background
  "${cmd[@]}" &
  pid=$!
  
  # Wait for process with timeout
  local elapsed=0
  while kill -0 "$pid" 2>/dev/null; do
    sleep 1
    ((elapsed++))
    if [[ $elapsed -ge $timeout_seconds ]]; then
      log_error "Command timed out after ${timeout_seconds}s: ${cmd[*]}"
      kill "$pid" 2>/dev/null || true
      sleep 2
      kill -9 "$pid" 2>/dev/null || true
      return 124  # Exit code 124 = timeout (matches timeout command)
    fi
  done
  
  # Wait for process to finish and get exit code
  wait "$pid"
  return $?
}

# Updated do_pull with timeout
do_pull() {
  log_info "Executing: git pull (timeout: ${GIT_TIMEOUT}s)"
  cd "$NIXFLEET_NIXCFG"
  
  local output
  local exit_code
  
  if output=$(run_with_timeout "$GIT_TIMEOUT" git pull 2>&1); then
    log_info "Pull successful"
    refresh_git_hash >/dev/null
    report_status "ok" "$(get_generation)" "$output"
    return 0
  else
    exit_code=$?
    if [[ $exit_code -eq 124 ]]; then
      log_error "Pull timed out after ${GIT_TIMEOUT}s"
      report_status "error" "$(get_generation)" "Git pull timed out after ${GIT_TIMEOUT}s"
    else
      log_error "Pull failed (exit code: $exit_code)"
      report_status "error" "$(get_generation)" "$output"
    fi
    return $exit_code
  fi
}

# Updated do_switch with timeout
do_switch() {
  log_info "Executing: switch ($HOST_TYPE) (timeout: ${SWITCH_TIMEOUT}s)"
  cd "$NIXFLEET_NIXCFG"
  
  local output
  local exit_code=0
  
  if [[ "$HOST_TYPE" == "nixos" ]]; then
    local sudo_cmd=""
    if [[ "$(id -u)" != "0" ]]; then
      sudo_cmd="sudo"
    fi
    
    if output=$(run_with_timeout "$SWITCH_TIMEOUT" $sudo_cmd nixos-rebuild switch --flake ".#${HOSTNAME}" 2>&1); then
      log_info "Switch successful"
    else
      exit_code=$?
      if [[ $exit_code -eq 124 ]]; then
        log_error "Switch timed out after ${SWITCH_TIMEOUT}s"
      else
        log_error "Switch failed (exit code: $exit_code)"
      fi
    fi
  else
    local user="${USER:-$(whoami)}"
    if output=$(run_with_timeout "$SWITCH_TIMEOUT" home-manager switch --flake ".#${user}@${HOSTNAME}" 2>&1); then
      log_info "Switch successful"
    else
      exit_code=$?
      if [[ $exit_code -eq 124 ]]; then
        log_error "Switch timed out after ${SWITCH_TIMEOUT}s"
      else
        log_error "Switch failed (exit code: $exit_code)"
      fi
    fi
  fi
  
  local gen
  gen="$(get_generation)"
  
  if [[ $exit_code -eq 0 ]]; then
    report_status "ok" "$gen" "$output"
  elif [[ $exit_code -eq 124 ]]; then
    report_status "error" "$gen" "Operation timed out after ${SWITCH_TIMEOUT}s"
  else
    report_status "error" "$gen" "$output"
  fi
  
  return $exit_code
}

# Updated do_test with per-test timeout
do_test() {
  log_info "Executing: test suite (per-test timeout: ${TEST_TIMEOUT}s)"
  cd "$NIXFLEET_NIXCFG"
  
  local test_dir="hosts/$HOSTNAME/tests"
  
  if [[ ! -d "$test_dir" ]]; then
    log_info "No tests directory"
    report_test_progress 0 0 0 false "no tests"
    report_status "ok" "$(get_generation)" "No tests found" "no tests"
    return 0
  fi
  
  # Count total tests first
  local total=0
  for script in "$test_dir"/T*.sh; do
    [[ -f "$script" ]] && ((total++))
  done
  
  if [[ $total -eq 0 ]]; then
    log_info "No test scripts found in $test_dir"
    report_test_progress 0 0 0 false "no tests"
    report_status "ok" "$(get_generation)" "No test scripts found" "no tests"
    return 0
  fi
  
  local current=0 passed=0 failed=0
  local output=""
  local fail_comment=""
  
  report_test_progress 0 "$total" 0 true
  
  for script in "$test_dir"/T*.sh; do
    [[ -f "$script" ]] || continue
    ((current++))
    
    local name
    name=$(basename "$script" .sh)
    log_info "Running $name... (timeout: ${TEST_TIMEOUT}s)"
    
    report_test_progress "$current" "$total" "$passed" true
    
    local result
    local test_exit_code
    
    if result=$(run_with_timeout "$TEST_TIMEOUT" "$script" 2>&1); then
      ((passed++))
      output+="✅ $name"$'\n'
    else
      test_exit_code=$?
      ((failed++))
      if [[ $test_exit_code -eq 124 ]]; then
        output+="⏱️ $name: TIMEOUT (exceeded ${TEST_TIMEOUT}s)"$'\n'
        [[ -z "$fail_comment" ]] && fail_comment="$name timed out"
      else
        output+="❌ $name: ${result:0:100}"$'\n'
        [[ -z "$fail_comment" ]] && fail_comment="$name failed"
      fi
    fi
  done
  
  local test_status="$passed/$total pass"
  [[ $failed -gt 0 ]] && test_status="$passed/$total pass, $failed fail"
  
  log_info "Tests complete: $test_status"
  
  report_test_progress "$total" "$total" "$passed" false "$test_status" "$fail_comment"
  
  if [[ $failed -eq 0 ]]; then
    report_status "ok" "$(get_generation)" "$output" "$test_status"
  else
    report_status "error" "$(get_generation)" "$output" "$test_status"
  fi
  
  return $failed
}
```

**Timeout Values**:

- `git pull`: 2 minutes (default)
- `git push`: 2 minutes (default)
- `git add/commit`: 30 seconds (default)
- `nixos-rebuild switch`: 10 minutes (last resort, configurable)
- `home-manager switch`: 10 minutes (last resort, configurable)
- `nix flake update`: 5 minutes (default)
- Test scripts: 5 minutes per test (default)

**Note**: The 10-minute timeout for rebuilds is intentionally long as a "last resort" - rebuilds can legitimately take 5-10 minutes on slower systems or large configurations. This prevents truly stuck processes while allowing normal operations to complete.

### 5. Startup Sequence

**Enhanced startup with all checks**:

```bash
main() {
  # ... existing startup logging ...
  
  check_prerequisites
  
  # 1. Check for and kill duplicate processes
  log_info "Checking for duplicate processes..."
  kill_stale_processes
  
  # 2. Acquire lock (ensures single instance)
  acquire_lock || exit 1
  
  # 3. Initialize heartbeat tracking
  update_heartbeat_timestamp
  
  # 4. Register with dashboard
  register
  
  # 5. Main loop with health checks
  local failures=0
  local health_check_counter=0
  
  while true; do
    # Periodic health check (every 10 iterations)
    if ((health_check_counter % 10 == 0)); then
      if ! check_health; then
        log_warn "Health check failed - attempting recovery"
        kill_stale_processes
      fi
    fi
    ((health_check_counter++))
    
    # ... existing heartbeat/command logic ...
    
    if response=$(heartbeat); then
      update_heartbeat_timestamp  # Mark successful heartbeat
      # ... rest of logic ...
    fi
    
    # ... sleep logic ...
  done
}
```

## Edge Cases

| Scenario | Handling |
| --- | --- |
| Lock file exists but process is dead | Lock is stale - remove and acquire |
| Multiple processes during startup race | Lock prevents duplicates, cleanup kills stragglers |
| Agent stuck in infinite loop | Health check detects stale heartbeat, triggers recovery |
| Lock file permissions issue | Fall back to PID file approach |
| Service restart while command executing | Lock prevents new instance until old one releases |
| Network outage causing heartbeat failures | Health check distinguishes network vs process issues |
| Child process hangs indefinitely | Timeout kills process and reports error |
| Long-running rebuild exceeds timeout | 10-minute timeout allows normal rebuilds, kills truly stuck processes |

## Implementation Tasks

- [ ] Add `acquire_lock()` function with file-based locking (flock)
- [ ] Add `detect_duplicate_processes()` function
- [ ] Add `kill_stale_processes()` function
- [ ] Add `check_health()` function with heartbeat timeout
- [ ] Add `update_heartbeat_timestamp()` function
- [ ] Integrate lock acquisition at startup
- [ ] Integrate duplicate detection at startup
- [ ] Integrate health checks in main loop
- [ ] Update heartbeat success to update timestamp
- [ ] Add cleanup trap for lock file on exit
- [ ] Test duplicate process scenario (manual + service restart)
- [ ] Test lock file cleanup on normal exit
- [ ] Test lock file cleanup on crash/force kill
- [ ] Test health check recovery mechanism
- [ ] Document lock file location in agent comments
- [ ] Update service files to ensure proper cleanup
- [ ] Add `run_with_timeout()` function for child process execution
- [ ] Add timeout configuration variables (GIT_TIMEOUT, SWITCH_TIMEOUT, TEST_TIMEOUT)
- [ ] Update `do_pull()` to use timeout wrapper
- [ ] Update `do_switch()` to use timeout wrapper (10min last resort)
- [ ] Update `do_test()` to use timeout wrapper per test script
- [ ] Update `do_update()` to use timeout for git and flake operations
- [ ] Test timeout behavior with short operations (git pull)
- [ ] Test timeout behavior with long operations (rebuilds)
- [ ] Verify timeout kills processes correctly
- [ ] Verify timeout reports appropriate error messages

## Testing Plan

1. **Duplicate Process Test**:
   - Start agent via service
   - Manually run agent script
   - Verify duplicate is detected and killed
   - Verify only one instance continues

2. **Lock File Test**:
   - Start agent (creates lock)
   - Try to start second instance
   - Verify second instance exits with error
   - Kill first instance
   - Verify lock is cleaned up
   - Start new instance (should succeed)

3. **Health Check Test**:
   - Start agent
   - Simulate stuck state (kill -STOP)
   - Wait for health check timeout
   - Verify recovery attempt
   - Resume process
   - Verify normal operation resumes

4. **Service Restart Test**:
   - Start agent via systemd/launchd
   - Restart service
   - Verify no duplicate processes
   - Verify agent continues normally

5. **Timeout Test - Short Operations**:
   - Simulate slow git pull (add delay to git command)
   - Set short timeout (30s)
   - Verify operation times out correctly
   - Verify process is killed
   - Verify error status is reported

6. **Timeout Test - Long Operations**:
   - Start a rebuild operation
   - Verify normal rebuild completes within 10min timeout
   - Simulate stuck rebuild (kill -STOP on nix process)
   - Verify timeout triggers after 10 minutes
   - Verify stuck process is killed
   - Verify timeout error is reported

7. **Timeout Test - Test Scripts**:
   - Create test script that hangs indefinitely
   - Run test suite
   - Verify test times out after 5 minutes
   - Verify other tests continue running
   - Verify timeout is reported in test results

## Security Considerations

- Lock file permissions: `umask 077` to prevent other users from interfering
- Process killing: Only kill processes matching exact script name/path
- PID validation: Verify PID exists and matches expected process before killing
- No privilege escalation: Agent runs with same privileges, no sudo needed for cleanup

## Related

- Agent restart command already exists (`restart` command exits cleanly)
- Service management (systemd/launchd) handles process lifecycle
- This complements existing error handling and retry logic

## Notes

This resilience layer will:

- Prevent duplicate processes from causing conflicts
- Automatically recover from stuck states
- Provide better visibility into agent health
- Reduce need for manual intervention

The lock file approach is lightweight and works across both NixOS (systemd) and macOS (launchd) without requiring additional dependencies.
