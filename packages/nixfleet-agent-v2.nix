# NixFleet Agent v2 (Go)
#
# This package builds the Go-based agent that communicates
# with the dashboard via WebSocket.
{
  lib,
  buildGoModule,
  # P2810: Source commit for binary freshness verification
  # Passed from flake.nix when building from a clean git state
  sourceCommit ? "unknown",
  ...
}:
buildGoModule rec {
  pname = "nixfleet-agent";
  version = "2.2.0";

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
    # P2810: Embed source commit for binary freshness verification
    "-X github.com/markus-barta/nixfleet/v2/internal/agent.SourceCommit=${sourceCommit}"
  ];

  meta = with lib; {
    description = "NixFleet Agent - Fleet management daemon (v2 Go)";
    homepage = "https://github.com/markus-barta/nixfleet";
    license = licenses.gpl3Plus;
    maintainers = [ ];
    mainProgram = "nixfleet-agent";
  };
}
