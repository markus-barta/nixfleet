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
          "2.0.0-${self.shortRev}"
        else if self ? rev then
          "2.0.0-${builtins.substring 0 7 self.rev}"
        else
          "2.0.0-dev";

      # Helper to create the Go agent package
      mkAgentPackage = pkgs: pkgs.callPackage ./packages/nixfleet-agent-v2.nix { };
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
            # Go for v2 backend and agent
            go
            gopls
            golangci-lint
            templ # Template code generator for Go

            # Agent dependencies
            curl
            jq
            git

            # Development tools
            nixfmt-rfc-style
            shellcheck
          ];

          shellHook = ''
            echo "🚀 NixFleet v2 development shell"
            echo ""
            echo "Commands:"
            echo "  cd v2 && templ generate                     # Generate template code"
            echo "  cd v2 && go build ./cmd/nixfleet-agent      # Build agent"
            echo "  cd v2 && go build ./cmd/nixfleet-dashboard  # Build dashboard"
            echo "  cd v2 && go test ./tests/integration/...    # Run tests"
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
