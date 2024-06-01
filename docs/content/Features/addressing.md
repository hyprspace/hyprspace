# Automatic IP Address Assignment

Hyprspace automatically assigns IPv4 and IPv6 addresses to all VPN nodes. These addresses are derived from the node's PeerID.

IP addresses are not exchanged between nodes. Instead, it is assumed that all nodes use the same algorithm to determine each other's addresses.

## Address types

### Built-in address

This is a node's primary IP address in the virtual network. It is used as the source address for all traffic originating from a node, and as the destination address when talking to a node directly. These addresses should not be routed outside of the virtual network.

### Service address

These are addresses used by the [[service-network]]. Each service gets its own address. These addresses should never leave the machine, neither through Hyprspace nor through other networks. Traffic in the [[service-network]] is handled at a higher layer.

## IPv4 ranges

For IPv4, part of the CGNAT range `100.64.0.0/16` is used. This was inspired by [Tailscale's use of CGNAT addresses](https://tailscale.com/kb/1015/100.x-addresses).

IPv4 addresses are currently only used for built-in addresses.

## IPv6 ranges

For IPv6, `fd00:6879:7072:7370:/64` is used, based on [RFC 4193: Unique Local IPv6 Unicast Addresses](https://datatracker.ietf.org/doc/html/rfc4193).

`fd00:6879:7072:7370:6163:6500::/96` is the range used for built-in IPv6 addresses. The last 32 bits are used for the node identifier. The hex value `0x6879_7072_7370_6163_65` is the ASCII string `hyprspace`.

`fd00:6879:7072:7370:7376::/80` is used for the [[service-network]]. The first 32 bits of the host part represent the node identifier, which is identical to the node identifier of built-in addresses. The last 16 bits represent the service identifier.

## Known Issues

Because the IP address space is more limited than the typical hash space of PeerIDs, automatically generated IP addresses may conflict.
