{
  description = "Reproducible QuickNotes builds with Nix";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [
        "aarch64-linux"
        "x86_64-linux"
      ];

      forAllSystems = f:
        nixpkgs.lib.genAttrs systems
          (system: f (import nixpkgs { inherit system; }));

    in
    {
      packages = forAllSystems (pkgs:
        let
          quicknotes = pkgs.buildGoModule {
            pname = "quicknotes";
            version = "1.0.0";

            src = ./app;

            vendorHash = null;

            env.CGO_ENABLED = "0";

            ldflags = [
              "-s"
              "-w"
            ];

            doCheck = true;
          };

          docker = pkgs.dockerTools.buildImage {
            name = "quicknotes-nix";
            tag = "latest";

            copyToRoot = pkgs.buildEnv {
              name = "quicknotes-image-root";

              paths = [
                quicknotes
              ];

              pathsToLink = [
                "/bin"
              ];
            };

            # Create a writable working directory in the image layer.
            extraCommands = ''
              mkdir -p app
              chmod 1777 app
            '';

            config = {
              Entrypoint = [
                "${quicknotes}/bin/quicknotes"
              ];

              ExposedPorts = {
                "8080/tcp" = {};
              };

              User = "10001:10001";

              WorkingDir = "/app";
            };
          };

        in
        {
          inherit quicknotes docker;

          default = quicknotes;
        });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            golangci-lint
          ];
        };
      });
    };
}