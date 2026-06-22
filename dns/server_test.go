package dns

import (
	"testing"

	"github.com/hyprspace/hyprspace/config"
)

func TestDomainSuffix(t *testing.T) {
	tests := []struct {
		name   string
		iface  string
		domain string
		want   string
	}{
		{"default iface, default domain", "hyprspace", "hyprspace", "hyprspace."},
		{"default iface, custom domain", "hyprspace", "vpn", "vpn."},
		{"custom iface, default domain", "hs0", "hyprspace", "hs0.hyprspace."},
		{"custom iface, custom domain", "hs0", "vpn", "hs0.vpn."},
		{"multi-label domain", "hyprspace", "vpn.internal", "vpn.internal."},
		{"custom iface, multi-label domain", "hs0", "vpn.internal", "hs0.vpn.internal."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{Interface: tt.iface, Domain: tt.domain}
			got := domainSuffix(cfg)
			if got != tt.want {
				t.Errorf("domainSuffix() = %q, want %q", got, tt.want)
			}
		})
	}
}
