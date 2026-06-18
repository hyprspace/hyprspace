package config

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_MkServiceID_NonCommutative(t *testing.T) {
	id1 := MkServiceID("ab")
	id2 := MkServiceID("ba")

	assert.NotEqual(t, id1, id2, "MkServiceID should be non-commutative")
	assert.Equal(t, [2]byte{0xff, 0x9c}, id1, "MkServiceID(\"ab\") should be [0xff, 0x9c]")
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
