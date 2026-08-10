{
  description = "praktik — CLI for the praktik skill library";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      overlay = final: prev: {
        praktik = final.callPackage ./nix/package.nix { };
      };
    in
    {
      overlays.default = overlay;
    }
    // flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        };
      in
      {
        packages = {
          default = pkgs.praktik;
          praktik = pkgs.praktik;
        };

        apps.default = {
          type = "app";
          program = "${pkgs.praktik}/bin/praktik";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.delve
            pkgs.git
            pkgs.fzf
          ];
        };

        checks.default = pkgs.praktik;
      });
}
