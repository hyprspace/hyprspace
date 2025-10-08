//go:build !linux
// +build !linux

package dns

import (
	"context"

	"github.com/hyprspace/hyprspace/config"
)

// configureSystemdResolved is a no-op on non-Linux platforms
func configureSystemdResolved(ctx context.Context, config config.Config, dnsServerAddrBytes []byte, dnsServerPort uint16) {
	// systemd-resolved is Linux-only, nothing to do on other platforms
}
