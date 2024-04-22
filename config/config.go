package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"github.com/yl2chen/cidranger"
)

// Config is the main Configuration Struct for Hyprspace.
type Config struct {
	Path                   string                `json:"-"`
	Interface              string                `json:"-"`
	EncodedListenAddresses []string              `json:"listenAddresses"`
	ListenAddresses        []multiaddr.Multiaddr `json:"-"`
	Peers                  []Peer                `json:"peers"`
	PeerLookup             PeerLookup            `json:"-"`
	EncodedPrivateKey      string                `json:"privateKey"`
	PrivateKey             crypto.PrivKey        `json:"-"`
	BuiltinAddr            net.IP                `json:"-"`
}

// Peer defines a peer in the configuration. We might add more to this later.
type Peer struct {
	ID          peer.ID `json:"id"`
	Name        string  `json:"name"`
	BuiltinAddr net.IP  `json:"-"`
	Routes      []Route `json:"routes"`
}

type Route struct {
	NetworkStr string    `json:"net"`
	Network    net.IPNet `json:"-"`
}

// PeerLookup is a helper struct for quickly looking up a peer based on various parameters
type PeerLookup struct {
	ByRoute cidranger.Ranger
	ByName  map[string]Peer
}

type RouteTableEntry struct {
	Net    net.IPNet
	Target Peer
}

func (rte RouteTableEntry) Network() net.IPNet {
	return rte.Net
}

// Read initializes a config from a file.
func Read(path string) (*Config, error) {
	in, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := Config{
		EncodedListenAddresses: []string{
			"/ip4/0.0.0.0/tcp/8001",
			"/ip4/0.0.0.0/udp/8001/quic-v1",
			"/ip6/::/tcp/8001",
			"/ip6/::/udp/8001/quic-v1",
		},
	}

	// Read in config settings from file.
	err = json.Unmarshal(in, &result)
	if err != nil {
		return nil, err
	}

	_, keyBytes, err := multibase.Decode(result.EncodedPrivateKey)
	if err != nil {
		return nil, err
	}

	pk, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}

	result.PrivateKey = pk
	if err != nil {
		return nil, err
	}

	peerID, err := peer.IDFromPrivateKey(result.PrivateKey)
	if err != nil {
		return nil, err
	}

	result.BuiltinAddr = mkBuiltinAddr(peerID)

	for _, addrString := range result.EncodedListenAddresses {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			return nil, err
		}
		result.ListenAddresses = append(result.ListenAddresses, addr)
	}

	result.PeerLookup.ByRoute = cidranger.NewPCTrieRanger()
	result.PeerLookup.ByName = make(map[string]Peer)

	for i, p := range result.Peers {
		p.BuiltinAddr = mkBuiltinAddr(p.ID)
		p.Routes = append(p.Routes, Route{
			Network: net.IPNet{
				IP:   p.BuiltinAddr,
				Mask: net.IPv4Mask(255, 255, 255, 255),
			},
		})
		for _, r := range p.Routes {
			if r.NetworkStr != "" {
				_, n, err := net.ParseCIDR(r.NetworkStr)
				if err != nil {
					log.Fatal("[!] Invalid network:", r.NetworkStr)
				}
				r.Network = *n
			}

			result.PeerLookup.ByRoute.Insert(&RouteTableEntry{
				Net:    r.Network,
				Target: p,
			})

			fmt.Printf("[+] Route %s via /p2p/%s\n", r.Network.String(), p.ID)
		}
		if p.Name != "" {
			result.PeerLookup.ByName[strings.ToLower(p.Name)] = p
		}
		result.Peers[i] = p
	}

	// Overwrite path of config to input.
	result.Path = path
	return &result, nil
}

func mkBuiltinAddr(p peer.ID) net.IP {
	builtinAddr := []byte{100, 64, 1, 2}
	for i, b := range []byte(p) {
		builtinAddr[(i%2)+2] ^= b
	}
	return net.IP(builtinAddr)
}

func FindPeer(peers []Peer, needle peer.ID) (*Peer, bool) {
	for _, p := range peers {
		if p.ID == needle {
			return &p, true
		}
	}
	return nil, false
}

func FindPeerByName(peers []Peer, needle string) (*Peer, bool) {
	for _, p := range peers {
		if p.Name == needle {
			return &p, true
		}
	}
	return nil, false
}

func FindPeerByIDPrefix(peers []Peer, needle string) (*Peer, bool) {
	for _, p := range peers {
		if strings.HasPrefix(p.ID.String(), needle) {
			return &p, true
		}
	}
	return nil, false
}

func FindPeerByCLIRef(peers []Peer, needle string) (*Peer, bool) {
	if strings.HasPrefix(needle, "@") {
		name := strings.TrimPrefix(needle, "@")
		return FindPeerByName(peers, name)
	} else {
		return FindPeerByIDPrefix(peers, needle)
	}
}

func (cfg Config) FindRoute(needle net.IPNet) (*RouteTableEntry, bool) {
	networks, err := cfg.PeerLookup.ByRoute.CoveredNetworks(needle)
	if err != nil {
		fmt.Println(err)
		return nil, false
	} else if len(networks) == 0 {
		return nil, false
	} else if len(networks) > 1 {
		for _, n := range networks {
			fmt.Printf("[!] Found duplicate route %s to /p2p/%s for %s\n", n.Network(), n.(RouteTableEntry).Target.ID, needle)
		}
	}
	return networks[0].(*RouteTableEntry), true
}

func (cfg Config) FindRouteForIP(needle net.IP) (*RouteTableEntry, bool) {
	networks, err := cfg.PeerLookup.ByRoute.ContainingNetworks(needle)
	if err != nil {
		fmt.Println(err)
		return nil, false
	} else if len(networks) == 0 {
		return nil, false
	} else if len(networks) > 1 {
		for _, n := range networks {
			fmt.Printf("[!] Found duplicate route %s to /p2p/%s for %s\n", n.Network(), n.(RouteTableEntry).Target.ID, needle)
		}
	}
	return networks[0].(*RouteTableEntry), true
}
