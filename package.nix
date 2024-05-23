{ lib, buildGoModule }:

let
  fs = lib.fileset;
in

buildGoModule rec {
  pname = "hyprspace";
  version = "0.9.0";

  src = fs.toSource {
    root = ./.;
    fileset = fs.unions [
      ./go.mod
      ./go.sum

      (fs.fileFilter (file: file.hasExt "go") ./.)
    ];
  };

  CGO_ENABLED = "0";

  vendorHash = "sha256-LJpgGeD47Bs+Cq9Z7WWFa49F8/n3exOyxRcd6EkkL2g=";

  ldflags = [ "-s" "-w" "-X github.com/hyprspace/hyprspace/cli.appVersion=${version}" ];

  meta = with lib; {
    description = "A Lightweight VPN Built on top of Libp2p for Truly Distributed Networks.";
    homepage = "https://github.com/hyprspace/hyprspace";
    license = licenses.asl20;
    maintainers = with maintainers; [ yusdacra ];
    platforms = platforms.linux ++ platforms.darwin;
  };
}

