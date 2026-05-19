package config

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestPeers(t *testing.T) []Peer {
	peers := make([]Peer, 3)
	for i := 0; i < 3; i++ {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
		peers[i] = Peer{
			ID:   pid,
			Name: fmt.Sprintf("peer%d", i),
		}
	}
	return peers
}

func makeNamedPeers(t *testing.T, names ...string) []Peer {
	peers := make([]Peer, len(names))
	for i, name := range names {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)
		peers[i] = Peer{ID: pid, Name: name}
	}
	return peers
}



func Test_FindPeer(t *testing.T) {
	peers := makeTestPeers(t)

	// Exact match: peer 1's ID
	target, found := FindPeer(peers, peers[1].ID)
	require.True(t, found)
	assert.Equal(t, peers[1].ID, target.ID)
	assert.Equal(t, "peer1", target.Name)
}

func Test_FindPeer_NoMatch(t *testing.T) {
	peers := makeTestPeers(t)
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	_, found := FindPeer(peers, pid)
	assert.False(t, found)
}

func Test_FindPeer_SinglePeer(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: "solo"}}
	target, found := FindPeer(peers, pid)
	require.True(t, found)
	assert.Equal(t, pid, target.ID)
}

func Test_FindPeer_EmptySlice(t *testing.T) {
	// Non-nil empty slice — different from nil slice
	peers := []Peer{}
	_, found := FindPeer(peers, peer.ID("any"))
	assert.False(t, found)
}

func Test_FindPeer_NilSlice(t *testing.T) {
	// Nil slice — zero-value slice
	var peers []Peer
	_, found := FindPeer(peers, peer.ID("any"))
	assert.False(t, found)
}

func Test_FindPeerByName(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob", "charlie")

	target, found := FindPeerByName(peers, "bob")
	require.True(t, found)
	assert.Equal(t, "bob", target.Name)
}

func Test_FindPeerByName_CaseMismatch(t *testing.T) {
	peers := makeNamedPeers(t, "Alice", "Bob")

	_, found := FindPeerByName(peers, "alice")
	assert.False(t, found, "should be case-sensitive")
}

func Test_FindPeerByName_NoMatch(t *testing.T) {
	peers := makeNamedPeers(t, "alice")

	_, found := FindPeerByName(peers, "charlie")
	assert.False(t, found)
}

func Test_FindPeerByName_EmptyName(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: ""}}
	target, found := FindPeerByName(peers, "")
	require.True(t, found)
	assert.Equal(t, "", target.Name)
}

func Test_FindPeerByIDPrefix(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob")

	// Full ID match
	target, found := FindPeerByIDPrefix(peers, peers[0].ID.String())
	require.True(t, found)
	assert.Equal(t, peers[0].ID, target.ID)
}

func Test_FindPeerByIDPrefix_ShortPrefix(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob")
	shortPrefix := peers[0].ID.String()[:5]

	target, found := FindPeerByIDPrefix(peers, shortPrefix)
	require.True(t, found)
	assert.Equal(t, peers[0].ID, target.ID)
}

func Test_FindPeerByIDPrefix_Collision(t *testing.T) {
	// Generate two real peer IDs and use a prefix that matches both
	pk1, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid1, err := peer.IDFromPrivateKey(pk1)
	require.NoError(t, err)

	pk2, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid2, err := peer.IDFromPrivateKey(pk2)
	require.NoError(t, err)

	peers := []Peer{{ID: pid1, Name: "first"}, {ID: pid2, Name: "second"}}

	// All Ed25519 peer IDs start with "12D3KooW", so a 6-char prefix matches both
	// Verify this holds for both generated peers
	commonPrefix := pid1.String()[:6]
	assert.Equal(t, pid2.String()[:6], commonPrefix, "both Ed25519 peer IDs should share the first 6 chars")

	target, found := FindPeerByIDPrefix(peers, commonPrefix)
	require.True(t, found, "6-char prefix should match (collision)")
	assert.Equal(t, "first", target.Name, "should return first match in slice")
}

func Test_FindPeerByIDPrefix_NoMatch(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: "test"}}

	_, found := FindPeerByIDPrefix(peers, "12D3KooZ")
	assert.False(t, found)
}

func Test_FindPeerByCLIRef_Name(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob")

	target, found := FindPeerByCLIRef(peers, "@alice")
	require.True(t, found)
	assert.Equal(t, "alice", target.Name)
}

func Test_FindPeerByCLIRef_NameNotFound(t *testing.T) {
	peers := makeNamedPeers(t, "alice")

	_, found := FindPeerByCLIRef(peers, "@charlie")
	assert.False(t, found)
}

func Test_FindPeerByCLIRef_IDPrefix(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob")

	target, found := FindPeerByCLIRef(peers, peers[0].ID.String()[:4])
	require.True(t, found)
	assert.Equal(t, peers[0].ID, target.ID)
}

func Test_FindPeerByCLIRef_Empty(t *testing.T) {
	pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	peers := []Peer{{ID: pid, Name: "test"}}

	// Empty string should not match any peer — empty CLI ref is invalid
	_, found := FindPeerByCLIRef(peers, "")
	assert.False(t, found, "empty string should not match any peer")
}

func Test_FindPeerByCLIRef_BareStringNoMatch(t *testing.T) {
	peers := makeNamedPeers(t, "alice", "bob")

	// Bare string without @ and not matching any ID prefix
	_, found := FindPeerByCLIRef(peers, "random")
	assert.False(t, found, "FindPeerByCLIRef('random') should return nil, false")
}
