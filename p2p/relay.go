package p2p

import (
	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"
)

type ClosedCircuitRelayFilter struct {
	allowedPeers []config.Peer
}

func (ccr ClosedCircuitRelayFilter) AllowReserve(p peer.ID, a multiaddr.Multiaddr) bool {
	_, found := config.FindPeer(ccr.allowedPeers, p)
	return found
}

func (ccr ClosedCircuitRelayFilter) AllowConnect(src peer.ID, srcAddr multiaddr.Multiaddr, dest peer.ID) bool {
	_, foundSrc := config.FindPeer(ccr.allowedPeers, src)
	_, foundDst := config.FindPeer(ccr.allowedPeers, dest)
	return foundSrc && foundDst
}

func NewClosedCircuitRelayFilter(allowedPeers []config.Peer) relay.ACLFilter {
	return ClosedCircuitRelayFilter{
		allowedPeers: allowedPeers,
	}
}
