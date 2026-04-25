package p2p

import (
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// resolveListenAddrs expands unspecified listen addresses (0.0.0.0, ::) into
// concrete per-interface addresses of physical interfaces.
// Specific addresses are returned as-is.
func resolveListenAddrs(addrs []ma.Multiaddr) ([]ma.Multiaddr, error) {
	net.Interfaces()
	physAddrs, err := physicalInterfaceAddrs()
	if err != nil {
		// If we can't enumerate interfaces, fall back to the
		// original (unresolved) addresses.
		logger.With(err).Warn("failed to enumerate interfaces")
		return addrs, nil
	}
	if len(physAddrs) == 0 {
		return addrs, nil
	}

	ifaceAddrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return addrs, nil
	}

	filtered := ma.FilterAddrs(
		ifaceAddrs,
		func(addr ma.Multiaddr) bool {
			return !manet.IsIPLoopback(addr)
		},
		func(addr ma.Multiaddr) bool {
			ipAddr, err := manet.ToIP(addr)
			if err != nil {
				return true
			} else {
				for _, physAddr := range physAddrs {
					if physAddr.Equal(ipAddr) {
						return true
					}
				}
				return false
			}
		},
	)

	resolved, err := manet.ResolveUnspecifiedAddresses(addrs, filtered)
	if err != nil {
		return addrs, nil
	}
	return resolved, nil
}
