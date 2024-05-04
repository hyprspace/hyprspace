{ pkgs }: rec {
  uniqueConfigFileName = pkgs.writeShellScript "hyprspace-dev-gen-config-name" ''
    inst="$(echo "$(id -u)" "$PWD" "$1" | md5sum | cut -d' ' -f1)"
    echo "''${TMPDIR:-/tmp}/hyprspace-dev-config-$1-$inst.json"
  '';
  writeConfigs = pkgs.writeShellScript "hyprspace-dev-write-configs" ''
    peersFile="$(mktemp ''${TMPDIR:-/tmp}/hyprspace-dev-peers-XXXXXXXX.ndjson)"
    trap "rm -f $peersFile" EXIT
    for i in $(seq 0 1 $(($1 - 1))); do
      intf="hsdev$i"
      name="dev$i"
      configFile="$(${uniqueConfigFileName} $intf)"
      echo Creating config file for $name
      ./hyprspace init -i "$intf" -c "$configFile" | tee /dev/stderr | tail -n +3 | ${pkgs.jq}/bin/jq ".name = \"$name\" | .routes = [{\"net\":\"169.254.$i.1/32\"}]" -c >> "$peersFile"
    done
    for i in $(seq 0 1 $(($1 - 1))); do
      intf="hsdev$i"
      name="dev$i"
      configFile="$(${uniqueConfigFileName} $intf)"
      echo Configuring peers for $name
      ${pkgs.jq}/bin/jq <"$configFile" ".listenAddresses = [
        \"/ip6/::/tcp/$(echo "$((48101 + i))")\",
        \"/ip6/::/udp/$(echo "$((48101 + i))")/quic-v1\"
      ] | .peers = $(${pkgs.jq}/bin/jq -cs <"$peersFile" "map(select(.name != \"$name\"))")" | ${pkgs.moreutils}/bin/sponge "$configFile"
    done
    echo Done
  '';
  runHyprspaceDevProcess = pkgs.writeShellScript "hyprspace-dev" ''
    intf="hsdev$1"
    name="dev$1"
    configFile="$(${uniqueConfigFileName} $intf)"
    trap "rm -vf $configFile" EXIT
    rm -f "/run/hyprspace-rpc.$intf.sock"
    ./hyprspace up -i "$intf" -c "$configFile"
  '';
  repeat = pkgs.writeShellScript "hyprspace-dev-repeat-cmd" ''
    while sleep 10; do
      echo ========================================
      "$@"
    done
  '';
}
