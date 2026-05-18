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
