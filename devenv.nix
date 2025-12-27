{ pkgs, lib, ... }:

{
  # https://devenv.sh/languages/
  languages.go = {
    enable = true;
    # Go 1.23 is current stable (Docker Hub compatible)
    # devenv-nixpkgs uses rolling channel with latest
  };

  # Use standard GOMODCACHE outside workspace to avoid gopls overlay conflicts
  # (templ LSP generates overlays for *_templ.go files, which fails for module cache)
  env.GOMODCACHE = "/Users/markus/go/pkg/mod";

  # Development tools
  packages = with pkgs; [
    # Go tools
    gopls # Language server
    golangci-lint # Linter
    gotools # goimports, godoc, etc.
    templ # Template generation
    delve # Debugger
    gotestsum # Better test output

    # General dev tools
    jq
    curl
    websocat # WebSocket testing
  ];

  # Direnv integration
  dotenv.enable = true;

  # Scripts for common tasks
  scripts = {
    test-agent.exec = ''
      cd src && go test ./tests/integration/... -v -count=1 "$@"
    '';
    build-agent.exec = ''
      cd src && go build -o ../bin/nixfleet-agent ./cmd/nixfleet-agent
    '';
    run-agent.exec = ''
      cd src && go run ./cmd/nixfleet-agent "$@"
    '';
    lint.exec = ''
      cd src && golangci-lint run ./...
    '';
  };

  enterShell = ''
    echo "🚀 NixFleet development environment"
    echo ""
    echo "Commands:"
    echo "  test-agent   - Run agent integration tests"
    echo "  build-agent  - Build agent binary"
    echo "  run-agent    - Run agent (needs env vars)"
    echo "  lint         - Run golangci-lint"
    echo ""
    go version
  '';

  # Use lib to satisfy deadnix
  _module.args.lib' = lib;
}
