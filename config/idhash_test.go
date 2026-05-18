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
