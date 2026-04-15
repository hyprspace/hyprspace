package p2p

import (
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/vishvananda/netlink"
)

// resolveListenAddrs expands unspecified listen addresses (0.0.0.0, ::) into
// concrete per-interface addresses, excluding IPs that belong to TUN/TAP
// devices. This prevents hyprspace from advertising tunnel IPs via mDNS,
// which would cause VPN-over-VPN routing loops.
//
// Addresses that are already bound to a specific IP are returned as-is.
func resolveListenAddrs(addrs []ma.Multiaddr) ([]ma.Multiaddr, error) {
	tunnelIPs, err := tunnelInterfaceIPs()
	if err != nil {
		// If we can't enumerate tunnel interfaces, fall back to the
		// original (unresolved) addresses so libp2p can still start.
		logger.Warnf("failed to enumerate tunnel interfaces, skipping address filtering: %s", err)
		return addrs, nil
	}
	if len(tunnelIPs) == 0 {
		return addrs, nil
	}

	ifaceAddrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return addrs, nil
	}

	// Remove tunnel IPs from the interface address list.
	var filtered []ma.Multiaddr
	for _, ia := range ifaceAddrs {
		ip, err := manet.ToIP(ia)
		if err != nil {
			continue
		}
		if !isTunnelIP(ip, tunnelIPs) {
			filtered = append(filtered, ia)
		}
	}

	resolved, err := manet.ResolveUnspecifiedAddresses(addrs, filtered)
	if err != nil {
		return addrs, nil
	}
	return resolved, nil
}

// tunnelInterfaceIPs returns all IPs assigned to TUN/TAP interfaces.
func tunnelInterfaceIPs() ([]net.IP, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, link := range links {
		if _, ok := link.(*netlink.Tuntap); !ok {
			if link.Type() != "tun" && link.Type() != "tap" {
				continue
			}
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ips = append(ips, addr.IP)
		}
	}
	return ips, nil
}

func isTunnelIP(ip net.IP, tunnelIPs []net.IP) bool {
	for _, tip := range tunnelIPs {
		if tip.Equal(ip) {
			return true
		}
	}
	return false
}
