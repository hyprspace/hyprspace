{
  lib,
  runCommand,
  nixosOptionsDoc,
  ndg,
  hyprspace,
}:

let
  modules = lib.evalModules { modules = [ ../nixos/settings.nix ]; };

  optionsDoc = nixosOptionsDoc {
    options = builtins.removeAttrs modules.options [ "_module" ];
    transformOptions = option: builtins.removeAttrs option [ "declarations" ];
  };
in

runCommand "hyprspace-docs-${hyprspace.version}" { } ''
  ${ndg}/bin/ndg --config-file ${./ndg.toml} \
      --config input_dir=${./content} \
      --config output_dir=$out/share/www/hyprspace-docs \
      --config module_options=${optionsDoc.optionsJSON}/share/doc/nixos/options.json
''
