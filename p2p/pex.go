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
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
)

type PeXRouting struct {
	host     host.Host
	vpnPeers []config.Peer
}

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

func RequestPeX(ctx context.Context, host host.Host, peers []peer.ID) (addrInfos []peer.AddrInfo, e error) {
	for _, p := range peers {
		s, err := host.NewStream(ctx, p, PeXProtocol)
		if err != nil {
			return nil, err
		}
		s.Write([]byte("r\n"))
		s.SetDeadline(time.Now().Add(10 * time.Second))
		buf := bufio.NewReader(s)
		for {
			str, err := buf.ReadString('\n')
			if err == io.EOF {
				return addrInfos, nil
			} else if checkErrPeX(err, s) {
				return nil, err
			}
			str = strings.TrimSuffix(str, "\n")
			splits := strings.Split(str, "|")
			idStr := splits[0]
			addrStr := splits[1]
			peerId, err := peer.Decode(idStr)
			if checkErrPeX(err, s) {
				return nil, err
			}
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if checkErrPeX(err, s) {
				return nil, err
			}
			fmt.Printf("[-] Got PeX peer: %s/p2p/%s\n", addrStr, idStr)
			addrInfos = append(addrInfos, peer.AddrInfo{
				ID:    peerId,
				Addrs: []multiaddr.Multiaddr{ma},
			})
		}
	}
	return addrInfos, nil
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
						go func() {
							addrInfos, err := RequestPeX(ctx, host, []peer.ID{evt.Peer})
							if err != nil {
								for _, addrInfo := range addrInfos {
									host.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, 30*time.Second)
								}
							}
						}()
					} else if evt.Connectedness == network.NotConnected {
						peers := []peer.ID{}
						for _, p := range cfg.Peers {
							peers = append(peers, p.ID)
						}
						go func() {
							addrInfos, err := RequestPeX(ctx, host, peers)
							if err != nil {
								for _, addrInfo := range addrInfos {
									host.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, 30*time.Second)
								}
							}
						}()
					}
					break
				}
			}
		}
	}
}

func (pexr PeXRouting) FindPeer(ctx context.Context, targetPeer peer.ID) (peer.AddrInfo, error) {
	found := false
	peers := []peer.ID{}
	addrInfo := peer.AddrInfo{
		ID: targetPeer,
	}
	for _, p := range pexr.vpnPeers {
		peers = append(peers, p.ID)
		if p.ID == targetPeer {
			found = true
		}
	}
	// PeX routing only returns VPN node addresses
	if !found {
		return addrInfo, routing.ErrNotFound
	}
	addrInfos, err := RequestPeX(ctx, pexr.host, peers)
	if err != nil {
		return addrInfo, err
	}
	for _, ai := range addrInfos {
		if ai.ID == targetPeer {
			addrInfo.Addrs = append(addrInfo.Addrs, ai.Addrs...)
		}
	}
	return addrInfo, nil
}
