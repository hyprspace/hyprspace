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
	t.Run("exact match", func(t *testing.T) {
		peers := makeTestPeers(t)
		target, found := FindPeer(peers, peers[1].ID)
		require.True(t, found)
		assert.Equal(t, peers[1].ID, target.ID)
		assert.Equal(t, "peer1", target.Name)
	})
	t.Run("no match", func(t *testing.T) {
		peers := makeTestPeers(t)
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)

		_, found := FindPeer(peers, pid)
		assert.False(t, found)
	})
}

func Test_FindPeerByName(t *testing.T) {
	t.Run("found one", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice", "bob", "charlie")
		target, found := FindPeerByName(peers, "bob")
		require.True(t, found)
		assert.Equal(t, "bob", target.Name)
	})
	t.Run("case mismatch", func(t *testing.T) {
		peers := makeNamedPeers(t, "Alice", "Bob")
		_, found := FindPeerByName(peers, "alice")
		assert.False(t, found, "should be case-sensitive")
	})
	t.Run("no match", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice")

		_, found := FindPeerByName(peers, "charlie")
		assert.False(t, found)
	})
	t.Run("empty name", func(t *testing.T) {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)

		peers := []Peer{{ID: pid, Name: ""}}
		target, found := FindPeerByName(peers, "")
		require.True(t, found)
		assert.Equal(t, "", target.Name)
	})
}

func Test_FindPeerByIDPrefix(t *testing.T) {
	t.Run("full id match", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice", "bob")
		target, err := FindPeerByIDPrefix(peers, peers[0].ID.String())
		require.NoError(t, err)
		require.NotNil(t, target)
		assert.Equal(t, peers[0].ID, target.ID)
	})

	t.Run("short prefix", func(t *testing.T) {
		// Use a single peer so any prefix is unambiguously unique.
		// (Two Ed25519 peer IDs always share the leading "12D3KooW".)
		peers := makeNamedPeers(t, "alice")
		shortPrefix := peers[0].ID.String()[:5]
		target, err := FindPeerByIDPrefix(peers, shortPrefix)
		require.NoError(t, err)
		require.NotNil(t, target)
		assert.Equal(t, peers[0].ID, target.ID)
	})

	t.Run("collision", func(t *testing.T) {
		// Generate two real peer IDs and use a prefix that matches both.
		// Ambiguous prefixes must return an error rather than silently
		// resolving to the first match.
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
		commonPrefix := pid1.String()[:6]
		require.Equal(t, pid2.String()[:6], commonPrefix, "both Ed25519 peer IDs should share the first 6 chars")

		target, err := FindPeerByIDPrefix(peers, commonPrefix)
		assert.Error(t, err, "ambiguous prefix should return an error")
		assert.Nil(t, target, "ambiguous prefix should not return a peer")
	})

	t.Run("no match", func(t *testing.T) {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)

		peers := []Peer{{ID: pid, Name: "test"}}

		target, err := FindPeerByIDPrefix(peers, "12D3KooZ")
		assert.NoError(t, err)
		assert.Nil(t, target)
	})
}

func Test_FindPeerByCLIRef(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice", "bob")
		target, err := FindPeerByCLIRef(peers, "@alice")
		require.NoError(t, err)
		require.NotNil(t, target)
		assert.Equal(t, "alice", target.Name)
	})

	t.Run("name not found", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice")
		target, err := FindPeerByCLIRef(peers, "@charlie")
		assert.NoError(t, err)
		assert.Nil(t, target)
	})

	t.Run("id prefix", func(t *testing.T) {
		// Single-peer slice so the short prefix is unambiguous.
		peers := makeNamedPeers(t, "alice")
		target, err := FindPeerByCLIRef(peers, peers[0].ID.String()[:4])
		require.NoError(t, err)
		require.NotNil(t, target)
		assert.Equal(t, peers[0].ID, target.ID)
	})

	t.Run("id prefix collision", func(t *testing.T) {
		// Ambiguous ID prefixes must propagate as an error through FindPeerByCLIRef.
		peers := makeNamedPeers(t, "alice", "bob")
		commonPrefix := peers[0].ID.String()[:6] // shared "12D3Ko" for Ed25519
		target, err := FindPeerByCLIRef(peers, commonPrefix)
		assert.Error(t, err, "ambiguous prefix should propagate from FindPeerByIDPrefix")
		assert.Nil(t, target)
	})

	t.Run("empty", func(t *testing.T) {
		pk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
		require.NoError(t, err)
		pid, err := peer.IDFromPrivateKey(pk)
		require.NoError(t, err)

		peers := []Peer{{ID: pid, Name: "test"}}
		// Empty string should not match any peer and is not an error.
		target, err := FindPeerByCLIRef(peers, "")
		assert.NoError(t, err)
		assert.Nil(t, target, "empty string should not match any peer")
	})

	t.Run("bare string no match", func(t *testing.T) {
		peers := makeNamedPeers(t, "alice", "bob")
		// Bare string without @ and not matching any ID prefix
		target, err := FindPeerByCLIRef(peers, "random")
		assert.NoError(t, err)
		assert.Nil(t, target, "FindPeerByCLIRef('random') should return nil, nil")
	})
}
