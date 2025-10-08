//go:build darwin
// +build darwin

package p2p

import (
	"bufio"
	"bytes"
	"net"
	"os/exec"
	"strings"
)

// getRouteInterface returns the interface index for the route to the given IP address.
// Returns 0 if no route is found or an error occurs.
func getRouteInterface(ip net.IP) int {
	// Use the route command to get routing information
	cmd := exec.Command("route", "-n", "get", ip.String())
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse the output to find the interface name
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var ifaceName string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "interface:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ifaceName = parts[1]
				break
			}
		}
	}

	if ifaceName == "" {
		return 0
	}

	// Convert interface name to index
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return 0
	}

	return iface.Index
}
