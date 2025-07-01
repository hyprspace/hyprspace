{
  config,
  lib,
  withSystem,
  ...
}:

{
  hercules-ci.github-releases = {
    condition = { branch, ... }: branch == "master";
    releaseTag =
      _: "v${withSystem config.defaultEffectSystem ({ config, ... }: config.packages.hyprspace.version)}";
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
