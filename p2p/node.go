package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/pnet"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
)

// Protocol is a descriptor for the Hyprspace P2P Protocol.
const Protocol = "/hyprspace/0.0.1"

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

	ip6quic := fmt.Sprintf("/ip6/::/udp/%d/quic", port)
	ip4quic := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", port)

	ip6tcp := fmt.Sprintf("/ip6/::/tcp/%d", port)
	ip4tcp := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)

	key, _ := pnet.DecodeV1PSK(swarmKey)

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
		libp2p.FallbackDefaults,
	)
	if err != nil {
		return
	}

	// Setup Hyprspace Stream Handler
	node.SetStreamHandler(Protocol, handler)

	// Create DHT Subsystem
	dhtOut = dht.NewDHTClient(ctx, node, datastore.NewMapDatastore())

	// Define Bootstrap Nodes.
	peers := []string{
		"/ip4/168.235.67.108/tcp/4001/p2p/QmRMA5pWXtfuW1y5w2t9gYxrDDD6bPRLKdWAYnHTeCxZMm",
		"/ip4/95.216.8.12/tcp/4001/p2p/Qmd7QHZU8UjfYdwmjmq1SBh9pvER9AwHpfwQvnvNo3HBBo",
		"/ip6/2001:41d0:800:1402::3f16:3fb5/tcp/4001/p2p/12D3KooWDUgNsoLVauCDpRAo54mc4whoBudgeXQnZZK2iVYhBLCN",
		"/ip6/2001:818:da65:e400:a553:fbc1:f0b1:5743/tcp/4001/p2p/12D3KooWC1RZxLvAeEFNTZWk1FWc1sZZ3yemF4FNNRYa3X854KJ8",
	}

	// Convert Bootstap Nodes into usable addresses.
	BootstrapPeers := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range append(peers, extraBootstrapNodes...) {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return node, dhtOut, err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return node, dhtOut, err
		}
		pi, ok := BootstrapPeers[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			BootstrapPeers[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	lock := sync.Mutex{}
	count := 0
	wg.Add(len(BootstrapPeers))
	for _, peerInfo := range BootstrapPeers {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			err := node.Connect(ctx, *peerInfo)
			if err == nil {
				lock.Lock()
				count++
				lock.Unlock()

			}
		}(peerInfo)
	}
	wg.Wait()

	if count < 1 {
		return node, dhtOut, errors.New("unable to bootstrap libp2p node")
	}
	fmt.Printf("[+] Connected to %d bootstrap peers\n", count)

	return node, dhtOut, nil
}
