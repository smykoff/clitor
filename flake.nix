{
  description = "clitor — switch git identity in the current repository";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      # Systems that have binaries published by CI.
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in {
      packages = forAll (pkgs:
        let clitor = pkgs.callPackage ./package.nix { }; in {
          inherit clitor;
          default = clitor;
        });

      overlays.default = final: prev: {
        clitor = final.callPackage ./package.nix { };
      };
    };
}
