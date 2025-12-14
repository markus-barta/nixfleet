# Go Agent: Nix Packaging

**Created**: 2025-12-14
**Priority**: P4100 (Critical)
**Status**: Backlog
**Depends on**: P4000 (Go Agent Core)

---

## Overview

Package the Go agent for NixOS and macOS via Nix flake.

---

## Deliverables

### 1. Nix Package

```nix
# packages/nixfleet-agent/default.nix
{ buildGoModule, ... }:

buildGoModule {
  pname = "nixfleet-agent";
  version = "2.0.0";

  src = ../../agent;

  vendorHash = "sha256-...";

  ldflags = [
    "-s" "-w"
    "-X main.Version=${version}"
  ];

  meta = {
    description = "NixFleet agent for fleet management";
    license = lib.licenses.agpl3Plus;
    platforms = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
  };
}
```

### 2. NixOS Module

```nix
# modules/nixos.nix
{ config, lib, pkgs, ... }:

{
  options.services.nixfleet-agent = {
    enable = lib.mkEnableOption "NixFleet agent";

    url = lib.mkOption {
      type = lib.types.str;
      description = "Dashboard WebSocket URL";
    };

    tokenFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to agent token file";
    };

    repoUrl = lib.mkOption {
      type = lib.types.str;
      description = "Git repository URL";
    };

    # ... more options
  };

  config = lib.mkIf cfg.enable {
    systemd.services.nixfleet-agent = {
      description = "NixFleet Agent";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];

      serviceConfig = {
        Type = "simple";
        ExecStart = "${pkgs.nixfleet-agent}/bin/nixfleet-agent";
        Restart = "always";
        RestartSec = 10;
        # Security hardening
        DynamicUser = true;
        StateDirectory = "nixfleet-agent";
        # ...
      };

      environment = {
        NIXFLEET_URL = cfg.url;
        # ...
      };
    };
  };
}
```

### 3. Home Manager Module

```nix
# modules/home-manager.nix
{ config, lib, pkgs, ... }:

{
  options.services.nixfleet-agent = {
    # Similar to NixOS but for launchd
  };

  config = lib.mkIf cfg.enable {
    launchd.agents.nixfleet-agent = {
      enable = true;
      config = {
        Label = "com.nixfleet.agent";
        ProgramArguments = [ "${pkgs.nixfleet-agent}/bin/nixfleet-agent" ];
        KeepAlive = true;
        RunAtLoad = true;
        EnvironmentVariables = {
          NIXFLEET_URL = cfg.url;
          # ...
        };
      };
    };
  };
}
```

### 4. macOS Watchdog

Separate launchd job that ensures agent stays running:

```nix
launchd.agents.nixfleet-watchdog = {
  enable = true;
  config = {
    Label = "com.nixfleet.watchdog";
    ProgramArguments = [
      "/bin/sh" "-c"
      "launchctl list | grep -q com.nixfleet.agent || launchctl kickstart gui/$(id -u)/com.nixfleet.agent"
    ];
    StartInterval = 30;
    RunAtLoad = true;
  };
};
```

---

## Flake Structure

```nix
# flake.nix
{
  outputs = { self, nixpkgs, ... }: {
    packages = forAllSystems (system: {
      nixfleet-agent = ...;
      nixfleet-dashboard = ...;
    });

    nixosModules = {
      nixfleet-agent = import ./modules/nixos.nix;
    };

    homeManagerModules = {
      nixfleet-agent = import ./modules/home-manager.nix;
    };
  };
}
```

---

## Acceptance Criteria

- [ ] `nix build .#nixfleet-agent` works on x86_64-linux
- [ ] `nix build .#nixfleet-agent` works on aarch64-darwin
- [ ] NixOS module creates working systemd service
- [ ] Home Manager module creates working launchd agent
- [ ] Watchdog module for macOS reliability
- [ ] Version injected at build time
- [ ] Secrets handled via tokenFile (no hardcoding)

---

## Related

- Depends on: P4000 (Go Agent Core)
- Enables: Deployment to all hosts
