package cli

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/hyprspace/hyprspace/rpc"
)

const pollInterval = 2 * time.Second

// TUI starts the interactive TUI dashboard.
var TUI = cmd.Sub{
	Name:  "tui",
	Short: "Interactive TUI dashboard for monitoring Hyprspace",
	Run:   TUIRun,
}

func TUIRun(r *cmd.Root, c *cmd.Sub) {
	ifName := r.Flags.(*GlobalFlags).InterfaceName
	if ifName == "" {
		ifName = "hyprspace"
	}
	runTUI(ifName)
}

// runTUI starts the TUI dashboard application. It blocks until the user quits.
func runTUI(ifName string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tview.NewApplication()
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault

	statusView := tview.NewTextView()
	statusView.SetDynamicColors(true)
	statusView.SetWordWrap(false)
	statusView.SetScrollable(true)
	statusView.SetBorder(true)
	statusView.SetTitle(" Status ")

	peersView := tview.NewTextView()
	peersView.SetDynamicColors(true)
	peersView.SetScrollable(true)
	peersView.SetWrap(true)
	peersView.SetWordWrap(false)
	peersView.SetBorder(true)
	peersView.SetTitle(" Peers ")

	routesTable := tview.NewTable()
	routesTable.SetBorders(false)
	routesTable.SetSelectable(false, false)
	routesTable.SetBorder(true)
	routesTable.SetTitle(" Routes ")

	quitHint := tview.NewTextView()
	quitHint.SetDynamicColors(true)
	quitHint.SetTextAlign(tview.AlignCenter)
	quitHint.SetText("[gray]Tab to rotate panes · ↑↓ to scroll bottom · q / Esc / Ctrl-C to quit[-]")

	// panes holds the three views assigned to [top-left, top-right, bottom].
	// Tab rotates them clockwise: top-left → top-right → bottom → top-left.
	panes := []tview.Primitive{statusView, peersView, routesTable}

	buildLayout := func() {
		topRow := tview.NewFlex().SetDirection(tview.FlexColumn)
		topRow.AddItem(panes[0], 0, 1, false)
		topRow.AddItem(panes[1], 0, 1, false)
		flex := tview.NewFlex().SetDirection(tview.FlexRow)
		flex.AddItem(topRow, 0, 1, false)
		flex.AddItem(panes[2], 0, 1, false)
		flex.AddItem(quitHint, 1, 0, false)
		app.SetRoot(flex, true)
		app.SetFocus(panes[2])
	}

	buildLayout()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			// Clockwise: each view advances one slot (top-left→top-right→bottom→top-left).
			panes[0], panes[1], panes[2] = panes[2], panes[0], panes[1]
			buildLayout()
			return nil
		case tcell.KeyESC:
			cancel()
			app.Stop()
			return nil
		case tcell.KeyCtrlC:
			cancel()
			app.Stop()
			return nil
		}
		if event.Rune() == 'q' || event.Rune() == 'Q' {
			app.Stop()
			return nil
		}
		return event
	})

	// Poll goroutine — immediately fetches, then polls every 2s.
	go func() {
		fetchAndUpdate(app, ifName, statusView, peersView, routesTable)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fetchAndUpdate(app, ifName, statusView, peersView, routesTable)
			case <-ctx.Done():
				return
			}
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func fetchAndUpdate(
	app *tview.Application,
	ifName string,
	statusView *tview.TextView,
	peersView *tview.TextView,
	routesTable *tview.Table,
) {
	status, err := rpc.TryStatus(ifName)
	if err != nil {
		app.QueueUpdateDraw(func() {
			statusView.Clear()
			fmt.Fprintf(statusView, "[red]⚠ RPC connection failed: %v[-]\n\n", err)
			peersView.Clear()
			fmt.Fprintf(peersView, "[gray]No data (RPC unavailable)[-]")
			routesTable.Clear()
			routesTable.SetCell(0, 0, tview.NewTableCell("[gray]No data (RPC unavailable)[-]"))
		})
		return
	}

	// Fetch routes in the same poll cycle.
	routeReply, routeErr := rpc.TryRoute(ifName, rpc.RouteArgs{Action: rpc.Show})

	app.QueueUpdateDraw(func() {
		updateStatusView(statusView, status)
		updatePeersView(peersView, status)
		if routeErr != nil {
			routesTable.Clear()
			routesTable.SetCell(0, 0, tview.NewTableCell("[red]⚠ Route data unavailable[-]"))
		} else {
			updateRoutesTable(routesTable, routeReply)
		}
	})
}

func updateStatusView(v *tview.TextView, s rpc.StatusReply) {
	v.Clear()
	fmt.Fprintf(v, "Peer ID:        [yellow]%s[-]\n", s.PeerID)
	fmt.Fprintf(v, "Swarm Peers:    %d\n", s.SwarmPeersCurrent)
	fmt.Fprintf(v, "VPN Nodes:      %d/%d\n", s.NetPeersCurrent, s.NetPeersMax)

	if len(s.ListenAddrs) > 0 {
		fmt.Fprintf(v, "\nListen Addresses:\n")
		for _, addr := range s.ListenAddrs {
			disp := addr
			if strings.HasSuffix(addr, "/p2p-circuit") || strings.Contains(addr, "/p2p-circuit/p2p/") {
				disp = "[gray]" + addr + "[-]"
			}
			fmt.Fprintf(v, "  %s\n", disp)
		}
	}
}

func updatePeersView(v *tview.TextView, s rpc.StatusReply) {
	v.Clear()
	if len(s.NetPeerAddrsCurrent) == 0 {
		fmt.Fprintf(v, "[gray]No peers connected.[-]")
		return
	}
	fmt.Fprintf(v, "[::b]%-20s %-12s %s[-]\n", "Name", "Latency", "Multiaddr")
	for _, entry := range s.NetPeerAddrsCurrent {
		name, latency, addr := parsePeerEntry(entry)
		fmt.Fprintf(v, "%-20s %-12s [gray]%s[-]\n", name, latency, addr)
	}
}

func updateRoutesTable(t *tview.Table, reply rpc.RouteReply) {
	t.Clear()
	if len(reply.Routes) == 0 {
		t.SetCell(0, 0, tview.NewTableCell("[gray]No routes configured.[-]"))
		return
	}

	// Header row.
	t.SetCell(0, 0, tview.NewTableCell("[::b]Network").SetSelectable(false))
	t.SetCell(0, 1, tview.NewTableCell("[::b]Target").SetSelectable(false))
	t.SetCell(0, 2, tview.NewTableCell("[::b]Status").SetSelectable(false))
	t.SetCell(0, 3, tview.NewTableCell("[::b]Relay").SetSelectable(false))

	for i, r := range reply.Routes {
		row := i + 1
		target := r.TargetName
		if target == "" {
			target = r.TargetAddr.String()
		}
		status := "[green]connected[-]"
		if !r.IsConnected {
			status = "[red]disconnected[-]"
		}
		relay := ""
		if r.IsRelay {
			relay = r.RelayAddr.String()
		}
		t.SetCell(row, 0, tview.NewTableCell(r.Network.String()).SetExpansion(0))
		t.SetCell(row, 1, tview.NewTableCell(target).SetExpansion(1))
		t.SetCell(row, 2, tview.NewTableCell(status).SetExpansion(0))
		t.SetCell(row, 3, tview.NewTableCell(relay).SetExpansion(1))
	}
}

// parsePeerEntry extracts name, latency, and multiaddr from a peer entry string.
// Input format: @name (latency) multiaddr/p2p/peerid
func parsePeerEntry(s string) (name, latency, addr string) {
	s = strings.TrimPrefix(s, "@")
	name, rest, _ := strings.Cut(s, " ")
	latency, rest, _ = strings.Cut(rest, ") ")
	latency = strings.TrimLeft(latency, "(")
	addr = rest
	return
}
