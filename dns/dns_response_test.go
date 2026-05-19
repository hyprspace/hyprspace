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

func Test_domainSuffix_hyprspace(t *testing.T) {
	cfg := config.Config{Interface: "hyprspace"}
	assert.Equal(t, "hyprspace.", domainSuffix(cfg))
}

func Test_domainSuffix_custom(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "hs0.hyprspace.", domainSuffix(cfg))
}

func Test_domainSuffix_other(t *testing.T) {
	cfg := config.Config{Interface: "myvpn"}
	assert.Equal(t, "myvpn.hyprspace.", domainSuffix(cfg))
}

func Test_domainSuffix_empty(t *testing.T) {
	cfg := config.Config{Interface: ""}
	assert.Equal(t, ".hyprspace.", domainSuffix(cfg))
}

func Test_withDomainSuffix(t *testing.T) {
	cfg := config.Config{Interface: "hyprspace"}
	assert.Equal(t, "alice.hyprspace.", withDomainSuffix(cfg, "alice"))
}

func Test_withDomainSuffix_customInterface(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "bob.hs0.hyprspace.", withDomainSuffix(cfg, "bob"))
}

func Test_withDomainSuffix_servicePrefix(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "http.alice.hs0.hyprspace.", withDomainSuffix(cfg, "http.alice"))
}

func Test_mkAliasRecord_emptyService(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
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
	cfg := config.Config{Interface: "hs0"}
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
	cfg := config.Config{Interface: "hs0"}
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

func Test_mkIDRecord4(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	// Derive builtin addr for this peer to pass in
	addr := net.ParseIP("100.64.1.2")
	record := mkIDRecord4(cfg, pid, addr)

	assert.Equal(t, uint16(dns.TypeA), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(86400), record.Hdr.Ttl)
	assert.Equal(t, addr.To4(), record.A)
	assert.Contains(t, record.Hdr.Name, "hs0.hyprspace.")
}

func Test_mkIDRecord4_nilAddress(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	// nil address — mkIDRecord4 calls addr.To4() which returns nil for nil input
	// The record is still created, but A field is nil
	record := mkIDRecord4(cfg, pid, nil)

	assert.Equal(t, uint16(dns.TypeA), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(86400), record.Hdr.Ttl)
	assert.Nil(t, record.A, "A record with nil address should have nil A field")
	assert.Contains(t, record.Hdr.Name, "hs0.hyprspace.")
}

func Test_mkIDRecord6_noService(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addr := net.ParseIP("fd00::1")
	record := mkIDRecord6(cfg, pid, "", addr)

	assert.Equal(t, uint16(dns.TypeAAAA), record.Hdr.Rrtype)
	assert.Equal(t, uint16(dns.ClassINET), record.Hdr.Class)
	assert.Equal(t, uint32(86400), record.Hdr.Ttl)
	assert.Equal(t, addr.To16(), record.AAAA)
	assert.Contains(t, record.Hdr.Name, "hs0.hyprspace.")
}

func Test_mkIDRecord6_withService(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	// Create a service-scoped address
	svcAddr := config.MkServiceAddr6(pid, "http")
	record := mkIDRecord6(cfg, pid, "http", svcAddr)

	assert.Equal(t, uint16(dns.TypeAAAA), record.Hdr.Rrtype)
	assert.Contains(t, record.Hdr.Name, "http.")
	assert.Contains(t, record.Hdr.Name, "hs0.hyprspace.")
	assert.Equal(t, svcAddr.To16(), record.AAAA)
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

func Test_writeResponse_UnknownType(t *testing.T) {
	msg := new(dns.Msg)
	q := dns.Question{Name: "test.", Qtype: dns.TypeMX}
	addr := net.ParseIP("100.64.1.2")

	// writeResponse is a helper that always adds an A+TXT record regardless of qtype.
	// The server's switch statement handles qtype filtering before calling writeResponse,
	// so writeResponse itself is intentionally agnostic to qtype.
	writeResponse(msg, q, peer.ID("dummy"), addr)

	require.Len(t, msg.Answer, 1, "writeResponse always produces an A record regardless of qtype")
	aRecord, ok := msg.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "test.", aRecord.Hdr.Name)
	assert.Equal(t, uint32(0), aRecord.Hdr.Ttl)
}
