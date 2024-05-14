package svc

import (
	"net"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
)

// dumb wrapper around network.Stream so it implements net.Conn
type StreamWrapper struct {
	Stream network.Stream
}

func WrapStream(s network.Stream) net.Conn {
	return StreamWrapper{
		Stream: s,
	}
}

// does anything actually need these?
func (sw StreamWrapper) LocalAddr() net.Addr {
	panic("unimplemented")
}

func (sw StreamWrapper) RemoteAddr() net.Addr {
	panic("unimplemented")
}

// everything else is already implemented
func (sw StreamWrapper) Close() error {
	return sw.Stream.Close()
}

func (sw StreamWrapper) Read(b []byte) (n int, err error) {
	return sw.Stream.Read(b)
}

func (sw StreamWrapper) SetDeadline(t time.Time) error {
	return sw.Stream.SetDeadline(t)
}

func (sw StreamWrapper) SetReadDeadline(t time.Time) error {
	return sw.Stream.SetReadDeadline(t)
}

func (sw StreamWrapper) SetWriteDeadline(t time.Time) error {
	return sw.Stream.SetWriteDeadline(t)
}

func (sw StreamWrapper) Write(b []byte) (n int, err error) {
	return sw.Stream.Write(b)
}
