package config

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestPeers() []Peer {
	peers := make([]Peer, 3)
	for i := 0; i < 3; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(nil, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(nil, err)
		peers[i] = Peer{
			ID:   pid,
			Name: fmt.Sprintf("peer%d", i),
		}
	}
	return peers
}

func makeNamedPeers(names ...string) []Peer {
	peers := make([]Peer, len(names))
	for i, name := range names {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(nil, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(nil, err)
		peers[i] = Peer{ID: pid, Name: name}
	}
	return peers
}



func TestT14_FindPeer(t *testing.T) {
	peers := makeTestPeers()

	// Exact match: peer 1's ID
	target, found := FindPeer(peers, peers[1].ID)
	require.True(t, found)
	assert.Equal(t, peers[1].ID, target.ID)
	assert.Equal(t, "peer1", target.Name)
}

func TestT14_FindPeer_NoMatch(t *testing.T) {
	peers := makeTestPeers()
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	_, found := FindPeer(peers, pid)
	assert.False(t, found)
}

func TestT14_FindPeer_SinglePeer(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(nil, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(nil, err)

	peers := []Peer{{ID: pid, Name: "solo"}}
	target, found := FindPeer(peers, pid)
	require.True(t, found)
	assert.Equal(t, pid, target.ID)
}

func TestT14_FindPeer_EmptySlice(t *testing.T) {
	var peers []Peer
	_, found := FindPeer(peers, peer.ID("any"))
	assert.False(t, found)
}

func TestT14_FindPeer_NilSlice(t *testing.T) {
	var peers []Peer
	_, found := FindPeer(peers, peer.ID("any"))
	assert.False(t, found)
}

func TestT15_FindPeerByName(t *testing.T) {
	peers := makeNamedPeers("alice", "bob", "charlie")

	target, found := FindPeerByName(peers, "bob")
	require.True(t, found)
	assert.Equal(t, "bob", target.Name)
}

func TestT15_FindPeerByName_CaseMismatch(t *testing.T) {
	peers := makeNamedPeers("Alice", "Bob")

	_, found := FindPeerByName(peers, "alice")
	assert.False(t, found, "should be case-sensitive")
}

func TestT15_FindPeerByName_NoMatch(t *testing.T) {
	peers := makeNamedPeers("alice")

	_, found := FindPeerByName(peers, "charlie")
	assert.False(t, found)
}

func TestT15_FindPeerByName_EmptyName(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: ""}}
	target, found := FindPeerByName(peers, "")
	require.True(t, found)
	assert.Equal(t, "", target.Name)
}

func TestT16_FindPeerByIDPrefix(t *testing.T) {
	peers := makeNamedPeers("alice", "bob")

	// Full ID match
	target, found := FindPeerByIDPrefix(peers, peers[0].ID.String())
	require.True(t, found)
	assert.Equal(t, peers[0].ID, target.ID)
}

func TestT16_FindPeerByIDPrefix_ShortPrefix(t *testing.T) {
	peers := makeNamedPeers("alice", "bob")
	shortPrefix := peers[0].ID.String()[:5]

	target, found := FindPeerByIDPrefix(peers, shortPrefix)
	require.True(t, found)
	assert.Equal(t, peers[0].ID, target.ID)
}

func TestT16_FindPeerByIDPrefix_Collision(t *testing.T) {
	// Generate two peers whose IDs share a prefix
	pk1, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid1, err := peer.IDFromPrivateKey(pk1)
	require.NoError(t, err)

	pk2, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid2, err := peer.IDFromPrivateKey(pk2)
	require.NoError(t, err)

	peers := []Peer{{ID: pid1, Name: "first"}, {ID: pid2, Name: "second"}}

	// Short prefix likely matches first peer
	short := pid1.String()[:4]
	target, found := FindPeerByIDPrefix(peers, short)
	require.True(t, found)
	assert.Equal(t, "first", target.Name, "should return first match in slice")
}

func TestT16_FindPeerByIDPrefix_NoMatch(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: "test"}}

	_, found := FindPeerByIDPrefix(peers, "12D3KooZ")
	assert.False(t, found)
}
