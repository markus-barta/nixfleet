# P7200: Agent CLI Interface

**Priority**: Medium  
**Effort**: Small  
**Status**: Done

## Problem

The agent binary has zero CLI support. Running `nixfleet-agent --version` or `--help` does nothing useful — it just starts the daemon and fails if config is missing.

For operational tasks (debugging, verification, scripting), basic CLI flags are expected.

## Solution

Add standard CLI flags using Go's stdlib `flag` package:

| Flag              | Behavior                                                 |
| ----------------- | -------------------------------------------------------- |
| `--version`, `-v` | Print version and exit                                   |
| `--help`, `-h`    | Print usage and exit                                     |
| `--check`         | Validate config + test dashboard connectivity, then exit |

### Example Output

```bash
$ nixfleet-agent --version
nixfleet-agent 2.3.1

$ nixfleet-agent --check
Config OK
  Hostname: hsb1
  Dashboard: wss://fleet.barta.link/ws/agent
Testing connection... OK (latency: 42ms)

$ nixfleet-agent --help
Usage: nixfleet-agent [options]

Options:
  -v, --version   Print version and exit
  -h, --help      Print this help and exit
  --check         Validate config and test connectivity

Environment variables:
  NIXFLEET_DASHBOARD_URL   Dashboard WebSocket URL (required)
  NIXFLEET_API_TOKEN       Authentication token (required)
  NIXFLEET_HOSTNAME        Override hostname detection
  NIXFLEET_LOG_LEVEL       Log level: debug, info, warn, error
```

## Implementation

Minimal change to `v2/cmd/nixfleet-agent/main.go`:

```go
func main() {
    showVersion := flag.Bool("version", false, "print version and exit")
    showHelp := flag.Bool("help", false, "show usage")
    runCheck := flag.Bool("check", false, "validate config and test connectivity")

    // Short flags
    flag.BoolVar(showVersion, "v", false, "print version and exit")
    flag.BoolVar(showHelp, "h", false, "show usage")

    flag.Parse()

    if *showVersion {
        fmt.Printf("nixfleet-agent %s\n", agent.Version)
        os.Exit(0)
    }
    if *showHelp {
        printUsage()
        os.Exit(0)
    }
    if *runCheck {
        os.Exit(runConfigCheck())
    }

    // ... existing daemon startup ...
}
```

## Acceptance Criteria

- [ ] `nixfleet-agent --version` prints version and exits 0
- [ ] `nixfleet-agent --help` prints usage and exits 0
- [ ] `nixfleet-agent --check` validates config and tests dashboard connection
- [ ] `--check` exits 0 on success, 1 on failure
- [ ] Short flags `-v` and `-h` work
- [ ] Unknown flags print error and usage

## Out of Scope

- Self-update mechanism (see P7210)
- Force rebuild (see P7220)
- Subcommands (`nixfleet-agent serve`, etc.) — not needed for a daemon

## Related

- P7210: Dashboard Bump Agent Version
- P7220: Dashboard Force Uncached Rebuild
