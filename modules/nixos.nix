# NixFleet Agent Module (NixOS)
#
# Provides a systemd service that polls the NixFleet dashboard
# for commands and reports host status.
#
# Usage (via flake):
#   inputs.nixfleet.url = "github:markus-barta/nixfleet";
#
#   # In nixosSystem modules list:
#   inputs.nixfleet.nixosModules.nixfleet-agent
#
#   # In configuration.nix:
#   services.nixfleet-agent = {
#     enable = true;
#     url = "https://fleet.example.com";
#     tokenFile = config.age.secrets.nixfleet-token.path;
#     configRepo = "/home/admin/Code/nixcfg";
#   };
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.nixfleet-agent;

  # Substitute version placeholder in agent script
  agentScriptSrc = pkgs.substituteAll {
    src = ../agent/nixfleet-agent.sh;
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
      type = lib.types.path;
      description = ''
        Path to a file containing the API token for authentication.
        The file should contain just the token, optionally with NIXFLEET_TOKEN= prefix.
        For NixOS, use agenix or sops-nix to manage this secret.
      '';
      example = lib.literalExpression "config.age.secrets.nixfleet-token.path";
    };

    configRepo = lib.mkOption {
      type = lib.types.str;
      description = "Absolute path to the Nix configuration repository.";
      example = "/home/admin/Code/nixcfg";
    };

    interval = lib.mkOption {
      type = lib.types.ints.between 1 3600;
      default = 30;
      description = "Poll interval in seconds (1-3600).";
      example = 30;
    };

    user = lib.mkOption {
      type = lib.types.str;
      description = ''
        User to run the agent as. This user needs:
        - Read access to the config repository
        - Sudo access to run nixos-rebuild (configured automatically)
      '';
      example = "admin";
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
      example = "cloud";
    };

    deviceType = lib.mkOption {
      type = lib.types.enum [
        "server"
        "desktop"
        "laptop"
        "gaming"
        "other"
      ];
      default = "server";
      description = "Device type for dashboard display.";
      example = "server";
    };

    themeColor = lib.mkOption {
      type = lib.types.strMatching "^#[0-9a-fA-F]{6}$";
      default = "#769ff0";
      description = "Theme color hex code for dashboard display.";
      example = "#ff6b6b";
    };

    runAsRoot = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Run the agent as root instead of the configured user.
        This bypasses sudo entirely and is more reliable on systems
        with sudo-rs or other sudo configurations.
      '';
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
    # Validate required options
    assertions = [
      {
        assertion = cfg.url != "";
        message = "services.nixfleet-agent.url must be set";
      }
      {
        assertion = cfg.configRepo != "";
        message = "services.nixfleet-agent.configRepo must be set";
      }
      {
        assertion = cfg.user != "";
        message = "services.nixfleet-agent.user must be set";
      }
    ];

    # Allow agent user to run nixos-rebuild without password
    security.sudo.extraRules = [
      {
        users = [ cfg.user ];
        commands = [
          {
            command = "/run/current-system/sw/bin/nixos-rebuild";
            options = [ "NOPASSWD" ];
          }
        ];
      }
    ];

    systemd.services.nixfleet-agent = {
      description = "NixFleet Agent - Fleet management daemon";
      documentation = [ "https://github.com/markus-barta/nixfleet" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      environment = {
        NIXFLEET_URL = cfg.url;
        NIXFLEET_NIXCFG = cfg.configRepo;
        NIXFLEET_INTERVAL = toString cfg.interval;
        NIXFLEET_LOCATION = cfg.location;
        NIXFLEET_DEVICE_TYPE = cfg.deviceType;
        NIXFLEET_THEME_COLOR = cfg.themeColor;
        # Persist per-host agent token across restarts (used for migration away from shared token)
        NIXFLEET_TOKEN_CACHE = "/var/lib/nixfleet-agent/token";
        HOME = "/home/${cfg.user}";
      };

      path = [ "/run/current-system/sw" ];

      serviceConfig = {
        Type = "simple";
        ExecStart = "${agentScript}/bin/nixfleet-agent";
        Restart = "always";
        RestartSec = 30;

        # Read token from file
        EnvironmentFile = cfg.tokenFile;

        # Logging
        StandardOutput = "journal";
        StandardError = "journal";

        # Token cache state (for per-host token migration)
        StateDirectory = "nixfleet-agent";
        StateDirectoryMode = "0700";
      }
      // (
        if cfg.runAsRoot then
          {
            # Run as root (no sudo needed)
            User = "root";
            Group = "root";
            ProtectSystem = "strict";
            ProtectHome = "read-only";
            ReadWritePaths = [
              cfg.configRepo
              "/root"
            ];
            PrivateTmp = true;
          }
        else
          {
            # Run as specified user with sudo
            User = cfg.user;
            Group = "users";
            NoNewPrivileges = false; # Needs sudo for nixos-rebuild
            ProtectSystem = "strict";
            ProtectHome = "read-only";
            ReadWritePaths = [ cfg.configRepo ];
            PrivateTmp = true;
          }
      );
    };
  };
}
