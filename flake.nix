{
  description = "Public fctl registry validation environment";
  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2511";
  inputs.nixpkgs-unstable.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1";
  inputs.go-tooling = {
    url = "github:NixOS/nixpkgs/8cec5ef1131078ea76689e84c362b0ed6a1fb6a1";
    flake = false;
  };
  outputs = { self, nixpkgs, nixpkgs-unstable, go-tooling }: {
    devShells = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ] (system:
      let pkgs = import nixpkgs { inherit system; };
          go = go127;
          go127 = pkgs.callPackage "${go-tooling}/pkgs/development/compilers/go/1.27.nix" {
            buildGo127Module = buildGo127Module;
          };
          buildGo127Module = pkgs.buildGoModule.override { go = go127; };
          golangci-lint = pkgs.callPackage "${go-tooling}/pkgs/by-name/go/golangci-lint/package.nix" {
            inherit buildGo127Module;
          };
      in { default = pkgs.mkShell {
        packages = [ go pkgs.just golangci-lint ];
        GOTOOLCHAIN = "local";
        GOROOT = "${go}/share/go";
      }; });
  };
}
