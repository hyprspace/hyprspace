package p2p

import (
	"net"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Reroute struct {
	Network net.IPNet
	To      peer.ID
}

var (
	reroutes []Reroute
	mut      sync.Mutex
)

func findReroute(network net.IPNet, doDelete bool) (int, *Reroute, bool) {
	for i, r := range reroutes {
		bits1, _ := r.Network.Mask.Size()
		bits2, _ := network.Mask.Size()
		if r.Network.IP.Equal(network.IP) && bits1 == bits2 {
			if doDelete {
				reroutes = append(reroutes[:i], reroutes[i+1:]...)
			}
			return i, &r, true
		}
	}
	return 0, nil, false
}

func FindReroute(network net.IPNet, doDelete bool) (*Reroute, bool) {
	mut.Lock()
	defer mut.Unlock()
	_, i, r := findReroute(network, doDelete)
	return i, r
}

func AddReroute(network net.IPNet, peerID peer.ID) {
	mut.Lock()
	defer mut.Unlock()
	if i, _, found := findReroute(network, false); found {
		reroutes[i].To = peerID
	} else {
		reroutes = append(reroutes, Reroute{
			Network: network,
			To:      peerID,
		})
	}
}
