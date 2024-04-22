package rpc

import (
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Args struct {
}

type StatusReply struct {
	PeerID              string
	SwarmPeersCurrent   int
	NetPeersCurrent     int
	NetPeerAddrsCurrent []string
	NetPeersMax         int
	ListenAddrs         []string
}

type PeersReply struct {
	PeerAddrs []string
}

type RouteAction string

const (
	Show RouteAction = "show"
	Add              = "add"
	Del              = "del"
)

type RouteInfo struct {
	Network     net.IPNet
	TargetName  string
	TargetAddr  peer.ID
	RelayAddr   peer.ID
	IsRelay     bool
	IsConnected bool
}

type RouteArgs struct {
	Action RouteAction
	Args   []string
}

type RouteReply struct {
	Out    string
	Routes []RouteInfo
	Err    error
}
