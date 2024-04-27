package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
)

// Protocol is a descriptor for the Hyprspace P2P Protocol.
const Protocol = "/hyprspace/0.0.1"

var bootstrapTriggerChan = make(chan bool)

func getExtraPeers(addr ma.Multiaddr) (nodesList []string) {
	nodesList = []string{}
	ip4, err := addr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return
	}
	port, err := addr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return
	}
	resp, err := http.PostForm("http://"+ip4+":"+port+"/api/v0/swarm/peers", url.Values{})

	if err != nil {
		return
	}
	defer resp.Body.Close()

	apiResponse, err := io.ReadAll(resp.Body)

	if err != nil {
		return
	}
	var obj = map[string][]map[string]interface{}{}
	json.Unmarshal([]byte(apiResponse), &obj)
	for _, v := range obj["Peers"] {
		nodesList = append(nodesList, (v["Addr"].(string) + "/p2p/" + v["Peer"].(string)))
	}
	return
}

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
	resp, err := http.PostForm("http://"+ip4+":"+port+"/api/v0/bootstrap", url.Values{})

	if err != nil {
		return
	}
	defer resp.Body.Close()

	apiResponse, err := io.ReadAll(resp.Body)

	if err != nil {
		return
	}
	var obj = map[string][]string{}
	json.Unmarshal([]byte(apiResponse), &obj)
	return obj["Peers"]
}

// CreateNode creates an internal Libp2p nodes and returns it and it's DHT Discovery service.
func CreateNode(ctx context.Context, privateKey crypto.PrivKey, listenAddreses []ma.Multiaddr, handler network.StreamHandler, acl relay.ACLFilter, vpnPeers []config.Peer) (node host.Host, dhtOut *dht.IpfsDHT, err error) {
	maybePrivateNet := libp2p.ChainOptions()
	swarmKeyFile, ok := os.LookupEnv("HYPRSPACE_SWARM_KEY")
	if ok {
		fmt.Println("[+] Using swarm key " + swarmKeyFile)
		var swarmKey *os.File
		swarmKey, err = os.Open(swarmKeyFile)
		if err != nil {
			return
		}
		defer swarmKey.Close()
		key, _ := pnet.DecodeV1PSK(swarmKey)
		maybePrivateNet = libp2p.PrivateNetwork(key)
	}

	peerChan := make(chan peer.AddrInfo)

	// Create libp2p node
	basicHost, err := libp2p.New(
		maybePrivateNet,
		libp2p.ListenAddrs(listenAddreses...),
		libp2p.Identity(privateKey),
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.DefaultMuxers,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelayService(relay.WithLimit(nil), relay.WithACL(acl)),
		libp2p.EnableNATService(),
		libp2p.EnableAutoRelayWithPeerSource(
			func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
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
			},
			autorelay.WithNumRelays(2),
			autorelay.WithBootDelay(10*time.Second),
		),
		libp2p.WithDialTimeout(time.Second*5),
		libp2p.FallbackDefaults,
	)
	if err != nil {
		return
	}

	// Define Bootstrap Nodes.
	peers := []string{
		"/ip4/152.67.75.145/tcp/110/p2p/12D3KooWQWsHPUUeFhe4b6pyCaD1hBoj8j6Z7S7kTznRTh1p1eVt",
		"/ip4/152.67.75.145/udp/110/quic-v1/p2p/12D3KooWQWsHPUUeFhe4b6pyCaD1hBoj8j6Z7S7kTznRTh1p1eVt",
		"/ip4/152.67.75.145/tcp/995/p2p/QmbrAHuh4RYcyN9fWePCZMVmQjbaNXtyvrDCWz4VrchbXh",
		"/ip4/152.67.75.145/udp/995/quic-v1/p2p/QmbrAHuh4RYcyN9fWePCZMVmQjbaNXtyvrDCWz4VrchbXh",
		"/ip4/95.216.8.12/tcp/110/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo",
		"/ip4/95.216.8.12/udp/110/quic-v1/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo",
		"/ip4/95.216.8.12/tcp/995/p2p/QmYs4xNBby2fTs8RnzfXEk161KD4mftBfCiR8yXtgGPj4J",
		"/ip4/95.216.8.12/udp/995/quic-v1/p2p/QmYs4xNBby2fTs8RnzfXEk161KD4mftBfCiR8yXtgGPj4J",
		"/ip4/152.67.73.164/tcp/995/p2p/12D3KooWL84sAtq1QTYwb7gVbhSNX5ZUfVt4kgYKz8pdif1zpGUh",
		"/ip4/152.67.73.164/udp/995/quic-v1/p2p/12D3KooWL84sAtq1QTYwb7gVbhSNX5ZUfVt4kgYKz8pdif1zpGUh",
		"/dnsaddr/bootstrap.libp2p.io/p2p/12D3KooWEZXjE41uU4EL2gpkAQeDXYok6wghN7wwNVPF5bwkaNfS",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	}

	// Convert Bootstap Nodes into usable addresses.
	staticBootstrapPeers, err := parsePeerAddrs(peers)
	if err != nil {
		return node, nil, err
	}

	ipfsApiStr, ok := os.LookupEnv("HYPRSPACE_IPFS_API")
	if ok {
		ipfsApiAddr, err := ma.NewMultiaddr(ipfsApiStr)
		if err == nil {
			fmt.Println("[+] Getting additional peers from IPFS API")
			extraPeers, err := parsePeerAddrs(getExtraPeers(ipfsApiAddr))
			if err == nil {
				fmt.Printf("[+] %d additional addresses\n", len(extraPeers))
				for _, p := range extraPeers {
					basicHost.Peerstore().AddAddrs(p.ID, p.Addrs, 5*time.Minute)
				}
			}
		}
	}

	// Create DHT Subsystem
	dhtOut, err = dht.New(
		ctx,
		basicHost,
		dht.Mode(dht.ModeClient),
		dht.BootstrapPeers(staticBootstrapPeers...),
		dht.BootstrapPeersFunc(func() []peer.AddrInfo {
			extraBootstrapNodes := []string{}
			ipfsApiStr, ok := os.LookupEnv("HYPRSPACE_IPFS_API")
			if ok {
				ipfsApiAddr, err := ma.NewMultiaddr(ipfsApiStr)
				if err == nil {
					fmt.Println("[+] Getting additional bootstrap nodes from IPFS API")
					extraBootstrapNodes = getExtraBootstrapNodes(ipfsApiAddr)
					fmt.Printf("[+] %d additional bootstrap nodes\n", len(extraBootstrapNodes))
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

	pexr := PeXRouting{basicHost, vpnPeers}
	pr := ParallelRouting{[]routedhost.Routing{pexr, dhtOut}}
	node = routedhost.Wrap(basicHost, pr)

	// Setup Hyprspace Stream Handler
	node.SetStreamHandler(Protocol, handler)

	if err != nil {
		return node, nil, err
	}

	// Continuously feed peers into the AutoRelay service
	go func() {
		delay := backoff.NewExponentialDecorrelatedJitter(time.Second, time.Second*60, 5.0, rand.NewSource(time.Now().UnixMilli()))()
		for {
			for _, p := range node.Network().Peers() {
				pi := node.Network().Peerstore().PeerInfo(p)
				if acl.AllowReserve(p, node.Addrs()[0]) {
					peerChan <- pi
				}
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
