# Service Network

The Service Network is a completely virtual network used to provide support for HTTP-style virtual hosting, except generically at the IP layer.

## Use case description

A Hyprspace node runs a web service on port 80. You can access said service at `http://mynode.hyprspace`. Now you want to add another web service, but you don't want to use a different port, because `http://mynode.hyprspace:81` is a really ugly URL.

You could set up a reverse proxy and make use of HTTP virtual hosting. Now you can use URLs such as `http://web.mynode.hyprspace` and `http://other-service.mynode.hyprspace`. But what if you don't want to run a reverse proxy? And what if you want to run non-HTTP services?

## Configuration

The config key `services` is used to configure the services a particular node should provide. The target addresses are defined in [Multiaddr](https://multiformats.io/multiaddr/) format. If only a port is given, the IP address is assumed to be `/ip4/127.0.0.1`.

```json
{
  "services": {
    "example": "/tcp/8080",
    "other": "/ip4/10.0.0.4/tcp/9092"
  }
}
```
This will make the services `example.mynode.hyprspace` and `other.mynode.hyrpspace` available. Upon connecting to the `example` service, the node establishes a connection to `/ip4/127.0.0.1/tcp/8080` and will relay all traffic between the service and the client.


## Implementation details

### Hostnames and addresses

To be able to address individual services by hostname, we need to assign an individual IP address to each service. These IP addresses are auto-generated based on the name of the service as well as the PeerID of the node providing the service. The DNS name of the service is a subdomain of the node's hostname.

### Ports

Regardless of what application protocol is in use, the service should always be accessible on the default port for that protocol. To facilitate this, the service effectively listens on all ports. This is accomplished by inspecting the packets destined to a particular service and dynamically creating a listener on the destination port specified in the packet.

### Protocol support

Currently, only TCP services are supported.

### Optimized TCP forwarding

Traffic handling in the Service Network is not implemented by simply forwarding IP packets. Instead, all TCP traffic is terminated locally using [gVisor's userspace TCP/IP stack](https://github.com/google/gvisor/tree/master/pkg/tcpip). Once decapsulated, the raw data stream is sent to the other peer, which in turn connects to the real service via TCP and forwards the data stream to it. This makes traffic in the Service Network less susceptible to TCP meltdown and may improve network performance if QUIC is used for the underlying connection.
