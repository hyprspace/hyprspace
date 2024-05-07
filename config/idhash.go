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

func MkServiceAddr6(p peer.ID, serviceName string) net.IP {
	serviceAddr := []byte("\xfd\x00hyprspsv\xde\xad\xbe\xef\x00\x00")
	for i, b := range []byte(p) {
		serviceAddr[(i%4)+10] ^= b
	}
	svcId := MkServiceID(serviceName)
	serviceAddr[14], serviceAddr[15] = svcId[0], svcId[1]
	return net.IP(serviceAddr).To16()
}

func MkServiceID(serviceName string) [2]byte {
	var id = [2]byte{0xff, 0xfe}
	for i, b := range []byte(serviceName) {
		id[i%2] ^= b * byte(i)
	}
	return id
}
