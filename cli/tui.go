package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/hyprspace/hyprspace/rpc"
)

const pollInterval = 2 * time.Second

type viewMode int

const (
	modeAll viewMode = iota
	modeStatus
	modePeers
	modeRoutes
)

const (
	smallMinHeight = 20
	smallMinWidth  = 100
)

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

	statusView := tview.NewTextView()
	statusView.SetDynamicColors(true)
	statusView.SetWordWrap(false)
	statusView.SetScrollable(true)
	statusView.SetBorder(true)
	statusView.SetTitle(" Status ")

	peersTable := tview.NewTable()
	peersTable.SetBorders(false)
	peersTable.SetSelectable(true, false)
	peersTable.SetBorder(true)
	peersTable.SetTitle(" Peers ")

	routesTable := tview.NewTable()
	routesTable.SetBorders(false)
	routesTable.SetSelectable(true, false)
	routesTable.SetBorder(true)
	routesTable.SetTitle(" Routes ")

	quitHint := tview.NewTextView()
	quitHint.SetDynamicColors(true)
	quitHint.SetTextAlign(tview.AlignCenter)

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 100, 24
	}
	isSmall := h < smallMinHeight || w < smallMinWidth

	var defaultModes []viewMode
	if isSmall {
		defaultModes = []viewMode{modeStatus, modePeers, modeRoutes}
	} else {
		defaultModes = []viewMode{modeAll, modePeers, modeRoutes}
	}

	mode := defaultModes[0]

	// focusedInAll tracks which pane has keyboard focus in modeAll.
	focusedInAll := 0
	allPanes := []tview.Primitive{statusView, peersTable, routesTable}

	var updateAllFocus func()
	updateAllFocus = func() {
		statusView.SetBorderColor(tcell.ColorDefault)
		peersTable.SetBorderColor(tcell.ColorDefault)
		routesTable.SetBorderColor(tcell.ColorDefault)
		switch focusedInAll {
		case 0:
			statusView.SetBorderColor(tcell.ColorYellow)
		case 1:
			peersTable.SetBorderColor(tcell.ColorYellow)
		case 2:
			routesTable.SetBorderColor(tcell.ColorYellow)
		}
		app.SetFocus(allPanes[focusedInAll])
	}

	rebuildLayout := func() {
		flex := tview.NewFlex().SetDirection(tview.FlexRow)

		switch mode {
		case modeAll:
			topRow := tview.NewFlex().SetDirection(tview.FlexColumn)
			topRow.AddItem(statusView, 0, 1, false)
			topRow.AddItem(peersTable, 0, 1, false)
			flex.AddItem(topRow, 0, 1, false)
			flex.AddItem(routesTable, 0, 1, false)
			quitHint.SetText("[gray]Tab to cycle focus · q / Esc / Ctrl-C to quit[-]")
		case modeStatus:
			flex.AddItem(statusView, 0, 1, false)
			quitHint.SetText("[gray]status · Tab to cycle · q / Esc / Ctrl-C to quit[-]")
		case modePeers:
			flex.AddItem(peersTable, 0, 1, false)
			quitHint.SetText("[gray]peers · Tab to cycle · q / Esc / Ctrl-C to quit[-]")
		case modeRoutes:
			flex.AddItem(routesTable, 0, 1, false)
			quitHint.SetText("[gray]routes · Tab to cycle · q / Esc / Ctrl-C to quit[-]")
		}

		flex.AddItem(quitHint, 1, 0, false)
		app.SetRoot(flex, true)
		if mode == modeAll {
			updateAllFocus()
		}
	}

	rebuildLayout()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			if isSmall {
				for i, m := range defaultModes {
					if m == mode {
						mode = defaultModes[(i+1)%len(defaultModes)]
						break
					}
				}
				rebuildLayout()
			} else {
				focusedInAll = (focusedInAll + 1) % len(allPanes)
				updateAllFocus()
			}
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
		fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
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

	app.QueueUpdateDraw(func() {
		updateStatusView(statusView, status)
		updatePeersTable(peersTable, status)
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