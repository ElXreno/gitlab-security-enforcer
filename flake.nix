{
  description = "gitlab-security-enforcer - GitLab webhook security enforcer";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [ "x86_64-linux" ];
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.buildGoModule {
            pname = "gitlab-security-enforcer";
            version = "0.0.0"; # x-release-please-version

            src = self;
            subPackages = [ "cmd/server" ];

            vendorHash = "sha256-gu2NwS6I7oUbyMNipjeC/fOsvDSyo2GbC6U8gSB1dF0=";

            postInstall = ''
              if [ -f "$out/bin/server" ]; then
                mv "$out/bin/server" "$out/bin/gitlab-security-enforcer"
              fi
            '';
          };
        }
      );

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/gitlab-security-enforcer";
        };
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.golangci-lint
              pkgs.goreleaser
            ];
          };
        }
      );
    };
}
