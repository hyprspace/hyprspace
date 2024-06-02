{
  imports = [
    ./generate-schemas.nix
    ./formatting.nix
  ];

  perSystem =
    { config, pkgs, ... }:
    {
      devShells.default = pkgs.mkShell {
        packages = [ pkgs.go ];

        shellHook = ''
          export GOPATH="$PWD/.data/go";
          ${config.apps.dev-generate-schemas.program}
        '';
      };
    };
}
