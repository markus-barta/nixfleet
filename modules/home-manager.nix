# NixFleet Agent for Home Manager (macOS/Linux user-level)
#
# Provides a launchd agent (macOS) or systemd user service (Linux)
# that polls the NixFleet dashboard for commands.
#
# Usage (via flake):
#   inputs.nixfleet.url = "github:markus-barta/nixfleet";
#
#   # In homeManagerConfiguration modules list:
#   inputs.nixfleet.homeManagerModules.nixfleet-agent
#
#   # In home.nix:
#   services.nixfleet-agent = {
#     enable = true;
#     url = "https://fleet.example.com";
#     tokenFile = "/Users/myuser/.config/nixfleet/token";
#     configRepo = "/Users/myuser/Code/nixcfg";
#   };
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.nixfleet-agent;
  shared = import ./shared.nix { inherit lib pkgs; };
  agentScript = shared.mkAgentScript { inherit cfg; };
in
{
  options.services.nixfleet-agent = shared.mkCommonOptions // {
    # Home Manager-specific options
    tokenFile = lib.mkOption {
      type = lib.types.str;
      description = ''
        Path to a file containing the API token for authentication.
        The file should contain just the token (no variable prefix).
        Use an absolute path (~ is not expanded in launchd).
      '';
      example = "/Users/myuser/.config/nixfleet/token";
    };
  };

  config = lib.mkIf cfg.enable {
    # macOS launchd agent
    launchd.agents.nixfleet-agent = lib.mkIf pkgs.stdenv.isDarwin {
      enable = true;
      config = {
        Label = "com.nixfleet.agent";
        ProgramArguments = [
          "/bin/bash"
          "-c"
          ''
            # Wait for network to be ready (DNS resolution working)
            # This prevents timeout errors when launchd starts the agent at boot
            MAX_WAIT=60
            WAITED=0
            while ! /usr/bin/host -W 2 fleet.barta.cm >/dev/null 2>&1; do
              if [ $WAITED -ge $MAX_WAIT ]; then
                echo "Warning: Network not ready after ''${MAX_WAIT}s, starting anyway..." >&2
                break
              fi
              sleep 2
              WAITED=$((WAITED + 2))
            done
            [ $WAITED -gt 0 ] && echo "Network ready after ''${WAITED}s" >&2

            # Add Nix paths so home-manager is available
            export PATH="$HOME/.nix-profile/bin:/nix/var/nix/profiles/default/bin:/run/current-system/sw/bin:$PATH"
            export NIXFLEET_URL="${cfg.url}"
            export NIXFLEET_NIXCFG="${cfg.configRepo}"
            export NIXFLEET_INTERVAL="${toString cfg.interval}"
            export NIXFLEET_LOCATION="${cfg.location}"
            export NIXFLEET_DEVICE_TYPE="${cfg.deviceType}"
            export NIXFLEET_THEME_COLOR="${cfg.themeColor}"
            export NIXFLEET_TOKEN_CACHE="$HOME/.local/state/nixfleet-agent/token"
            export NIXFLEET_TOKEN="$(cat '${cfg.tokenFile}')"
            exec ${agentScript}/bin/nixfleet-agent
          ''
        ];
        RunAtLoad = true;
        KeepAlive = true;
        StandardOutPath = "/tmp/nixfleet-agent.log";
        StandardErrorPath = "/tmp/nixfleet-agent.err";
      };
    };

    # Self-healing: Force launchd to reload the agent after home-manager switch
    # This ensures the agent uses the new nix store path after updates
    home.activation.reloadNixfleetAgent = lib.mkIf pkgs.stdenv.isDarwin (
      lib.hm.dag.entryAfter [ "setupLaunchAgents" ] ''
        LABEL="com.nixfleet.agent"
        PLIST="$HOME/Library/LaunchAgents/com.nixfleet.agent.plist"

        if [ -f "$PLIST" ]; then
          # Get the expected agent path from the new plist
          EXPECTED_PATH="${agentScript}/bin/nixfleet-agent"
          
          # Check if agent is running and get its current path
          # Use /usr/bin/awk since nix awk may not be in PATH during activation
          CURRENT_PID=$(/bin/launchctl list | /usr/bin/grep "$LABEL" | /usr/bin/awk '{print $1}')
          
          if [ -n "$CURRENT_PID" ] && [ "$CURRENT_PID" != "-" ]; then
            # Agent is running, check if it's using the correct path
            CURRENT_PATH=$(/bin/ps -p "$CURRENT_PID" -o args= 2>/dev/null | /usr/bin/grep -o '/nix/store/[^/]*/bin/nixfleet-agent' || echo "")
            
            if [ "$CURRENT_PATH" != "$EXPECTED_PATH" ]; then
              echo "NixFleet agent path changed, reloading..."
              /bin/launchctl bootout gui/$(/usr/bin/id -u)/$LABEL 2>/dev/null || true
              sleep 1
              /bin/launchctl bootstrap gui/$(/usr/bin/id -u) "$PLIST" 2>/dev/null || /bin/launchctl load "$PLIST" 2>/dev/null || true
              echo "NixFleet agent reloaded with new version"
            fi
          else
            # Agent not running, try to start it
            echo "NixFleet agent not running, starting..."
            /bin/launchctl bootstrap gui/$(/usr/bin/id -u) "$PLIST" 2>/dev/null || /bin/launchctl load "$PLIST" 2>/dev/null || true
          fi
        fi
      ''
    );

    # Linux systemd user service
    systemd.user.services.nixfleet-agent = lib.mkIf pkgs.stdenv.isLinux {
      Unit = {
        Description = "NixFleet Agent - Fleet management daemon";
        Documentation = "https://github.com/markus-barta/nixfleet";
        After = [ "network-online.target" ];
      };
      Service = {
        Type = "simple";
        ExecStart = "${agentScript}/bin/nixfleet-agent";
        Restart = "always";
        RestartSec = 30;
        Environment = [
          "NIXFLEET_URL=${cfg.url}"
          "NIXFLEET_NIXCFG=${cfg.configRepo}"
          "NIXFLEET_INTERVAL=${toString cfg.interval}"
          "NIXFLEET_LOCATION=${cfg.location}"
          "NIXFLEET_DEVICE_TYPE=${cfg.deviceType}"
          "NIXFLEET_THEME_COLOR=${cfg.themeColor}"
          "NIXFLEET_TOKEN_CACHE=%h/.local/state/nixfleet-agent/token"
        ];
        EnvironmentFile = cfg.tokenFile;
      };
      Install = {
        WantedBy = [ "default.target" ];
      };
    };
  };
}
