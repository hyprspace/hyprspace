package dns

import (
	"net"
	"testing"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_domainSuffix(t *testing.T) {
	tests := []struct {
		iface, want string
	}{
		{"hyprspace", "hyprspace."},
		{"hs0", "hs0.hyprspace."},
		{"", ".hyprspace."},
	}
	for _, tt := range tests {
		t.Run(tt.iface, func(t *testing.T) {
			assert.Equal(t, tt.want, domainSuffix(config.Config{Interface: tt.iface, Domain: "hyprspace"}))
		})
	}
}

func Test_withDomainSuffix(t *testing.T) {
	tests := []struct {
		prefix, iface, want string
	}{
		{"alice", "hyprspace", "alice.hyprspace."},
		{"bob", "hs0", "bob.hs0.hyprspace."},
		{"http.alice", "hs0", "http.alice.hs0.hyprspace."},
	}
	for _, tt := range tests {
		t.Run(tt.iface, func(t *testing.T) {
			assert.Equal(t, tt.want, withDomainSuffix(config.Config{Interface: tt.iface, Domain: "hyprspace"}, tt.prefix))
		})
	}
}

func Test_mkAliasRecord_emptyService(t *testing.T) {
	cfg := config.Config{Interface: "hs0", Domain: "hyprspace"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	record := mkAliasRecord(cfg, "alice", "", pid)

	assert.Equal(t, uint16(dns.TypeCNAME), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(0), record.Hdr.Ttl)
	assert.Equal(t, "alice.hs0.hyprspace.", record.Hdr.Name)
	assert.True(t, len(record.Target) > 0, "CNAME target should not be empty")
	assert.Contains(t, record.Target, "hs0.hyprspace.", "CNAME target should contain domain suffix")
}

func Test_mkAliasRecord_withService(t *testing.T) {
	cfg := config.Config{Interface: "hs0", Domain: "hyprspace"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	record := mkAliasRecord(cfg, "alice", "http", pid)

	assert.Equal(t, dns.TypeCNAME, record.Hdr.Rrtype)
	assert.Equal(t, "http.alice.hs0.hyprspace.", record.Hdr.Name)
	assert.Contains(t, record.Target, "hs0.hyprspace.")
}

func Test_mkAliasRecord_emptyName(t *testing.T) {
	cfg := config.Config{Interface: "hs0", Domain: "hyprspace"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	// Empty alias name should still produce a valid CNAME record
	// The Hdr.Name will be just the domain suffix ("hs0.hyprspace.")
	// because withDomainSuffix("", "") = ".hs0.hyprspace."
	record := mkAliasRecord(cfg, "", "", pid)

	assert.Equal(t, uint16(dns.TypeCNAME), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(0), record.Hdr.Ttl)
	assert.True(t, len(record.Hdr.Name) > 0, "CNAME name should not be empty")
	assert.True(t, len(record.Target) > 0, "CNAME target should not be empty")
	assert.Contains(t, record.Target, "hs0.hyprspace.", "CNAME target should contain domain suffix")
}

func Test_mkIDRecord(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	type testCase struct {
		fn     func(config.Config, peer.ID, string, net.IP) dns.RR
		wantRr uint16
		addr   net.IP
		is4    bool
		hasSvc bool
	}

	tests := []testCase{
		{
			fn:     func(cfg config.Config, id peer.ID, _ string, addr net.IP) dns.RR { return mkIDRecord4(cfg, id, addr) },
			wantRr: dns.TypeA,
			addr:   net.ParseIP("100.64.1.2"),
			is4:    true,
		},
		{
			fn: func(cfg config.Config, id peer.ID, _ string, addr net.IP) dns.RR {
				return mkIDRecord6(cfg, id, "", addr)
			},
			wantRr: dns.TypeAAAA,
			addr:   net.ParseIP("fd00::1"),
		},
		{
			fn: func(cfg config.Config, id peer.ID, _ string, addr net.IP) dns.RR {
				return mkIDRecord6(cfg, id, "http", addr)
			},
			wantRr: dns.TypeAAAA,
			addr:   config.MkServiceAddr6(pid, "http"),
			hasSvc: true,
		},
	}

	for i, tt := range tests {
		name := []string{"ipv4", "ipv6", "ipv6-svc"}[i]
		t.Run(name, func(t *testing.T) {
			cfg := config.Config{Interface: "hs0", Domain: "hyprspace"}
			record := tt.fn(cfg, pid, "", tt.addr)

			hdr := record.Header()
			assert.Equal(t, tt.wantRr, hdr.Rrtype)
			assert.Equal(t, uint16(dns.ClassINET), hdr.Class)
			assert.Equal(t, uint32(86400), hdr.Ttl)
			assert.Contains(t, hdr.Name, "hs0.hyprspace.")

			if tt.is4 {
				aRecord, ok := record.(*dns.A)
				require.True(t, ok)
				assert.Equal(t, tt.addr.To4(), aRecord.A)
			} else {
				aaaaRecord, ok := record.(*dns.AAAA)
				require.True(t, ok)
				assert.Equal(t, tt.addr.To16(), aaaaRecord.AAAA)
			}
			if tt.hasSvc {
				assert.Contains(t, hdr.Name, "http.")
			}
		})
	}
}

func Test_mkIDRecord_nil_addr(t *testing.T) {
	cfg := config.Config{Interface: "hs0", Domain: "hyprspace"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	record := mkIDRecord4(cfg, pid, nil)

	// Record structure is still valid
	assert.Equal(t, uint16(dns.TypeA), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(86400), record.Hdr.Ttl)
	assert.Nil(t, record.A, "A record with nil address should have nil A field")
	assert.Contains(t, record.Hdr.Name, "hs0.hyprspace.")

	// Verify the DNS library handles nil A fields without panicking on write.
	// This exercises the live WriteMsg/Pack path, which is the actual production
	// code path used by the DNS server when BuiltinAddr4 is unexpectedly nil.
	msg := new(dns.Msg)
	msg.SetReply(new(dns.Msg))
	msg.Answer = append(msg.Answer, record)
	buf, err := msg.Pack()
	require.NoError(t, err)
	assert.NotNil(t, buf, "DNS library should serialize message without panicking on nil A")
}

func Test_writeResponse(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	msg := new(dns.Msg)
	q := dns.Question{Name: "test.", Qtype: dns.TypeA}
	addr := net.ParseIP("100.64.1.2")

	writeResponse(msg, q, pid, addr)

	// Should have one A record
	require.Len(t, msg.Answer, 1, "should have 1 A record")
	aRecord, ok := msg.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "test.", aRecord.Hdr.Name)
	assert.Equal(t, dns.TypeA, aRecord.Hdr.Rrtype)
	assert.Equal(t, uint32(0), aRecord.Hdr.Ttl)
	assert.Equal(t, addr.To4(), aRecord.A)

	// Should have one TXT record in Extra
	require.Len(t, msg.Extra, 1)
	txtRecord, ok := msg.Extra[0].(*dns.TXT)
	require.True(t, ok)
	assert.Equal(t, "test.", txtRecord.Hdr.Name)
	assert.Equal(t, dns.TypeTXT, txtRecord.Hdr.Rrtype)
	assert.Equal(t, 2, len(txtRecord.Txt))
	assert.Contains(t, txtRecord.Txt[0], pid.String())
}

func Test_writeResponse_MultipleQuestions(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	msg := new(dns.Msg)
	q1 := dns.Question{Name: "alice.", Qtype: dns.TypeA}
	q2 := dns.Question{Name: "bob.", Qtype: dns.TypeA}
	addr1 := net.ParseIP("100.64.1.2")
	addr2 := net.ParseIP("100.64.1.3")

	writeResponse(msg, q1, pid, addr1)
	writeResponse(msg, q2, pid, addr2)

	// Should have 2 A records
	require.Len(t, msg.Answer, 2, "should have 2 A records")
	assert.Equal(t, "alice.", msg.Answer[0].(*dns.A).Hdr.Name)
	assert.Equal(t, addr1.To4(), msg.Answer[0].(*dns.A).A)
	assert.Equal(t, "bob.", msg.Answer[1].(*dns.A).Hdr.Name)
	assert.Equal(t, addr2.To4(), msg.Answer[1].(*dns.A).A)

	// Should have 2 TXT records in Extra
	require.Len(t, msg.Extra, 2, "should have 2 TXT records")
	assert.Equal(t, "alice.", msg.Extra[0].(*dns.TXT).Hdr.Name)
	assert.Equal(t, "bob.", msg.Extra[1].(*dns.TXT).Hdr.Name)
	assert.Equal(t, 2, len(msg.Extra[0].(*dns.TXT).Txt))
	assert.Equal(t, 2, len(msg.Extra[1].(*dns.TXT).Txt))
}
