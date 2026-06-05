# TUI Dashboard

Hyprspace includes a read-only terminal dashboard for monitoring a running node in real time.

```
hyprspace tui [-i <interface>]
```

The `-i` flag selects the network interface to monitor. It defaults to `hyprspace` if omitted. The daemon must already be running; the TUI polls its existing RPC socket and requires no server-side changes.

State is refreshed every 2 seconds.

## Layout

The layout is determined once at startup based on the terminal size. Resizing the terminal during a session requires restarting the TUI.

### Large terminals (≥ 20 rows and ≥ 100 columns)

All three panes are visible simultaneously:

```
┌─────────────────────┬─────────────────────┐
│  Status             │  Peers              │
├─────────────────────┴─────────────────────┤
│  Routes                                   │
├───────────────────────────────────────────┤
│  Tab to cycle focus · q / Esc / Ctrl-C    │
└───────────────────────────────────────────┘
```

`Tab` cycles keyboard focus between the three panes. The focused pane is highlighted with a yellow border and responds to arrow key scrolling.

### Small terminals (< 20 rows or < 100 columns)

A single pane fills the screen. `Tab` cycles through the panes.

```
┌───────────────────────────────────────────┐
│  Peers                                    │
├───────────────────────────────────────────┤
│  peers · Tab to cycle · q / Esc / Ctrl-C  │
└───────────────────────────────────────────┘
```

## Panes

### Status

Displays a summary of the local node:

| Field | Source |
|-------|--------|
| Peer ID | Node's libp2p PeerID (highlighted in yellow) |
| Swarm Peers | Number of currently connected swarm peers |
| VPN Nodes | Connected VPN nodes out of the configured total |
| Listen Addresses | Active listen addresses; relay addresses are shown in gray |

### Peers

Lists the nodes currently connected to the VPN, one per line:

```
Name                 Latency      Multiaddr
alice                5ms          /ip4/1.2.3.4/tcp/9090/p2p/QmXXX…
```

Long multiaddrs wrap at the pane boundary rather than being truncated.

### Routes

Lists all configured routes:

| Column | Description |
|--------|-------------|
| Network | The destination network (CIDR) |
| Target | Target node name or address |
| Status | `connected` (green) or `disconnected` (red) |
| Relay | Relay address, if the route is using a relay |

## Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus between panes (large) / cycle active pane (small) |
| `↑` / `↓` | Scroll the focused pane |
| `q` / `Q` | Quit |
| `Esc` | Quit |
| `Ctrl-C` | Quit |

## Error handling

| Condition | Behaviour |
|-----------|-----------|
| Daemon not running or socket missing | Red error message in the Status pane; Peers and Routes show "No data (RPC unavailable)". Polling continues. |
| Status RPC succeeds, Route RPC fails | Status and Peers update normally; Routes shows "Route data unavailable" in red. |
| No peers connected | Peers pane shows "No peers connected." |
| No routes configured | Routes pane shows "No routes configured." |
