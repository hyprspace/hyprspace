package p2p

import (
	"context"
	"net"
	"os"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/multiformats/go-multiaddr"
)

// a FilterGater that automatically filters all addresses present in the current unit's IPAddressDeny list
func NewAutoFilterGater() connmgr.ConnectionGater {
	fg := NewFilterGater()

	conn, err := dbus.NewSystemConnectionContext(context.Background())
	if err != nil {
		return fg
	}
	unit, err := conn.GetUnitNameByPID(context.Background(), uint32(os.Getpid()))
	if err != nil {
		return fg
	}
	prop, err := conn.GetServicePropertyContext(context.Background(), unit, "IPAddressDeny")
	if err != nil {
		return fg
	}
	for _, v := range prop.Value.Value().([][]interface{}) {
		addr := net.IP(v[1].([]byte))
		cidr := v[2].(uint32)
		var mask net.IPMask
		switch v[0].(int32) {
		case 0x2:
			mask = net.CIDRMask(int(cidr), 32)
		case 0xa:
			mask = net.CIDRMask(int(cidr), 128)
		default:
			panic("unknown IP address type")
		}
		fg.(FilterGater).Filters.AddFilter(net.IPNet{
			IP:   addr,
			Mask: mask,
		}, multiaddr.ActionDeny)
	}
	return fg
}
