{ lib, ... }:

{
  perSystem =
    { config, pkgs, ... }:
    {
      formatter = pkgs.nixfmt-rfc-style;
      checks =
        let
          inherit (lib.fileset) toSource fileFilter;
          filesWithExtension =
            ext:
            toSource {
              root = ../.;
              fileset = fileFilter (file: file.hasExt ext) ../.;
            };
        in
        {
          format-nix = pkgs.runCommandNoCC "format-nix" { } ''
            ${lib.getExe config.formatter} -c ${filesWithExtension "nix"}
            touch $out
          '';
          format-go = pkgs.runCommandNoCC "format-go" { } ''
            test "$(${lib.getExe' pkgs.go "gofmt"} -l ${filesWithExtension "go"} | tee /dev/stderr | wc -l)" == 0
            touch $out
          '';
        };
    };
}
