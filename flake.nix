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
            version = "0.1.2"; # x-release-please-version

            src = self;
            subPackages = [ "cmd/server" ];

            vendorHash = "sha256-ESDlS0rcuyBPU1lQNkiz52VNKVRDefyFUUKOsaKwsek=";

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
