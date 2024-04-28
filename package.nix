{ lib, buildGoModule }:

let
  fs = lib.fileset;
in

buildGoModule rec {
  pname = "hyprspace";
  version = "0.8.5";

  src = fs.toSource {
    root = ./.;
    fileset = fs.unions [
      ./go.mod
      ./go.sum

      (fs.fileFilter (file: file.hasExt "go") ./.)
    ];
  };

  vendorHash = "sha256-VBCgFbJixBh+pKfYGJVapHqWBpUFfvjl1cwOER2Li6Y=";

  ldflags = [ "-s" "-w" "-X github.com/hyprspace/hyprspace/cli.appVersion=${version}" ];

  meta = with lib; {
    description = "A Lightweight VPN Built on top of Libp2p for Truly Distributed Networks.";
    homepage = "https://github.com/hyprspace/hyprspace";
    license = licenses.asl20;
    maintainers = with maintainers; [ yusdacra ];
    platforms = platforms.linux ++ platforms.darwin;
  };
}

