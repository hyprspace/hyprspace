package netstack

import (
	"fmt"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

func (tun *Net) AddProtocolAddress(protoAddr tcpip.ProtocolAddress) error {
	tcpipErr := tun.stack.AddProtocolAddress(1, protoAddr, stack.AddressProperties{})
	if tcpipErr != nil {
		return fmt.Errorf("AddProtocolAddress(%v): %v", protoAddr.AddressWithPrefix, tcpipErr)
	}
	return nil
}
