# NixFleet Agent v2 (Go)
#
# This package builds the Go-based agent that communicates
# with the dashboard via WebSocket.
{
  lib,
  buildGoModule,
  ...
}:
buildGoModule rec {
  pname = "nixfleet-agent";
  version = "2.0.0";

  src = ../v2;

  # Computed by running: nix-build -E 'with import <nixpkgs> {}; buildGoModule { src = ./v2; vendorHash = lib.fakeHash; }'
  # and extracting the expected hash from the error message
  vendorHash = "sha256-UIPfKQ2cDYcTKZrMfP5V6pJaDQDvG4D6KTgbeVT7JDE=";

  # Only build the agent, not the dashboard
  subPackages = [ "cmd/nixfleet-agent" ];

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];

  meta = with lib; {
    description = "NixFleet Agent - Fleet management daemon (v2 Go)";
    homepage = "https://github.com/markus-barta/nixfleet";
    license = licenses.gpl3Plus;
    maintainers = [ ];
    mainProgram = "nixfleet-agent";
  };
}
