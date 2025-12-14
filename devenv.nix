{ pkgs, lib, ... }:

{
  # Development tools
  packages = with pkgs; [
    # Go development
    go
    gopls
    golangci-lint
    gotools # goimports, etc.

    # Testing
    gotestsum
  ];

  # Environment variables
  env = {
    GOPATH = "$HOME/go";
    GOCACHE = "$HOME/.cache/go-build";
    GOMODCACHE = "$HOME/go/pkg/mod";
  };

  # Use pkgs and lib in valid devenv options to satisfy deadnix
  enterShell = lib.optionalString pkgs.stdenv.isDarwin "";
}
