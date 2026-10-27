{
  description = "Public fctl registry validation environment";
  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2511";
  inputs.nixpkgs-unstable.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1";
  inputs.go-tooling = {
    url = "github:NixOS/nixpkgs/8cec5ef1131078ea76689e84c362b0ed6a1fb6a1";
    flake = false;
  };
  # Published stable Go compiler and matching module builder only.
  inputs.nixpkgs-go-stable = {
    url = "https://api.flakehub.com/f/pinned/NixOS/nixpkgs/0.2605.1016949%2Brev-1f607aa79b6356ed45d3cd3c8079c196e79efa2c/01a17424-ae2d-7b20-83c4-cf14527294c3/source.tar.gz";
    flake = false;
  };
  outputs = { self, nixpkgs, nixpkgs-unstable, go-tooling, nixpkgs-go-stable }: {
    devShells = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ] (system:
      let pkgs = import nixpkgs { inherit system; };
          goPkgs = import nixpkgs-go-stable { inherit system; };
          go = goPkgs.go_1_27;
          buildGo127Module = goPkgs.buildGo127Module;
          # Retain golangci-lint 2.15.0; stable currently provides 2.12.2.
          golangci-lint = goPkgs.callPackage "${go-tooling}/pkgs/by-name/go/golangci-lint/package.nix" {
            inherit buildGo127Module;
          };
      in { default = pkgs.mkShell {
        packages = [ go pkgs.just golangci-lint ];
        GOTOOLCHAIN = "local";
        GOROOT = "${go}/share/go";
      }; });
  };
}
