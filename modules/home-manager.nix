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
#     repoUrl = "https://github.com/user/nixcfg.git";  # Agent manages isolated clone
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
  agentScript = shared.mkAgentScript { };
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

    sshKeyFile = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = ''
        Path to SSH private key for cloning private repositories.
        Only needed when using repoUrl with SSH URLs.
        Use an absolute path (~ is not expanded in launchd).
      '';
      example = "/Users/myuser/.ssh/nixfleet-deploy-key";
    };
  };

  config = lib.mkIf cfg.enable {
    # Validate required options
    assertions = [
      {
        assertion = cfg.url != "";
        message = "services.nixfleet-agent.url must be set";
      }
      {
        assertion = cfg.configRepo != "" || cfg.repoUrl != "";
        message = "services.nixfleet-agent: either configRepo or repoUrl must be set";
      }
      {
        assertion = !(cfg.configRepo != "" && cfg.repoUrl != "");
        message = "services.nixfleet-agent: cannot set both configRepo and repoUrl";
      }
    ];

    # Warn about deprecated configRepo
    warnings = lib.optional (cfg.configRepo != "") ''
      services.nixfleet-agent.configRepo is deprecated. Use repoUrl instead.
      The agent will manage its own isolated repository clone.
    '';

    # macOS launchd agent
    launchd.agents.nixfleet-agent = lib.mkIf pkgs.stdenv.isDarwin {
      enable = true;
      config = {
        Label = "com.nixfleet.agent";
        ProgramArguments = [
          "/bin/bash"
          "-c"
          ''
            # Wait for network to be ready before starting agent
            # Convert wss:// URL to https:// for health check (curl can't do WebSocket)
            HEALTH_URL="${cfg.url}"
            HEALTH_URL="''${HEALTH_URL/wss:\/\//https://}"
            HEALTH_URL="''${HEALTH_URL/ws:\/\//http://}"
            HEALTH_URL="''${HEALTH_URL%/ws}"  # Remove /ws suffix if present
            HEALTH_URL="''${HEALTH_URL}/health"

            MAX_WAIT=60
            WAITED=0
            while ! /usr/bin/curl -sf --connect-timeout 2 --max-time 5 "$HEALTH_URL" >/dev/null 2>&1; do
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
            export NIXFLEET_REPO_URL="${cfg.repoUrl}"
            export NIXFLEET_BRANCH="${cfg.branch}"
            export NIXFLEET_REPO_DIR="$HOME/.local/state/nixfleet-agent/repo"
            export NIXFLEET_INTERVAL="${toString cfg.interval}"
            export NIXFLEET_LOG_LEVEL="${cfg.logLevel}"
            export NIXFLEET_TOKEN="$(cat '${cfg.tokenFile}')"
            ${lib.optionalString (cfg.hostname != "") ''export NIXFLEET_HOSTNAME="${cfg.hostname}"''}
            ${lib.optionalString (
              cfg.nixpkgsVersion != ""
            ) ''export NIXFLEET_NIXPKGS_VERSION="${cfg.nixpkgsVersion}"''}
            ${lib.optionalString (cfg.themeColor != "") ''export NIXFLEET_THEME_COLOR="${cfg.themeColor}"''}
            ${lib.optionalString (cfg.sshKeyFile != null) ''export NIXFLEET_SSH_KEY="${cfg.sshKeyFile}"''}
            exec ${agentScript}/bin/nixfleet-agent
          ''
        ];
        RunAtLoad = true;
        KeepAlive = true;
        StandardOutPath = "/tmp/nixfleet-agent.log";
        StandardErrorPath = "/tmp/nixfleet-agent.err";
      };
    };

    # NOTE: No custom activation hook needed - home-manager's setupLaunchAgents
    # already handles agent lifecycle (bootout → bootstrap) correctly.
    # A previous custom hook was causing double-reloads that left the agent dead.

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
          "NIXFLEET_REPO_URL=${cfg.repoUrl}"
          "NIXFLEET_BRANCH=${cfg.branch}"
          "NIXFLEET_REPO_DIR=%h/.local/state/nixfleet-agent/repo"
          "NIXFLEET_INTERVAL=${toString cfg.interval}"
          "NIXFLEET_LOG_LEVEL=${cfg.logLevel}"
        ]
        ++ lib.optional (cfg.hostname != "") "NIXFLEET_HOSTNAME=${cfg.hostname}"
        ++ lib.optional (cfg.nixpkgsVersion != "") "NIXFLEET_NIXPKGS_VERSION=${cfg.nixpkgsVersion}"
        ++ lib.optional (cfg.themeColor != "") "NIXFLEET_THEME_COLOR=${cfg.themeColor}"
        ++ lib.optional (cfg.sshKeyFile != null) "NIXFLEET_SSH_KEY=${cfg.sshKeyFile}";
        EnvironmentFile = cfg.tokenFile;
      };
      Install = {
        WantedBy = [ "default.target" ];
      };
    };
  };
}
