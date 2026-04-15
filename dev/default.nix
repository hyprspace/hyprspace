{
  imports = [
    ./formatting.nix
    ./jobs/release.nix
  ];

  perSystem =
    { config, pkgs, ... }:
    let
      generateSchemasProgram = pkgs.callPackage ./generate-schemas.nix {
        go-jsonschema = pkgs.callPackage ./pkgs/go-jsonschema { };
        settingsModule = ../nixos/settings.nix;
      };
    in
    {
      apps.dev-generate-schemas.program = generateSchemasProgram;

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
