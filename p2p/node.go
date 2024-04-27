package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
)

// Protocol is a descriptor for the Hyprspace P2P Protocol.
const Protocol = "/hyprspace/0.0.1"

var bootstrapTriggerChan = make(chan bool)

func getExtraBootstrapNodes(addr ma.Multiaddr) (nodesList []string) {
	nodesList = []string{}
	ip4, err := addr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return
	}
	port, err := addr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return
	}
	resp, err := http.PostForm("http://"+ip4+":"+port+"/api/v0/swarm/addrs", url.Values{})

	if err != nil {
		return
	}
	defer resp.Body.Close()

	apiResponse, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return
	}
	var obj = map[string]map[string][]string{}
	json.Unmarshal([]byte(apiResponse), &obj)
	for k, v := range obj["Addrs"] {
		for _, addr := range v {
			nodesList = append(nodesList, (addr + "/p2p/" + k))
		}
	}
	return
}

// CreateNode creates an internal Libp2p nodes and returns it and it's DHT Discovery service.
func CreateNode(ctx context.Context, inputKey []byte, port int, handler network.StreamHandler) (node host.Host, dhtOut *dht.IpfsDHT, err error) {
	// Unmarshal Private Key
	privateKey, err := crypto.UnmarshalPrivateKey(inputKey)
	if err != nil {
		return
	}

	var swarmKey *os.File
	swarmKeyFile, ok := os.LookupEnv("HYPRSPACE_SWARM_KEY")
	if ok {
		fmt.Println("[+] Using swarm key " + swarmKeyFile)
		swarmKey, err = os.Open(swarmKeyFile)
		if err != nil {
			return
		}
		defer swarmKey.Close()
	}

	ip6quic := fmt.Sprintf("/ip6/::/udp/%d/quic", port)
	ip4quic := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", port)

	ip6tcp := fmt.Sprintf("/ip6/::/tcp/%d", port)
	ip4tcp := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)

	key, _ := pnet.DecodeV1PSK(swarmKey)

	peerChan := make(chan peer.AddrInfo)

	// Create libp2p node
	node, err = libp2p.New(
		libp2p.PrivateNetwork(key),
		libp2p.ListenAddrStrings(ip6tcp, ip4tcp, ip4quic, ip6quic),
		libp2p.Identity(privateKey),
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.DefaultMuxers,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoRelay(
			autorelay.WithNumRelays(2),
			autorelay.WithPeerSource(func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
				r := make(chan peer.AddrInfo)
				go func() {
					defer close(r)
					for ; numPeers != 0; numPeers-- {
						select {
						case v, ok := <-peerChan:
							if !ok {
								return
							}
							select {
							case r <- v:
							case <-ctx.Done():
								return
							}
						case <-ctx.Done():
							return
						}
					}
				}()
				return r
			}, time.Second*1),
		),
		libp2p.WithDialTimeout(time.Second*5),
		libp2p.FallbackDefaults,
	)
	if err != nil {
		return
	}

	// Setup Hyprspace Stream Handler
	node.SetStreamHandler(Protocol, handler)

	// Define Bootstrap Nodes.
	peers := []string{
		"/ip4/168.235.67.108/tcp/4001/p2p/QmRMA5pWXtfuW1y5w2t9gYxrDDD6bPRLKdWAYnHTeCxZMm",
		"/ip4/95.216.8.12/tcp/4001/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo",
		"/ip6/2001:41d0:800:1402::3f16:3fb5/tcp/4001/p2p/12D3KooWDUgNsoLVauCDpRAo54mc4whoBudgeXQnZZK2iVYhBLCN",
		"/ip6/2001:818:da65:e400:a553:fbc1:f0b1:5743/tcp/4001/p2p/12D3KooWC1RZxLvAeEFNTZWk1FWc1sZZ3yemF4FNNRYa3X854KJ8",
	}

	// Convert Bootstap Nodes into usable addresses.
	staticBootstrapPeers, err := parsePeerAddrs(peers)
	if err != nil {
		return node, nil, err
	}

	// Create DHT Subsystem
	dhtOut, err = dht.New(
		ctx,
		node,
		dht.Mode(dht.ModeAuto),
		dht.BootstrapPeers(staticBootstrapPeers...),
		dht.BootstrapPeersFunc(func() []peer.AddrInfo {
			extraBootstrapNodes := []string{}
			ipfsApiStr, ok := os.LookupEnv("HYPRSPACE_IPFS_API")
			if ok {
				ipfsApiAddr, err := ma.NewMultiaddr(ipfsApiStr)
				if err == nil {
					fmt.Println("[+] Getting additional peers from IPFS API")
					extraBootstrapNodes = getExtraBootstrapNodes(ipfsApiAddr)
					fmt.Printf("[+] %d additional addresses\n", len(extraBootstrapNodes))
				}
			}
			dynamicBootstrapPeers, err := parsePeerAddrs(extraBootstrapNodes)
			if err != nil {
				return staticBootstrapPeers
			} else {
				return append(staticBootstrapPeers, dynamicBootstrapPeers...)
			}
		}),
	)

	if err != nil {
		return node, nil, err
	}

	// Continuously feed peers into the AutoRelay service
	go func() {
		delay := backoff.NewExponentialDecorrelatedJitter(time.Second, time.Second*60, 5.0, rand.NewSource(time.Now().UnixMilli()))()
		for {
			for _, p := range node.Network().Peers() {
				pi := node.Network().Peerstore().PeerInfo(p)
				peerChan <- pi
			}
			time.Sleep(delay.Delay())
		}
	}()

	return node, dhtOut, nil
}

func parsePeerAddrs(peers []string) (addrs []peer.AddrInfo, err error) {
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return nil, err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, *pii)
	}
	return addrs, nil
}
