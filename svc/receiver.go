package svc

import (
	"fmt"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

func (sn *ServiceNetwork) isRemoteBlocked(svcId [2]byte, remotePeer peer.ID) bool {
	sv := sn.services[svcId]
	_, isWhitelisted := sv.Whitelist[remotePeer]
	_, isBlacklisted := sv.Blacklist[remotePeer]
	return isBlacklisted || (sv.EnableWhitelist && !isWhitelisted)
}

func (sn *ServiceNetwork) streamHandler() func(network.Stream) {
	return func(stream network.Stream) {
		if _, ok := config.FindPeer(sn.config.Peers, stream.Conn().RemotePeer()); !ok {
			logger.Debug("Connection attempt from untrusted peer")
			stream.Reset()
			return
		}
		defer stream.Close()
		buf := make([]byte, 2)
		_, err := stream.Read(buf)
		if err != nil {
			logger.With(err).Error("Failed to read stream")
			return
		}
		svcId := [2]byte(buf)
		if proxy, ok := sn.listeners[svcId]; ok {
			remotePeer := stream.Conn().RemotePeer()
			if sn.isRemoteBlocked(svcId, remotePeer) {
				logger.With(zap.String("service ID", fmt.Sprintf("%x", svcId[:]))).Debug("Connection from non-allowed peer")
				_, err := stream.Write([]byte{byte(RS_NOT_AUTHORIZED)})
				if err != nil {
					logger.With(err).Error("Failed to send RS_NOT_AUTHORIZED")
					return
				}
				return
			}

			_, err := stream.Write([]byte{byte(RS_OK)})
			if err != nil {
				logger.With(err).Error("Failed to write stream")
				return
			}
			proxy.Handle(WrapStream(stream))
		} else {
			logger.Info(fmt.Sprintf("%s tried to connect to unknown service %x", stream.Conn().RemotePeer(), svcId))
			_, err := stream.Write([]byte{byte(RS_NOT_SUPPORTED)})
			if err != nil {
				logger.With(err).Error("Failed to send RS_NOT_SUPPORTED")
				return
			}
		}
	}
}
