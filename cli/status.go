package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/rpc"
)

var Status = cmd.Sub{
	Name:  "status",
	Alias: "s",
	Short: "Display Hyprspace daemon status",
	Args:  &StatusArgs{},
	Run:   StatusRun,
}

type StatusArgs struct {
	InterfaceName string
}

func StatusRun(r *cmd.Root, c *cmd.Sub) {
	// Parse Command Args
	args := c.Args.(*StatusArgs)

	status := rpc.Status(args.InterfaceName)
	fmt.Println("PeerID:", status.PeerID)
	fmt.Println("Swarm peers:", status.SwarmPeersCurrent)
	fmt.Printf("Connected VPN nodes: %d/%d\n", status.NetPeersCurrent, status.NetPeersMax)
	printList(status.NetPeerAddrsCurrent)
	fmt.Println("Addresses:")
	printList(status.ListenAddrs)
}
