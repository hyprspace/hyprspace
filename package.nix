{
  lib,
  buildGoModule,
  generateSchemasProgram,
}:
let
  inherit (lib.fileset) toSource unions fileFilter;
  pname = "hyprspace";
  version = "0.11.0";
in
buildGoModule {
  inherit pname version;
  src = toSource {
    root = ./.;
    fileset = unions [
      ./go.mod
      ./go.sum

      (fileFilter (file: file.hasExt "go") ./.)
    ];
  };

  env.CGO_ENABLED = "0";

  vendorHash = "sha256-97uIl3b3hs3BCLH7UZX8NU3kLloVQOCN9ygsdxsfass=";

  ldflags = [
    "-s"
    "-w"
    "-X github.com/hyprspace/hyprspace/cli.appVersion=${version}"
  ];

  postPatch = ''
    ( set -x; ${generateSchemasProgram} )
  '';

  meta = {
    description = "A Lightweight VPN Built on top of Libp2p for Truly Distributed Networks.";
    homepage = "https://github.com/hyprspace/hyprspace";
    license = lib.licenses.asl20;
    maintainers = with lib; [
      notashelf
      yusdacra
    ];
    platforms = lib.platforms.linux ++ lib.platforms.darwin;
    mainProgram = "hyprspace";
  };
}
