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
  shared = import ./shared.nix { inherit lib pkgs; };
  agentScript = shared.mkAgentScript { inherit cfg; };
in
{
  options.services.nixfleet-agent = shared.mkCommonOptions // {
    # NixOS-specific options
    tokenFile = lib.mkOption {
      type = lib.types.path;
      description = ''
        Path to a file containing the API token for authentication.
        The file should contain just the token, optionally with NIXFLEET_TOKEN= prefix.
        For NixOS, use agenix or sops-nix to manage this secret.
      '';
      example = lib.literalExpression "config.age.secrets.nixfleet-token.path";
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
    # Configure both sudo and sudo-rs for compatibility
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
    # sudo-rs uses a different config namespace
    security.sudo-rs.extraRules = [
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

      environment = shared.mkEnvironment { inherit cfg; } // {
        # NixOS-specific environment
        NIXFLEET_TOKEN_CACHE = "/var/lib/nixfleet-agent/token";
        HOME = "/home/${cfg.user}";
      };

      # CRITICAL: /run/wrappers/bin must come FIRST for setuid sudo wrapper
      # Without this, the agent finds the unwrapped sudo-rs binary which fails
      # with "sudo must be owned by uid 0"
      path = [ "/run/wrappers" "/run/current-system/sw" ];

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

        # Run as specified user with sudo
        User = cfg.user;
        Group = "users";
        # Note: We intentionally avoid strict sandboxing here because:
        # - NoNewPrivileges must be false for sudo/sudo-rs to work
        # - ProtectSystem=strict can interfere with sudo-rs setuid detection
        # - The agent needs to run nixos-rebuild which requires elevated privileges
        NoNewPrivileges = false;
      };
    };
  };
}
