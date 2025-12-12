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

  # Replace version placeholder in agent script
  agentScriptSrc = pkgs.replaceVars ../agent/nixfleet-agent.sh {
    agentVersion = cfg.version;
  };

  agentScript = pkgs.writeShellApplication {
    name = "nixfleet-agent";
    runtimeInputs = with pkgs; [
      curl
      jq
      git
      hostname
    ];
    text = builtins.readFile agentScriptSrc;
  };
in
{
  options.services.nixfleet-agent = {
    enable = lib.mkEnableOption "NixFleet agent for fleet management";

    url = lib.mkOption {
      type = lib.types.str;
      description = "URL of the NixFleet dashboard.";
      example = "https://fleet.example.com";
    };

    tokenFile = lib.mkOption {
      type = lib.types.str;
      description = ''
        Path to a file containing the API token for authentication.
        The file should contain just the token (no variable prefix).
        Use an absolute path (~ is not expanded in launchd).
      '';
      example = "/Users/myuser/.config/nixfleet/token";
    };

    configRepo = lib.mkOption {
      type = lib.types.str;
      description = "Absolute path to the Nix configuration repository.";
      example = "/Users/myuser/Code/nixcfg";
    };

    interval = lib.mkOption {
      type = lib.types.ints.between 1 3600;
      default = 30;
      description = "Poll interval in seconds (1-3600).";
      example = 30;
    };

    location = lib.mkOption {
      type = lib.types.enum [
        "cloud"
        "home"
        "work"
        "other"
      ];
      default = "other";
      description = "Physical location category for dashboard grouping.";
      example = "home";
    };

    deviceType = lib.mkOption {
      type = lib.types.enum [
        "server"
        "desktop"
        "laptop"
        "gaming"
        "other"
      ];
      default = "desktop";
      description = "Device type for dashboard display.";
      example = "laptop";
    };

    themeColor = lib.mkOption {
      type = lib.types.strMatching "^#[0-9a-fA-F]{6}$";
      default = "#769ff0";
      description = "Theme color hex code for dashboard display.";
      example = "#ff6b6b";
    };

    version = lib.mkOption {
      type = lib.types.str;
      default = "0.0.0";
      description = ''
        Agent version string. This is automatically set by the flake
        to the current nixfleet version. You can override it for testing.
      '';
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
          CURRENT_PID=$(/bin/launchctl list | grep "$LABEL" | awk '{print $1}')
          
          if [ -n "$CURRENT_PID" ] && [ "$CURRENT_PID" != "-" ]; then
            # Agent is running, check if it's using the correct path
            CURRENT_PATH=$(/bin/ps -p "$CURRENT_PID" -o args= 2>/dev/null | grep -o '/nix/store/[^/]*/bin/nixfleet-agent' || echo "")
            
            if [ "$CURRENT_PATH" != "$EXPECTED_PATH" ]; then
              echo "NixFleet agent path changed, reloading..."
              /bin/launchctl bootout gui/$(id -u)/$LABEL 2>/dev/null || true
              sleep 1
              /bin/launchctl bootstrap gui/$(id -u) "$PLIST" 2>/dev/null || /bin/launchctl load "$PLIST" 2>/dev/null || true
              echo "NixFleet agent reloaded with new version"
            fi
          else
            # Agent not running, try to start it
            echo "NixFleet agent not running, starting..."
            /bin/launchctl bootstrap gui/$(id -u) "$PLIST" 2>/dev/null || /bin/launchctl load "$PLIST" 2>/dev/null || true
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
