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
	"strconv"
	"syscall"
	"time"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/config"
	"github.com/hyprspace/hyprspace/p2p"
	hsrpc "github.com/hyprspace/hyprspace/rpc"
	"github.com/hyprspace/hyprspace/tun"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cfg  *config.Config
	node host.Host
	// iface is the tun device used to pass packets between
	// Hyprspace and the user's machine.
	tunDev *tun.TUN
	// activeStreams is a map of active streams to a peer
	activeStreams map[peer.ID]network.Stream
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
	Args:  &UpArgs{},
	Flags: &UpFlags{},
	Run:   UpRun,
}

// UpArgs handles the specific arguments for the up command.
type UpArgs struct {
	InterfaceName string
}

// UpFlags handles the specific flags for the up command.
type UpFlags struct {
	Foreground bool `short:"f" long:"foreground" desc:"Don't Create Background Daemon."`
}

// UpRun handles the execution of the up command.
func UpRun(r *cmd.Root, c *cmd.Sub) {
	// Parse Command Args
	args := c.Args.(*UpArgs)

	// Parse Global Config Flag for Custom Config Path
	configPath := r.Flags.(*GlobalFlags).Config
	if configPath == "" {
		configPath = "/etc/hyprspace/" + args.InterfaceName + ".yaml"
	}

	// Read in configuration from file.
	cfg2, err := config.Read(configPath)
	cfg = cfg2
	checkErr(err)

	fmt.Println("[+] Creating TUN Device")

	// Create new TUN device
	tunDev, err = tun.New(
		cfg.Interface.Name,
		tun.Address(cfg.Interface.Address),
		tun.MTU(1420),
	)
	if err != nil {
		checkErr(err)
	}

	// Setup System Context
	ctx, ctxCancel = context.WithCancel(context.Background())

	fmt.Println("[+] Creating LibP2P Node")

	// Check that the listener port is available.
	port, err := verifyPort(cfg.Interface.ListenPort)
	checkErr(err)

	_, privateKey, err := multibase.Decode(cfg.Interface.PrivateKey)
	// Create P2P Node
	host, dht, err := p2p.CreateNode(
		ctx,
		privateKey,
		port,
		streamHandler,
		p2p.NewClosedCircuitRelayFilter(cfg.Peers),
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
	lockPath := filepath.Join(filepath.Dir(cfg.Path), cfg.Interface.Name+".lock")

	// PeX
	go p2p.PeXService(ctx, host, cfg)

	// Route metrics and latency
	go p2p.RouteMetricsService(ctx, host, cfg)

	// Register the application to listen for signals
	go signalHandler(ctx, host, lockPath, dht)

	// Log about various events
	go eventLogger(ctx, host)

	// RPC server
	go hsrpc.RpcServer(ctx, multiaddr.StringCast(fmt.Sprintf("/unix/run/hyprspace-rpc.%s.sock", cfg.Interface.Name)), host, *cfg)

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

	// Write lock to filesystem to indicate an existing running daemon.
	err = os.WriteFile(lockPath, []byte(fmt.Sprint(os.Getpid())), os.ModePerm)
	checkErr(err)

	// Bring Up TUN Device
	err = tunDev.Up()
	if err != nil {
		checkErr(errors.New("unable to bring up tun device"))
	}

	fmt.Println("[+] Network setup complete")

	// + ----------------------------------------+
	// | Listen For New Packets on TUN Interface |
	// + ----------------------------------------+

	// Initialize active streams map and packet byte array.
	activeStreams = make(map[peer.ID]network.Stream)
	var packet = make([]byte, 1420)
	ip, _, err := net.ParseCIDR(cfg.Interface.Address)
	if err != nil {
		checkErr(errors.New("unable to parse address"))
	}
	for {
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

		dstIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
		if ip.Equal(dstIP) {
			continue
		}
		var dst *peer.ID

		// Check route table for destination address.
		for _, route := range cfg.Routes {
			if route.Network.Contains(dstIP) {
				reroute, found := p2p.FindReroute(route.Network, false)
				if found {
					dst = &reroute.To
				} else {
					dst = &route.Target.ID
				}
				break
			}
		}
		if dst == nil {
			continue
		}

		sendPacket(*dst, packet, plen)
	}
}

func sendPacket(dst peer.ID, packet []byte, plen int) {
	// Check if we already have an open connection to the destination peer.
	stream, ok := activeStreams[dst]
	if ok {
		// Write out the packet's length to the libp2p stream to ensure
		// we know the full size of the packet at the other end.
		err := binary.Write(stream, binary.LittleEndian, uint16(plen))
		if err == nil {
			// Write the packet out to the libp2p stream.
			// If everyting succeeds continue on to the next packet.
			_, err = stream.Write(packet[:plen])
			if err == nil {
				stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
				return
			}
		}
		// If we encounter an error when writing to a stream we should
		// close that stream and delete it from the active stream map.
		stream.Close()
		delete(activeStreams, dst)
	}

	stream, err := node.NewStream(ctx, dst, p2p.Protocol)
	if err != nil {
		fmt.Println("[!] Failed to open stream to " + dst.String())
		go p2p.Rediscover()
		return
	}
	stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
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
	activeStreams[dst] = stream
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
			tunDev.Down()
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
		stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
		dstIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
		route, found := config.FindRouteForIP(cfg.Routes, dstIP)
		if !found {
			// not found means the packet is for us
			tunDev.Iface.Write(packet[:size])
		} else {
			// FIXME: should decrease the TTL here
			sendPacket(route.Target.ID, packet, int(plen))
		}
	}
}

func verifyPort(port int) (int, error) {
	var ln net.Listener
	var err error

	// If a user manually sets a port don't try to automatically
	// find an open port.
	if port != 8001 {
		ln, err = net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			return port, errors.New("could not create node, listen port already in use by something else")
		}
	} else {
		// Automatically look for an open port when a custom port isn't
		// selected by a user.
		for {
			ln, err = net.Listen("tcp", ":"+strconv.Itoa(port))
			if err == nil {
				break
			}
			if port >= 65535 {
				return port, errors.New("failed to find open port")
			}
			port++
		}
	}
	if ln != nil {
		ln.Close()
	}
	return port, nil
}
