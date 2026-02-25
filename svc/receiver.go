package svc

import (
	"fmt"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

func (sn *ServiceNetwork) isRemoteBlocked(svcId [2]byte, remotePeer peer.ID) bool {
	if _, ok := sn.acl[svcId]; ok {
		isBlacklisted := false // without blacklist it's NOT-blacklisted by default
		isWhitelisted := true  // without whitelist it IS whitelisted by default
		if sn.acl[svcId].Blacklist != nil {
			_, isBlacklisted = sn.acl[svcId].Blacklist[remotePeer]
			if isBlacklisted {
				return true
			}
		}
		if sn.acl[svcId].Whitelist != nil {
			_, isWhitelisted = sn.acl[svcId].Whitelist[remotePeer]
		}
		return isBlacklisted || !isWhitelisted
	}
	return false
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
