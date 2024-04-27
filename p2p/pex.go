package p2p

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const PeXProtocol = "/hyprspace/pex/0.0.1"

func checkErrPeX(err error, stream network.Stream) bool {
	if err != nil {
		stream.Reset()
		fmt.Println("[!] PeX:", err)
		return true
	}
	return false
}

func NewPeXStreamHandler(host host.Host, cfg *config.Config) func(network.Stream) {
	return func(stream network.Stream) {
		found := false
		for _, p := range cfg.Peers {
			if p.ID == stream.Conn().RemotePeer() {
				found = true
				break
			}
		}
		if !found {
			stream.Reset()
			return
		}
		buf := bufio.NewReader(stream)
		str, err := buf.ReadString('\n')
		if checkErrPeX(err, stream) {
			return
		}
		str = strings.TrimSuffix(str, "\n")
		if str == "r" {
			// peer requests addresses
			for _, p := range cfg.Peers {
				if p.ID != stream.Conn().RemotePeer() {
					for _, c := range host.Network().ConnsToPeer(p.ID) {
						_, err := stream.Write([]byte(fmt.Sprintf("%s|%s\n", c.RemotePeer().String(), c.RemoteMultiaddr().String())))
						if checkErrPeX(err, stream) {
							return
						}
					}
				}
			}
			stream.Close()
		}
	}
}

func RequestPeX(ctx context.Context, host host.Host, peers []peer.ID) error {
	for _, p := range peers {
		s, err := host.NewStream(ctx, p, PeXProtocol)
		if err != nil {
			return err
		}
		s.Write([]byte("r\n"))
		s.SetDeadline(time.Now().Add(10 * time.Second))
		buf := bufio.NewReader(s)
		for {
			str, err := buf.ReadString('\n')
			if err == io.EOF {
				return nil
			} else if checkErrPeX(err, s) {
				return err
			}
			str = strings.TrimSuffix(str, "\n")
			splits := strings.Split(str, "|")
			idStr := splits[0]
			addrStr := splits[1]
			peerId, err := peer.Decode(idStr)
			if checkErrPeX(err, s) {
				return err
			}
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if checkErrPeX(err, s) {
				return err
			}
			fmt.Printf("[-] Got PeX peer: %s/p2p/%s\n", addrStr, idStr)
			host.Peerstore().AddAddr(peerId, ma, 24*time.Hour)
			host.Network().DialPeer(ctx, peerId)
		}
	}
	return nil
}

func PeXService(ctx context.Context, host host.Host, cfg *config.Config) {
	subCon, err := host.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("[-] PeX service ready")
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-subCon.Out():
			evt := ev.(event.EvtPeerConnectednessChanged)
			for _, vpnPeer := range cfg.Peers {
				if vpnPeer.ID == evt.Peer {
					if evt.Connectedness == network.Connected {
						go RequestPeX(ctx, host, []peer.ID{evt.Peer})
					} else if evt.Connectedness == network.NotConnected {
						peers := []peer.ID{}
						for _, p := range cfg.Peers {
							peers = append(peers, p.ID)
						}
						go RequestPeX(ctx, host, peers)
					}
					break
				}
			}
		}
	}
}
