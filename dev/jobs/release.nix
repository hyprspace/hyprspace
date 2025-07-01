{ lib, ... }:

{
  hercules-ci.github-releases = {
    filesPerSystem =
      { config, system, ... }:
      [
        {
          label = "hyprspace-${system}";
          path = lib.getExe config.packages.hyprspace;
        }
      ];
  };
}
