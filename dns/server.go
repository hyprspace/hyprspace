package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/hyprspace/hyprspace/config"
	"github.com/iguanesolutions/go-systemd/v5/resolved"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/miekg/dns"
	"github.com/multiformats/go-multibase"
	"github.com/vishvananda/netlink"
)

func domainSuffix(config config.Config) string {
	if config.Interface == "hyprspace" {
		return "hyprspace."
	} else {
		return fmt.Sprintf("%s.hyprspace.", config.Interface)
	}
}

func withDomainSuffix(config config.Config, str string) string {
	return fmt.Sprintf("%s.%s", str, domainSuffix(config))
}

func mkAliasRecord(config config.Config, alias string, p peer.ID) *dns.CNAME {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(config, alias),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		Target: withDomainSuffix(config, cid),
	}
}

func mkIDRecord(config config.Config, p peer.ID, addr net.IP) *dns.A {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(config, cid),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    86400,
		},
		A: addr.To4(),
	}
}

func writeResponse(msg *dns.Msg, q dns.Question, p peer.ID, addr net.IP) {
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		A: addr.To4(),
	})
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	msg.Extra = append(msg.Extra, &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		Txt: []string{p.String(), cid},
	})
}

func MagicDnsServer(ctx context.Context, config config.Config, node host.Host) {
	dns.HandleFunc(domainSuffix(config), func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)

		for _, q := range r.Question {
			switch q.Qtype {
			case dns.TypeA:
				if qpeer, err := peer.Decode(strings.TrimSuffix(q.Name, "."+domainSuffix(config))); err == nil {
					if qpeer == node.ID() {
						m.Answer = append(m.Answer, mkIDRecord(config, node.ID(), config.BuiltinAddr4))
					} else {
						for _, p := range config.Peers {
							if p.ID == qpeer {
								m.Answer = append(m.Answer, mkIDRecord(config, p.ID, p.BuiltinAddr4))
								break
							}
						}
					}
				} else {
					hostname, err := os.Hostname()
					if err != nil {
						fmt.Println("[!] [dns] " + err.Error())
					}

					qName := strings.ToLower(strings.TrimSuffix(q.Name, "."+domainSuffix(config)))

					if qName == strings.ToLower(hostname) {
						m.Answer = append(m.Answer, mkAliasRecord(config, qName, node.ID()))
						m.Answer = append(m.Answer, mkIDRecord(config, node.ID(), config.BuiltinAddr4))
					} else if p, found := config.PeerLookup.ByName[qName]; found {
						m.Answer = append(m.Answer, mkAliasRecord(config, qName, p.ID))
						m.Answer = append(m.Answer, mkIDRecord(config, p.ID, p.BuiltinAddr4))
					}
				}
			}
		}

		w.WriteMsg(m)
	})

	dnsServerAddrBytes := []byte{127, 80, 01, 53}
	var dnsServerPort uint16 = 5380
	for i, b := range []byte(config.Interface) {
		dnsServerAddrBytes[(i%3)+1] ^= b
		dnsServerPort = (dnsServerPort+uint16(b))%40000 + 5000
	}
	dnsServerAddr := net.IP(dnsServerAddrBytes)
	for _, netType := range []string{"tcp", "udp"} {
		sv := &dns.Server{
			Addr:      fmt.Sprintf("%s:%d", dnsServerAddr, dnsServerPort),
			Net:       netType,
			ReusePort: true,
		}
		fmt.Printf("[-] Starting DNS server on /ip4/%s/%s/%d\n", dnsServerAddr, sv.Net, dnsServerPort)
		go func(server *dns.Server) {
			if err := server.ListenAndServe(); err != nil {
				fmt.Printf("[!] DNS server error: %s, %s\n", server.Net, err.Error())
			}
		}(sv)
	}

	conn, err := resolved.NewConn()
	if err != nil {
		fmt.Println("[!] [dns] Failed to connect to D-Bus:", err)
		return
	}
	defer conn.Close()

	link, err := netlink.LinkByName(config.Interface)
	if err != nil {
		fmt.Println("[!] [dns] Failed to get link ID:", err)
		return
	}
	linkID := link.Attrs().Index

	for _, f := range [](func() error){
		func() error {
			return conn.SetLinkDNSEx(ctx, linkID, []resolved.LinkDNSEx{{
				Family:  syscall.AF_INET,
				Address: dnsServerAddrBytes,
				Port:    dnsServerPort,
			}})
		},
		func() error {
			return conn.SetLinkDomains(ctx, linkID, []resolved.LinkDomain{{
				Domain:        domainSuffix(config),
				RoutingDomain: false,
			}})
		},
	} {
		if err := f(); err != nil {
			fmt.Println("[!] [dns] Failed to configure resolved:", err)
			return
		}
	}
}
