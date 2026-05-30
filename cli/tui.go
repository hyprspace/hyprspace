package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/hyprspace/hyprspace/rpc"
)

const pollInterval = 2 * time.Second

var tabs = []struct {
	id    string
	title string
}{
	{"status", "Status"},
	{"peers", "Peers"},
	{"routes", "Routes"},
}

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
	app := tview.NewApplication()
	pages := tview.NewPages()

	navBar := tview.NewTextView()
	navBar.SetDynamicColors(true)
	navBar.SetTextAlign(tview.AlignCenter)
	navBar.SetTextStyle(tcell.StyleDefault.Background(tcell.ColorDarkSlateGray))

	statusView := tview.NewTextView()
	statusView.SetDynamicColors(true)
	statusView.SetWordWrap(true)
	statusView.SetScrollable(true)
	statusView.SetBorder(true)
	statusView.SetTitle(" Status ")

	peersTable := tview.NewTable()
	peersTable.SetBorders(false)
	peersTable.SetSelectable(false, false)
	peersTable.SetBorder(true)
	peersTable.SetTitle(" Peers ")

	routesTable := tview.NewTable()
	routesTable.SetBorders(false)
	routesTable.SetSelectable(false, false)
	routesTable.SetBorder(true)
	routesTable.SetTitle(" Routes ")

	pages.AddPage("status", statusView, true, true)
	pages.AddPage("peers", peersTable, true, false)
	pages.AddPage("routes", routesTable, true, false)

	currentTab := 0
	updateNavBar(navBar, currentTab)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(navBar, 1, 0, false).
		AddItem(pages, 0, 1, true)

	app.SetRoot(flex, true)

	switchTab := func(idx int) {
		if idx < 0 {
			idx = len(tabs) - 1
		} else if idx >= len(tabs) {
			idx = 0
		}
		currentTab = idx
		pages.SwitchToPage(tabs[idx].id)
		updateNavBar(navBar, idx)
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			switchTab(currentTab + 1)
			return nil
		case tcell.KeyBacktab:
			switchTab(currentTab - 1)
			return nil
		case tcell.KeyESC:
			app.Stop()
			return nil
		case tcell.KeyCtrlC:
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
		fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for range ticker.C {
			fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
		}
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func updateNavBar(navBar *tview.TextView, active int) {
	var b strings.Builder
	for i, tab := range tabs {
		if i > 0 {
			b.WriteString("  ")
		}
		if i == active {
			fmt.Fprintf(&b, "[white:darkcyan] %s ", tab.title)
		} else {
			fmt.Fprintf(&b, "[gray:darkolivegreen] %s ", tab.title)
		}
	}
	navBar.SetText(b.String())
}

func fetchAndUpdate(
	app *tview.Application,
	ifName string,
	statusView *tview.TextView,
	peersTable *tview.Table,
	routesTable *tview.Table,
) {
	status, err := rpc.TryStatus(ifName)
	if err != nil {
		app.QueueUpdateDraw(func() {
			statusView.Clear()
			fmt.Fprintf(statusView, "[red]⚠ RPC connection failed: %v[-]\n\n", err)
			peersTable.Clear()
			peersTable.SetCell(0, 0, tview.NewTableCell("[gray]No data (RPC unavailable)[-]"))
			routesTable.Clear()
			routesTable.SetCell(0, 0, tview.NewTableCell("[gray]No data (RPC unavailable)[-]"))
		})
		return
	}

	// Fetch routes in the same poll cycle.
	routeReply, routeErr := rpc.TryRoute(ifName, rpc.RouteArgs{Action: rpc.Show})
	if routeErr != nil {
		app.QueueUpdateDraw(func() {
			statusView.Clear()
			fmt.Fprintf(statusView, "[red]⚠ RPC route call failed: %v[-]\n\n", routeErr)
			routesTable.Clear()
			routesTable.SetCell(0, 0, tview.NewTableCell("[gray]No data (RPC unavailable)[-]"))
		})
		return
	}

	app.QueueUpdateDraw(func() {
		updateStatusView(statusView, status)
		updatePeersTable(peersTable, status)
		updateRoutesTable(routesTable, routeReply)
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

func updatePeersTable(t *tview.Table, s rpc.StatusReply) {
	t.Clear()
	if len(s.NetPeerAddrsCurrent) == 0 {
		t.SetCell(0, 0, tview.NewTableCell("[gray]No peers connected.[-]"))
		return
	}

	// Header row.
	t.SetCell(0, 0, tview.NewTableCell("[::b]Name").SetSelectable(false))
	t.SetCell(0, 1, tview.NewTableCell("[::b]Latency").SetSelectable(false))
	t.SetCell(0, 2, tview.NewTableCell("[::b]Multiaddr").SetSelectable(false))

	for i, entry := range s.NetPeerAddrsCurrent {
		row := i + 1
		name, latency, addr := parsePeerEntry(entry)
		t.SetCell(row, 0, tview.NewTableCell(name).SetExpansion(0))
		t.SetCell(row, 1, tview.NewTableCell(latency).SetExpansion(0))
		t.SetCell(row, 2, tview.NewTableCell(addr).SetExpansion(1))
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
	t.SetCell(0, 2, tview.NewTableCell("[::b]Relay").SetSelectable(false))
	t.SetCell(0, 3, tview.NewTableCell("[::b]Status").SetSelectable(false))

	for i, r := range reply.Routes {
		row := i + 1
		target := r.TargetName
		if target == "" {
			target = r.TargetAddr.String()
		}
		relay := ""
		if r.IsRelay {
			relay = r.RelayAddr.String()
		}
		status := "[green]connected[-]"
		if !r.IsConnected {
			status = "[red]disconnected[-]"
		}
		t.SetCell(row, 0, tview.NewTableCell(r.Network.String()).SetExpansion(0))
		t.SetCell(row, 1, tview.NewTableCell(target).SetExpansion(1))
		t.SetCell(row, 2, tview.NewTableCell(relay).SetExpansion(1))
		t.SetCell(row, 3, tview.NewTableCell(status).SetExpansion(0))
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
