{ lib, ... }:

{
  perSystem = { pkgs, ... }: {
    process-compose.dev-run = { config, ... }: let
      scripts = import ./scripts.nix {
        inherit pkgs;
      };
      replicas = 2;
      forReplicas = f: lib.listToAttrs (map f (lib.range 0 (replicas - 1)));
    in {
      settings.processes = {
        write-configs = {
          command = "${scripts.writeConfigs} ${toString replicas}";
          namespace = "init";
        };
      }
      // forReplicas (n: {
        name = "hyprspace-dev${toString n}";
        value = {
          command = "${scripts.runHyprspaceDevProcess} ${toString n}";
          depends_on.write-configs.condition = "process_completed_successfully";
        };
      })
      // forReplicas (n: {
        name = "hyprspace-status-dev${toString n}";
        value = {
          command = "${scripts.repeat} ./hyprspace status -i hsdev${toString n}";
          namespace = "monitor";
          depends_on.write-configs.condition = "process_completed_successfully";
        };
      })
      // forReplicas (n: {
        name = "hyprspace-route-show-dev${toString n}";
        value = {
          command = "${scripts.repeat} ./hyprspace route show -i hsdev${toString n}";
          namespace = "monitor";
          depends_on.write-configs.condition = "process_completed_successfully";
        };
      });
    };
  };
}
