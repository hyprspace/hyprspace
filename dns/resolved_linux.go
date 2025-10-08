//go:build linux
// +build linux

package dns

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"github.com/hyprspace/hyprspace/config"
	"github.com/iguanesolutions/go-systemd/v5/resolved"
)

// configureSystemdResolved configures systemd-resolved to use the DNS server for the domain
func configureSystemdResolved(ctx context.Context, config config.Config, dnsServerAddrBytes []byte, dnsServerPort uint16) {
	conn, err := resolved.NewConn()
	if err != nil {
		fmt.Println("[!] [dns] Failed to connect to D-Bus:", err)
		return
	}
	defer conn.Close()

	iface, err := net.InterfaceByName(config.Interface)
	if err != nil {
		fmt.Println("[!] [dns] Failed to get link ID:", err)
		return
	}
	linkID := iface.Index

	for _, f := range [](func() error){
		func() error {
			return conn.SetLinkDNSEx(ctx, linkID, []resolved.LinkDNSEx{{
				Family:  syscall.AF_INET,
				Address: dnsServerAddrBytes,
				Port:    dnsServerPort,
			}})
		},
		func() error {
			return conn.SetLinkDomains(ctx, linkID, []resolved.LinkDomain{{
				Domain:        domainSuffix(config),
				RoutingDomain: false,
			}})
		},
	} {
		if err := f(); err != nil {
			fmt.Println("[!] [dns] Failed to configure resolved:", err)
			return
		}
	}
}
