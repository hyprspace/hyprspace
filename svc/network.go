package svc

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/hyprspace/hyprspace/config"
	"github.com/hyprspace/hyprspace/netstack"
	hstun "github.com/hyprspace/hyprspace/tun"
	"github.com/libp2p/go-libp2p/core/host"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
)

const Protocol = "/hyprspace/service/0.0.1"

type ServiceNetwork struct {
	host         host.Host
	config       *config.Config
	self         [4]byte
	NetworkRange net.IPNet
	Tun          *tun.Device
	netx         *netstack.Net
	activeAddrs  map[[16]byte]struct{}
	activePorts  map[[16]byte]map[uint16]struct{}
	listeners    map[[2]byte]Proxy
}

func (sn *ServiceNetwork) Register(serviceName string, proxy Proxy) {
	svcId := config.MkServiceID(serviceName)
	sn.listeners[svcId] = proxy
	fmt.Printf("[-] Registered service \"%s\" [%x]\n", serviceName, svcId)
}

func (sn *ServiceNetwork) EnsureListener(addr [16]byte, port uint16) bool {
	registerAddr := true
	if _, ok := sn.activeAddrs[addr]; ok {
		if _, ok := sn.activePorts[addr][port]; ok {
			return true
		}
		registerAddr = false
	}
	netId := [4]byte(addr[10:14])
	svcId := [2]byte(addr[14:16])
	var proxy Proxy
	if netId == sn.self {
		// local service
		if s, ok := sn.listeners[svcId]; ok {
			proxy = s
		} else {
			fmt.Printf("[!] [svc] Unknown service: %x\n", addr[10:16])
			return false
		}
	} else if p, ok := sn.config.PeerLookup.ByNetID[netId]; ok {
		proxy = RemoteServiceProxy(sn.host, p.ID, svcId)
	}
	tcpAddr := net.TCPAddr{
		IP:   net.IP(addr[:]),
		Port: int(port),
	}
	if registerAddr {
		fmt.Printf("[-] [svc] Registering /ip6/%s\n", tcpAddr.IP)
		sn.netx.AddProtocolAddress(tcpip.ProtocolAddress{
			Protocol:          ipv6.ProtocolNumber,
			AddressWithPrefix: tcpip.AddrFrom16(addr).WithPrefix(),
		})
		sn.activeAddrs[addr] = struct{}{}
		sn.activePorts[addr] = make(map[uint16]struct{})
	}

	tcpL, err := sn.netx.ListenTCP(&tcpAddr)
	sn.activePorts[addr][port] = struct{}{}
	if err != nil {
		panic(err)
	}

	go proxy.ServeFunc()(tcpL)
	fmt.Printf("[-] [svc] Listening on /ip6/%s/tcp/%d\n", tcpAddr.IP, tcpAddr.Port)
	return true
}

func NewServiceNetwork(host host.Host, cfg *config.Config, tunDev *hstun.TUN) ServiceNetwork {
	tun, netx, err := netstack.CreateNetTUN(
		[]netip.Addr{
			netip.AddrFrom16([16]byte([]byte("\xfd\x00hyprspinternal"))),
		},
		[]netip.Addr{},
		1420,
	)
	if err != nil {
		panic(err)
	}

	go func() {
		sizes := make([]int, 1)
		buffer := make([]byte, 1420)
		buffers := make([][]byte, 1)
		buffers[0] = buffer
		for {
			count, err := tun.Read(buffers, sizes, 0)
			if err != nil {
				panic(err)
			}
			if count == 1 {
				_, err := tunDev.Iface.Write(buffers[0])
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	fmt.Println("[+] Service Network ready")

	return ServiceNetwork{
		host:   host,
		config: cfg,
		self:   [4]byte(cfg.BuiltinAddr6[12:16]),
		NetworkRange: net.IPNet{
			IP:   []byte("\xfd\x00hyprspsv\x00\x00\x00\x00\x00\x00"),
			Mask: net.CIDRMask(80, 128),
		},
		Tun:         &tun,
		netx:        netx,
		activeAddrs: make(map[[16]byte]struct{}),
		activePorts: make(map[[16]byte]map[uint16]struct{}),
		listeners:   make(map[[2]byte]Proxy),
	}
}
