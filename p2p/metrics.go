package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

func RouteMetricsService(ctx context.Context, wg *sync.WaitGroup, host host.Host, cfg *config.Config) {
	subCon, err := host.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		logger.With(err).Fatal("Failed to subscribe eventbus")
	}
	logger.Debug("Route metrics service ready")
	wg.Add(1)
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-subCon.Out():
			evt := ev.(event.EvtPeerConnectednessChanged)
			_, found := config.FindPeer(cfg.Peers, evt.Peer)
			if found {
				if evt.Connectedness == network.Connected {
					ctx2, cancel := context.WithDeadline(ctx, time.Now().Add(30*time.Second))
					ch := ping.Ping(ctx2, host, evt.Peer)
					go func() {
						t := time.NewTimer(15 * time.Second)
						for {
							select {
							case <-t.C:
								cancel()
							case <-ctx2.Done():
								return
							case res := <-ch:
								if res.Error == nil {
									host.Peerstore().RecordLatency(evt.Peer, res.RTT)
								}
								time.Sleep(5 * time.Second)
							}
						}
					}()
					// wait a little before spawning another ping goroutine
					time.Sleep(1 * time.Second)
				}
			}
		}
	}
}
