package cli

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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
)

var (
	// iface is the tun device used to pass packets between
	// Hyprspace and the user's machine.
	tunDev *tun.TUN
	// RevLookup allow quick lookups of an incoming stream
	// for security before accepting or responding to any data.
	RevLookup map[string]string
	// activeStreams is a map of active streams to a peer
	activeStreams map[string]network.Stream
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
	cfg, err := config.Read(configPath)
	checkErr(err)

	// Setup reverse lookup hash map for authentication.
	RevLookup = make(map[string]string, len(cfg.Peers))
	for ip, id := range cfg.Peers {
		RevLookup[id.ID] = ip
	}

	fmt.Println("[+] Creating TUN Device")

	if runtime.GOOS == "darwin" {
		if len(cfg.Peers) > 1 {
			checkErr(errors.New("cannot create interface macos does not support more than one peer"))
		}

		// Grab ip address of only peer in config
		var destPeer string
		for ip := range cfg.Peers {
			destPeer = ip
		}

		// Create new TUN device
		tunDev, err = tun.New(
			cfg.Interface.Name,
			tun.Address(cfg.Interface.Address),
			tun.DestAddress(destPeer),
			tun.MTU(1420),
		)
	} else {
		// Create new TUN device
		tunDev, err = tun.New(
			cfg.Interface.Name,
			tun.Address(cfg.Interface.Address),
			tun.MTU(1420),
		)
	}
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
	)
	checkErr(err)

	for _, id := range cfg.Peers {
		p, err := peer.Decode(id.ID)
		checkErr(err)
		host.ConnManager().Protect(p, "/hyprspace/peer")
	}

	// Setup Peer Table for Quick Packet --> Dest ID lookup
	peerTable := make(map[string]peer.ID)
	for ip, id := range cfg.Peers {
		peerTable[ip], err = peer.Decode(id.ID)
		checkErr(err)
	}

	fmt.Println("[+] Setting Up Node Discovery via DHT")

	// Setup P2P Discovery
	go p2p.Discover(ctx, host, dht, peerTable)

	// Configure path for lock
	lockPath := filepath.Join(filepath.Dir(cfg.Path), cfg.Interface.Name+".lock")

	// Register the application to listen for signals
	go signalHandler(ctx, host, lockPath, dht)

	// Log about various events
	go eventLogger(ctx, host, cfg)

	// RPC server
	go hsrpc.RpcServer(ctx, multiaddr.StringCast(fmt.Sprintf("/unix/run/hyprspace-rpc.%s.sock", cfg.Interface.Name)), host, *cfg)

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
	activeStreams = make(map[string]network.Stream)
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
		dst := dstIP.String()

		// Check route table for destination address.
		for route, _ := range cfg.Routes {
			_, network, _ := net.ParseCIDR(route)
			if network.Contains(dstIP) {
				src := net.IPv4(packet[12], packet[13], packet[14], packet[15])
				_, ok := peerTable[dst]
				// Only rewrite if initiator is us or receiver is not a known peer
				if src.Equal(ip) && !ok {
					dst = cfg.Routes[route].IP
				}
			}
		}

		// Check if we already have an open connection to the destination peer.
		stream, ok := activeStreams[dst]
		if ok {
			// Write out the packet's length to the libp2p stream to ensure
			// we know the full size of the packet at the other end.
			err = binary.Write(stream, binary.LittleEndian, uint16(plen))
			if err == nil {
				// Write the packet out to the libp2p stream.
				// If everyting succeeds continue on to the next packet.
				_, err = stream.Write(packet[:plen])
				if err == nil {
					stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
					continue
				}
			}
			// If we encounter an error when writing to a stream we should
			// close that stream and delete it from the active stream map.
			stream.Close()
			delete(activeStreams, dst)
		}

		// Check if the destination of the packet is a known peer to
		// the interface.
		if peer, ok := peerTable[dst]; ok {
			stream, err = host.NewStream(ctx, peer, p2p.Protocol)
			if err != nil {
				fmt.Println("[!] Failed to open stream to " + dst)
				go p2p.Rediscover()
				continue
			}
			stream.SetWriteDeadline(time.Now().Add(25 * time.Second))
			// Write packet length
			err = binary.Write(stream, binary.LittleEndian, uint16(plen))
			if err != nil {
				stream.Close()
				continue
			}
			// Write the packet
			_, err = stream.Write(packet[:plen])
			if err != nil {
				stream.Close()
				continue
			}

			// If all succeeds when writing the packet to the stream
			// we should reuse this stream by adding it active streams map.
			activeStreams[dst] = stream
		}
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
			tunDev.Down()
			ctxCancel()
		}
	}
}

func eventLogger(ctx context.Context, host host.Host, cfg *config.Config) {
	subCon, err := host.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	checkErr(err)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-subCon.Out():
			evt := ev.(event.EvtPeerConnectednessChanged)
			for vpnIp, vpnPeer := range cfg.Peers {
				if vpnPeer.ID == evt.Peer.Pretty() {
					if evt.Connectedness == network.Connected {
						for _, c := range host.Network().ConnsToPeer(evt.Peer) {
							fmt.Printf("[+] Connected to %s at %s/p2p/%s\n", vpnIp, c.RemoteMultiaddr().String(), evt.Peer.Pretty())
						}
					} else if evt.Connectedness == network.NotConnected {
						fmt.Printf("[!] Disconnected from %s\n", vpnIp)
					}
					break
				}
			}
		}
	}
}

func streamHandler(stream network.Stream) {
	// If the remote node ID isn't in the list of known nodes don't respond.
	if _, ok := RevLookup[stream.Conn().RemotePeer().Pretty()]; !ok {
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
		tunDev.Iface.Write(packet[:size])
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
