package rpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

type HyprspaceRPC struct {
	host   host.Host
	config config.Config
}

func (hsr *HyprspaceRPC) Status(args *Args, reply *StatusReply) error {
	netPeersCurrent := 0
	var netPeerAddrsCurrent []string
	for _, p := range hsr.config.Peers {
		if hsr.host.Network().Connectedness(p.ID) == network.Connected {
			netPeersCurrent = netPeersCurrent + 1
			for _, c := range hsr.host.Network().ConnsToPeer(p.ID) {
				netPeerAddrsCurrent = append(netPeerAddrsCurrent, fmt.Sprintf("%s/p2p/%s", c.RemoteMultiaddr().String(), p.ID.String()))
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

func (hsr *HyprspaceRPC) Peers(args *Args, reply *PeersReply) error {
	var peerAddrs []string
	for _, c := range hsr.host.Network().Conns() {
		peerAddrs = append(peerAddrs, fmt.Sprintf("%s/p2p/%s", c.RemoteMultiaddr().String(), c.RemotePeer().String()))
	}
	*reply = PeersReply{peerAddrs}
	return nil
}

func RpcServer(ctx context.Context, ma multiaddr.Multiaddr, host host.Host, config config.Config) {
	hsr := HyprspaceRPC{host, config}
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
