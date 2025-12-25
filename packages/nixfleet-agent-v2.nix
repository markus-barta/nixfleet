# NixFleet Agent v2 (Go)
#
# This package builds the Go-based agent that communicates
# with the dashboard via WebSocket.
#
# P7400: Version is read from VERSION file at repo root
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

  # P7400: Read version from single source of truth
  version = lib.strings.trim (builtins.readFile ../VERSION);

  src = ../v2;

  # Computed by running: nix-build -E 'with import <nixpkgs> {}; buildGoModule { src = ./v2; vendorHash = lib.fakeHash; }'
  # and extracting the expected hash from the error message
  vendorHash = "sha256-UIPfKQ2cDYcTKZrMfP5V6pJaDQDvG4D6KTgbeVT7JDE=";

  # Only build the agent, not the dashboard
  subPackages = [ "cmd/nixfleet-agent" ];

  ldflags = [
    "-s"
    "-w"
    # P7400: Inject version from VERSION file
    "-X github.com/markus-barta/nixfleet/v2/internal/agent.Version=${version}"
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
