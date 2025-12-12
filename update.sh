#!/usr/bin/env bash
# NixFleet update script - pulls latest and rebuilds with embedded git hash
#
# Usage: ./update.sh
#
# This script:
# 1. Pulls latest from git
# 2. Rebuilds container with current git hash embedded
# 3. Restarts the container
#
# Run this from the nixfleet directory on csb1:
#   cd ~/docker/nixfleet && ./update.sh

set -euo pipefail

cd "$(dirname "$0")"

echo "📦 Pulling latest..."
git pull

# Get the current git hash for embedding
GIT_HASH=$(git rev-parse HEAD)
echo "📌 Git hash: ${GIT_HASH:0:7}"

echo "🔨 Rebuilding container..."

# Export git hash for docker-compose build args
export GIT_HASH

# Use csb1-specific compose file if it exists, otherwise default
if [[ -f docker/docker-compose.csb1.yml ]]; then
    # Copy .env to docker/ directory where docker-compose.csb1.yml expects it
    if [[ -f .env ]]; then
        cp .env docker/.env
    fi
    docker compose -f docker/docker-compose.csb1.yml build --no-cache
    docker compose -f docker/docker-compose.csb1.yml up -d
else
    docker compose build --no-cache
    docker compose up -d
fi

echo "✅ NixFleet updated! (version: ${GIT_HASH:0:7})"
echo ""
echo "View logs: docker logs -f nixfleet"
