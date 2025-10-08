//go:build linux
// +build linux

package dns

import "net"

// getDNSServerAddr returns a unique loopback address for the DNS server
// On Linux, we can use any address in the 127.0.0.0/8 range
func getDNSServerAddr(interfaceName string) net.IP {
	dnsServerAddrBytes := []byte{127, 80, 01, 53}
	for i, b := range []byte(interfaceName) {
		dnsServerAddrBytes[(i%3)+1] ^= b
	}
	return net.IP(dnsServerAddrBytes)
}
