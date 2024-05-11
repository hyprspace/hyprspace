package config

import (
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
)

func mkBuiltinAddr4(p peer.ID) net.IP {
	builtinAddr := []byte{100, 64, 1, 2}
	for i, b := range []byte(p) {
		builtinAddr[(i%2)+2] ^= b
	}
	return net.IP(builtinAddr).To4()
}

func mkBuiltinAddr6(p peer.ID) net.IP {
	builtinAddr := []byte("\xfd\x00hyprspace\x00\xde\xad\xbe\xef")
	for i, b := range []byte(p) {
		builtinAddr[(i%4)+12] ^= b
	}
	return net.IP(builtinAddr).To16()
}
