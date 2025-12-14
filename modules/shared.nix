# Shared NixFleet Agent definitions
#
# This module contains shared code between nixos.nix and home-manager.nix
# to avoid duplication (DRY principle).
{ lib, pkgs }:
let
  # Common option definitions
  mkCommonOptions = {
    enable = lib.mkEnableOption "NixFleet agent for fleet management";

    url = lib.mkOption {
      type = lib.types.str;
      description = ''
        WebSocket URL of the NixFleet dashboard.
        For v2 agents, use wss:// protocol (e.g., wss://fleet.example.com/ws).
      '';
      example = "wss://fleet.example.com/ws";
    };

    configRepo = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = ''
        DEPRECATED: Use repoUrl instead.
        Absolute path to a user-managed Nix configuration repository.
        If set, the agent uses this existing directory (legacy mode).
      '';
      example = "/home/user/Code/nixcfg";
    };

    repoUrl = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = ''
        Git remote URL for the Nix configuration repository.
        When set, the agent clones and manages its own isolated copy.
        This is the recommended approach over configRepo.
      '';
      example = "git@github.com:user/nixcfg.git";
    };

    branch = lib.mkOption {
      type = lib.types.str;
      default = "main";
      description = "Git branch to track when using repoUrl.";
      example = "main";
    };

    interval = lib.mkOption {
      type = lib.types.ints.between 1 3600;
      default = 5;
      description = "Heartbeat interval in seconds (1-3600).";
      example = 5;
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [
        "debug"
        "info"
        "warn"
        "error"
      ];
      default = "info";
      description = "Agent log level.";
      example = "debug";
    };

    hostname = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = ''
        Override the auto-detected hostname. Useful on macOS where
        the hostname can change based on network interface (e.g., imac0.lan vs imac0).
        If empty, the agent uses the system hostname with domain suffix stripped.
      '';
      example = "imac0";
    };

    version = lib.mkOption {
      type = lib.types.str;
      default = "0.0.0";
      description = ''
        Agent version string. This is automatically set by the flake
        to the current nixfleet version. You can override it for testing.
      '';
    };

    nixpkgsVersion = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = ''
        The nixpkgs version/commit to report. This should be set to the
        nixpkgs input revision (e.g., inputs.nixpkgs.shortRev) in flake.nix.
        If empty, the agent attempts to detect it automatically.
      '';
      example = "24.05pre123456";
    };

    themeColor = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = ''
        Hex color code for this host in the NixFleet dashboard.
        If empty, uses default: #7aa2f7 (blue) for NixOS, #bb9af7 (purple) for macOS.
        Should match the host's terminal/editor theme color for visual consistency.
      '';
      example = "#f7768e";
    };
  };

  # Build the Go agent package
  # Note: The package is now built via buildGoModule, not a shell script
  mkAgentScript = _: pkgs.callPackage ../packages/nixfleet-agent-v2.nix { };

  # Common environment variables for v2 Go agent
  mkEnvironment =
    { cfg }:
    {
      NIXFLEET_URL = cfg.url;
      NIXFLEET_NIXCFG = cfg.configRepo; # Legacy, deprecated
      NIXFLEET_REPO_URL = cfg.repoUrl;
      NIXFLEET_BRANCH = cfg.branch;
      NIXFLEET_INTERVAL = toString cfg.interval;
      NIXFLEET_LOG_LEVEL = cfg.logLevel;
    }
    // lib.optionalAttrs (cfg.hostname != "") { NIXFLEET_HOSTNAME = cfg.hostname; }
    // lib.optionalAttrs (cfg.nixpkgsVersion != "") { NIXFLEET_NIXPKGS_VERSION = cfg.nixpkgsVersion; }
    // lib.optionalAttrs (cfg.themeColor != "") { NIXFLEET_THEME_COLOR = cfg.themeColor; };
in
{
  inherit mkCommonOptions mkAgentScript mkEnvironment;
}
