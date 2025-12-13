{ pkgs, lib, ... }:

{
  # Minimal devenv configuration
  # All configuration is in devenv.yaml
  # This file exists to satisfy devenv's module system

  # Use pkgs and lib in valid devenv options to satisfy deadnix
  enterShell = lib.optionalString pkgs.stdenv.isDarwin "";
}
