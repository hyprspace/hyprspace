package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/yl2chen/cidranger"
)

// Config is the main Configuration Struct for Hyprspace.
type Config struct {
	Path       string     `json:"-"`
	Interface  Interface  `json:"interface"`
	Peers      []Peer     `json:"peers"`
	PeerLookup PeerLookup `json:"-"`
}

// Interface defines all of the fields that a local node needs to know about itself!
type Interface struct {
	Name        string  `json:"name"`
	ID          peer.ID `json:"id"`
	ListenPort  int     `json:"listen_port"`
	BuiltinAddr net.IP  `json:"-"`
	PrivateKey  string  `json:"private_key"`
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
		Interface: Interface{
			Name:       "hs0",
			ListenPort: 8001,
			ID:         "",
			PrivateKey: "",
		},
	}

	// Read in config settings from file.
	err = json.Unmarshal(in, &result)
	if err != nil {
		return nil, err
	}

	result.Interface.BuiltinAddr = mkBuiltinAddr(result.Interface.ID)

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
