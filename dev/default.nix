{
  imports = [
    ./generate-schemas.nix
    ./formatting.nix
    ./jobs/release.nix
  ];

  perSystem =
    { config, pkgs, ... }:
    {
      devShells.default = pkgs.mkShell {
        packages = [
          pkgs.go
          config.formatter
        ];

        env.GOTOOLCHAIN = "local";

        shellHook = ''
          export GOPATH="$PWD/.data/go";
          ${config.apps.dev-generate-schemas.program}
        '';
      };
    };
}
