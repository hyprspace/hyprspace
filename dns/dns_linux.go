//go:build linux
// +build linux

package dns

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hyprspace/hyprspace/config"
	"github.com/iguanesolutions/go-systemd/v5/resolved"
	"github.com/vishvananda/netlink"
)

func registerOnSystemResolver(ctx context.Context, config config.Config, dnsServerAddrBytes []byte, dnsServerPort uint16) error {
	conn, err := resolved.NewConn()
	if err != nil {
		return fmt.Errorf("[!] [dns] Failed to connect to D-Bus: %s", err)
	}
	defer conn.Close()

	link, err := netlink.LinkByName(config.Interface)
	if err != nil {
		return fmt.Errorf("[!] [dns] Failed to get link ID: %s", err)
	}
	linkID := link.Attrs().Index

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
			return fmt.Errorf("[!] [dns] Failed to configure resolved: %s", err)
		}
	}
	return nil
}
