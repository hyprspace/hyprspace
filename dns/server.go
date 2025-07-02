package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/hyprspace/hyprspace/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/miekg/dns"
	"github.com/multiformats/go-multibase"
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

func mkAliasRecord(config config.Config, alias string, serviceName string, p peer.ID) *dns.CNAME {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	var aliasWithSvc string
	var cidWithSvc string
	if serviceName == "" {
		aliasWithSvc = alias
		cidWithSvc = cid
	} else {
		aliasWithSvc = serviceName + "." + alias
		cidWithSvc = serviceName + "." + cid
	}
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(config, aliasWithSvc),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    0,
		},
		Target: withDomainSuffix(config, cidWithSvc),
	}
}

func mkIDRecord4(config config.Config, p peer.ID, addr net.IP) *dns.A {
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

func mkIDRecord6(cfg config.Config, p peer.ID, serviceName string, addr net.IP) *dns.AAAA {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	var addrWithSvc net.IP
	var cidWithSvc string
	if serviceName == "" {
		addrWithSvc = addr
		cidWithSvc = cid
	} else {
		addrWithSvc = config.MkServiceAddr6(p, serviceName)
		cidWithSvc = serviceName + "." + cid
	}
	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(cfg, cidWithSvc),
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    86400,
		},
		AAAA: addrWithSvc.To16(),
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
				fallthrough
			case dns.TypeAAAA:
				nameParts := strings.Split(strings.TrimSuffix(q.Name, "."+domainSuffix(config)), ".")
				var qNodeName string
				var qServiceName string
				if len(nameParts) == 2 {
					qServiceName = nameParts[0]
					qNodeName = nameParts[1]
				} else if len(nameParts) == 1 {
					qNodeName = nameParts[0]
				} else {
					return
				}
				isService := qServiceName != ""
				if qpeer, err := peer.Decode(qNodeName); err == nil {
					if qpeer == node.ID() {
						if !isService {
							m.Answer = append(m.Answer, mkIDRecord4(config, node.ID(), config.BuiltinAddr4))
						}
						m.Answer = append(m.Answer, mkIDRecord6(config, node.ID(), qServiceName, config.BuiltinAddr6))
					} else {
						for _, p := range config.Peers {
							if p.ID == qpeer {
								if !isService {
									m.Answer = append(m.Answer, mkIDRecord4(config, p.ID, p.BuiltinAddr4))
								}
								m.Answer = append(m.Answer, mkIDRecord6(config, p.ID, qServiceName, p.BuiltinAddr6))
								break
							}
						}
					}
				} else {
					hostname, err := os.Hostname()
					if err != nil {
						fmt.Println("[!] [dns] " + err.Error())
					}

					qName := strings.ToLower(qNodeName)

					if qName == strings.ToLower(hostname) {
						m.Answer = append(m.Answer, mkAliasRecord(config, qName, qServiceName, node.ID()))
						if !isService {
							m.Answer = append(m.Answer, mkIDRecord4(config, node.ID(), config.BuiltinAddr4))
						}
						m.Answer = append(m.Answer, mkIDRecord6(config, node.ID(), qServiceName, config.BuiltinAddr6))
					} else if p, found := config.PeerLookup.ByName[qName]; found {
						m.Answer = append(m.Answer, mkAliasRecord(config, qName, qServiceName, p.ID))
						if !isService {
							m.Answer = append(m.Answer, mkIDRecord4(config, p.ID, p.BuiltinAddr4))
						}
						m.Answer = append(m.Answer, mkIDRecord6(config, p.ID, qServiceName, p.BuiltinAddr6))
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

	if err := registerOnSystemResolver(ctx, config, dnsServerAddrBytes, dnsServerPort); err != nil {
		fmt.Println(err)
	}
}
