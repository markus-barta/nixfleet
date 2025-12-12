{
  description = "NixFleet - Fleet management dashboard for NixOS and macOS hosts";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    {
      # NixOS module for the agent (systemd service)
      nixosModules.nixfleet-agent = import ./modules/nixos.nix;
      nixosModules.default = self.nixosModules.nixfleet-agent;

      # Home Manager module for the agent (launchd/user systemd)
      homeManagerModules.nixfleet-agent = import ./modules/home-manager.nix;
      homeManagerModules.default = self.homeManagerModules.nixfleet-agent;
    };
}
