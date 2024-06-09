{
  config,
  lib,
  options,
  pkgs,
  ...
}:

let
  inherit (lib)
    types
    mkOption
    mkEnableOption
    mkIf
    mkMerge
    toInt
    last
    head
    escapeShellArg
    ;

  cfg = config.services.hyprspace;
  opt = options.services.hyprspace;

  usePrivateKeyFromFile = options.services.hyprspace.privateKeyFile.isDefined;
  privKeyMarker = "@HYPRSPACEPRIVATEKEY@";
  runConfig = "/run/hyprspace.json";
  configFile = pkgs.writeText "hyprspace-config.json" (
    builtins.toJSON (
      cfg.settings
      // {
        privateKey = if usePrivateKeyFromFile then privKeyMarker else cfg.settings.privateKey;
      }
    )
  );

  maybeMetricsPort = mkIf opt.metricsPort.isDefined (toString opt.metricsPort.value);

  listenPorts = map (builtins.match "/.*/(tcp|udp)/([0-9]*).*") cfg.settings.listenAddresses;
in

{
  options.services.hyprspace = {
    enable = mkEnableOption "Hyprspace";

    package = mkOption {
      type = types.package;
      description = "Hyprspace package to use.";
    };

    interface = mkOption {
      type = types.str;
      description = "Interface name.";
      default = "hyprspace";
    };

    settings = mkOption {
      type = types.submodule ./settings.nix;
      description = "Hyprspace configuration options.";
      default = { };
    };

    privateKeyFile = mkOption {
      type = types.path;
      description = "File containing this node's private key.";
      example = "/etc/secrets/hyprspace-key";
    };

    metricsPort = mkOption {
      type = types.port;
      description = "Prometheus metrics endpoint port.";
    };
  };

  config = mkIf cfg.enable {
    systemd.services.hyprspace = {
      description = "Hyprspace Distributed Network";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      preStart = mkIf usePrivateKeyFromFile ''
        test -e ${runConfig} && rm ${runConfig}
        cp ${configFile} ${runConfig}
        chmod 0600 ${runConfig}
        ${lib.getExe pkgs.replace-secret} '${privKeyMarker}' "${cfg.privateKeyFile}" ${runConfig}
        chmod 0400 ${runConfig}
      '';

      serviceConfig = {
        Group = "wheel";
        Restart = "on-failure";
        RestartSec = "5s";
        ExecStart = "${lib.getExe cfg.package} up -c ${
          if usePrivateKeyFromFile then runConfig else configFile
        } -i ${escapeShellArg cfg.interface}";
        ExecStopPost = "${lib.getExe' pkgs.coreutils "rm"} -f ${escapeShellArg "/run/hyprspace-rpc.${cfg.interface}.sock"}";
        ExecReload = "${lib.getExe' pkgs.coreutils "kill"} -USR1 $MAINPID";
      };

      environment.HYPRSPACE_METRICS_PORT = maybeMetricsPort;
    };

    networking.firewall = mkMerge (
      map (
        x:
        let
          port = toInt (last x);
          proto = head x;
        in
        if proto == "tcp" then
          { allowedTCPPorts = [ port ]; }
        else if proto == "udp" then
          { allowedUDPPorts = [ port ]; }
        else
          throw "unsupported protocol: ${proto}"
      ) listenPorts
    );

    environment.systemPackages = [ cfg.package ];
  };
}
