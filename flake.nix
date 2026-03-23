{
  description = "Go Development Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        nativeBuildInputs = [ pkgs.pkg-config ];
        buildInputs = with pkgs; [
          go
          go-task
          lefthook
          air
          golangci-lint
          govulncheck
        ];

        shellHook = ''
          export GOPATH=$HOME/go
          export PATH=$GOPATH/bin:$PATH

          export CGO_ENABLED=1

          echo "🚀 PodPloy DevShell Ready"
          task setup
          echo "📦 Go $(go version) "
        '';
      };
    };
}
