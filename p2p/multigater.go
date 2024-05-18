package p2p

import (
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// MultiGater combines a list of ConnectionGaters. All gaters must return true to allow the connection.
type MultiGater struct {
	gaters []connmgr.ConnectionGater
}

func NewMultiGater(gaters ...connmgr.ConnectionGater) connmgr.ConnectionGater {
	return MultiGater{
		gaters: gaters,
	}
}

func (m MultiGater) InterceptAccept(addrs network.ConnMultiaddrs) (allow bool) {
	for _, g := range m.gaters {
		if !g.InterceptAccept(addrs) {
			return false
		}
	}
	return true
}

func (m MultiGater) InterceptAddrDial(p peer.ID, addr multiaddr.Multiaddr) (allow bool) {
	for _, g := range m.gaters {
		if !g.InterceptAddrDial(p, addr) {
			return false
		}
	}
	return true
}

func (m MultiGater) InterceptPeerDial(p peer.ID) (allow bool) {
	for _, g := range m.gaters {
		if !g.InterceptPeerDial(p) {
			return false
		}
	}
	return true
}

func (m MultiGater) InterceptSecured(d network.Direction, p peer.ID, addrs network.ConnMultiaddrs) (allow bool) {
	for _, g := range m.gaters {
		if !g.InterceptSecured(d, p, addrs) {
			return false
		}
	}
	return true
}

func (m MultiGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	for _, g := range m.gaters {
		if ok, reason := g.InterceptUpgraded(conn); !ok {
			return false, reason
		}
	}
	return true, 0
}
