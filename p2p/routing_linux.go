//go:build linux
// +build linux

package p2p

import (
	"net"

	"github.com/vishvananda/netlink"
)

// getRouteInterface returns the interface index for the route to the given IP address.
// Returns 0 if no route is found or an error occurs.
func getRouteInterface(ip net.IP) int {
	routes, err := netlink.RouteGet(ip)
	if err != nil {
		return 0
	}
	if len(routes) > 0 {
		return routes[0].LinkIndex
	}
	return 0
}
