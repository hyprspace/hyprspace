<img src="hyprspace.png" width="150">

# Hyprspace

A Lightweight VPN Built on top of IPFS & Libp2p for Truly Distributed Networks. 

[Documentation](https://docs.hyprspace.privatevoid.net/)

https://user-images.githubusercontent.com/19558067/152407636-a5f4ae1f-9493-4346-bf73-0de109928415.mp4


## Table of Contents
- [About the Fork](#about-the-fork)
- [A Bit of Backstory](#a-bit-of-backstory)
- [Use Cases](#use-cases)
  - [A Digital Nomad](#a-digital-nomad)
  - [A Privacy Advocate](#a-privacy-advocate)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
  - [Commands](#commands)
- [Tutorial](#tutorial)

## About the Fork

This fork has started on [Sep 25, 2022 in the Depot monorepo](https://github.com/privatevoid-net/depot/commit/9df627a8d2b82a1dad06e3c7b22fae263c8cd0dc), when I (@max-privatevoid) decided I'd hack on the project a bit and improve it in ways that would benefit my personal use cases. This has led to some significant changes.

The original author of this project has [archived it on Feb 24, 2024](https://github.com/alecbcs/hyprspace). I've extracted my modified codebase from the monorepo, as a way to revive the project for others who also want to continue using and improving it.

## A Bit of Backstory
[Libp2p](https://libp2p.io) is a networking library created by [Protocol Labs](https://protocol.ai) that allows nodes to discover each other using a Distributed Hash Table. Paired with [NAT hole punching](https://en.wikipedia.org/wiki/Hole_punching_(networking)) this allows Hyprspace to create a direct encrypted tunnel between two nodes even if they're both behind firewalls.

**Moreover! Each node doesn't even need to know the other's ip address prior to starting up the connection.** This makes Hyprspace perfect for devices that frequently migrate between locations but still require a constant virtual ip address.

### So How Does Hyprspace Compare to Something Like Wireguard?
[WireGuard](https://wireguard.com) is an amazing VPN written by Jason A. Donenfeld. If you haven't already, definitely go check it out! WireGuard actually inspired me to write Hyprspace. That said, although WireGuard is in a class of its own as a great VPN, it requires at least one of your nodes to have a public IP address. In this mode, as long as one of your nodes is publicly accessible, it can be used as a central relay to reach the other nodes in the network. However, this means that all of the traffic for your entire system is going through that one system which can slow down your network and make it fragile in the case that node goes down and you lose the whole network. So instead say that you want each node to be able to directly connect to each other as they do in Hyprspace. Unfortunately through WireGuard this would require every node to be publicly addressable which means manual port forwarding and no travelling nodes.

By contrast Hyprspace allows all of your nodes to connect directly to each other creating a strong reliable network even if they're all behind their own NATs/firewalls. No manual port forwarding required! 

## Use Cases:
##### A Digital Nomad
I use this system when travelling, if I'm staying in a rental or hotel and want to try something out on a Raspberry Pi I can plug the Pi into the location's router or ethernet port and then just ssh into the system using the same-old internal Hyprspace ip address without having to worry about their NAT or local firewall. Furthermore, if I'm connected to the Virtual Hyprspace Network I can ssh into my machines at home without requiring me to set up any sort of port forwarding.

##### A Privacy Advocate
Honestly, I even use this system when I'm at home and could connect directly to my local infrastructure. Using Hyprspace however, I don't have to trust the security of my local network and Hyprspace will intelligently connect to my machines using their local ip addresses for maximum speed.

If anyone else has some use cases please add them! Pull requests welcome!

| :exclamation: | Hyprspace is still a very new project. Although we've tested the code locally for security, it hasn't been audited by a third party yet. We probably wouldn't trust it yet in high security environments. |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |

## Getting Started

### Prerequisites

If you're running Hyprspace on Windows you'll need to install [tap-windows](http://build.openvpn.net/downloads/releases/).

## Usage

### Commands

| Command             |  Alias  | Description                                                                |
| ------------------- | ------- | -------------------------------------------------------------------------- |
| `help`              | `?`     | Get help with a specific subcommand.                                       |
| `init`              | `i`     | Initialize an interface's configuration.                                   |
| `up`                | `up`    | Create and bring up a Hyprspace interface                                  |
| `status`            | `s`     | Inspect the status of a Hyprspace daemon                                   |
| `peers`             |         | List connected LibP2P peers                                                |
| `route`             | `r`     | Inspect and modify the route table                                         |

### Global Flags
| Flag                |  Alias  | Description                                                                |
| ------------------- | ------- | -------------------------------------------------------------------------- |
| `--config`          | `-c`    | Specify the path to a hyprspace config for an interface.                   |
| `--interface`       | `-i`    | The Hyprspace interface to operate on.                                     |


## Tutorial

### Initializing an Interface

The first thing we'll want to do once we've got Hyprspace installed is
initialize the configuration for an interface. The default interface name is `hyprspace`.
In this example we'll call the interface on our local machine `hs0` (for hypr-space 0) and `hs1` on our remote server but yours could be anything you'd like.

(Note: if you're using a Mac you'll have to use the interface name `utun[0-9]`. Check which interfaces are already in use by running `ip a` once you've got `iproute2mac` installed.)

(Note: if you're using Windows you'll have to use the interface name as seen in Control Panel. IP address will be set automatically only if you run Hyprspace as Administrator.)

###### Local Machine
```shell-session
$ sudo hyprspace init -i hs0
```

###### Remote Machine
```shell-session
$ sudo hyprspace init -i hs1
```

### Add Each Machine As A Peer Of The Other

Now that we've got a set of configurations we'll want to
tell the machines about each other. By default Hyprspace will
put the interface configurations in `/etc/hyprspace/interface-name.json`.
You can also create the config file elsewhere by specifying a custom config path.

```shell-session
$ hyprspace init -c hyprspace-config-hs0.json
```

So for our example we'll run

###### Local Machine
```shell-session
$ sudo $EDITOR /etc/hyprspace/hs0.json
```

and

###### Remote Machine
```shell-session
$ sudo $EDITOR /etc/hyprspace/hs1.json
```

### Update Peer Configs

Now in each config we'll add the other machine's ID as a peer.
Hyprspace will print a config snippet like this one and instruct you to add it to your other peers:
```json
{
  "name": "hostname1",
  "id": "12D3KExamplePeer1"
}
```

Update

```json
{
  "peers": [],
  "privateKey": "z23ExamplePrivateKey"
}
```
to 
```json
{
  "peers": [
    {
      "name": "hostname1",
      "id": "12D3KExamplePeer1"
    }
  ],
  "privateKey": "z23ExamplePrivateKey"
}
```

Previously, it was necessary to manually configure IP addresses for all peers. Now, an IP address from the 100.64.0.0/16 range is automatically allocated to each peer based on its peer ID.

You can specify additional routes for each peer as well:

```json
{
  "peers": [
    {
      "name": "hostname1",
      "id": "12D3KExamplePeer1"
      "routes": [
        { "net": "10.1.1.1/32" },
        { "net": "10.1.2.0/24" }
      ]
    }
  ],
  "privateKey": "z23ExamplePrivateKey"
}
```

### Starting Up the Interfaces!
Now that we've got our configs all sorted we can start up the two interfaces!

###### Local Machine
```shell-session
$ sudo hyprspace up -i hs0
```

and

###### Remote Machine
```shell-session
$ sudo hyprspace up -i hs1
```

After a few seconds you should see a the network finish setting up
and find your other machine. We can now test the connection by
pinging back and forth across the network.

###### Local Machine
```shell-session
$ ping 100.64.90.181
```

We can get some more information about the status of the network as well.

###### Local Machine
```shell-session
$ sudo hyprspace status -i hs0
PeerID: 12D3KExamplePeer1
Swarm peers: 7
Connected VPN nodes: 1/1
    @hostname2 (365.516µs) /ip4/.../udp/8001/quic-v1/p2p/12D3KExamplePeer2
Addresses:
    /ip4/.../tcp/8001
    (...)
```

### Stopping the Interface and Cleaning Up
Now to stop the interface and clean up the system, simply kill the proceses (for example, by pressing Ctrl+C where you started it).

## Disclaimer & Copyright

WireGuard is a registered trademark of Jason A. Donenfeld.

## License

Copyright 2022-2024 Max Headroom <max@privatevoid.net>

Copyright 2021-2022 Alec Scott <hi@alecbcs.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
