# $ nix-shell shell.nix

{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixpkgs-unstable.tar.gz") {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.delve  # go debugging
    pkgs.go_1_25
    pkgs.python314  # pre-commit hooks
  ];

  shellHook = ''
    export GOPATH=$(pwd)/.go
    export PATH=$GOPATH/bin:$PATH
  '';
}
