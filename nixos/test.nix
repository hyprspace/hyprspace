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

  # Each test peer gets a dummy network interface ("dummytun") that simulates
  # another VPN's tunnel. hyprspace MUST NOT advertise its IP.
  dummyTunPrefix = "10.99.0";

  mkPeerConfig =
    {
      privateKeyFile,
      peers,
      dummyTunAddr,
    }:
    {
      imports = [ self.nixosModules.default ];
      networking.firewall.enable = false;
      services.resolved.enable = true;
      services.hyprspace = {
        enable = true;
        inherit privateKeyFile;
        settings.peers = peers;
      };
      # Dummy interface that simulates another VPN's tunnel device.
      # hyprspace MUST NOT advertise its IP via mDNS.
      systemd.services.dummytun = {
        description = "Dummy tunnel interface for anti-loop test";
        before = [ "hyprspace.service" ];
        wantedBy = [ "multi-user.target" ];
        serviceConfig.Type = "oneshot";
        serviceConfig.RemainAfterExit = true;
        script = ''
          ${pkgs.iproute2}/bin/ip tuntap add dev dummytun mode tun
          ${pkgs.iproute2}/bin/ip addr add ${dummyTunAddr}/24 dev dummytun
          ${pkgs.iproute2}/bin/ip link set dummytun up
        '';
      };
    };
in
pkgs.testers.nixosTest {
  name = "hyprspace";

  nodes = {
    peer1 = mkPeerConfig {
      privateKeyFile = pkgs.writeText "hs-key-peer1" peer1.privateKey;
      dummyTunAddr = "${dummyTunPrefix}.1";
      peers = [
        {
          id = peer2.id;
          name = "peer2";
        }
      ];
    };
    peer2 = mkPeerConfig {
      privateKeyFile = pkgs.writeText "hs-key-peer2" peer2.privateKey;
      dummyTunAddr = "${dummyTunPrefix}.2";
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

    # Verify that tunnel device IPs are NOT in the mDNS-advertised addresses.
    # "Advertised addresses (mDNS)" in hyprspace status shows the resolved
    # interface listen addresses — exactly what gets broadcast via mDNS.
    dummy_tun_ip = "${dummyTunPrefix}.{}"
    for i, node in enumerate([peer1, peer2], start=1):
        ip = dummy_tun_ip.format(i)
        status = node.succeed("hyprspace status")
        # Extract only the mDNS-advertised section
        mdns_section = status.split("Advertised addresses (mDNS):")[1] if "Advertised addresses (mDNS):" in status else status
        print(f"peer{i} mDNS advertised:\n{mdns_section}")
        assert ip not in mdns_section, (
            f"peer{i} advertises dummy tunnel IP {ip} via mDNS \u2014 VPN-over-VPN loop risk.\n"
            f"mDNS advertised addresses:\n{mdns_section}"
        )
  '';
}
