{
  lib,
  writeShellScriptBin,
  go-jsonschema,
  settingsModule,
}:
let
  jsonschema = import ./lib/jsonschema.nix { inherit lib; };
  schema = jsonschema.parseModule settingsModule;
  schemaFile = builtins.toFile "hyprspace-config-schema.json" (
    builtins.toJSON (schema // { title = "Config"; })
  );
in
(writeShellScriptBin "hyprspace-generate-schemas" ''
  if [[ "$GOFILE" != "generate.go" ]]; then
    cd schema
  fi
  ${lib.getExe go-jsonschema} -p schema ${schemaFile} --tags json -t -o config_generated.go
'')
// {
  meta.mainProgram = "hyprspace-generate-schemas";
}
