#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════════╗
# ║                           NIXFLEET AGENT                                     ║
# ╚══════════════════════════════════════════════════════════════════════════════╝
#
# Polls the NixFleet dashboard for commands and executes them.
# Supports both NixOS (nixos-rebuild) and macOS (home-manager).
#
# Usage: nixfleet-agent.sh
#
# Environment (all required unless noted):
#   NIXFLEET_URL      - Dashboard URL (e.g., https://fleet.example.com)
#   NIXFLEET_TOKEN    - API authentication token
#   NIXFLEET_NIXCFG   - Absolute path to config repository
#   NIXFLEET_INTERVAL - Poll interval in seconds (optional, default: 30)
#
# Requirements:
#   - curl
#   - jq (for JSON parsing)
#   - git
#   - nixos-rebuild (NixOS) or home-manager (macOS)

set -euo pipefail

# ════════════════════════════════════════════════════════════════════════════════
# Configuration
# ════════════════════════════════════════════════════════════════════════════════

# Agent version - injected at build time by Nix, or falls back to environment/default
# shellcheck disable=SC2016
readonly AGENT_VERSION="${NIXFLEET_AGENT_VERSION:-@agentVersion@}"

# Required: Dashboard URL (no default - must be explicitly configured)
readonly NIXFLEET_URL="${NIXFLEET_URL:?ERROR: NIXFLEET_URL environment variable must be set}"
# Required: API token for authentication
NIXFLEET_TOKEN="${NIXFLEET_TOKEN:?ERROR: NIXFLEET_TOKEN environment variable must be set}"
# Required: Path to config repository
readonly NIXFLEET_NIXCFG="${NIXFLEET_NIXCFG:?ERROR: NIXFLEET_NIXCFG environment variable must be set}"
# Configure git to trust the config repo directory (required for systemd services)
# This prevents "dubious ownership" errors when running as a different user
git config --global --add safe.directory "$NIXFLEET_NIXCFG" 2>/dev/null || true
# Optional: Poll interval (default 30 seconds)
readonly NIXFLEET_INTERVAL="${NIXFLEET_INTERVAL:-30}"

# Host detection - always get short hostname (strip domain)
DETECTED_HOSTNAME="$(hostname -s 2>/dev/null || hostname)"
# Strip any domain suffix if hostname -s didn't work
readonly HOST_ID="${DETECTED_HOSTNAME%%.*}"
readonly HOSTNAME="${HOST_ID}" # For backwards compatibility

# Token cache (for per-host token migration)
detect_token_cache() {
  if [[ -n "${NIXFLEET_TOKEN_CACHE:-}" ]]; then
    echo "$NIXFLEET_TOKEN_CACHE"
    return
  fi

  if [[ "$(uname)" == "Darwin" ]]; then
    echo "${HOME}/.local/state/nixfleet-agent/token"
    return
  fi

  # Prefer systemd StateDirectory if configured via module
  if [[ -w "/var/lib" ]]; then
    echo "/var/lib/nixfleet-agent/token"
    return
  fi

  echo "/tmp/nixfleet-agent-token-${HOST_ID}"
}

TOKEN_CACHE_FILE="$(detect_token_cache)"

load_cached_token() {
  if [[ -f "$TOKEN_CACHE_FILE" ]]; then
    local t
    t="$(cat "$TOKEN_CACHE_FILE" 2>/dev/null || true)"
    t="${t#"NIXFLEET_TOKEN="}"
    t="${t//$'\r'/}"
    if [[ -n "$t" ]]; then
      echo "$t"
      return
    fi
  fi
  echo "$NIXFLEET_TOKEN"
}

save_cached_token() {
  local token="$1"
  local dir
  dir="$(dirname "$TOKEN_CACHE_FILE")"
  mkdir -p "$dir" 2>/dev/null || true
  # best-effort permissions
  umask 077
  printf "NIXFLEET_TOKEN=%s\n" "$token" >"$TOKEN_CACHE_FILE" 2>/dev/null || true
}

CURRENT_TOKEN="$(load_cached_token)"

# ════════════════════════════════════════════════════════════════════════════════
# Host Type Detection
# ════════════════════════════════════════════════════════════════════════════════

detect_host_type() {
  if [[ -f /etc/NIXOS ]]; then
    echo "nixos"
  elif [[ "$(uname)" == "Darwin" ]]; then
    echo "macos"
  else
    echo "linux"
  fi
}

detect_location() {
  # Use env var if set, otherwise detect from hostname
  if [[ -n "${NIXFLEET_LOCATION:-}" ]]; then
    echo "$NIXFLEET_LOCATION"
    return
  fi
  case "$HOSTNAME" in
  hsb* | gpc* | imac*) echo "home" ;;
  csb*) echo "cloud" ;;
  mba-*-work) echo "work" ;;
  *) echo "home" ;;
  esac
}

detect_device_type() {
  # Use env var if set, otherwise detect from hostname
  if [[ -n "${NIXFLEET_DEVICE_TYPE:-}" ]]; then
    echo "$NIXFLEET_DEVICE_TYPE"
    return
  fi
  case "$HOSTNAME" in
  hsb* | csb*) echo "server" ;;
  gpc*) echo "gaming" ;;
  *mbp* | *mba*) echo "laptop" ;;
  imac*) echo "desktop" ;;
  *) echo "server" ;;
  esac
}

detect_criticality() {
  case "$HOSTNAME" in
  hsb0 | csb0) echo "high" ;;
  hsb1 | hsb8 | csb1) echo "medium" ;;
  *) echo "low" ;;
  esac
}

# ════════════════════════════════════════════════════════════════════════════════
# OS Version Detection
# ════════════════════════════════════════════════════════════════════════════════

detect_nixpkgs_version() {
  # Get the nixpkgs revision the system was built with
  if [[ -f /etc/NIXOS ]]; then
    # NixOS: use nixos-version --json
    if command -v nixos-version &>/dev/null; then
      nixos-version --json 2>/dev/null | jq -r '.nixpkgsRevision // empty' 2>/dev/null || echo ""
    else
      echo ""
    fi
  elif [[ "$(uname)" == "Darwin" ]]; then
    # macOS with nix: try to get nixpkgs from current profile
    # This is best-effort - may not always work
    local profile_path
    profile_path="$(readlink -f ~/.nix-profile 2>/dev/null || echo "")"
    if [[ -n "$profile_path" && -f "${profile_path}/.nixpkgs-version" ]]; then
      cat "${profile_path}/.nixpkgs-version" 2>/dev/null || echo ""
    else
      echo ""
    fi
  else
    echo ""
  fi
}

detect_os_version() {
  # Get human-readable OS version string
  if [[ -f /etc/NIXOS ]]; then
    # NixOS: get full version string (e.g., "24.11.20241210.abc1234")
    if command -v nixos-version &>/dev/null; then
      nixos-version 2>/dev/null | head -1 || echo ""
    else
      echo ""
    fi
  elif [[ "$(uname)" == "Darwin" ]]; then
    # macOS: get version (e.g., "14.7.1")
    sw_vers -productVersion 2>/dev/null || echo ""
  else
    # Generic Linux: try /etc/os-release
    if [[ -f /etc/os-release ]]; then
      # shellcheck source=/dev/null
      . /etc/os-release
      echo "${VERSION_ID:-unknown}"
    else
      echo ""
    fi
  fi
}

detect_os_name() {
  # Get OS name for display (e.g., "macOS Sonoma", "NixOS")
  if [[ -f /etc/NIXOS ]]; then
    echo "NixOS"
  elif [[ "$(uname)" == "Darwin" ]]; then
    # Get macOS marketing name (e.g., "Sonoma")
    local name
    name=$(awk '/SOFTWARE LICENSE AGREEMENT FOR macOS/' '/System/Library/CoreServices/Setup Assistant.app/Contents/Resources/en.lproj/OSXSoftwareLicense.rtf' 2>/dev/null | awk -F 'macOS ' '{print $NF}' | awk '{print $1}' || echo "")
    if [[ -z "$name" ]]; then
      # Fallback: just say macOS
      echo "macOS"
    else
      echo "macOS $name"
    fi
  else
    echo "Linux"
  fi
}

HOST_TYPE="$(detect_host_type)"
readonly HOST_TYPE
LOCATION="$(detect_location)"
readonly LOCATION
DEVICE_TYPE="$(detect_device_type)"
readonly DEVICE_TYPE
THEME_COLOR="${NIXFLEET_THEME_COLOR:-#769ff0}"
readonly THEME_COLOR
CRITICALITY="$(detect_criticality)"
readonly CRITICALITY
NIXPKGS_VERSION="$(detect_nixpkgs_version)"
readonly NIXPKGS_VERSION
OS_VERSION="$(detect_os_version)"
readonly OS_VERSION
OS_NAME="$(detect_os_name)"
readonly OS_NAME

# ════════════════════════════════════════════════════════════════════════════════
# StaSysMo Metrics (optional)
# ════════════════════════════════════════════════════════════════════════════════

get_stasysmo_metrics() {
  # Detect StaSysMo directory based on platform
  local stasysmo_dir
  if [[ "$(uname)" == "Darwin" ]]; then
    stasysmo_dir="/tmp/stasysmo"
  else
    stasysmo_dir="/dev/shm/stasysmo"
  fi

  # Check if StaSysMo is running (timestamp file exists and is fresh)
  local timestamp_file="$stasysmo_dir/timestamp"
  if [[ ! -f "$timestamp_file" ]]; then
    echo ""
    return
  fi

  # Check if data is stale (older than 30 seconds)
  local file_ts now age
  file_ts=$(cat "$timestamp_file" 2>/dev/null || echo 0)
  now=$(date +%s)
  age=$((now - file_ts))
  if [[ $age -gt 30 ]]; then
    echo ""
    return
  fi

  # Read metrics
  local cpu ram swap load
  cpu=$(cat "$stasysmo_dir/cpu" 2>/dev/null || echo "")
  ram=$(cat "$stasysmo_dir/ram" 2>/dev/null || echo "")
  swap=$(cat "$stasysmo_dir/swap" 2>/dev/null || echo "")
  load=$(cat "$stasysmo_dir/load" 2>/dev/null || echo "")

  # Return JSON if we have at least CPU and RAM
  if [[ -n "$cpu" && -n "$ram" ]]; then
    jq -n \
      --argjson cpu "${cpu:-0}" \
      --argjson ram "${ram:-0}" \
      --argjson swap "${swap:-0}" \
      --arg load "${load:-0}" \
      '{cpu: $cpu, ram: $ram, swap: $swap, load: $load}'
  else
    echo ""
  fi
}

# ════════════════════════════════════════════════════════════════════════════════
# Logging
# ════════════════════════════════════════════════════════════════════════════════

log() {
  local level="$1"
  shift
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $*" >&2
}

log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "$@"; }
log_error() { log "ERROR" "$@"; }

# ════════════════════════════════════════════════════════════════════════════════
# Prerequisites Check
# ════════════════════════════════════════════════════════════════════════════════

check_prerequisites() {
  local missing=()

  command -v curl &>/dev/null || missing+=("curl")
  command -v jq &>/dev/null || missing+=("jq")
  command -v git &>/dev/null || missing+=("git")

  if [[ "$HOST_TYPE" == "nixos" ]]; then
    command -v nixos-rebuild &>/dev/null || missing+=("nixos-rebuild")
  elif [[ "$HOST_TYPE" == "macos" ]]; then
    command -v home-manager &>/dev/null || missing+=("home-manager")
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "Missing required commands: ${missing[*]}"
    exit 1
  fi

  if [[ ! -d "$NIXFLEET_NIXCFG" ]]; then
    log_error "nixcfg directory not found: $NIXFLEET_NIXCFG"
    exit 1
  fi
}

# ════════════════════════════════════════════════════════════════════════════════
# API Helpers
# ════════════════════════════════════════════════════════════════════════════════

api_call() {
  local method="$1"
  local endpoint="$2"
  local data="${3:-}"

  local args=(-sS -X "$method" --connect-timeout 5 --max-time 30)
  if [[ "$NIXFLEET_URL" == https://* ]]; then
    args+=(--proto '=https' --tlsv1.2)
  else
    if [[ "${NIXFLEET_ALLOW_INSECURE_HTTP:-}" =~ ^(1|true|yes)$ ]]; then
      log_warn "Using insecure HTTP for NIXFLEET_URL (dev only)"
    else
      log_error "Refusing insecure NIXFLEET_URL (not https). Set NIXFLEET_ALLOW_INSECURE_HTTP=true for local dev."
      API_HTTP_CODE=0
      return 1
    fi
  fi
  args+=(-H "Content-Type: application/json")

  # Optional TLS hardening
  if [[ "$NIXFLEET_URL" == https://* && -n "${NIXFLEET_CA_FILE:-}" ]]; then
    args+=(--cacert "$NIXFLEET_CA_FILE")
  fi
  if [[ "$NIXFLEET_URL" == https://* && -n "${NIXFLEET_PINNED_PUBKEY:-}" ]]; then
    args+=(--pinnedpubkey "$NIXFLEET_PINNED_PUBKEY")
  fi

  if [[ -n "$CURRENT_TOKEN" ]]; then
    args+=(-H "Authorization: Bearer $CURRENT_TOKEN")
  fi

  if [[ -n "$data" ]]; then
    args+=(-d "$data")
  fi

  local out http_code body
  if ! out=$(curl "${args[@]}" --write-out $'\n%{http_code}' "${NIXFLEET_URL}${endpoint}"); then
    API_HTTP_CODE=0
    return 1
  fi

  http_code="${out##*$'\n'}"
  body="${out%$'\n'*}"
  API_HTTP_CODE="$http_code"

  # Per-host token migration: if server returns agent_token, switch and persist.
  local new_token
  new_token="$(echo "$body" | jq -r '.agent_token // empty' 2>/dev/null || true)"
  if [[ -n "$new_token" && "$new_token" != "$CURRENT_TOKEN" ]]; then
    log_info "Received per-host agent token from server; switching auth token"
    CURRENT_TOKEN="$new_token"
    save_cached_token "$CURRENT_TOKEN"
  fi

  # Treat 2xx as success
  if [[ "$API_HTTP_CODE" =~ ^2 ]]; then
    printf "%s" "$body"
    return 0
  fi

  return 2
}

# ════════════════════════════════════════════════════════════════════════════════
# Host Information (with caching)
# ════════════════════════════════════════════════════════════════════════════════

# Cache file for git hash (avoid calling git every poll)
readonly GIT_HASH_CACHE="/tmp/nixfleet-git-hash-${HOST_ID}"

refresh_git_hash() {
  # Fetch git hash and update cache - call after pull/switch
  local hash
  if hash=$(git -C "$NIXFLEET_NIXCFG" rev-parse HEAD 2>/dev/null); then
    :
  else
    hash="unknown"
  fi
  echo "$hash" >"$GIT_HASH_CACHE"
  echo "$hash"
}

get_git_hash() {
  # Read from cache, or refresh if cache doesn't exist
  if [[ -f "$GIT_HASH_CACHE" ]]; then
    cat "$GIT_HASH_CACHE"
  else
    refresh_git_hash
  fi
}

get_generation() {
  # Return current git hash
  get_git_hash
}

get_test_status() {
  local test_dir="$NIXFLEET_NIXCFG/hosts/$HOSTNAME/tests"

  if [[ ! -d "$test_dir" ]]; then
    echo "no tests"
    return
  fi

  local count=0
  for script in "$test_dir"/T*.sh; do
    [[ -f "$script" ]] && ((count++))
  done

  if [[ $count -eq 0 ]]; then
    echo "no tests"
  else
    echo "$count tests"
  fi
}

# ════════════════════════════════════════════════════════════════════════════════
# Registration
# ════════════════════════════════════════════════════════════════════════════════

register() {
  local gen
  gen="$(get_generation)"

  log_info "Registering: $HOSTNAME ($HOST_TYPE, $LOCATION, $DEVICE_TYPE)"

  # Get optional StaSysMo metrics
  local metrics_json
  metrics_json=$(get_stasysmo_metrics)

  local payload
  if [[ -n "$metrics_json" ]]; then
    payload=$(jq -n \
      --arg hostname "$HOSTNAME" \
      --arg host_type "$HOST_TYPE" \
      --arg location "$LOCATION" \
      --arg device_type "$DEVICE_TYPE" \
      --arg theme_color "$THEME_COLOR" \
      --arg criticality "$CRITICALITY" \
      --arg generation "$gen" \
      --arg config_repo "$NIXFLEET_NIXCFG" \
      --arg agent_version "$AGENT_VERSION" \
      --arg nixpkgs_version "$NIXPKGS_VERSION" \
      --arg os_version "$OS_VERSION" \
      --arg os_name "$OS_NAME" \
      --argjson poll_interval "$NIXFLEET_INTERVAL" \
      --argjson metrics "$metrics_json" \
      '{
              hostname: $hostname,
              host_type: $host_type,
              location: $location,
              device_type: $device_type,
              theme_color: $theme_color,
              criticality: $criticality,
              current_generation: $generation,
              config_repo: $config_repo,
              poll_interval: $poll_interval,
              agent_version: $agent_version,
              nixpkgs_version: (if $nixpkgs_version == "" then null else $nixpkgs_version end),
              os_version: (if $os_version == "" then null else $os_version end),
              os_name: (if $os_name == "" then null else $os_name end),
              metrics: $metrics
          }')
  else
    payload=$(jq -n \
      --arg hostname "$HOSTNAME" \
      --arg host_type "$HOST_TYPE" \
      --arg location "$LOCATION" \
      --arg device_type "$DEVICE_TYPE" \
      --arg theme_color "$THEME_COLOR" \
      --arg criticality "$CRITICALITY" \
      --arg generation "$gen" \
      --arg config_repo "$NIXFLEET_NIXCFG" \
      --arg agent_version "$AGENT_VERSION" \
      --arg nixpkgs_version "$NIXPKGS_VERSION" \
      --arg os_version "$OS_VERSION" \
      --arg os_name "$OS_NAME" \
      --argjson poll_interval "$NIXFLEET_INTERVAL" \
      '{
              hostname: $hostname,
              host_type: $host_type,
              location: $location,
              device_type: $device_type,
              theme_color: $theme_color,
              criticality: $criticality,
              current_generation: $generation,
              config_repo: $config_repo,
              poll_interval: $poll_interval,
              agent_version: $agent_version,
              nixpkgs_version: (if $nixpkgs_version == "" then null else $nixpkgs_version end),
              os_version: (if $os_version == "" then null else $os_version end),
              os_name: (if $os_name == "" then null else $os_name end)
          }')
  fi

  if api_call POST "/api/hosts/${HOST_ID}/register" "$payload" >/dev/null; then
    log_info "Registration successful"
  else
    if [[ "${API_HTTP_CODE:-0}" == "401" || "${API_HTTP_CODE:-0}" == "403" ]]; then
      log_error "Registration failed: auth rejected (HTTP ${API_HTTP_CODE})"
    else
      log_warn "Registration failed (HTTP ${API_HTTP_CODE:-0}; will retry)"
    fi
  fi
}

# ════════════════════════════════════════════════════════════════════════════════
# Heartbeat (replaces separate poll + periodic register)
# ════════════════════════════════════════════════════════════════════════════════

heartbeat() {
  # Heartbeat sends current state (metrics, generation, etc.) and receives any pending command
  # This is a single API call that does both registration update and command polling
  local gen metrics_json payload
  gen="$(get_generation)"
  metrics_json="$(get_stasysmo_metrics)"
  
  if [[ -n "$metrics_json" ]]; then
    payload=$(jq -n \
      --arg hostname "$HOSTNAME" \
      --arg host_type "$HOST_TYPE" \
      --arg location "$LOCATION" \
      --arg device_type "$DEVICE_TYPE" \
      --arg theme_color "$THEME_COLOR" \
      --arg criticality "$CRITICALITY" \
      --arg generation "$gen" \
      --arg config_repo "$NIXFLEET_NIXCFG" \
      --arg agent_version "$AGENT_VERSION" \
      --arg nixpkgs_version "$NIXPKGS_VERSION" \
      --arg os_version "$OS_VERSION" \
      --arg os_name "$OS_NAME" \
      --argjson poll_interval "$NIXFLEET_INTERVAL" \
      --argjson metrics "$metrics_json" \
      '{
          hostname: $hostname,
          host_type: $host_type,
          location: $location,
          device_type: $device_type,
          theme_color: $theme_color,
          criticality: $criticality,
          current_generation: $generation,
          config_repo: $config_repo,
          poll_interval: $poll_interval,
          agent_version: $agent_version,
          nixpkgs_version: (if $nixpkgs_version == "" then null else $nixpkgs_version end),
          os_version: (if $os_version == "" then null else $os_version end),
          os_name: (if $os_name == "" then null else $os_name end),
          metrics: $metrics
      }')
  else
    payload=$(jq -n \
      --arg hostname "$HOSTNAME" \
      --arg host_type "$HOST_TYPE" \
      --arg location "$LOCATION" \
      --arg device_type "$DEVICE_TYPE" \
      --arg theme_color "$THEME_COLOR" \
      --arg criticality "$CRITICALITY" \
      --arg generation "$gen" \
      --arg config_repo "$NIXFLEET_NIXCFG" \
      --arg agent_version "$AGENT_VERSION" \
      --arg nixpkgs_version "$NIXPKGS_VERSION" \
      --arg os_version "$OS_VERSION" \
      --arg os_name "$OS_NAME" \
      --argjson poll_interval "$NIXFLEET_INTERVAL" \
      '{
          hostname: $hostname,
          host_type: $host_type,
          location: $location,
          device_type: $device_type,
          theme_color: $theme_color,
          criticality: $criticality,
          current_generation: $generation,
          config_repo: $config_repo,
          poll_interval: $poll_interval,
          agent_version: $agent_version,
          nixpkgs_version: (if $nixpkgs_version == "" then null else $nixpkgs_version end),
          os_version: (if $os_version == "" then null else $os_version end),
          os_name: (if $os_name == "" then null else $os_name end)
      }')
  fi
  
  # Call register endpoint which now returns any pending command
  api_call POST "/api/hosts/${HOST_ID}/register" "$payload"
}

# ════════════════════════════════════════════════════════════════════════════════
# Status Reporting
# ════════════════════════════════════════════════════════════════════════════════

report_status() {
  local status="$1"
  local gen="$2"
  local output="${3:-}"
  local test_status="${4:-}"

  # Truncate output to 10KB and escape for JSON
  local truncated_output
  truncated_output=$(echo "$output" | head -c 10000)

  local payload
  payload=$(jq -n \
    --arg status "$status" \
    --arg generation "$gen" \
    --arg output "$truncated_output" \
    --arg test_status "$test_status" \
    '{
            status: $status,
            current_generation: $generation,
            output: (if $output == "" then null else $output end),
            test_status: (if $test_status == "" then null else $test_status end)
        }')

  if api_call POST "/api/hosts/${HOST_ID}/status" "$payload" >/dev/null; then
    log_info "Status reported: $status"
  else
    log_warn "Failed to report status"
  fi
}

report_test_progress() {
  local current="$1"
  local total="$2"
  local passed="$3"
  local running="$4"
  local result="${5:-}"
  local comment="${6:-}"

  local payload
  payload=$(jq -n \
    --argjson current "$current" \
    --argjson total "$total" \
    --argjson passed "$passed" \
    --argjson running "$running" \
    --arg result "$result" \
    --arg comment "$comment" \
    '{
            current: $current,
            total: $total,
            passed: $passed,
            running: $running,
            result: (if $result == "" then null else $result end),
            comment: (if $comment == "" then null else $comment end)
        }')

  api_call POST "/api/hosts/${HOST_ID}/test-progress" "$payload" >/dev/null 2>&1 || true
}

# ════════════════════════════════════════════════════════════════════════════════
# Command Execution
# ════════════════════════════════════════════════════════════════════════════════

do_pull() {
  log_info "Executing: git pull"
  cd "$NIXFLEET_NIXCFG"

  local output
  if output=$(git pull 2>&1); then
    log_info "Pull successful"
    refresh_git_hash >/dev/null # Update cached hash after pull
    report_status "ok" "$(get_generation)" "$output"
    return 0
  else
    log_error "Pull failed"
    report_status "error" "$(get_generation)" "$output"
    return 1
  fi
}

do_switch() {
  log_info "Executing: switch ($HOST_TYPE)"
  cd "$NIXFLEET_NIXCFG"

  local output
  local exit_code=0

  if [[ "$HOST_TYPE" == "nixos" ]]; then
    # Use sudo only if not already root
    local sudo_cmd=""
    if [[ "$(id -u)" != "0" ]]; then
      sudo_cmd="sudo"
    fi
    if output=$($sudo_cmd nixos-rebuild switch --flake ".#${HOSTNAME}" 2>&1); then
      log_info "Switch successful"
    else
      exit_code=1
      log_error "Switch failed"
    fi
  else
    local user="${USER:-$(whoami)}"
    if output=$(home-manager switch --flake ".#${user}@${HOSTNAME}" 2>&1); then
      log_info "Switch successful"
    else
      exit_code=1
      log_error "Switch failed"
    fi
  fi

  local gen
  gen="$(get_generation)"

  if [[ $exit_code -eq 0 ]]; then
    report_status "ok" "$gen" "$output"
  else
    report_status "error" "$gen" "$output"
  fi

  return $exit_code
}

do_update() {
  log_info "Executing: update agent (flake update + switch)"
  cd "$NIXFLEET_NIXCFG"

  # Update nixfleet input
  log_info "Updating nixfleet flake input..."
  local update_output
  if ! update_output=$(nix flake update nixfleet 2>&1); then
    log_error "Flake update failed"
    report_status "error" "$(get_generation)" "$update_output" "update failed"
    return 1
  fi

  # Check if flake.lock changed
  if git diff --quiet flake.lock 2>/dev/null; then
    log_info "No changes to flake.lock (already up to date)"
  else
    # Commit and push the change
    log_info "Committing flake.lock update..."
    git add flake.lock
    git commit -m "chore: Update nixfleet flake input"

    log_info "Pushing to remote..."
    if ! git push 2>&1; then
      log_warn "Push failed (may already be updated by another host)"
      git reset --soft HEAD~1  # Undo commit if push failed
      git checkout flake.lock  # Restore original
      git pull --rebase        # Get latest
    fi
  fi

  # Refresh git hash after potential changes
  refresh_git_hash >/dev/null

  # Now rebuild with the updated flake
  do_switch
}

do_test() {
  log_info "Executing: test suite"
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

  # Handle case where test directory exists but has no test scripts
  if [[ $total -eq 0 ]]; then
    log_info "No test scripts found in $test_dir"
    report_test_progress 0 0 0 false "no tests"
    report_status "ok" "$(get_generation)" "No test scripts found" "no tests"
    return 0
  fi

  local current=0 passed=0 failed=0
  local output=""
  local fail_comment=""

  # Report initial progress
  report_test_progress 0 "$total" 0 true

  for script in "$test_dir"/T*.sh; do
    [[ -f "$script" ]] || continue
    ((current++))

    local name
    name=$(basename "$script" .sh)
    log_info "Running $name..."

    # Report progress before running test
    report_test_progress "$current" "$total" "$passed" true

    local result
    if result=$("$script" 2>&1); then
      ((passed++))
      output+="✅ $name"$'\n'
    else
      ((failed++))
      output+="❌ $name: ${result:0:100}"$'\n'
      [[ -z "$fail_comment" ]] && fail_comment="$name failed"
    fi
  done

  local test_status="$passed/$total pass"
  [[ $failed -gt 0 ]] && test_status="$passed/$total pass, $failed fail"

  log_info "Tests complete: $test_status"

  # Report final progress (running=false)
  report_test_progress "$total" "$total" "$passed" false "$test_status" "$fail_comment"

  if [[ $failed -eq 0 ]]; then
    report_status "ok" "$(get_generation)" "$output" "$test_status"
  else
    report_status "error" "$(get_generation)" "$output" "$test_status"
  fi

  return $failed
}

# ════════════════════════════════════════════════════════════════════════════════
# Main Loop
# ════════════════════════════════════════════════════════════════════════════════

main() {
  log_info "╔══════════════════════════════════════════════════════════════╗"
  log_info "║                    NixFleet Agent Starting                   ║"
  log_info "╚══════════════════════════════════════════════════════════════╝"
  log_info "Agent:       v$AGENT_VERSION"
  log_info "URL:         $NIXFLEET_URL"
  log_info "Host:        $HOSTNAME ($HOST_TYPE)"
  log_info "OS:          $OS_NAME $OS_VERSION"
  [[ -n "$NIXPKGS_VERSION" ]] && log_info "Nixpkgs:     ${NIXPKGS_VERSION:0:12}"
  log_info "Location:    $LOCATION"
  log_info "Device:      $DEVICE_TYPE"
  log_info "Theme:       $THEME_COLOR"
  log_info "Criticality: $CRITICALITY"
  log_info "nixcfg:      $NIXFLEET_NIXCFG"
  log_info "Interval:    ${NIXFLEET_INTERVAL}s"

  check_prerequisites

  # Cache git hash at startup (avoids repeated git calls)
  refresh_git_hash >/dev/null

  # Initial registration
  register

  local failures=0

  while true; do
    local response command sleep_s
    
    # Heartbeat: register sends metrics AND returns any pending command
    # This is more efficient than separate poll + periodic register
    if response=$(heartbeat); then
      command=$(echo "$response" | jq -r '.command // empty')
      failures=0
    else
      command=""
      failures=$((failures + 1))
      if [[ "${API_HTTP_CODE:-0}" == "401" || "${API_HTTP_CODE:-0}" == "403" ]]; then
        # Auth failures: back off aggressively to avoid spam/overload
        ((failures < 5)) && failures=5
      fi
      log_warn "Heartbeat failed (HTTP ${API_HTTP_CODE:-0}); failures=$failures"
    fi

    if [[ -n "$command" ]]; then
      log_info "Received command: $command"

      case "$command" in
      pull)
        do_pull || true
        ;;
      switch)
        do_switch || true
        ;;
      pull-switch)
        if do_pull; then
          do_switch || true
        fi
        ;;
      test)
        do_test || true
        ;;
      update)
        do_update || true
        ;;
      restart)
        log_info "Restart command received - exiting for service restart"
        report_status "ok" "$(get_generation)" "Agent restarting..."
        exit 0  # Exit cleanly; systemd/launchd will restart us
        ;;
      *)
        log_warn "Unknown command: $command"
        report_status "error" "$(get_generation)" "Unknown command: $command"
        ;;
      esac
    fi

    sleep_s="$NIXFLEET_INTERVAL"
    if ((failures > 0)); then
      local capped
      capped=$failures
      ((capped > 8)) && capped=8
      sleep_s=$(( NIXFLEET_INTERVAL * (1 << capped) ))
      ((sleep_s > 300)) && sleep_s=300
      sleep_s=$(( sleep_s + (RANDOM % 3) ))
    fi

    sleep "$sleep_s"
  done
}

# Run
main "$@"
