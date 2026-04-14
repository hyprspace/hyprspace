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
	"github.com/vishvananda/netlink"
	"github.com/yl2chen/cidranger"
	"go.uber.org/zap"
)

// privateNetworks is a CIDR ranger containing IP ranges that should be
// filtered when FilterPrivateAddresses is enabled. This includes RFC1918
// private addresses, IPv4/IPv6 link-local, and loopback ranges.
var privateNetworks = initializePrivateNetworks()

func initializePrivateNetworks() cidranger.Ranger {
	ranger := cidranger.NewPCTrieRanger()
	privateNetworkCIDRs := []string{
		// RFC1918 private ranges
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		// IPv4 link-local
		"169.254.0.0/16",
		// IPv4 loopback
		"127.0.0.0/8",
		// IPv6 link-local
		"fe80::/10",
		// IPv6 loopback
		"::1/128",
	}

	for _, cidr := range privateNetworkCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid CIDR in privateNetworks: " + cidr)
		}
		if err := ranger.Insert(cidranger.NewBasicRangerEntry(*n)); err != nil {
			panic("failed to insert CIDR into privateNetworks: " + cidr)
		}
	}

	return ranger
}

// isPrivateIP returns true if the given IP falls within any of the
// private/link-local/loopback ranges defined in privateNetworks.
func isPrivateIP(ip net.IP) bool {
	contains, err := privateNetworks.Contains(ip)
	return err == nil && contains
}

// isPrivateMultiaddr returns true if the multiaddr contains an IP
// component (IPv4 or IPv6) that is a private/link-local/loopback address.
func isPrivateMultiaddr(addr ma.Multiaddr) bool {
	if ip4str, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		if ip := net.ParseIP(ip4str); ip != nil {
			return isPrivateIP(ip)
		}
	}
	if ip6str, err := addr.ValueForProtocol(ma.P_IP6); err == nil {
		if ip := net.ParseIP(ip6str); ip != nil {
			return isPrivateIP(ip)
		}
	}
	return false
}

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
	link, err := netlink.LinkByName(config.Interface)
	if err != nil {
		panic(err)
	}
	return RecursionGater{
		config:  config,
		ifindex: link.Attrs().Index,
	}
}

func (rg RecursionGater) InterceptAddrDial(pid peer.ID, addr ma.Multiaddr) bool {
	// When FilterPrivateAddresses is enabled, reject any dial attempt
	// targeting a private/link-local/loopback IP address. This prevents
	// the node from trying to connect to RFC1918 addresses advertised
	// by other peers, which in a corporate network would hit unrelated
	// hosts and potentially trigger security alerts.
	if rg.config.FilterPrivateAddresses && isPrivateMultiaddr(addr) {
		logger.With(zap.String("peer", pid.String()), zap.String("addr", addr.String())).Debug("Filtered dial to private address")
		return false
	}

	if ip4str, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		ip4 := net.ParseIP(ip4str)
		if rte, ok := rg.config.FindRouteForIP(ip4); ok {
			if rte.Target.ID == pid {
				routes, err := netlink.RouteGet(ip4)
				if err == nil {
					if len(routes) > 0 && routes[0].LinkIndex == rg.ifindex {
						return false
					}
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
