//go:build !linux
// +build !linux

package dns

import "net"

// getDNSServerAddr returns the loopback address for the DNS server
// On non-Linux systems (Darwin, etc), we must use 127.0.0.1 as other
// addresses in the 127.0.0.0/8 range are not automatically available
func getDNSServerAddr(interfaceName string) net.IP {
	return net.IPv4(127, 0, 0, 1)
}
