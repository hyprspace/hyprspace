package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/hyprspace/hyprspace/schema"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"github.com/yl2chen/cidranger"
)

// Config is the main Configuration Struct for Hyprspace.
type Config struct {
	Path            string                         `json:"-"`
	Interface       string                         `json:"-"`
	ListenAddresses []multiaddr.Multiaddr          `json:"-"`
	Peers           []Peer                         `json:"peers"`
	PeerLookup      PeerLookup                     `json:"-"`
	PrivateKey      crypto.PrivKey                 `json:"-"`
	BuiltinAddr4    net.IP                         `json:"-"`
	BuiltinAddr6    net.IP                         `json:"-"`
	Services        map[string]multiaddr.Multiaddr `json:"-"`
}

// Peer defines a peer in the configuration. We might add more to this later.
type Peer struct {
	ID           peer.ID `json:"id"`
	Name         string  `json:"name"`
	BuiltinAddr4 net.IP  `json:"-"`
	BuiltinAddr6 net.IP  `json:"-"`
}

// PeerLookup is a helper struct for quickly looking up a peer based on various parameters
type PeerLookup struct {
	ByRoute cidranger.Ranger
	ByName  map[string]Peer
	ByNetID map[[4]byte]Peer
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
	input := schema.Config{}
	result := Config{}

	// Read in config settings from file.
	err = json.Unmarshal(in, &input)
	if err != nil {
		return nil, err
	}

	_, keyBytes, err := multibase.Decode(input.PrivateKey)
	if err != nil {
		return nil, err
	}

	pk, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}

	result.PrivateKey = pk

	peerID, err := peer.IDFromPrivateKey(result.PrivateKey)
	if err != nil {
		return nil, err
	}

	result.BuiltinAddr4 = mkBuiltinAddr4(peerID)
	result.BuiltinAddr6 = mkBuiltinAddr6(peerID)

	for _, addrString := range input.ListenAddresses {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			return nil, err
		}
		result.ListenAddresses = append(result.ListenAddresses, addr)
	}

	result.PeerLookup.ByRoute = cidranger.NewPCTrieRanger()
	result.PeerLookup.ByName = make(map[string]Peer)
	result.PeerLookup.ByNetID = make(map[[4]byte]Peer)
	result.Peers = make([]Peer, len(input.Peers))

	for i, configPeer := range input.Peers {
		p := Peer{}
		p.ID, err = peer.Decode(configPeer.Id)
		if err != nil {
			return nil, err
		}
		p.Name = configPeer.Name
		p.BuiltinAddr4 = mkBuiltinAddr4(p.ID)
		p.BuiltinAddr6 = mkBuiltinAddr6(p.ID)
		for _, r := range configPeer.Routes {
			_, network, err := net.ParseCIDR(r.Net)
			if err != nil {
				log.Fatal("[!] Invalid network:", r.Net)
			}

			result.PeerLookup.ByRoute.Insert(&RouteTableEntry{
				Net:    *network,
				Target: p,
			})

			fmt.Printf("[+] Route %s via /p2p/%s\n", network.String(), p.ID)
		}
		result.PeerLookup.ByRoute.Insert(&RouteTableEntry{
			Net: net.IPNet{
				IP:   p.BuiltinAddr4,
				Mask: net.CIDRMask(32, 32),
			},
			Target: p,
		})
		result.PeerLookup.ByRoute.Insert(&RouteTableEntry{
			Net: net.IPNet{
				IP:   p.BuiltinAddr6,
				Mask: net.CIDRMask(128, 128),
			},
			Target: p,
		})
		if configPeer.Name != "" {
			result.PeerLookup.ByName[strings.ToLower(configPeer.Name)] = p
		}
		result.PeerLookup.ByNetID[[4]byte(p.BuiltinAddr6[12:16])] = p
		result.Peers[i] = p
	}

	result.Services = make(map[string]multiaddr.Multiaddr)
	for name, addrString := range input.Services {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			return nil, err
		}
		result.Services[name] = addr
	}

	// Overwrite path of config to input.
	result.Path = path
	return &result, nil
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
