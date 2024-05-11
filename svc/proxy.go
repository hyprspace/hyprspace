package svc

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Proxy struct {
	Handle func(net.Conn)
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
	}
}
