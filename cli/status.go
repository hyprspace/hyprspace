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
	Run:   StatusRun,
}

func StatusRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	status := rpc.Status(ifName)
	fmt.Println("PeerID:", status.PeerID)
	fmt.Println("Swarm peers:", status.SwarmPeersCurrent)
	fmt.Printf("Connected VPN nodes: %d/%d\n", status.NetPeersCurrent, status.NetPeersMax)
	printList(status.NetPeerAddrsCurrent)
	fmt.Println("Addresses:")
	printList(status.ListenAddrs)
}
