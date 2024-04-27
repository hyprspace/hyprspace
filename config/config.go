package config

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v2"
)

// Config is the main Configuration Struct for Hyprspace.
type Config struct {
	Path      string    `yaml:"path,omitempty"`
	Interface Interface `yaml:"interface"`
	Peers     []Peer    `yaml:"peers"`
	Routes    []Route
}

// Interface defines all of the fields that a local node needs to know about itself!
type Interface struct {
	Name       string  `yaml:"name"`
	ID         peer.ID `yaml:"id"`
	ListenPort int     `yaml:"listen_port"`
	Address    string  `yaml:"address"`
	PrivateKey string  `yaml:"private_key"`
}

// Peer defines a peer in the configuration. We might add more to this later.
type Peer struct {
	ID     peer.ID `yaml:"id"`
	Routes []Route `yaml:"routes"`
}

type Route struct {
	Target     Peer
	NetworkStr string `yaml:"net"`
	Network    net.IPNet
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
			Address:    "10.1.1.1/24",
			ID:         "",
			PrivateKey: "",
		},
	}

	// Read in config settings from file.
	err = yaml.Unmarshal(in, &result)
	if err != nil {
		return nil, err
	}

	for _, p := range result.Peers {
		for _, r := range p.Routes {
			r.Target = p
			_, n, err := net.ParseCIDR(r.NetworkStr)
			if err != nil {
				log.Fatal("[!] Invalid network:", r.NetworkStr)
			}
			r.Network = *n
			result.Routes = append(result.Routes, r)
			fmt.Printf("[+] Route %s via %s\n", r.Network.String(), p.ID.String())
		}
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

func FindRoute(routes []Route, needle net.IPNet) (*Route, bool) {
	for _, r := range routes {
		bits1, _ := r.Network.Mask.Size()
		bits2, _ := needle.Mask.Size()
		if r.Network.IP.Equal(needle.IP) && bits1 == bits2 {
			return &r, true
		}
	}
	return nil, false
}
