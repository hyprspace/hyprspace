// Package mobile provides a gomobile-compatible facade for running a Hyprspace
// node on Android. It bridges Android's VpnService TUN file descriptor with the
// Hyprspace libp2p networking layer.
//
// Build with:
//
//	gomobile bind -javapkg hyprspace -target=android -androidapi 26 -o hyprspace.aar ./mobile
package mobile

import (
	"context"
	"encoding/binary"
	"errors"
	"io/fs"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/hyprspace/hyprspace/p2p"
	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-cidranger"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

var logger = log.Logger("hyprspace/mobile")

// VPNConfig contains the network parameters that Android's VpnService.Builder
// needs before calling establish() to create the TUN device.
type VPNConfig struct {
	// Address4 is the IPv4 address with prefix, e.g. "100.64.1.2/32".
	Address4 string
	// Address6 is the IPv6 address with prefix, e.g. "fd00:...:abcd/128".
	Address6 string
	// MTU for the tunnel interface.
	MTU int
	// Routes is a newline-separated list of CIDRs to route through the VPN.
	Routes string
}

// GetVPNConfig reads the hyprspace configuration and returns the network
// parameters needed to configure Android's VpnService.Builder.
// Call this before StartNode to know what addresses/routes to set up.
func GetVPNConfig(configPath string) (*VPNConfig, error) {
	cfg, err := config.Read(configPath)
	if err != nil {
		return nil, err
	}

	allRoutes4, err := cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		return nil, err
	}
	allRoutes6, err := cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv6)
	if err != nil {
		return nil, err
	}

	var routes []string
	for _, r := range allRoutes4 {
		n := r.Network()
		routes = append(routes, n.String())
	}
	for _, r := range allRoutes6 {
		n := r.Network()
		routes = append(routes, n.String())
	}

	return &VPNConfig{
		Address4: cfg.BuiltinAddr4.String() + "/32",
		Address6: cfg.BuiltinAddr6.String() + "/128",
		MTU:      1420,
		Routes:   strings.Join(routes, "\n"),
	}, nil
}

// Node is a running Hyprspace instance on Android.
type Node struct {
	cfg           *config.Config
	host          host.Host
	dht           *dht.IpfsDHT
	tunFile       *os.File
	activeStreams map[peer.ID]*sharedStream
	streamsMu     sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	events        Events
	stateMu       sync.Mutex
	lastState     string
	lastConnected int
}

// Events receives node lifecycle and health notifications. It is implemented on
// the Android side and passed to StartNode.
//
// Methods may be called from arbitrary goroutines (libp2p network
// notifications, the TUN read loop, etc.), so the implementation must be
// thread-safe and MUST NOT block — post to a Handler or update a thread-safe
// holder such as a StateFlow. Do not call back into Go synchronously from these
// callbacks.
type Events interface {
	// OnStateChange reports a lifecycle transition. state is one of:
	//   "running"   – node started; no configured peer connected yet
	//   "connected" – at least one configured peer is reachable
	//   "stopped"   – clean shutdown
	//   "error"     – fatal; detail holds the message; the node is down
	OnStateChange(state string, detail string)

	// OnPeerCountChange reports how many configured peers are currently
	// connected, out of the total configured.
	OnPeerCountChange(connected int, total int)
}

// emit reports a lifecycle state change, deduplicating repeated states so the
// Android side is not spammed. The Events callback is invoked without holding
// stateMu to avoid deadlocks if the implementation re-enters.
func (n *Node) emit(state, detail string) {
	if n.events == nil {
		return
	}
	n.stateMu.Lock()
	if state == n.lastState {
		n.stateMu.Unlock()
		return
	}
	n.lastState = state
	n.stateMu.Unlock()
	n.events.OnStateChange(state, detail)
}

// emitPeers reports a change in the number of connected configured peers,
// deduplicating identical counts.
func (n *Node) emitPeers(connected, total int) {
	if n.events == nil {
		return
	}
	n.stateMu.Lock()
	if connected == n.lastConnected {
		n.stateMu.Unlock()
		return
	}
	n.lastConnected = connected
	n.stateMu.Unlock()
	n.events.OnPeerCountChange(connected, total)
}

// registerHealth subscribes to libp2p connection events and reports how many
// configured peers are currently reachable, transitioning between the
// "running" (no peers) and "connected" (>=1 peer) states.
func (n *Node) registerHealth() {
	update := func() {
		connected := 0
		for _, p := range n.cfg.Peers {
			if n.host.Network().Connectedness(p.ID) == network.Connected {
				connected++
			}
		}
		n.emitPeers(connected, len(n.cfg.Peers))
		if connected > 0 {
			n.emit("connected", "")
		} else {
			n.emit("running", "")
		}
	}
	n.host.Network().Notify(&network.NotifyBundle{
		ConnectedF:    func(network.Network, network.Conn) { update() },
		DisconnectedF: func(network.Network, network.Conn) { update() },
	})
}

type sharedStream struct {
	stream network.Stream
	mu     sync.Mutex
}

// StartNode starts a Hyprspace node using a TUN file descriptor from Android's
// VpnService. The fd must be obtained via ParcelFileDescriptor.detachFd() after
// VpnService.Builder.establish(). configPath is the path to the hyprspace JSON
// config file on the Android filesystem. events receives lifecycle and health
// notifications and may be nil.
// For Android: Call `pfd.detachFd()` and pass that int; never close the `ParcelFileDescriptor`
// yourself. (it would double-close)
func StartNode(fd int, configPath string, events Events) (*Node, error) {
	log.SetLogLevel("hyprspace", "info")
	log.SetLogLevelRegex("^hyprspace/", "info")

	cfg, err := config.Read(configPath)
	if err != nil {
		return nil, err
	}
	cfg.Interface = "hyprspace"

	tunFile := os.NewFile(uintptr(fd), "vpn-tun")
	if tunFile == nil {
		return nil, errors.New("invalid file descriptor")
	}

	ctx, cancel := context.WithCancel(context.Background())

	n := &Node{
		cfg:           cfg,
		tunFile:       tunFile,
		activeStreams: make(map[peer.ID]*sharedStream),
		ctx:           ctx,
		cancel:        cancel,
		events:        events,
	}

	// Use passthrough gater since Android VpnService handles routing protection
	// and netlink is unavailable.
	var gater connmgr.ConnectionGater = passthroughGater{}

	n.host, n.dht, err = p2p.CreateNode(
		ctx,
		cfg.PrivateKey,
		cfg.ListenAddresses,
		n.streamHandler,
		p2p.NewClosedCircuitRelayFilter(cfg.Peers),
		gater,
		cfg.Peers,
	)
	if err != nil {
		cancel()
		tunFile.Close()
		return nil, err
	}

	n.host.SetStreamHandler(p2p.PeXProtocol, p2p.NewPeXStreamHandler(n.host, cfg))

	for _, p := range cfg.Peers {
		n.host.ConnManager().Protect(p.ID, "/hyprspace/peer")
	}

	go p2p.Discover(ctx, &n.wg, n.host, n.dht, cfg.Peers)
	go p2p.PeXService(ctx, &n.wg, n.host, cfg)
	go p2p.RouteMetricsService(ctx, &n.wg, n.host, cfg)

	go n.readLoop()

	n.registerHealth()
	n.emit("running", "")

	logger.Info("Mobile node started")
	return n, nil
}

// Stop gracefully shuts down the Hyprspace node.
func (n *Node) Stop() error {
	logger.Info("Stopping mobile node")
	n.cancel()
	err := n.host.Close()
	n.tunFile.Close()
	n.wg.Wait()
	n.emit("stopped", "")
	return err
}

// Rebootstrap forces a DHT refresh and peer rediscovery.
func (n *Node) Rebootstrap() {
	n.host.ConnManager().TrimOpenConns(context.Background())
	<-n.dht.ForceRefresh()
	p2p.Rediscover()
}

// readLoop reads IP packets from the TUN fd and dispatches them to peers.
func (n *Node) readLoop() {
	for {
		packet := make([]byte, 1420)
		plen, err := n.tunFile.Read(packet)
		if err != nil {
			if errors.Is(err, fs.ErrClosed) || errors.Is(err, os.ErrClosed) {
				logger.Warn("TUN closed, stopping read loop")
				// A close that did not originate from Stop() means the tunnel
				// died on its own; surface it as a fatal error.
				if n.ctx.Err() == nil {
					n.emit("error", "tunnel closed unexpectedly")
				}
				return
			}
			if n.ctx.Err() != nil {
				return
			}
			logger.With(zap.Error(err)).Error("Failed to read from TUN")
			continue
		}

		var dstIP net.IP
		proto := packet[0] & 0xf0

		if proto == 0x40 {
			dstIP = net.IP(packet[16:20])
			if n.cfg.BuiltinAddr4.Equal(dstIP) {
				continue
			}
		} else if proto == 0x60 {
			dstIP = net.IP(packet[24:40])
			if n.cfg.BuiltinAddr6.Equal(dstIP) {
				continue
			}
		} else {
			continue
		}

		route, found := n.cfg.FindRouteForIP(dstIP)
		if found {
			go n.sendPacket(route.Target.ID, packet, plen)
		}
	}
}

// streamHandler handles incoming packets from peers and writes them to the TUN.
func (n *Node) streamHandler(stream network.Stream) {
	if _, ok := config.FindPeer(n.cfg.Peers, stream.Conn().RemotePeer()); !ok {
		stream.Reset()
		return
	}
	packet := make([]byte, 1420)
	packetSize := make([]byte, 2)
	for {
		_, err := stream.Read(packetSize)
		if err != nil {
			stream.Close()
			return
		}

		size := binary.LittleEndian.Uint16(packetSize)

		var plen uint16
		for plen < size {
			tmp, err := stream.Read(packet[plen:size])
			plen += uint16(tmp)
			if err != nil {
				stream.Close()
				return
			}
		}
		_ = stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
		_, _ = n.tunFile.Write(packet[:size])
	}
}

// sendPacket sends a packet to a peer, reusing existing streams when possible.
func (n *Node) sendPacket(dst peer.ID, packet []byte, plen int) {
	n.streamsMu.Lock()
	ss, ok := n.activeStreams[dst]
	n.streamsMu.Unlock()

	if ok {
		ss.mu.Lock()
		err := binary.Write(ss.stream, binary.LittleEndian, uint16(plen))
		if err == nil {
			_, err = ss.stream.Write(packet[:plen])
			if err == nil {
				err = ss.stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
			}
		}
		ss.mu.Unlock()

		if err == nil {
			return
		}
		ss.stream.Close()
		n.streamsMu.Lock()
		delete(n.activeStreams, dst)
		n.streamsMu.Unlock()
	}

	stream, err := n.host.NewStream(n.ctx, dst, p2p.Protocol)
	if err != nil {
		logger.With(zap.String("dst", dst.String()), zap.Error(err)).Error("Failed to open stream")
		go p2p.Rediscover()
		return
	}
	_ = stream.SetWriteDeadline(time.Now().Add(25 * time.Second))

	err = binary.Write(stream, binary.LittleEndian, uint16(plen))
	if err != nil {
		stream.Close()
		return
	}
	_, err = stream.Write(packet[:plen])
	if err != nil {
		stream.Close()
		return
	}

	n.streamsMu.Lock()
	n.activeStreams[dst] = &sharedStream{stream: stream}
	n.streamsMu.Unlock()
}

// passthroughGater allows all connections. On Android, the VpnService routing
// prevents recursion (tunnel traffic going back into the VPN), so the
// netlink-based RecursionGater from desktop is unnecessary.
type passthroughGater struct{}

func (passthroughGater) InterceptPeerDial(_ peer.ID) bool { return true }
func (passthroughGater) InterceptAddrDial(_ peer.ID, _ ma.Multiaddr) bool {
	return true
}
func (passthroughGater) InterceptAccept(_ network.ConnMultiaddrs) bool { return true }
func (passthroughGater) InterceptSecured(_ network.Direction, _ peer.ID, _ network.ConnMultiaddrs) bool {
	return true
}
func (passthroughGater) InterceptUpgraded(_ network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}
