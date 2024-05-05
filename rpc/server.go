package rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"

	"github.com/hyprspace/hyprspace/config"
	"github.com/hyprspace/hyprspace/p2p"
	"github.com/hyprspace/hyprspace/tun"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/yl2chen/cidranger"
)

type HyprspaceRPC struct {
	host   host.Host
	config config.Config
	tunDev tun.TUN
}

func (hsr *HyprspaceRPC) Status(args *Args, reply *StatusReply) error {
	netPeersCurrent := 0
	var netPeerAddrsCurrent []string
	for _, p := range hsr.config.Peers {
		if hsr.host.Network().Connectedness(p.ID) == network.Connected {
			netPeersCurrent = netPeersCurrent + 1
			for _, c := range hsr.host.Network().ConnsToPeer(p.ID) {
				netPeerAddrsCurrent = append(netPeerAddrsCurrent, fmt.Sprintf("@%s (%s) %s/p2p/%s",
					p.Name,
					hsr.host.Peerstore().LatencyEWMA(p.ID).String(),
					c.RemoteMultiaddr().String(),
					p.ID.String(),
				))
			}
		}
	}
	var addrStrings []string
	for _, ma := range hsr.host.Addrs() {
		addrStrings = append(addrStrings, ma.String())
	}
	*reply = StatusReply{
		hsr.host.ID().String(),
		len(hsr.host.Network().Conns()),
		netPeersCurrent,
		netPeerAddrsCurrent,
		len(hsr.config.Peers),
		addrStrings,
	}
	return nil
}

func (hsr *HyprspaceRPC) Route(args *RouteArgs, reply *RouteReply) error {
	switch args.Action {
	case Show:
		var routeInfos []RouteInfo
		allRoutes4, err := hsr.config.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv4)
		if err != nil {
			return err
		}
		allRoutes6, err := hsr.config.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv6)
		if err != nil {
			return err
		}
		allRoutes := append(allRoutes4, allRoutes6...)
		for _, r := range allRoutes {
			rte := *r.(*config.RouteTableEntry)
			connected := hsr.host.Network().Connectedness(rte.Target.ID) == network.Connected
			relay := false
			relayAddr := rte.Target.ID
			if connected {
			ConnLoop:
				for _, c := range hsr.host.Network().ConnsToPeer(rte.Target.ID) {
					for _, s := range c.GetStreams() {
						if s.Protocol() == p2p.Protocol {
							if _, err := c.RemoteMultiaddr().ValueForProtocol(multiaddr.P_CIRCUIT); err == nil {
								relay = true
								if ra, err := c.RemoteMultiaddr().ValueForProtocol(multiaddr.P_P2P); err == nil {
									relayAddr, err = peer.Decode(ra)
									if err != nil {
										relayAddr = rte.Target.ID
									}
								}
							} else {
								relay = false
								relayAddr = rte.Target.ID
								break ConnLoop
							}
						}
					}
				}
			}
			routeInfos = append(routeInfos, RouteInfo{
				Network:     rte.Network(),
				TargetName:  rte.Target.Name,
				TargetAddr:  rte.Target.ID,
				RelayAddr:   relayAddr,
				IsRelay:     relay,
				IsConnected: connected,
			})
		}
		*reply = RouteReply{
			Routes: routeInfos,
		}
	case Add:
		if len(args.Args) != 2 {
			return errors.New("expected exactly 2 arguments")
		}
		_, network, err := net.ParseCIDR(args.Args[0])
		if err != nil {
			return err
		}
		target, found := config.FindPeerByCLIRef(hsr.config.Peers, args.Args[1])
		if !found {
			return errors.New("no such peer")
		}
		err = hsr.tunDev.Apply(tun.Route(*network))
		if err != nil {
			return err
		}

		hsr.config.PeerLookup.ByRoute.Insert(&config.RouteTableEntry{
			Net:    *network,
			Target: *target,
		})
	case Del:
		if len(args.Args) != 1 {
			return errors.New("expected exactly 1 argument")
		}
		_, network, err := net.ParseCIDR(args.Args[0])
		if err != nil {
			return err
		}

		err = hsr.tunDev.Apply(tun.RemoveRoute(*network))
		if err != nil {
			return err
		}

		_, err = hsr.config.PeerLookup.ByRoute.Remove(*network)
		if err != nil {
			_ = hsr.tunDev.Apply(tun.Route(*network))
			return err
		}
	default:
		return errors.New("no such action")
	}
	return nil
}

func (hsr *HyprspaceRPC) Peers(args *Args, reply *PeersReply) error {
	var peerAddrs []string
	for _, c := range hsr.host.Network().Conns() {
		peerAddrs = append(peerAddrs, fmt.Sprintf("%s/p2p/%s", c.RemoteMultiaddr().String(), c.RemotePeer().String()))
	}
	*reply = PeersReply{peerAddrs}
	return nil
}

func RpcServer(ctx context.Context, ma multiaddr.Multiaddr, host host.Host, config config.Config, tunDev tun.TUN) {
	hsr := HyprspaceRPC{host, config, tunDev}
	rpc.Register(&hsr)

	addr, err := ma.ValueForProtocol(multiaddr.P_UNIX)
	if err != nil {
		log.Fatal("[!] Failed to parse multiaddr: ", err)
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", addr)
	os.Chmod(addr, 0o0770)
	if err != nil {
		log.Fatal("[!] Failed to launch RPC server: ", err)
	}
	fmt.Println("[-] RPC server ready")
	go rpc.Accept(l)
	<-ctx.Done()
	fmt.Println("[-] Closing RPC server")
	l.Close()
}
