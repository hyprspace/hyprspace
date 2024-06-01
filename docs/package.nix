{ stdenvNoCC, lib, nixosOptionsDoc, emanote, hyprspace }:

let
  modules = lib.evalModules {
    modules = [ ../nixos/settings.nix ];
  };

  optionsDoc = nixosOptionsDoc {
    options = builtins.removeAttrs modules.options [ "_module" ];
    transformOptions = option: builtins.removeAttrs option [ "declarations" ];
  };
in

stdenvNoCC.mkDerivation {
  pname = "hyprspace-docs";
  inherit (hyprspace) version;

  src = ./content;

  nativeBuildInputs = [
    emanote
  ];

  buildCommand = ''
    unpackPhase
    cd "$sourceRoot"

    cp ${../hyprspace.png} ./favicon.png
    cp ${./emanote-config.json} ./index.yaml
    cat ${optionsDoc.optionsCommonMark} >> ./configuration.md
    mkdir -p $out/share/www/$pname
    emanote -L . gen $out/share/www/$pname
  '';
}
