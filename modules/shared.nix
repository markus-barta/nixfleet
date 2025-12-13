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
      description = "URL of the NixFleet dashboard.";
      example = "https://fleet.example.com";
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
      default = 30;
      description = "Poll interval in seconds (1-3600).";
      example = 30;
    };

    location = lib.mkOption {
      type = lib.types.enum [
        "cloud"
        "home"
        "work"
        "office"
        "mobile"
        "other"
      ];
      default = "home";
      description = "Physical location category for grouping hosts.";
      example = "cloud";
    };

    deviceType = lib.mkOption {
      type = lib.types.enum [
        "server"
        "desktop"
        "laptop"
        "gaming"
        "vm"
        "container"
        "other"
      ];
      default = "server";
      description = "Device type for categorization.";
      example = "desktop";
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

  # Build the agent script with version injection
  mkAgentScript =
    { cfg }:
    let
      agentScriptSrc = pkgs.replaceVars ../agent/nixfleet-agent.sh {
        agentVersion = cfg.version;
      };
    in
    pkgs.writeShellApplication {
      name = "nixfleet-agent";
      runtimeInputs = with pkgs; [
        curl
        jq
        git
        hostname
      ];
      text = builtins.readFile agentScriptSrc;
    };

  # Common environment variables
  mkEnvironment =
    { cfg }:
    {
      NIXFLEET_URL = cfg.url;
      # Legacy mode: use user-provided path; new mode: agent-managed path
      NIXFLEET_NIXCFG = cfg.configRepo;
      NIXFLEET_REPO_URL = cfg.repoUrl;
      NIXFLEET_BRANCH = cfg.branch;
      NIXFLEET_INTERVAL = toString cfg.interval;
      NIXFLEET_LOCATION = cfg.location;
      NIXFLEET_DEVICE_TYPE = cfg.deviceType;
      NIXFLEET_THEME_COLOR = cfg.themeColor;
    };
in
{
  inherit mkCommonOptions mkAgentScript mkEnvironment;
}
