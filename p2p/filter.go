package p2p

import (
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type FilterGater struct {
	Filters *multiaddr.Filters
}

func NewFilterGater() connmgr.ConnectionGater {
	return FilterGater{
		Filters: multiaddr.NewFilters(),
	}
}

func (f FilterGater) InterceptAccept(addrs network.ConnMultiaddrs) (allow bool) {
	return !f.Filters.AddrBlocked(addrs.RemoteMultiaddr())
}

func (f FilterGater) InterceptAddrDial(_ peer.ID, addr multiaddr.Multiaddr) (allow bool) {
	return !f.Filters.AddrBlocked(addr)
}

func (f FilterGater) InterceptPeerDial(peer.ID) (allow bool) {
	return true
}

func (f FilterGater) InterceptSecured(_ network.Direction, _ peer.ID, addrs network.ConnMultiaddrs) (allow bool) {
	return !f.Filters.AddrBlocked(addrs.RemoteMultiaddr())
}

func (f FilterGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}
