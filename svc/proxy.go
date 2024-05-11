package svc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Proxy struct {
	Handle      func(net.Conn)
	Description string
}

func (p Proxy) ServeFunc() func(net.Listener) error {
	return func(l net.Listener) error {
		for {
			conn, err := l.Accept()
			if err != nil {
				return err
			}
			go p.Handle(conn)
		}
	}
}

func ProxyTo(ma multiaddr.Multiaddr) (Proxy, error) {
	var proxy Proxy

	var err error
	var done bool

	var ipAddr net.IP = net.IPv4(127, 0, 0, 1)
	var tcpPort uint16

	multiaddr.ForEach(ma, func(c multiaddr.Component) bool {
		if done {
			err = errors.New("trailing components in address")
			return false
		}
		switch c.Protocol().Code {
		case multiaddr.P_IP4:
			fallthrough
		case multiaddr.P_IP6:
			ipAddr = net.IP(c.RawValue())
			return true
		case multiaddr.P_TCP:
			tcpPort = binary.BigEndian.Uint16(c.RawValue())
			proxy = TCPServiceProxy(net.TCPAddr{
				IP:   ipAddr,
				Port: int(tcpPort),
			})
			done = true
		default:
			err = errors.New("unsupported protocol: " + c.Protocol().Name)
			return false
		}
		return false
	})
	return proxy, err
}

type RemoteServiceProxyStatus byte

const (
	RS_OK            RemoteServiceProxyStatus = 0xf1
	RS_NOT_SUPPORTED RemoteServiceProxyStatus = 0xf2
)

func RemoteServiceProxy(host host.Host, p peer.ID, svcId [2]byte) Proxy {
	return Proxy{
		Handle: func(conn net.Conn) {
			ctx := context.Background()
			stream, err := host.NewStream(ctx, p, Protocol)
			defer conn.Close()
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			}
			defer stream.Close()
			_, err = stream.Write(svcId[:])
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			}
			buf := make([]byte, 1)
			_, err = stream.Read(buf)
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			} else if buf[0] != byte(RS_OK) {
				fmt.Printf("[!] [svc] Peer %s does not support service %x\n", p, svcId)
				return
			}
			pipe(conn, stream)
		},
		Description: fmt.Sprintf("RemoteServiceProxy to service [%x] on %s", svcId, p),
	}
}

func TCPServiceProxy(tcpAddr net.TCPAddr) Proxy {
	return Proxy{
		Handle: func(conn net.Conn) {
			stream, err := net.DialTCP("tcp", nil, &tcpAddr)
			defer conn.Close()
			if err != nil {
				fmt.Printf("[!] [svc] %s\n", err)
				return
			}
			defer stream.Close()
			pipe(conn, stream)
		},
		Description: fmt.Sprintf("TCPServiceProxy to %s", tcpAddr.AddrPort()),
	}
}
