package config

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestT1_MkNetID_Deterministic(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	result1 := MkNetID(pid)
	result2 := MkNetID(pid)

	assert.Equal(t, result1, result2, "MkNetID should return the same result for the same peer ID")
}

func TestT2_MkServiceID_Empty(t *testing.T) {
	result := MkServiceID("")

	expected := [2]byte{0xff, 0xfe}
	assert.Equal(t, expected, result, "MkServiceID with empty string should return magic bytes")
}

func TestT2_MkServiceID_SingleChar(t *testing.T) {
	result := MkServiceID("x")

	// Single char: id[0%2] ^= byte * 0 => id[0] ^= 0 => no change
	// Only i=0 is reached, so the first byte is never XORed
	expected := [2]byte{0xff, 0xfe}
	assert.Equal(t, expected, result, "Single-char service ID should equal empty (no XOR since i=0)")
}

func TestT2_MkServiceID_TwoChars(t *testing.T) {
	result := MkServiceID("ab")

	// "ab": id[0] ^= 'a' * 0 = 0, id[1] ^= 'b' * 1 = 'b'
	// id[0] stays 0xff, id[1] = 0xfe ^ 0x62 = 0x9c
	expected := [2]byte{0xff, 0x9c}
	assert.Equal(t, expected, result, "MkServiceID(\"ab\") should be [0xff, 0x9c]")
}

func TestT2_MkServiceID_NonCommutative(t *testing.T) {
	id1 := MkServiceID("ab")
	id2 := MkServiceID("ba")

	assert.NotEqual(t, id1, id2, "MkServiceID should be non-commutative")
	assert.Equal(t, [2]byte{0xff, 0x9c}, id1, "MkServiceID(\"ab\") should be [0xff, 0x9c]")
}

func TestT2_MkServiceID_Deterministic(t *testing.T) {
	result1 := MkServiceID("http-service")
	result2 := MkServiceID("http-service")

	assert.Equal(t, result1, result2, "MkServiceID should be deterministic")
}

func TestT3_MkBuiltinAddr4_Deterministic(t *testing.T) {
	 pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addr1 := mkBuiltinAddr4(pid)
	addr2 := mkBuiltinAddr4(pid)

	assert.Equal(t, addr1, addr2, "mkBuiltinAddr4 should be deterministic")
	assert.NotNil(t, addr1)
}

func TestT3_MkBuiltinAddr4_AllZeros(t *testing.T) {
	zeroPeer := peer.ID([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	result := mkBuiltinAddr4(zeroPeer)

	expected := []byte{100, 64, 1, 2}
	assert.Equal(t, expected, []byte(result.To4()), "mkBuiltinAddr4 with zero peer should return base address")
}

func TestT3_MkBuiltinAddr4_DifferentPeers(t *testing.T) {
	ids := make([]peer.ID, 20)
	for i := 0; i < 20; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		ids[i], err = peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
	}

	addrs := make(map[string]bool)
	for _, pid := range ids {
		addr := mkBuiltinAddr4(pid).To4()
		addrStr := addr.String()
		assert.False(t, addrs[addrStr], "mkBuiltinAddr4 collision for peer %s: %s", pid, addrStr)
		addrs[addrStr] = true
	}

	assert.Equal(t, 20, len(addrs), "All 20 peers should have unique IPv4 addresses")
}

func TestT3_MkBuiltinAddr4_VaryingLengths(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte{1, 2, 3}},
		{"odd", make([]byte, 7)},
		{"even", make([]byte, 8)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pid := peer.ID(tc.data)
			assert.NotPanics(t, func() {
				_ = mkBuiltinAddr4(pid)
			}, "mkBuiltinAddr4 should not panic on %s peer ID", tc.name)
		})
	}
}

func TestT3_MkBuiltinAddr4_StartsWith100_64(t *testing.T) {
	for i := 0; i < 10; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)

		addr := mkBuiltinAddr4(pid)
		assert.Equal(t, byte(100), addr[0], "IPv4 should start with 100 for peer %s", pid)
		assert.Equal(t, byte(64), addr[1], "IPv4 should start with 64 for peer %s", pid)
	}
}

func TestT4_MkBuiltinAddr6_Deterministic(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addr1 := mkBuiltinAddr6(pid)
	addr2 := mkBuiltinAddr6(pid)

	assert.Equal(t, addr1, addr2, "mkBuiltinAddr6 should be deterministic")
	assert.NotNil(t, addr1)
	assert.Equal(t, 16, len(addr1.To16()), "Should return a valid 16-byte IPv6 address")
}

func TestT4_MkBuiltinAddr6_NetIDConsistency(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	ipv6 := mkBuiltinAddr6(pid)
	netID := MkNetID(pid)

	assert.Equal(t, netID[0], ipv6[12], "IPv6 byte 12 should match netID[0]")
	assert.Equal(t, netID[1], ipv6[13], "IPv6 byte 13 should match netID[1]")
	assert.Equal(t, netID[2], ipv6[14], "IPv6 byte 14 should match netID[2]")
	assert.Equal(t, netID[3], ipv6[15], "IPv6 byte 15 should match netID[3]")
}

func TestT4_MkBuiltinAddr6_Prefix(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	ipv6 := mkBuiltinAddr6(pid)

	expectedPrefix := []byte{0xfd, 0x00, 'h', 'y', 'p', 'r', 's', 'p', 'a', 'c', 'e', 0x00}
	assert.Equal(t, expectedPrefix, []byte(ipv6[:12]), "IPv6 should have fixed prefix in first 12 bytes")
}

func TestT4_MkBuiltinAddr6_DifferentPeers(t *testing.T) {
	ids := make([]peer.ID, 10)
	for i := 0; i < 10; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		ids[i], err = peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
	}

	addrs := make(map[string]bool)
	for _, pid := range ids {
		addr := mkBuiltinAddr6(pid).To16()
		addrStr := addr.String()
		assert.False(t, addrs[addrStr], "mkBuiltinAddr6 collision for peer %s: %s", pid, addrStr)
		addrs[addrStr] = true
	}

	assert.Equal(t, 10, len(addrs), "All 10 peers should have unique IPv6 addresses")
}

func TestT5_MkServiceAddr6_Deterministic(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addr1 := MkServiceAddr6(pid, "http")
	addr2 := MkServiceAddr6(pid, "http")

	assert.Equal(t, addr1, addr2, "MkServiceAddr6 should be deterministic")
}

func TestT5_MkServiceAddr6_DifferentServices(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addrHTTP := MkServiceAddr6(pid, "http")
	addrSSH := MkServiceAddr6(pid, "ssh")

	assert.NotEqual(t, addrHTTP, addrSSH, "Different services should produce different addresses")
}

func TestT5_MkServiceAddr6_DifferentPeers(t *testing.T) {
	ids := make([]peer.ID, 5)
	for i := 0; i < 5; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		ids[i], err = peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
	}

	addrs := make(map[string]bool)
	for _, pid := range ids {
		addr := MkServiceAddr6(pid, "http")
		addrStr := addr.String()
		assert.False(t, addrs[addrStr], "MkServiceAddr6 collision for peer %s: %s", pid, addrStr)
		addrs[addrStr] = true
	}

	assert.Equal(t, 5, len(addrs), "All 5 peers should have unique service addresses")
}

func TestT5_MkServiceAddr6_CollisionResistance(t *testing.T) {
	ids := make([]peer.ID, 10)
	for i := 0; i < 10; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		ids[i], err = peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
	}
	services := []string{"http", "ssh", "grpc"}

	addrs := make(map[string]bool)
	for _, pid := range ids {
		for _, svc := range services {
			addr := MkServiceAddr6(pid, svc).String()
			assert.False(t, addrs[addr], "Collision: peer %s with service %s: %s", pid, svc, addr)
			addrs[addr] = true
		}
	}

	assert.Equal(t, 30, len(addrs), "All 30 addresses should be unique")
}

func TestT5_MkServiceAddr6_NetIDAndServiceByteLayout(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addr := MkServiceAddr6(pid, "http")
	netID := MkNetID(pid)
	svcID := MkServiceID("http")

	assert.Equal(t, netID[0], addr[10], "Address byte 10 should match netID[0]")
	assert.Equal(t, netID[1], addr[11], "Address byte 11 should match netID[1]")
	assert.Equal(t, netID[2], addr[12], "Address byte 12 should match netID[2]")
	assert.Equal(t, netID[3], addr[13], "Address byte 13 should match netID[3]")
	assert.Equal(t, svcID[0], addr[14], "Address byte 14 should match svcID[0]")
	assert.Equal(t, svcID[1], addr[15], "Address byte 15 should match svcID[1]")
}
