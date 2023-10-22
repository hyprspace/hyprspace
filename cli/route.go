package cli

import (
	"fmt"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/hyprspace/hyprspace/rpc"
)

var Route = cmd.Sub{
	Name:  "route",
	Alias: "r",
	Short: "Control routing",
	Args:  &RouteArgs{},
	Run:   RouteRun,
}

type RouteArgs struct {
	Action string
	Args   []string `zero:"true"`
}

func RouteRun(r *cmd.Root, c *cmd.Sub) {
	// Parse Command Args
	args := c.Args.(*RouteArgs)
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}

	action := rpc.RouteAction(args.Action)
	rArgs := rpc.RouteArgs{
		Action: action,
		Args:   args.Args,
	}
	reply := rpc.Route(ifName, rArgs)
	for _, r := range reply.Routes {
		var target string
		connectStatus := ""
		if r.IsRelay {
			target = fmt.Sprintf("/p2p/%s/p2p-circuit/p2p/%s", r.RelayAddr, r.TargetAddr)
		} else {
			target = fmt.Sprintf("/p2p/%s", r.TargetAddr)
		}
		if r.IsConnected {
			connectStatus = " connected"
		}
		fmt.Printf("%s via %s%s\n", &r.Network, target, connectStatus)
	}
}
