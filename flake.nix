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

      # Helper to create the agent package
      mkAgentPackage =
        pkgs:
        pkgs.writeShellApplication {
          name = "nixfleet-agent";
          runtimeInputs = with pkgs; [
            curl
            jq
            git
            hostname
          ];
          text = builtins.readFile ./agent/nixfleet-agent.sh;
        };
    in
    {
      # ════════════════════════════════════════════════════════════════════════
      # NixOS Module
      # ════════════════════════════════════════════════════════════════════════
      nixosModules.nixfleet-agent = import ./modules/nixos.nix;
      nixosModules.default = self.nixosModules.nixfleet-agent;

      # ════════════════════════════════════════════════════════════════════════
      # Home Manager Module
      # ════════════════════════════════════════════════════════════════════════
      homeManagerModules.nixfleet-agent = import ./modules/home-manager.nix;
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
