package config

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MkNetID_edgeCases(t *testing.T) {
	t.Run("all zeros", func(t *testing.T) {
		// All-zero bytes: XOR with 0 leaves magic bytes unchanged
		zeroPeer := peer.ID([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		expected := [4]byte{0xde, 0xad, 0xbe, 0xef}
		result := MkNetID(zeroPeer)
		assert.Equal(t, expected, result, "MkNetID with all-zero peer should return magic bytes")
	})
	t.Run("all ff", func(t *testing.T) {
		// MkNetID starts with [0xde, 0xad, 0xbe, 0xef], XORs each byte of peer ID
		// With exactly 4 bytes of 0xff:
		//   r[0] = 0xde ^ 0xff = 0x21
		//   r[1] = 0xad ^ 0xff = 0x52
		//   r[2] = 0xbe ^ 0xff = 0x41
		//   r[3] = 0xef ^ 0xff = 0x10
		ffPeer := peer.ID([]byte{0xff, 0xff, 0xff, 0xff})
		expected := [4]byte{0x21, 0x52, 0x41, 0x10}
		result := MkNetID(ffPeer)
		assert.Equal(t, expected, result, "MkNetID with all-0xFF bytes should XOR correctly")
	})
}

func Test_MkNetID_CollisionResistance(t *testing.T) {
	ids := make([]peer.ID, 20)
	for i := 0; i < 20; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		ids[i], err = peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
	}

	netIDs := make(map[[4]byte]bool)
	for _, pid := range ids {
		netID := MkNetID(pid)
		assert.False(t, netIDs[netID], "MkNetID collision for peer %s: %v", pid, netID)
		netIDs[netID] = true
	}

	assert.Equal(t, 20, len(netIDs), "All 20 MkNetID results should be unique")
}

func Test_MkServiceID_Empty(t *testing.T) {
	result := MkServiceID("")

	expected := [2]byte{0xff, 0xfe}
	assert.Equal(t, expected, result, "MkServiceID with empty string should return magic bytes")
}

func Test_MkServiceID_SingleChar(t *testing.T) {
	result := MkServiceID("x")

	// BUG: Single-character service names collide with the empty string.
	// MkServiceID computes `id[i%2] ^= b * byte(i)`, so for i=0 the contribution
	// is always zero and the first character has no effect on the resulting ID.
	// This is a known bug, not intended behavior. It is not fixed here because
	// changing MkServiceID is a wire-protocol breaking change (it would alter
	// every IPv6 service address and the svcId lookup keys in ServiceNetwork),
	// requiring a coordinated upgrade across all peers in a network. Single-char
	// service names are not used in practice, so the bug is harmless today.
	// This test pins the current (buggy) behavior so any accidental change to
	// MkServiceID is caught; fix it together with a protocol version bump.
	expected := [2]byte{0xff, 0xfe}
	assert.Equal(t, expected, result, "BUG: single-char service ID collides with empty string")
}

func Test_MkServiceID_NonCommutative(t *testing.T) {
	id1 := MkServiceID("ab")
	id2 := MkServiceID("ba")

	assert.NotEqual(t, id1, id2, "MkServiceID should be non-commutative")
	assert.Equal(t, [2]byte{0xff, 0x9c}, id1, "MkServiceID(\"ab\") should be [0xff, 0x9c]")
}

func Test_MkBuiltinAddr4_AllZeros(t *testing.T) {
	zeroPeer := peer.ID([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	result := mkBuiltinAddr4(zeroPeer)

	expected := []byte{100, 64, 1, 2}
	assert.Equal(t, expected, []byte(result.To4()), "mkBuiltinAddr4 with zero peer should return base address")
}

func Test_MkBuiltinAddr6_DifferentPeers(t *testing.T) {
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

func Test_MkServiceAddr6_DifferentServices(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	addrHTTP := MkServiceAddr6(pid, "http")
	addrSSH := MkServiceAddr6(pid, "ssh")

	assert.NotEqual(t, addrHTTP, addrSSH, "Different services should produce different addresses")
}

func Test_MkServiceAddr6_CollisionResistance(t *testing.T) {
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

func Test_MkServiceAddr6_NetIDAndServiceByteLayout(t *testing.T) {
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
