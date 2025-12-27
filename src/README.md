# NixFleet v2 - Go Implementation

This directory contains the Go rewrite of NixFleet, featuring WebSocket communication and concurrent heartbeats.

## Quick Start

```bash
# Enter development environment
cd nixfleet && direnv allow

# Run tests
test-agent

# Build agent binary
build-agent

# Run agent (requires env vars)
NIXFLEET_URL=ws://localhost:8000/ws \
NIXFLEET_TOKEN=your-token \
NIXFLEET_REPO_DIR=/path/to/nixcfg \
./bin/nixfleet-agent
```

## Project Structure

```text
v2/
├── cmd/
│   └── nixfleet-agent/       # Agent entry point
├── internal/
│   ├── agent/                # Agent implementation
│   │   ├── agent.go          # Main agent struct, Run/Shutdown
│   │   ├── websocket.go      # WebSocket client with reconnect
│   │   ├── heartbeat.go      # Concurrent heartbeat loop
│   │   └── commands.go       # Command execution, output streaming
│   ├── config/               # Environment configuration
│   └── protocol/             # Shared message types
└── tests/
    └── integration/          # Integration tests (T01-T03)
```

## Key Features

### ✅ Concurrent Heartbeats (Core v2.0 Win)

Unlike v1, heartbeats continue during command execution:

```text
Timeline:
  0s: Command starts (nixos-rebuild switch)
  1s: Heartbeat sent ✓
  2s: Heartbeat sent ✓
  ...
  5m: Command completes
  → Host never appears offline!
```

### ✅ WebSocket Communication

- Single persistent connection to dashboard
- Auto-reconnect with exponential backoff (1s → 60s max)
- Ping/pong keepalive (45s timeout)
- JSON message protocol

### ✅ Output Streaming

Command output is streamed line-by-line to the dashboard in real-time.

### ✅ Command Rejection

Concurrent commands are rejected (no queue for now):

```json
{
  "type": "command_rejected",
  "payload": { "reason": "command already running" }
}
```

## Configuration

| Variable             | Required | Description                       | Default |
| -------------------- | -------- | --------------------------------- | ------- |
| `NIXFLEET_URL`       | Yes      | Dashboard WebSocket URL           | -       |
| `NIXFLEET_TOKEN`     | Yes      | Agent authentication token        | -       |
| `NIXFLEET_REPO_DIR`  | Yes      | Local repository path             | -       |
| `NIXFLEET_REPO_URL`  | No       | Git repo URL (isolated mode)      | -       |
| `NIXFLEET_BRANCH`    | No       | Git branch                        | main    |
| `NIXFLEET_INTERVAL`  | No       | Heartbeat interval (seconds)      | 30      |
| `NIXFLEET_SSH_KEY`   | No       | SSH key for git operations        | -       |
| `NIXFLEET_HOSTNAME`  | No       | Override system hostname          | -       |
| `NIXFLEET_LOG_LEVEL` | No       | Log level (debug/info/warn/error) | info    |

## Testing

15 tests covering T01-T03 specifications:

```bash
test-agent
# ✅ TestAgentConnection_Success
# ✅ TestAgentConnection_InvalidToken
# ✅ TestAgentConnection_Reconnect
# ✅ TestAgentConnection_MalformedMessage
# ✅ TestAgentHeartbeat_Regular
# ✅ TestAgentHeartbeat_DuringCommand  ← Core v2.0 test!
# ✅ TestAgentHeartbeat_WithoutMetrics
# ✅ TestAgentHeartbeat_ConcurrentCommandRejection
# ✅ TestAgentHeartbeat_ImmediateFirst
# ✅ TestAgentCommand_OutputStreaming
# ✅ TestAgentCommand_Stop
# ✅ TestAgentCommand_Failure
# ✅ TestAgentCommand_Pull
# ✅ TestAgentCommand_HeartbeatsDuringLongCommand
```

## Development

```bash
# Run linter
lint

# Run specific test
test-agent -run TestAgentHeartbeat_DuringCommand

# Build binary
build-agent
```

## Protocol

### Message Format

```json
{"type": "message_type", "payload": {...}}
```

### Message Types

| Type               | Direction | Description                     |
| ------------------ | --------- | ------------------------------- |
| `register`         | A→D       | Agent registration              |
| `registered`       | D→A       | Registration acknowledged       |
| `heartbeat`        | A→D       | Periodic status update          |
| `command`          | D→A       | Execute command                 |
| `output`           | A→D       | Command output line             |
| `status`           | A→D       | Command completed               |
| `command_rejected` | A→D       | Command rejected (already busy) |

## Next Steps

- [ ] T04-T06: Dashboard implementation
- [ ] T07-T08: End-to-end tests
- [ ] Nix packaging (flake module)
