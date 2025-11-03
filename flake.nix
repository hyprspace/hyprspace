{
  description = "Hyprspace";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    hercules-ci-effects = {
      url = "github:max-privatevoid/hercules-ci-effects/pr/skip-if-exists";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-parts.follows = "flake-parts";
      };
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      herculesCI.ciSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      flake =
        { config, ... }:
        {
          nixosModules = {
            default = config.nixosModules.hyprspace;
            hyprspace =
              { lib, pkgs, ... }:
              {
                imports = [ ./nixos ];
                services.hyprspace.package = lib.mkOptionDefault inputs.self.packages.${pkgs.stdenv.hostPlatform.system}.default;
              };
          };
        };

      imports = [
        inputs.hercules-ci-effects.flakeModule
        ./dev
      ];

      perSystem =
        { config, pkgs, ... }:
        {
          packages = {
            default = config.packages.hyprspace;
            hyprspace = pkgs.callPackage ./package.nix {
              generateSchemasProgram = config.apps.dev-generate-schemas.program;
            };
            docs = pkgs.callPackage ./docs/package.nix { hyprspace = config.packages.default; };
            vendor = pkgs.callPackage ./dev/vendor.nix {
              generateSchemasProgram = config.apps.dev-generate-schemas.program;
            };
          };
        };
    };
}
