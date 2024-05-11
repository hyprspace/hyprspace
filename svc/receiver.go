package svc

import (
	"fmt"
	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/network"
)

func (sn *ServiceNetwork) streamHandler() func(network.Stream) {
	return func(stream network.Stream) {
		if _, ok := config.FindPeer(sn.config.Peers, stream.Conn().RemotePeer()); !ok {
			stream.Reset()
			return
		}
		defer stream.Close()
		buf := make([]byte, 2)
		_, err := stream.Read(buf)
		if err != nil {
			fmt.Printf("[!] [svc] %s\n", err)
			return
		}
		svcId := [2]byte(buf)
		if proxy, ok := sn.listeners[svcId]; ok {
			_, err := stream.Write([]byte{byte(RS_OK)})
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			}
			proxy.Handle(WrapStream(stream))
		} else {
			fmt.Printf("[!] [svc] %s tried to connect to unknown service %x\n", stream.Conn().RemotePeer(), svcId)
			_, err := stream.Write([]byte{byte(RS_NOT_SUPPORTED)})
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			}
		}
	}
}
