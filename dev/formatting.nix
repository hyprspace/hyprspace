{ lib, ... }:

{
  perSystem =
    { config, pkgs, ... }:
    {
      formatter = pkgs.nixfmt;
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
          format-nix = pkgs.runCommand "format-nix" { } ''
            ${lib.getExe config.formatter} -c ${filesWithExtension "nix"}
            touch $out
          '';
          format-go = pkgs.runCommand "format-go" { } ''
            test "$(${lib.getExe' pkgs.go "gofmt"} -l ${filesWithExtension "go"} | tee /dev/stderr | wc -l)" == 0
            touch $out
          '';
        };
    };
}
