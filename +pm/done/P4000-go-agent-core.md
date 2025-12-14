# Go Agent: Core

**Created**: 2025-12-14
**Priority**: P4000 (Critical)
**Status**: ✅ Done

---

## Tests to Pass

| Test                                             | Description                  |
| ------------------------------------------------ | ---------------------------- |
| [T01](../../tests/specs/T01-agent-connection.md) | Agent Connection             |
| [T02](../../tests/specs/T02-agent-heartbeat.md)  | Agent Heartbeat              |
| [T03](../../tests/specs/T03-agent-commands.md)   | Agent Commands               |
| [T07](../../tests/specs/T07-e2e-deploy-flow.md)  | E2E Deploy Flow (agent part) |
| [T08](../../tests/specs/T08-e2e-test-flow.md)    | E2E Test Flow (agent part)   |

---

## Overview

Rewrite the NixFleet agent in Go with WebSocket communication, proper concurrency, and output streaming.

---

## Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                     NixFleet Agent (Go)                          │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  WebSocket  │  │  Heartbeat  │  │  Command                │  │
│  │  Client     │  │  Goroutine  │  │  Executor               │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│         │               │                   │                    │
│         └───────────────┴───────────────────┘                    │
│                         │                                        │
│                   ┌─────────────┐                                │
│                   │   Main      │                                │
│                   │   Loop      │                                │
│                   └─────────────┘                                │
└──────────────────────────────────────────────────────────────────┘
```

---

## Key Features

### 1. WebSocket Connection

- Persistent connection to dashboard
- Auto-reconnect with exponential backoff
- Heartbeat ping/pong to detect disconnection
- JSON message protocol

```go
type Message struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// Message types:
// - "register"     Agent → Dashboard (initial)
// - "heartbeat"    Agent → Dashboard (periodic)
// - "command"      Dashboard → Agent
// - "output"       Agent → Dashboard (streaming)
// - "status"       Agent → Dashboard (command result)
```

### 2. Concurrent Heartbeats

```go
func (a *Agent) Run() {
    go a.heartbeatLoop()   // Never blocks
    go a.commandLoop()     // Executes commands
    a.websocketLoop()      // Main connection
}
```

Heartbeats continue during command execution (solves the original problem).

### 3. Command Execution

- Run commands as subprocesses
- Stream stdout/stderr to dashboard in real-time
- Track PID for stop capability
- Capture exit codes

### 4. Output Streaming

```go
func (a *Agent) executeCommand(cmd string) {
    proc := exec.Command("nixos-rebuild", "switch", ...)
    stdout, _ := proc.StdoutPipe()

    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        a.sendOutput(scanner.Text())  // Real-time to dashboard
    }
}
```

---

## Configuration

Environment variables (same as bash agent for compatibility):

| Variable            | Required | Description                       |
| ------------------- | -------- | --------------------------------- |
| `NIXFLEET_URL`      | Yes      | Dashboard WebSocket URL           |
| `NIXFLEET_TOKEN`    | Yes      | Agent authentication token        |
| `NIXFLEET_REPO_URL` | Yes\*    | Git repo URL (isolated mode)      |
| `NIXFLEET_REPO_DIR` | Yes\*    | Local repo path                   |
| `NIXFLEET_BRANCH`   | No       | Git branch (default: main)        |
| `NIXFLEET_INTERVAL` | No       | Heartbeat interval (default: 30s) |
| `NIXFLEET_SSH_KEY`  | No       | SSH key for git operations        |

\*Either `NIXFLEET_REPO_URL` or legacy `NIXFLEET_NIXCFG` required.

---

## Commands

| Command       | Description                                       |
| ------------- | ------------------------------------------------- |
| `pull`        | Git fetch + reset (isolated) or git pull (legacy) |
| `switch`      | nixos-rebuild switch or home-manager switch       |
| `pull-switch` | Pull then switch                                  |
| `test`        | Run host test suite                               |
| `stop`        | Kill running command (by PID)                     |
| `restart`     | Exit (systemd/launchd restarts)                   |
| `update`      | Flake update + switch                             |

---

## Platform Support

| Platform      | Init System | Notes                |
| ------------- | ----------- | -------------------- |
| NixOS         | systemd     | nixos-rebuild switch |
| macOS (Intel) | launchd     | home-manager switch  |
| macOS (ARM)   | launchd     | home-manager switch  |

---

## Acceptance Criteria

- [ ] WebSocket connection to dashboard
- [ ] Auto-reconnect with backoff
- [ ] Heartbeat every 30s (configurable)
- [ ] Heartbeats continue during command execution
- [ ] All commands implemented (pull, switch, test, etc.)
- [ ] Output streaming to dashboard
- [ ] PID tracking for stop command
- [ ] StaSysMo metrics in heartbeat
- [ ] OS/nixpkgs version detection
- [ ] Works on NixOS and macOS
- [ ] Graceful shutdown (SIGTERM)
- [ ] Logging (structured, configurable level)

---

## Dependencies

```go
// go.mod
require (
    github.com/gorilla/websocket v1.5.0
    github.com/rs/zerolog v1.31.0  // Structured logging
)
```

---

## File Structure

```text
agent/
├── cmd/
│   └── nixfleet-agent/
│       └── main.go
├── internal/
│   ├── agent/
│   │   ├── agent.go      // Main agent logic
│   │   ├── websocket.go  // WS connection
│   │   ├── commands.go   // Command execution
│   │   └── heartbeat.go  // Heartbeat loop
│   ├── config/
│   │   └── config.go     // Env vars, validation
│   └── platform/
│       ├── nixos.go      // NixOS-specific
│       └── darwin.go     // macOS-specific
├── go.mod
└── go.sum
```

---

## Related

- Prerequisite for: P4100 (Packaging), P4200 (Dashboard)
- Replaces: `agent/nixfleet-agent.sh` (bash agent)
