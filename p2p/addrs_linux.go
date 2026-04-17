//go:build linux

package p2p

import (
	"net"

	"github.com/vishvananda/netlink"
)

// physicalInterfaceAddrs returns all IP addresses assigned to physical interfaces.
func physicalInterfaceAddrs() ([]net.IP, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, link := range links {
		if link.Type() != "device" {
			continue
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
