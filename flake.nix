{
  description = "Public fctl registry validation environment";
  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.2511";
  inputs.nixpkgs-unstable.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1";
  outputs = { self, nixpkgs, nixpkgs-unstable }: {
    devShells = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ] (system:
      let pkgs = import nixpkgs { inherit system; };
          unstable = import nixpkgs-unstable { inherit system; };
      in { default = pkgs.mkShell { packages = [ pkgs.go_1_26 pkgs.just unstable.golangci-lint ]; }; });
  };
}
