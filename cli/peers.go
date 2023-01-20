package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/rpc"
)

var Peers = cmd.Sub{
	Name:  "peers",
	Short: "List peer connections",
	Args:  &PeersArgs{},
	Run:   PeersRun,
}

type PeersArgs struct {
	InterfaceName string
}

func PeersRun(r *cmd.Root, c *cmd.Sub) {
	args := c.Args.(*PeersArgs)

	peers := rpc.Peers(args.InterfaceName)
	for _, ma := range peers.PeerAddrs {
		fmt.Println(ma)
	}
}
