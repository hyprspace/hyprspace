{ lib, ... }:

with lib;

let
  t = {
    multiAddr = types.strMatching "/.*[^/]" // { description = "multiaddr"; };

    ipnet = types.strMatching "[^/]*/[0-9]*" // { description = "IP/CIDR network"; };

    peer = types.submodule {
      options = {
        id = mkOption {
          type = types.str;
          description = "PeerID of this peer.";
          example = "12D3KooWCcKjF5PNZ7uXTMyNode";
        };

        name = mkOption {
          type = types.str;
          description = "Friendly name for this peer. (optional)";
          default = "";
          example = "mynode";
        };

        routes = mkOption {
          type = types.listOf (types.submodule {
            options.net = mkOption {
              type = t.ipnet;
              description = "Network specification.";
            };
          });
          description = "Networks to route to this peer. (optional)";
          default = [];
          example = [
            { net = "10.10.0.0/16"; }
          ];
        };
      };
    };
  };
in

{
  options = {
    listenAddresses = mkOption {
      type = types.listOf t.multiAddr;
      description = "List of addresses to listen on for libp2p traffic.";
      default = [
        "/ip4/0.0.0.0/tcp/8001"
        "/ip4/0.0.0.0/udp/8001/quic-v1"
        "/ip6/::/tcp/8001"
        "/ip6/::/udp/8001/quic-v1"
      ];
    };

    privateKey = mkOption {
      type = types.str;
      description = "This node's private key.";
      example = "z23jhTd4Cvo9iq9oMAweQZkCnuHLRThisIsAnInvalidExampleKey";
    };

    peers = mkOption {
      type = types.listOf t.peer;
      description = "Trusted peers in the network.";
      default = [];
      example = [
        { id = "12D3KooWKgq4aJpZM8Simple"; }
        {
          name = "router";
          id = "12D3KooWQWiPeNvXFdHFRouter";
          routes = [
            { net = "10.10.0.0/16"; }
            { net = "2001:db8:1::/64"; }
          ];
        }
      ];
    };

    services = mkOption {
      type = types.attrsOf t.multiAddr;
      description = "The services this node provides via the Service Network.";
      example = {
        "www-local" = "/tcp/8080";
        "gameserver" = "/ip4/10.0.0.2/tcp/27015";
      };
    };
  };
}
