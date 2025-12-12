{
  description = "NixFleet - Fleet management dashboard for NixOS and macOS hosts";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    let
      # Systems to provide packages for
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      # Version from git - use short rev or "dev" for dirty trees
      version =
        if self ? shortRev then
          "0.3.0-${self.shortRev}"
        else if self ? rev then
          "0.3.0-${builtins.substring 0 7 self.rev}"
        else
          "0.3.0-dev";

      # Helper to create the agent package
      mkAgentPackage =
        pkgs:
        let
          agentScriptSrc = pkgs.substituteAll {
            src = ./agent/nixfleet-agent.sh;
            agentVersion = version;
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
    in
    {
      # ════════════════════════════════════════════════════════════════════════
      # NixOS Module
      # ════════════════════════════════════════════════════════════════════════
      nixosModules.nixfleet-agent =
        { lib, ... }:
        {
          imports = [ ./modules/nixos.nix ];
          # Set the version default from the flake
          config.services.nixfleet-agent.version = lib.mkDefault version;
        };
      nixosModules.default = self.nixosModules.nixfleet-agent;

      # ════════════════════════════════════════════════════════════════════════
      # Home Manager Module
      # ════════════════════════════════════════════════════════════════════════
      homeManagerModules.nixfleet-agent =
        { lib, ... }:
        {
          imports = [ ./modules/home-manager.nix ];
          # Set the version default from the flake
          config.services.nixfleet-agent.version = lib.mkDefault version;
        };
      homeManagerModules.default = self.homeManagerModules.nixfleet-agent;

      # ════════════════════════════════════════════════════════════════════════
      # Overlay
      # ════════════════════════════════════════════════════════════════════════
      overlays.default = final: _prev: { nixfleet-agent = mkAgentPackage final; };
    }
    // flake-utils.lib.eachSystem supportedSystems (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        # ══════════════════════════════════════════════════════════════════════
        # Packages
        # ══════════════════════════════════════════════════════════════════════
        packages = {
          nixfleet-agent = mkAgentPackage pkgs;
          default = self.packages.${system}.nixfleet-agent;
        };

        # ══════════════════════════════════════════════════════════════════════
        # Development Shell
        # ══════════════════════════════════════════════════════════════════════
        devShells.default = pkgs.mkShell {
          name = "nixfleet-dev";
          buildInputs = with pkgs; [
            # Python for backend
            python312
            python312Packages.fastapi
            python312Packages.uvicorn
            python312Packages.jinja2
            python312Packages.bcrypt
            python312Packages.pyotp
            python312Packages.pydantic
            python312Packages.slowapi

            # Agent dependencies
            curl
            jq
            git

            # Development tools
            nixfmt-rfc-style
            shellcheck
          ];

          shellHook = ''
            echo "🚀 NixFleet development shell"
            echo ""
            echo "Commands:"
            echo "  cd app && uvicorn main:app --reload  # Run dashboard"
            echo "  ./agent/nixfleet-agent.sh            # Test agent"
            echo ""
          '';
        };

        # ══════════════════════════════════════════════════════════════════════
        # Checks (for CI)
        # ══════════════════════════════════════════════════════════════════════
        checks = {
          # Verify agent script syntax
          agent-shellcheck = pkgs.runCommand "agent-shellcheck" { buildInputs = [ pkgs.shellcheck ]; } ''
            shellcheck ${./agent/nixfleet-agent.sh}
            touch $out
          '';

          # Verify flake formatting
          nixfmt = pkgs.runCommand "nixfmt-check" { buildInputs = [ pkgs.nixfmt-rfc-style ]; } ''
            nixfmt --check ${./flake.nix} ${./modules/nixos.nix} ${./modules/home-manager.nix}
            touch $out
          '';
        };
      }
    );
}
