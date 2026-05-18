package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestT18_RouteActionConstants(t *testing.T) {
	assert.Equal(t, RouteAction("show"), Show)
	assert.Equal(t, RouteAction("add"), Add)
	assert.Equal(t, RouteAction("del"), Del)
}

func TestT18_RouteActionConstants_StringValues(t *testing.T) {
	assert.Equal(t, "show", string(Show))
	assert.Equal(t, "add", string(Add))
	assert.Equal(t, "del", string(Del))
}

func TestT18_RouteActionConstants_AreDistinct(t *testing.T) {
	assert.NotEqual(t, Show, Add)
	assert.NotEqual(t, Add, Del)
	assert.NotEqual(t, Show, Del)
}

func TestT19_StatusReply_ZeroValue(t *testing.T) {
	reply := StatusReply{}
	assert.Equal(t, "", reply.PeerID)
	assert.Equal(t, 0, reply.SwarmPeersCurrent)
	assert.Equal(t, 0, reply.NetPeersCurrent)
	assert.Nil(t, reply.NetPeerAddrsCurrent)
	assert.Equal(t, 0, reply.NetPeersMax)
	assert.Nil(t, reply.ListenAddrs)
}

func TestT19_StatusReply_Populated(t *testing.T) {
	reply := StatusReply{
		PeerID:              "test-peer",
		SwarmPeersCurrent:   3,
		NetPeersCurrent:     2,
		NetPeerAddrsCurrent: []string{"@node1 (10ms) /ip4/1.2.3.4/p2p/node1"},
		NetPeersMax:         5,
		ListenAddrs:         []string{"/ip4/0.0.0.0/tcp/4001"},
	}

	assert.Equal(t, "test-peer", reply.PeerID)
	assert.Equal(t, 3, reply.SwarmPeersCurrent)
	assert.Equal(t, 2, reply.NetPeersCurrent)
	assert.Len(t, reply.NetPeerAddrsCurrent, 1)
	assert.Equal(t, "@node1 (10ms) /ip4/1.2.3.4/p2p/node1", reply.NetPeerAddrsCurrent[0])
	assert.Equal(t, 5, reply.NetPeersMax)
	assert.Len(t, reply.ListenAddrs, 1)
	assert.Equal(t, "/ip4/0.0.0.0/tcp/4001", reply.ListenAddrs[0])
}

func TestT20_PeersReply_ZeroValue(t *testing.T) {
	reply := PeersReply{}
	assert.Nil(t, reply.PeerAddrs)
}

func TestT20_PeersReply_Populated(t *testing.T) {
	reply := PeersReply{
		PeerAddrs: []string{"/ip4/1.2.3.4/p2p/abc", "/ip4/5.6.7.8/p2p/def"},
	}

	assert.Len(t, reply.PeerAddrs, 2)
	assert.Contains(t, reply.PeerAddrs, "/ip4/1.2.3.4/p2p/abc")
	assert.Contains(t, reply.PeerAddrs, "/ip4/5.6.7.8/p2p/def")
}

func TestT21_Args_ZeroValue(t *testing.T) {
	args := Args{}
	// Args is an empty struct — verify zero value creation
	assert.Equal(t, Args{}, args)
}

func TestT21_RouteArgs(t *testing.T) {
	routeArgs := RouteArgs{
		Action: Show,
		Args:   []string{"show"},
	}
	assert.Equal(t, Show, routeArgs.Action)
	assert.Len(t, routeArgs.Args, 1)
	assert.Equal(t, "show", routeArgs.Args[0])
}
