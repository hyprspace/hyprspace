package dns

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hyprspace/hyprspace/config"
	"github.com/iguanesolutions/go-systemd/v5/resolved"
	"github.com/vishvananda/netlink"
)

func configureSystemdResolved(ctx context.Context, config config.Config, dnsServerAddrBytes []byte, dnsServerPort uint16) error {
	conn, err := resolved.NewConn()
	if err != nil {
		return fmt.Errorf("Failed to connect to D-Bus: %w", err)
	}
	defer conn.Close()

	link, err := netlink.LinkByName(config.Interface)
	if err != nil {
		return fmt.Errorf("Failed to get link ID: %w", err)
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
		func() error {
			return conn.SetLinkDNSSEC(ctx, linkID, "no")
		},
		func() error {
			return conn.SetLinkDNSOverTLS(ctx, linkID, "no")
		},
		func() error {
			return conn.SetLinkLLMNR(ctx, linkID, "no")
		},
		func() error {
			return conn.SetLinkMulticastDNS(ctx, linkID, "no")
		},
	} {
		if err := f(); err != nil {
			return fmt.Errorf("Failed to configure systemd-resolved: %w", err)
		}
	}
	return nil
}
