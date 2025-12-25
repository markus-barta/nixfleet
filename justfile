# Use `just <recipe>` to run a recipe
# https://just.systems/man/en/

import ".shared/common.just"

# P7400: Version from single source of truth

version := `cat VERSION`
git_commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_time := `date -u +%Y-%m-%dT%H:%M:%SZ`

# By default, run the `--list` command
default:
    @just --list

# Build dashboard with version injection
build-dashboard:
    cd v2 && templ generate && go build \
        -ldflags="-s -w \
            -X github.com/markus-barta/nixfleet/v2/internal/dashboard.Version={{ version }} \
            -X github.com/markus-barta/nixfleet/v2/internal/dashboard.GitCommit={{ git_commit }} \
            -X github.com/markus-barta/nixfleet/v2/internal/dashboard.BuildTime={{ build_time }}" \
        -o ../bin/nixfleet-dashboard \
        ./cmd/nixfleet-dashboard

# Build agent with version injection
build-agent:
    cd v2 && go build \
        -ldflags="-s -w \
            -X github.com/markus-barta/nixfleet/v2/internal/agent.Version={{ version }}" \
        -o ../bin/nixfleet-agent \
        ./cmd/nixfleet-agent

# Build both binaries
build: build-dashboard build-agent

# P7400: Bump version - updates VERSION file
bump-version NEW_VERSION:
    @echo "{{ NEW_VERSION }}" > VERSION
    @echo "✅ Version bumped to {{ NEW_VERSION }}"
    @echo "Next steps:"
    @echo "  git add VERSION && git commit -m 'chore: bump version to {{ NEW_VERSION }}'"
    @echo "  git tag v{{ NEW_VERSION }}"
    @echo "  git push && git push --tags"

# Deploy to csb1: push, wait for Docker build, pull & restart
deploy:
    @echo "📤 Pushing to GitHub..."
    git push
    @echo "⏳ Waiting for Docker build..."
    @sleep 3
    gh run watch $(gh run list -w docker.yml -L 1 --json databaseId -q '.[0].databaseId') --exit-status
    @echo "🚀 Deploying to csb1..."
    ssh mba@cs1.barta.cm -p 2222 "cd ~/docker && docker compose pull nixfleet && docker compose up -d nixfleet"
    @echo "✅ Deployed!"

# Quick deploy: skip waiting (use when you know build is done)
deploy-now:
    ssh mba@cs1.barta.cm -p 2222 "cd ~/docker && docker compose pull nixfleet && docker compose up -d nixfleet"
