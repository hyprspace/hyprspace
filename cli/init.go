package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multibase"
)

// Init creates a configuration for a Hyprspace Interface.
var Init = cmd.Sub{
	Name:  "init",
	Alias: "i",
	Short: "Initialize An Interface Config",
	Run:   InitRun,
}

// InitRun handles the execution of the init command.
func InitRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	configPath := r.Flags.(*GlobalFlags).Config
	if configPath == "" {
		configPath = "/etc/hyprspace/" + ifName + ".json"
	}

	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	checkErr(err)

	keyBytes, err := crypto.MarshalPrivateKey(privKey)
	checkErr(err)

	// Setup an initial default command.
	new := config.Config{
		EncodedPrivateKey: multibase.MustNewEncoder(multibase.Base58BTC).Encode(keyBytes),
		EncodedListenAddresses: []string{
			"/ip4/0.0.0.0/tcp/8001",
			"/ip4/0.0.0.0/udp/8001/quic-v1",
			"/ip6/::/tcp/8001",
			"/ip6/::/udp/8001/quic-v1",
		},
		Peers: make([]config.Peer, 0),
	}

	out, err := json.MarshalIndent(&new, "", "  ")
	checkErr(err)

	err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
	checkErr(err)

	f, err := os.Create(configPath)
	checkErr(err)

	_, err = f.Write(out)
	checkErr(err)

	err = f.Close()
	checkErr(err)

	fmt.Printf("Initialized new config at %s\n", configPath)
	peerId, err := peer.IDFromPrivateKey(privKey)
	if err == nil {
		fmt.Println("Add this entry to your other peers:")
		fmt.Println("{")
		hostname, err := os.Hostname()
		if err == nil {
			fmt.Printf("  \"name\": \"%s\",\n", hostname)
		}
		fmt.Printf("  \"id\": \"%s\"\n", peerId)
		fmt.Println("}")
	}
}
