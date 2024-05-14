# DNS

Hyprspace runs a simple DNS server on the loopback interface for the purpose of looking up the IP addresses of nodes and services in the Hyprspace network.

## Domain

All DNS records are placed under a specific domain, based on the name of the network interface that Hyprspace controls. If the interface name is `hyprspace`, then the top-level domain `hyprspace.` is used. Otherwise, the domain `${interfaceName}.hyprspace.` is used. The rest of this document will assume that the domain is `hyprspace.`.

## Node lookup

### By Name

If a name (e.g. `"mynode"`) is assigned to the node in the config file, then that node can be looked up in DNS with the hostname `mynode.hyprspace.`. This will return the node's built-in addresses.

### By PeerID

A node can also be looked up by its PeerID. This may require converting the PeerID to base36. The format remains the same, except the PeerID is used in place of a name, e.g. `k2k4r8pp27b2t805cwk0rssqi4x7fcxrdumxcl6mospb2hqvtq2aoczg.hyprspace.`.

## Service lookup

Services in the [[service-network]] can be looked up in DNS by simply using the service name as a subdomain to the node, e.g. `myservice.mynode.hyprspace.`.

## DNS resolver configuration

In order to make use of this feature, you'll need to use a DNS resolver that can forward requests to different upstreams based on the domain, such as CoreDNS, `dnsmasq` or `systemd-resolved`.

Hyprspace's DNS server listens on a dynamically generated address in the `127.0.0.0/8` range and on a dynamically generated port, depending on the interface name. The address in use will be printed on startup.

For an interface named "`hyprspace`":
```
[-] Starting DNS server on /ip4/127.43.104.80/tcp/11355
[-] Starting DNS server on /ip4/127.43.104.80/udp/11355
```

## Integration with systemd-resolved

If the system uses `systemd-resolved`, Hyprspace will try to automatically configure it to point all DNS queries for the relevant domain to the Hyprspace DNS server.

```
Link 155 (hyprspace)
    Current Scopes: DNS LLMNR/IPv4 LLMNR/IPv6 mDNS/IPv4 mDNS/IPv6
         Protocols: +DefaultRoute +LLMNR +mDNS DNSOverTLS=opportunistic
                    DNSSEC=no/unsupported
Current DNS Server: 127.43.104.80:11355
       DNS Servers: 127.43.104.80:11355
        DNS Domain: hyprspace
```
