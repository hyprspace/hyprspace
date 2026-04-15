{
  pkgs,
  self,
}:
let
  # Pre-generated test keypairs (not secret — test-only)
  peer1 = {
    privateKey = "z23jhTbcjBgHmxFeeC11RXxeXb32Ce9eMvMy75ke2TrCsXpNrNvVsH98qSumzxPBbF7TqyBsd7macsAYfnVQzUApd51ZgC";
    id = "12D3KooWP6VEGDDp6eiCSubLai8A9hoAWuFN9A2A5BysyaSbXtRN";
  };
  peer2 = {
    privateKey = "z23jhTbwi6M5cGWn1GeC7eXV2TbJpFhgmtVpsUjKshKKrSCy2seByS6da3epoBtEsMc8HA5oToay8RVG2w9nACHdt5WUge";
    id = "12D3KooWNfMAtF1ZWb2j9MYpFz1ZmRzUAyq1CPYAwGttomNmqY46";
  };

  mkPeerConfig =
    { privateKeyFile, peers }:
    {
      imports = [ self.nixosModules.default ];
      networking.firewall.enable = false;
      services.resolved.enable = true;
      services.hyprspace = {
        enable = true;
        inherit privateKeyFile;
        settings.peers = peers;
      };
    };
in
pkgs.testers.nixosTest {
  name = "hyprspace";

  nodes = {
    peer1 = mkPeerConfig {
      privateKeyFile = pkgs.writeText "hs-key-peer1" peer1.privateKey;
      peers = [
        {
          id = peer2.id;
          name = "peer2";
        }
      ];
    };
    peer2 = mkPeerConfig {
      privateKeyFile = pkgs.writeText "hs-key-peer2" peer2.privateKey;
      peers = [
        {
          id = peer1.id;
          name = "peer1";
        }
      ];
    };
  };

  testScript = ''
    start_all()

    for node in [peer1, peer2]:
        node.wait_for_unit("hyprspace.service")

        # TUN interface with an IP in the hyprspace range (100.64.0.0/16)
        node.wait_until_succeeds(
            "${pkgs.iproute2}/bin/ip addr show hyprspace | grep -q 'inet 100.64.'",
            timeout=10,
        )

    # Peers discover each other via mDNS on the shared VLAN.
    # Verify end-to-end connectivity through the hyprspace tunnel.
    peer1.wait_until_succeeds("ping -c1 -W3 peer2.hyprspace", timeout=60)
    peer2.wait_until_succeeds("ping -c1 -W3 peer1.hyprspace", timeout=60)
  '';
}
