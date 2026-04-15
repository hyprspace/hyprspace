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
    ndg = {
      url = "github:feel-co/ndg";
      inputs.nixpkgs.follows = "nixpkgs";
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
                services.hyprspace.package = lib.mkOptionDefault (
                  pkgs.callPackage ./package.nix {
                    generateSchemasProgram = pkgs.callPackage ./dev/generate-schemas.nix {
                      go-jsonschema = pkgs.callPackage ./dev/pkgs/go-jsonschema { };
                      settingsModule = ./nixos/settings.nix;
                    };
                  }
                );
              };
          };
        };

      imports = [
        inputs.hercules-ci-effects.flakeModule
        ./dev
      ];

      perSystem =
        {
          config,
          inputs',
          pkgs,
          ...
        }:
        let
          generateSchemasProgram = pkgs.callPackage ./dev/generate-schemas.nix {
            go-jsonschema = pkgs.callPackage ./dev/pkgs/go-jsonschema { };
            settingsModule = ./nixos/settings.nix;
          };
        in
        {
          packages = {
            default = config.packages.hyprspace;
            hyprspace = pkgs.callPackage ./package.nix {
              inherit generateSchemasProgram;
            };
            docs = pkgs.callPackage ./docs/package.nix {
              hyprspace = config.packages.default;
              inherit (inputs'.ndg.packages) ndg;
            };
            vendor = pkgs.callPackage ./dev/vendor.nix {
              inherit generateSchemasProgram;
            };
          };

          checks = pkgs.lib.optionalAttrs (pkgs.stdenv.hostPlatform.system == "x86_64-linux") {
            vm-test = import ./nixos/test.nix {
              inherit pkgs;
              self = inputs.self;
            };
          };
        };
    };
}
