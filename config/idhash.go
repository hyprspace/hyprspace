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
	builtinAddr := []byte("\xfd\x00hyprspace\x00\x00\x00\x00\x00")
	for i, b := range []byte(p) {
		builtinAddr[(i%4)+12] ^= b
	}
	netId := MkNetID(p)
	builtinAddr[12], builtinAddr[13], builtinAddr[14], builtinAddr[15] = netId[0], netId[1], netId[2], netId[3]
	return net.IP(builtinAddr).To16()
}

func MkServiceAddr6(p peer.ID, serviceName string) net.IP {
	serviceAddr := []byte("\xfd\x00hyprspsv\x00\x00\x00\x00\x00\x00")
	netId := MkNetID(p)
	serviceAddr[10], serviceAddr[11], serviceAddr[12], serviceAddr[13] = netId[0], netId[1], netId[2], netId[3]
	svcId := MkServiceID(serviceName)
	serviceAddr[14], serviceAddr[15] = svcId[0], svcId[1]
	return net.IP(serviceAddr).To16()
}

func MkNetID(p peer.ID) [4]byte {
	r := [4]byte{0xde, 0xad, 0xbe, 0xef}
	for i, b := range []byte(p) {
		r[i%4] ^= b
	}
	return r
}

func MkServiceID(serviceName string) [2]byte {
	var id = [2]byte{0xff, 0xfe}
	for i, b := range []byte(serviceName) {
		id[i%2] ^= b * byte(i)
	}
	return id
}
