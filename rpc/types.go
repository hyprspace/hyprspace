package rpc

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
