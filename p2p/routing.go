package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

type ParallelRouting struct {
	routings []routedhost.Routing
}

func (pr ParallelRouting) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	var wg sync.WaitGroup
	var mutex sync.Mutex

	var info peer.AddrInfo
	info.ID = p
	subCtx, cancelSubCtx := context.WithTimeout(ctx, 30*time.Second)
	for _, r := range pr.routings {
		wg.Add(1)
		r2 := r
		go func() {
			defer wg.Done()
			ai, err := r2.FindPeer(subCtx, p)
			if err == nil {
				mutex.Lock()
				defer mutex.Unlock()
				info.Addrs = append(info.Addrs, ai.Addrs...)
				// give the other routings a short time period to find a better address
				time.AfterFunc(500*time.Millisecond, cancelSubCtx)
			}
		}()
	}

	wg.Wait()
	cancelSubCtx()
	return info, nil
}
