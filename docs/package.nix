{ stdenvNoCC, emanote, hyprspace }:

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
    mkdir -p $out/share/www/$pname
    emanote -L . gen $out/share/www/$pname
  '';
}
