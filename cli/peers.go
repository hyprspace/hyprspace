package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/rpc"
)

var Peers = cmd.Sub{
	Name:  "peers",
	Short: "List peer connections",
	Run:   PeersRun,
}

func PeersRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	peers := rpc.Peers(ifName)
	for _, ma := range peers.PeerAddrs {
		fmt.Println(ma)
	}
}
