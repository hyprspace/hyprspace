package cli

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/config"
	hsdns "github.com/hyprspace/hyprspace/dns"
	"github.com/hyprspace/hyprspace/p2p"
	hsrpc "github.com/hyprspace/hyprspace/rpc"
	"github.com/hyprspace/hyprspace/svc"
	"github.com/hyprspace/hyprspace/tun"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yl2chen/cidranger"
)

type MuxStream struct {
	Stream *network.Stream
	Lock   *sync.Mutex
}

var (
	cfg  *config.Config
	node host.Host
	// iface is the tun device used to pass packets between
	// Hyprspace and the user's machine.
	tunDev *tun.TUN
	// activeStreams is a map of active streams to a peer
	activeStreams map[peer.ID]MuxStream
	// context
	ctx context.Context
	// context cancel function
	ctxCancel func()
)

// Up creates and brings up a Hyprspace Interface.
var Up = cmd.Sub{
	Name:  "up",
	Alias: "up",
	Short: "Create and Bring Up a Hyprspace Interface.",
	Run:   UpRun,
}

// UpRun handles the execution of the up command.
func UpRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	// Parse Global Config Flag for Custom Config Path
	configPath := r.Flags.(*GlobalFlags).Config
	if configPath == "" {
		configPath = "/etc/hyprspace/" + ifName + ".json"
	}

	// Read in configuration from file.
	cfg2, err := config.Read(configPath)
	checkErr(err)
	cfg2.Interface = ifName
	cfg = cfg2

	fmt.Println("[+] Creating TUN Device")

	// Create new TUN device
	tunDev, err = tun.New(
		cfg.Interface,
		tun.Address(cfg.BuiltinAddr4.String()+"/32"),
		tun.Address(cfg.BuiltinAddr6.String()+"/128"),
		tun.MTU(1420),
	)
	if err != nil {
		checkErr(err)
	}
	allRoutes4, err := cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		checkErr(err)
	}
	allRoutes6, err := cfg.PeerLookup.ByRoute.CoveredNetworks(*cidranger.AllIPv6)
	if err != nil {
		checkErr(err)
	}
	var routeOpts []tun.Option

	for _, r := range allRoutes4 {
		routeOpts = append(routeOpts, tun.Route(r.Network()))
	}
	for _, r := range allRoutes6 {
		routeOpts = append(routeOpts, tun.Route(r.Network()))
	}

	// Setup System Context
	ctx, ctxCancel = context.WithCancel(context.Background())

	fmt.Println("[+] Creating LibP2P Node")

	// Create P2P Node
	host, dht, err := p2p.CreateNode(
		ctx,
		cfg.PrivateKey,
		cfg.ListenAddresses,
		streamHandler,
		p2p.NewClosedCircuitRelayFilter(cfg.Peers),
		p2p.NewRecursionGater(cfg),
		cfg.Peers,
	)
	checkErr(err)
	host.SetStreamHandler(p2p.PeXProtocol, p2p.NewPeXStreamHandler(host, cfg))
	node = host

	for _, p := range cfg.Peers {
		host.ConnManager().Protect(p.ID, "/hyprspace/peer")
	}

	fmt.Println("[+] Setting Up Node Discovery via DHT")

	// Setup P2P Discovery
	go p2p.Discover(ctx, host, dht, cfg.Peers)

	// Configure path for lock
	lockPath := filepath.Join(filepath.Dir(cfg.Path), cfg.Interface+".lock")

	// PeX
	go p2p.PeXService(ctx, host, cfg)

	// Route metrics and latency
	go p2p.RouteMetricsService(ctx, host, cfg)

	// Register the application to listen for signals
	go signalHandler(ctx, host, lockPath, dht)

	// Log about various events
	go eventLogger(ctx, host)

	// RPC server
	go hsrpc.RpcServer(ctx, multiaddr.StringCast(fmt.Sprintf("/unix/run/hyprspace-rpc.%s.sock", cfg.Interface)), host, *cfg, *tunDev)

	// Magic DNS server
	go hsdns.MagicDnsServer(ctx, *cfg, node)

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

	serviceNet := svc.NewServiceNetwork(host, cfg, tunDev)

	for name, addr := range cfg.Services {
		proxy, err := svc.ProxyTo(addr)
		checkErr(err)
		serviceNet.Register(
			name,
			proxy,
		)
	}

	var svcNetIds [][4]byte
	for _, p := range cfg.Peers {
		svcNetIds = append(svcNetIds, config.MkNetID(p.ID))
	}
	svcNetIds = append(svcNetIds, config.MkNetID(host.ID()))
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
	err = os.WriteFile(lockPath, []byte(fmt.Sprint(os.Getpid())), os.ModePerm)
	checkErr(err)

	// Bring Up TUN Device
	err = tunDev.Up()
	if err != nil {
		checkErr(errors.New("unable to bring up tun device"))
	}
	checkErr(tunDev.Apply(routeOpts...))

	fmt.Println("[+] Network setup complete")

	// + ----------------------------------------+
	// | Listen For New Packets on TUN Interface |
	// + ----------------------------------------+

	// Initialize active streams map and packet byte array.
	activeStreams = make(map[peer.ID]MuxStream)
	for {
		var packet = make([]byte, 1420)
		// Read in a packet from the tun device.
		plen, err := tunDev.Iface.Read(packet)
		if errors.Is(err, fs.ErrClosed) {
			fmt.Println("[-] Interface closed")
			<-ctx.Done()
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
			if cfg.BuiltinAddr4.Equal(dstIP) {
				continue
			}
		} else if proto == 0x60 {
			dstIP = net.IP(packet[24:40])
			if cfg.BuiltinAddr6.Equal(dstIP) {
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
		route, found := cfg.FindRouteForIP(dstIP)

		if found {
			dst = route.Target.ID
			go sendPacket(dst, packet, plen)
		}
	}
}

func sendPacket(dst peer.ID, packet []byte, plen int) {
	// Check if we already have an open connection to the destination peer.
	ms, ok := activeStreams[dst]
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
			delete(activeStreams, dst)
			return false
		}() {
			return
		}
	}

	stream, err := node.NewStream(ctx, dst, p2p.Protocol)
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
	activeStreams[dst] = MuxStream{
		Stream: &stream,
		Lock:   &sync.Mutex{},
	}
}

func signalHandler(ctx context.Context, host host.Host, lockPath string, dht *dht.IpfsDHT) {
	exitCh := make(chan os.Signal, 1)
	rebootstrapCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(rebootstrapCh, syscall.SIGUSR1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-rebootstrapCh:
			fmt.Println("[-] Rebootstrapping on SIGUSR1")
			host.ConnManager().TrimOpenConns(context.Background())
			<-dht.ForceRefresh()
			p2p.Rediscover()
		case <-exitCh:
			// Shut the node down
			err := host.Close()
			checkErr(err)

			// Remove daemon lock from file system.
			err = os.Remove(lockPath)
			checkErr(err)

			fmt.Println("Received signal, shutting down...")

			tunDev.Iface.Close()
			err = tunDev.Down()
			checkErr(err)
			ctxCancel()
		}
	}
}

func eventLogger(ctx context.Context, host host.Host) {
	subCon, err := host.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	checkErr(err)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-subCon.Out():
			evt := ev.(event.EvtPeerConnectednessChanged)
			for _, vpnPeer := range cfg.Peers {
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
}

func streamHandler(stream network.Stream) {
	// If the remote node ID isn't in the list of known nodes don't respond.
	if _, ok := config.FindPeer(cfg.Peers, stream.Conn().RemotePeer()); !ok {
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
		_, _ = tunDev.Iface.Write(packet[:size])
	}
}
