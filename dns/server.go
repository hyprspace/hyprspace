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

var domainSuffix = "hyprspace."

func mkAliasRecord(alias string, p peer.ID) *dns.CNAME {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   fmt.Sprintf("%s.%s", alias, domainSuffix),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		Target: fmt.Sprintf("%s.%s", cid, domainSuffix),
	}
}

func mkIDRecord(p peer.ID, addr net.IP) *dns.A {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   fmt.Sprintf("%s.%s", cid, domainSuffix),
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
	dns.HandleFunc(domainSuffix, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)

		for _, q := range r.Question {
			switch q.Qtype {
			case dns.TypeA:
				if qpeer, err := peer.Decode(strings.TrimSuffix(q.Name, "."+domainSuffix)); err == nil {
					if qpeer == node.ID() {
						m.Answer = append(m.Answer, mkIDRecord(node.ID(), config.BuiltinAddr))
					} else {
						for _, p := range config.Peers {
							if p.ID == qpeer {
								m.Answer = append(m.Answer, mkIDRecord(p.ID, p.BuiltinAddr))
								break
							}
						}
					}
				} else {
					hostname, err := os.Hostname()
					if err != nil {
						fmt.Println("[!] [dns] " + err.Error())
					}

					qName := strings.ToLower(strings.TrimSuffix(q.Name, "."+domainSuffix))

					if qName == strings.ToLower(hostname) {
						m.Answer = append(m.Answer, mkAliasRecord(qName, node.ID()))
						m.Answer = append(m.Answer, mkIDRecord(node.ID(), config.BuiltinAddr))
					} else if p, found := config.PeerLookup.ByName[qName]; found {
						m.Answer = append(m.Answer, mkAliasRecord(qName, p.ID))
						m.Answer = append(m.Answer, mkIDRecord(p.ID, p.BuiltinAddr))
					}
				}
			}
		}

		w.WriteMsg(m)
	})

	for _, netType := range []string{"tcp", "udp"} {
		sv := &dns.Server{
			Addr:      "127.80.1.53:5380",
			Net:       netType,
			ReusePort: true,
		}
		fmt.Printf("[-] Starting DNS server on /ip4/127.80.1.53/%s/5380\n", sv.Net)
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
				Address: []byte{127, 80, 1, 53},
				Port:    5380,
			}})
		},
		func() error {
			return conn.SetLinkDomains(ctx, linkID, []resolved.LinkDomain{{
				Domain:        domainSuffix,
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
