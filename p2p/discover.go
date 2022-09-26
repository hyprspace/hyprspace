package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

// Discover starts up a DHT based discovery system finding and adding nodes with the same rendezvous string.
func Discover(ctx context.Context, h host.Host, dht *dht.IpfsDHT, peerTable map[string]peer.ID, discoverNow chan bool) {
	dur := time.Second * 1
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoverNow:
			dur = time.Second * 1
			// Immediately trigger discovery
			ticker.Reset(time.Millisecond * 1)
		case <-ticker.C:
			for nd, id := range peerTable {
				if h.Network().Connectedness(id) != network.Connected {
					addrs, err := dht.FindPeer(ctx, id)
					if err != nil {
						continue
					}
					_, err = h.Network().DialPeer(ctx, addrs.ID)
					if err != nil {
						continue
					}
					fmt.Println("[+] Connected to " + nd)
				}
			}
			dur = dur * 2
			if dur >= time.Second*60 {
				dur = time.Second * 60
			}
			ticker.Reset(dur)
		}
	}
}

func Rediscover(discoverNow chan bool) {
	discoverNow <- true
}
