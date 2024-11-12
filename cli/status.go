package cli

import (
	"fmt"
	"strings"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/rpc"
)

var Status = cmd.Sub{
	Name:  "status",
	Alias: "s",
	Short: "Display Hyprspace daemon status",
	Run:   StatusRun,
}

func colorMultiaddr(s string) string {
	if strings.HasSuffix(s, "/p2p-circuit") || strings.Contains(s, "/p2p-circuit/p2p/") {
		return fmt.Sprintf("%s%s%s", "\033[90m", s, "\033[0m")
	} else {
		return s
	}
}

func StatusRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	maybeColorMultiaddr := colorMultiaddr
	if !isTTY() {
		maybeColorMultiaddr = func(s string) string { return s }
	}
	status := rpc.Status(ifName)
	fmt.Println("PeerID:", status.PeerID)
	fmt.Println("Swarm peers:", status.SwarmPeersCurrent)
	fmt.Printf("Connected VPN nodes: %d/%d\n", status.NetPeersCurrent, status.NetPeersMax)
	printListF(status.NetPeerAddrsCurrent, maybeColorMultiaddr)
	fmt.Println("Addresses:")
	printListF(status.ListenAddrs, maybeColorMultiaddr)
}
