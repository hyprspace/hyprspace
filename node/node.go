package node

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hyprspace/hyprspace/config"
	hsdns "github.com/hyprspace/hyprspace/dns"
	"github.com/hyprspace/hyprspace/p2p"
	hsrpc "github.com/hyprspace/hyprspace/rpc"
	"github.com/hyprspace/hyprspace/svc"
	"github.com/hyprspace/hyprspace/tun"
	"github.com/libp2p/go-cidranger"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type SharedStream struct {
	Stream *network.Stream
	Lock   *sync.Mutex
}

type Node struct {
	cfg           *config.Config
	p2p           host.Host
	dht           *dht.IpfsDHT
	tunDev        *tun.TUN
	activeStreams map[peer.ID]SharedStream
	ctx           context.Context
	cancel        func()
	lockPath      string
	configPath    string
	interfaceName string
}

func New(ctx context.Context, configPath string, ifName string) Node {
	innerCtx, ctxCancel := context.WithCancel(ctx)

	return Node{
		cfg:           &config.Config{},
		p2p:           nil,
		tunDev:        &tun.TUN{},
		activeStreams: map[peer.ID]SharedStream{},
		ctx:           innerCtx,
		cancel:        ctxCancel,
		configPath:    configPath,
		interfaceName: ifName,
	}
}

func (node *Node) Run() error {
	// Read in configuration from file.
	cfg2, err := config.Read(node.configPath)
	if err != nil {
		return err
	}

	cfg2.Interface = node.interfaceName
	node.cfg = cfg2

	fmt.Println("[+] Creating TUN Device")

	// Create new TUN device
	node.tunDev, err = tun.New(
		node.cfg.Interface,
		tun.Address(node.cfg.BuiltinAddr4.String()+"/32"),
		tun.Address(node.cfg.BuiltinAddr6.String()+"/128"),
		tun.MTU(1420),
	)
	if err != nil {
		return err
	}
	allRoutes4, err := node.cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		return err
	}
	allRoutes6, err := node.cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv6)
	if err != nil {
		return err
	}
	var routeOpts []tun.Option

	for _, r := range allRoutes4 {
		routeOpts = append(routeOpts, tun.Route(r.Network()))
	}
	for _, r := range allRoutes6 {
		routeOpts = append(routeOpts, tun.Route(r.Network()))
	}

	fmt.Println("[+] Creating LibP2P Node")

	// Create P2P Node
	node.p2p, node.dht, err = p2p.CreateNode(
		node.ctx,
		node.cfg.PrivateKey,
		node.cfg.ListenAddresses,
		node.streamHandler,
		p2p.NewClosedCircuitRelayFilter(node.cfg.Peers),
		p2p.NewRecursionGater(node.cfg),
		node.cfg.Peers,
	)
	if err != nil {
		return err
	}
	node.p2p.SetStreamHandler(p2p.PeXProtocol, p2p.NewPeXStreamHandler(node.p2p, node.cfg))

	for _, p := range node.cfg.Peers {
		node.p2p.ConnManager().Protect(p.ID, "/hyprspace/peer")
	}

	fmt.Println("[+] Setting Up Node Discovery via DHT")

	// Setup P2P Discovery
	go p2p.Discover(node.ctx, node.p2p, node.dht, node.cfg.Peers)

	// Configure path for lock
	node.lockPath = filepath.Join(filepath.Dir(node.cfg.Path), node.cfg.Interface+".lock")

	// PeX
	go p2p.PeXService(node.ctx, node.p2p, node.cfg)

	// Route metrics and latency
	go p2p.RouteMetricsService(node.ctx, node.p2p, node.cfg)

	// Log about various events
	err = node.eventLogger(node.ctx, node.p2p)
	if err != nil {
		return err
	}

	// RPC server
	go hsrpc.RpcServer(node.ctx, multiaddr.StringCast(fmt.Sprintf("/unix/run/hyprspace-rpc.%s.sock", node.cfg.Interface)), node.p2p, *node.cfg, *node.tunDev)

	// Magic DNS server
	go hsdns.MagicDnsServer(node.ctx, *node.cfg, node.p2p)

	// metrics endpoint
	metricsPort, ok := os.LookupEnv("HYPRSPACE_METRICS_PORT")
	if ok {
		metricsTuple := fmt.Sprintf("127.0.0.1:%s", metricsPort)
		http.Handle("/metrics", promhttp.Handler())
		go func() {
			http.ListenAndServe(metricsTuple, nil)
		}()
		fmt.Printf("[+] Listening for metrics scrape requests on http://%s/metrics\n", metricsTuple)
	}

	serviceNet := svc.NewServiceNetwork(node.p2p, node.cfg, node.tunDev)

	for name, addr := range node.cfg.Services {
		proxy, err := svc.ProxyTo(addr)
		if err != nil {
			return err
		}
		serviceNet.Register(
			name,
			proxy,
		)
	}

	var svcNetIds [][4]byte
	for _, p := range node.cfg.Peers {
		svcNetIds = append(svcNetIds, config.MkNetID(p.ID))
	}
	svcNetIds = append(svcNetIds, config.MkNetID(node.p2p.ID()))
	for _, netId := range svcNetIds {
		addr := make([]byte, 16)
		copy(addr, serviceNet.NetworkRange.IP)
		copy(addr[10:], netId[:])
		mask1, mask0 := serviceNet.NetworkRange.Mask.Size()
		routeOpts = append(routeOpts, tun.Route(net.IPNet{
			IP:   addr,
			Mask: net.CIDRMask(mask1+32, mask0),
		}))
	}

	// Write lock to filesystem to indicate an existing running daemon.
	err = os.WriteFile(node.lockPath, []byte(fmt.Sprint(os.Getpid())), os.ModePerm)
	if err != nil {
		return err
	}

	// Bring Up TUN Device
	err = node.tunDev.Up()
	if err != nil {
		return errors.New("unable to bring up tun device: " + err.Error())
	}
	err = node.tunDev.Apply(routeOpts...)
	if err != nil {
		return err
	}

	fmt.Println("[+] Network setup complete")

	// Initialize active streams map and packet byte array.
	node.activeStreams = make(map[peer.ID]SharedStream)
	go func() {
		for {
			var packet = make([]byte, 1420)
			// Read in a packet from the tun device.
			plen, err := node.tunDev.Iface.Read(packet)
			if errors.Is(err, fs.ErrClosed) {
				fmt.Println("[-] Interface closed")
				<-node.ctx.Done()
				time.Sleep(1 * time.Second)
				return
			} else if err != nil {
				fmt.Println(err)
				continue
			}

			var dstIP net.IP
			proto := packet[0] & 0xf0

			if proto == 0x40 {
				dstIP = net.IP(packet[16:20])
				if node.cfg.BuiltinAddr4.Equal(dstIP) {
					continue
				}
			} else if proto == 0x60 {
				dstIP = net.IP(packet[24:40])
				if node.cfg.BuiltinAddr6.Equal(dstIP) {
					continue
				} else if serviceNet.NetworkRange.Contains(dstIP) {
					// Are you TCP because your protocol is 6, or is your protocol 6 because you are TCP?
					if packet[6] == 0x06 {
						port := uint16(packet[42])*256 + uint16(packet[43])
						if serviceNet.EnsureListener([16]byte(packet[24:40]), port) {
							count, err := (*serviceNet.Tun).Write([][]byte{packet}, 0)
							if count == 0 {
								fmt.Printf("[!] To service network: %s\n", err)
							}
						}
					}
					continue
				}
			} else {
				continue
			}
			var dst peer.ID

			// Check route table for destination address.
			route, found := node.cfg.FindRouteForIP(dstIP)

			if found {
				dst = route.Target.ID
				go node.sendPacket(dst, packet, plen)
			}
		}
	}()
	return nil
}

func (node *Node) streamHandler(stream network.Stream) {
	// If the remote node ID isn't in the list of known nodes don't respond.
	if _, ok := config.FindPeer(node.cfg.Peers, stream.Conn().RemotePeer()); !ok {
		stream.Reset()
		return
	}
	var packet = make([]byte, 1420)
	var packetSize = make([]byte, 2)
	for {
		// Read the incoming packet's size as a binary value.
		_, err := stream.Read(packetSize)
		if err != nil {
			stream.Close()
			return
		}

		// Decode the incoming packet's size from binary.
		size := binary.LittleEndian.Uint16(packetSize)

		// Read in the packet until completion.
		var plen uint16 = 0
		for plen < size {
			tmp, err := stream.Read(packet[plen:size])
			plen += uint16(tmp)
			if err != nil {
				stream.Close()
				return
			}
		}
		err = stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
		if err != nil {
			fmt.Println("[!] Failed to set write deadline: " + err.Error())
			stream.Close()
			return
		}
		_, _ = node.tunDev.Iface.Write(packet[:size])
	}
}

func (node *Node) sendPacket(dst peer.ID, packet []byte, plen int) {
	// Check if we already have an open connection to the destination peer.
	ms, ok := node.activeStreams[dst]
	if ok {
		if func() bool {
			ms.Lock.Lock()
			defer ms.Lock.Unlock()
			// Write out the packet's length to the libp2p stream to ensure
			// we know the full size of the packet at the other end.
			err := binary.Write(*ms.Stream, binary.LittleEndian, uint16(plen))
			if err == nil {
				// Write the packet out to the libp2p stream.
				// If everyting succeeds continue on to the next packet.
				_, err = (*ms.Stream).Write(packet[:plen])
				if err == nil {
					err := (*ms.Stream).SetWriteDeadline(time.Now().Add(25 * time.Second))
					if err == nil {
						return true
					}
				}
			}
			// If we encounter an error when writing to a stream we should
			// close that stream and delete it from the active stream map.
			(*ms.Stream).Close()
			delete(node.activeStreams, dst)
			return false
		}() {
			return
		}
	}

	stream, err := node.p2p.NewStream(node.ctx, dst, p2p.Protocol)
	if err != nil {
		fmt.Println("[!] Failed to open stream to " + dst.String() + ": " + err.Error())
		go p2p.Rediscover()
		return
	}
	err = stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
	if err != nil {
		fmt.Println("[!] Failed to set write deadline: " + err.Error())
		stream.Close()
		return
	}
	// Write packet length
	err = binary.Write(stream, binary.LittleEndian, uint16(plen))
	if err != nil {
		stream.Close()
		return
	}
	// Write the packet
	_, err = stream.Write(packet[:plen])
	if err != nil {
		stream.Close()
		return
	}

	// If all succeeds when writing the packet to the stream
	// we should reuse this stream by adding it active streams map.
	node.activeStreams[dst] = SharedStream{
		Stream: &stream,
		Lock:   &sync.Mutex{},
	}
}

func (node *Node) eventLogger(ctx context.Context, host host.Host) error {
	subCon, err := host.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-subCon.Out():
				evt := ev.(event.EvtPeerConnectednessChanged)
				for _, vpnPeer := range node.cfg.Peers {
					if vpnPeer.ID == evt.Peer {
						if evt.Connectedness == network.Connected {
							for _, c := range host.Network().ConnsToPeer(evt.Peer) {
								fmt.Printf("[+] Connected to %s/p2p/%s\n", c.RemoteMultiaddr().String(), evt.Peer.String())
							}
						} else if evt.Connectedness == network.NotConnected {
							fmt.Printf("[!] Disconnected from %s\n", evt.Peer.String())
						}
						break
					}
				}
			}
		}
	}()
	return nil
}

func (node *Node) Rebootstrap() {
	node.p2p.ConnManager().TrimOpenConns(context.Background())
	<-node.dht.ForceRefresh()
	p2p.Rediscover()
}

func (node *Node) Stop() error {
	err := node.p2p.Close()
	if err != nil {
		return err
	}

	err = os.Remove(node.lockPath)
	if err != nil {
		return err
	}

	fmt.Println("Received signal, shutting down...")

	node.tunDev.Iface.Close()
	err = node.tunDev.Down()
	if err != nil {
		return err
	}
	node.cancel()
	return nil
}
