package p2p

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
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

type RecursionGater struct {
	config  *config.Config
	ifindex int
}

func NewRecursionGater(config *config.Config) RecursionGater {
	iface, err := net.InterfaceByName(config.Interface)
	if err != nil {
		panic(err)
	}
	return RecursionGater{
		config:  config,
		ifindex: iface.Index,
	}
}

func (rg RecursionGater) InterceptAddrDial(pid peer.ID, addr ma.Multiaddr) bool {
	if ip4str, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		ip4 := net.ParseIP(ip4str)
		if rte, ok := rg.config.FindRouteForIP(ip4); ok {
			if rte.Target.ID == pid {
				routeIfindex := getRouteInterface(ip4)
				if routeIfindex == rg.ifindex {
					return false
				}
			}
		}
	}
	return true
}

func (rg RecursionGater) InterceptPeerDial(pid peer.ID) bool {
	return true
}

func (rg RecursionGater) InterceptAccept(addrs network.ConnMultiaddrs) bool {
	return true
}

func (rg RecursionGater) InterceptSecured(direction network.Direction, pid peer.ID, addrs network.ConnMultiaddrs) bool {
	return true
}

func (rg RecursionGater) InterceptUpgraded(network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}
