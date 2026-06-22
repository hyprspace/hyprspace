{ lib, ... }:
let

  inherit (lib) types mkOption mkEnableOption;

  t = {
    multiAddr = types.strMatching "/.*[^/]" // {
      description = "multiaddr";
    };

    ipnet = types.strMatching "[^/]*/[0-9]*" // {
      description = "IP/CIDR network";
    };

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
          type = types.listOf (
            types.submodule {
              options.net = mkOption {
                type = t.ipnet;
                description = "Network specification.";
              };
            }
          );
          description = "Networks to route to this peer. (optional)";
          default = [ ];
          example = [ { net = "10.10.0.0/16"; } ];
        };
      };
    };

    service = types.submodule {
      options = {
        target = mkOption {
          type = t.multiAddr;
          description = "Target address.";
          example = "/tcp/8080";
        };

        acl = {
          enableWhitelist = mkEnableOption "whitelist enforcement";

          whitelist = mkOption {
            type = types.listOf types.str;
            description = "List of peers that are allowed to connect.";
            example = [
              "12D3KooWQWiPeNvXFdHFTrustedPeer"
              "@goodpeer"
            ];
            default = [ ];
          };

          blacklist = mkOption {
            type = types.listOf types.str;
            description = "List of peers that are explicitly not allowed to connect.";
            example = [
              "12D3KooWQWiPeNvXFdHFUntrustedPeer"
              "@badpeer"
            ];
            default = [ ];
          };
        };
      };
    };
  };
in

{
  options = {
    filterPrivateAddresses = mkEnableOption "filtering of private/link-local addresses from peer discovery. When enabled, the node will not attempt to connect to RFC1918, link-local, or loopback addresses advertised by other peers";

    domain = mkOption {
      type = types.str;
      description = "Domain suffix used for DNS names within the Hyprspace network.";
      default = "hyprspace";
      example = "vpn.internal";
    };

    bootstrapPeers = mkOption {
      type = types.listOf t.multiAddr;
      description = "List of libp2p bootstrap node multiaddresses for initial network discovery.";
      default = [
        "/ip4/152.67.75.145/tcp/110/p2p/12D3KooWQWsHPUUeFhe4b6pyCaD1hBoj8j6Z7S7kTznRTh1p1eVt"
        "/ip4/152.67.75.145/udp/110/quic-v1/p2p/12D3KooWQWsHPUUeFhe4b6pyCaD1hBoj8j6Z7S7kTznRTh1p1eVt"
        "/ip4/152.67.75.145/tcp/995/p2p/QmbrAHuh4RYcyN9fWePCZMVmQjbaNXtyvrDCWz4VrchbXh"
        "/ip4/152.67.75.145/udp/995/quic-v1/p2p/QmbrAHuh4RYcyN9fWePCZMVmQjbaNXtyvrDCWz4VrchbXh"
        "/ip4/95.216.8.12/tcp/110/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo"
        "/ip4/95.216.8.12/udp/110/quic-v1/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo"
        "/ip4/95.216.8.12/tcp/995/p2p/QmYs4xNBby2fTs8RnzfXEk161KD4mftBfCiR8yXtgGPj4J"
        "/ip4/95.216.8.12/udp/995/quic-v1/p2p/QmYs4xNBby2fTs8RnzfXEk161KD4mftBfCiR8yXtgGPj4J"
        "/ip4/152.67.73.164/tcp/995/p2p/12D3KooWL84sAtq1QTYwb7gVbhSNX5ZUfVt4kgYKz8pdif1zpGUh"
        "/ip4/152.67.73.164/udp/995/quic-v1/p2p/12D3KooWL84sAtq1QTYwb7gVbhSNX5ZUfVt4kgYKz8pdif1zpGUh"
        "/ip4/37.27.11.202/udp/21/quic-v1/p2p/12D3KooWN31twBvdEcxz2jTv4tBfPe3mkNueBwDJFCN4xn7ZwFbi"
        "/ip4/37.27.11.202/udp/443/quic-v1/p2p/12D3KooWN31twBvdEcxz2jTv4tBfPe3mkNueBwDJFCN4xn7ZwFbi"
        "/ip4/37.27.11.202/udp/500/quic-v1/p2p/12D3KooWN31twBvdEcxz2jTv4tBfPe3mkNueBwDJFCN4xn7ZwFbi"
        "/ip4/37.27.11.202/udp/995/quic-v1/p2p/12D3KooWN31twBvdEcxz2jTv4tBfPe3mkNueBwDJFCN4xn7ZwFbi"
        "/dnsaddr/bootstrap.libp2p.io/p2p/12D3KooWEZXjE41uU4EL2gpkAQeDXYok6wghN7wwNVPF5bwkaNfS"
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa"
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp"
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb"
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt"
      ];
    };

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
      default = [ ];
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
      type = types.attrsOf t.service;
      description = "The services this node provides via the Service Network.";
      default = { };
      example = {
        www-local = {
          target = "/tcp/8080";
        };
        gameserver = {
          target = "/ip4/10.0.0.2/tcp/27015";
          acl = {
            enableWhitelist = true;
            whitelist = [
              "@friend1"
              "@friend2"
            ];
          };
        };
      };
    };
  };
}
