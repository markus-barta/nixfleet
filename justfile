# Use `just <recipe>` to run a recipe
# https://just.systems/man/en/

import ".shared/common.just"

# Version info for builds

version := "2.0.0"
git_commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_time := `date -u +%Y-%m-%dT%H:%M:%SZ`

# By default, run the `--list` command
default:
    @just --list

# Build dashboard with version injection
build-dashboard:
    cd v2 && templ generate && go build \
        -ldflags="-s -w \
            -X github.com/pbek/nixfleet/v2/internal/dashboard.Version={{ version }} \
            -X github.com/pbek/nixfleet/v2/internal/dashboard.GitCommit={{ git_commit }} \
            -X github.com/pbek/nixfleet/v2/internal/dashboard.BuildTime={{ build_time }}" \
        -o ../bin/nixfleet-dashboard \
        ./cmd/nixfleet-dashboard

# Build agent with version injection
build-agent:
    cd v2 && go build \
        -ldflags="-s -w \
            -X github.com/pbek/nixfleet/v2/internal/agent.Version={{ version }} \
            -X github.com/pbek/nixfleet/v2/internal/agent.GitCommit={{ git_commit }} \
            -X github.com/pbek/nixfleet/v2/internal/agent.BuildTime={{ build_time }}" \
        -o ../bin/nixfleet-agent \
        ./cmd/nixfleet-agent

# Build both binaries
build: build-dashboard build-agent
