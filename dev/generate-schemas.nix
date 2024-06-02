{ lib, ... }:

{
  perSystem = { pkgs, ... }: let
    go-jsonschema = pkgs.callPackage ./pkgs/go-jsonschema {};

    jsonschema = import ./lib/jsonschema.nix { inherit lib; };

    schema = jsonschema.parseModule ../nixos/settings.nix;

    schemaFile = builtins.toFile "hyprspace-config-schema.json" (builtins.toJSON (schema // {
      title = "Config";
    }));

  in {
    apps.dev-generate-schemas.program = pkgs.writeShellScriptBin "hyprspace-generate-schemas" ''
      if [[ "$GOFILE" != "generate.go" ]]; then
        cd schema
      fi
      ${go-jsonschema}/bin/go-jsonschema -p schema ${schemaFile} --tags json -t -o config_generated.go
    '';
  };
}
