package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/ipfs/go-log/v2"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var discoverNow = make(chan bool)
var logger = log.Logger("hyprspace/p2p")

// Discover starts up a DHT based discovery system finding and adding nodes with the same rendezvous string.
func Discover(ctx context.Context, wg *sync.WaitGroup, h host.Host, dht *dht.IpfsDHT, peers []config.Peer) {
	dur := time.Second * 1
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoverNow:
			dur = time.Second * 3
			// Immediately trigger discovery
			ticker.Reset(time.Millisecond * 1)
		case <-ticker.C:
			connectedToAny := false
			for _, p := range peers {
				if h.Network().Connectedness(p.ID) != network.Connected {
					err := h.Connect(ctx, peer.AddrInfo{
						ID:    p.ID,
						Addrs: []multiaddr.Multiaddr{},
					})
					if err != nil {
						continue
					}
					connectedToAny = true
				} else {
					connectedToAny = true
				}
			}
			if !connectedToAny {
				logger.Debug("Not connected to any peers, attempting to bootstrap again")
				dht.Bootstrap(ctx)
				dht.RefreshRoutingTable()
				dur = time.Second * 10
				ticker.Reset(dur)
			} else {
				dur = min(dur*2, time.Minute)
				ticker.Reset(dur)
			}
		}
	}
}

func Rediscover() {
	discoverNow <- true
}
