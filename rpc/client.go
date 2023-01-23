package rpc

import (
	"fmt"
	"log"
	"net/rpc"
)

func connect(ifname string) *rpc.Client {
	client, err := rpc.Dial("unix", fmt.Sprintf("/run/hyprspace-rpc.%s.sock", ifname))
	if err != nil {
		log.Fatal("[!] Failed to connect to RPC server: ", err)
	}
	return client
}

func Status(ifname string) StatusReply {
	client := connect(ifname)
	var reply StatusReply
	if err := client.Call("HyprspaceRPC.Status", new(Args), &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

func Peers(ifname string) PeersReply {
	client := connect(ifname)
	var reply PeersReply
	if err := client.Call("HyprspaceRPC.Peers", new(Args), &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}

func Route(ifname string, args RouteArgs) RouteReply {
	client := connect(ifname)
	var reply RouteReply
	if err := client.Call("HyprspaceRPC.Route", args, &reply); err != nil {
		log.Fatal("[!] RPC call failed: ", err)
	}
	return reply
}
