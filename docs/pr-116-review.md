# PR Review: TUI Implementation

Review of commit `24dd159` on branch `tui-poc`, adding the `hyprspace tui` subcommand. Each issue below includes a precise fix with exact file paths, line references, and side-effect warnings so another developer can implement the changes from this document alone.

---

## Critical Issues (will cause crashes in production)

---

### 1. RPC socket file descriptor leak (will crash in ~34 minutes)

**Severity:** Critical — the TUI will exhaust file descriptors and crash with "too many open files" within 30–40 minutes of uptime.

**Root cause:** `rpc/client.go` — the three `Try*` functions (`TryStatus`, `TryRoute`, `TryPeers`) open a new Unix socket connection every time they're called and never close it. The original `Status()`, `Route()`, `Peers()` functions have the same pattern, but those are one-shot CLI commands so the process exits before FDs become a problem. The TUI calls these every 2 seconds, accumulating ~30 leaked FDs per minute.

**File:** `rpc/client.go`

**Implementation steps:**

1a. Add `defer client.Close()` to `TryStatus` right after the successful `rpc.Dial` call, before the early-return error branches:

Current code (~lines 50–60):
```go
func TryStatus(ifname string) (StatusReply, error) {
	client, err := rpc.Dial("unix", fmt.Sprintf("/run/hyprspace-rpc.%s.sock", ifname))
	if err != nil {
		return StatusReply{}, err
	}
	var reply StatusReply
	if err := client.Call("HyprspaceRPC.Status", new(Args), &reply); err != nil {
		return StatusReply{}, err
	}
	return reply, nil
}
```

After fix:
```go
func TryStatus(ifname string) (StatusReply, error) {
	client, err := rpc.Dial("unix", fmt.Sprintf("/run/hyprspace-rpc.%s.sock", ifname))
	if err != nil {
		return StatusReply{}, err
	}
	defer client.Close()
	var reply StatusReply
	if err := client.Call("HyprspaceRPC.Status", new(Args), &reply); err != nil {
		return StatusReply{}, err
	}
	return reply, nil
}
```

1b. Same one-line addition (`defer client.Close()`) for `TryPeers` (~line 63) and `TryRoute` (~line 76).

**Side effects:** None. `net/rpc.Client.Close()` is safe to call after the connection has been used. The one-shot `Status()`, `Route()`, `Peers()` functions have the same leak but are left untouched because they run once and exit — fixing them is optional but recommended for consistency.

**Verification:** Build with `go build ./...`. Run TUI for 30s, check `/proc/<pid>/fd/` — only one or two socket FDs should exist, not growing. Run `ulimit -n 64` then run TUI — should not crash after 30s.

---

### 2. Goroutine leak + possible panic on shutdown

**Severity:** Critical — the poll goroutine runs forever after the user quits, calling `QueueUpdateDraw` on a stopped `tview.Application`, which can panic or deadlock.

**Root cause:** `cli/tui.go` — the poll goroutine (~line 108) has no mechanism to be told to stop. When the user presses `q`/`Esc`/`Ctrl+C`, `app.Stop()` is called via `SetInputCapture`, which unblocks `app.Run()`, but the goroutine keeps iterating and calling `fetchAndUpdate` → `app.QueueUpdateDraw(...)`.

**File:** `cli/tui.go`

**Implementation steps:**

2a. Add `"context"` to the import block.

2b. In `runTUI`, create a cancellable context at the top of the function:

```go
import (
    "context"
    // ... existing imports
)

func runTUI(ifName string) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    // ... rest of function
```

2c. Replace the simple `for range ticker.C` loop in the poll goroutine with a select that also watches `ctx.Done()`:

Current code:
```go
go func() {
    fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()
    for range ticker.C {
        fetchAndUpdate(app, ifName, statusView, peersTable, routesTable)
    }
}()
```

After fix:
```go
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
```

2d. In the `SetInputCapture` handler, replace all direct `app.Stop()` calls with `cancel()` followed by `app.Stop()`:

Current pattern (appears 3 times: `KeyESC`, `KeyCtrlC`, and `'q'` rune):
```go
case tcell.KeyESC:
    app.Stop()
    return nil
```

After fix (for all three quit triggers):
```go
case tcell.KeyESC:
    cancel()
    app.Stop()
    return nil
```

**Side effects:** `cancel()` is idempotent — calling it multiple times (e.g. user mashes `q`) is safe. The goroutine may execute one more `fetchAndUpdate` after `cancel()` if the ticker fires before the select observes the channel close — this is harmless because `QueueUpdateDraw` on a stopped app is a no-op in tview.

**Verification:** Build, start TUI, press `q`. Process should exit cleanly. No goroutine should remain (check `runtime.NumGoroutine()` or `SIGQUIT` dump).

---

## Important Issues

---

### 3. Duplicated socket path string

**Severity:** Medium — maintainability hazard. If the socket path changes, the three `Try*` functions and `connect()` will diverge silently.

**Root cause:** The path format `/run/hyprspace-rpc.%s.sock` is hardcoded in 4 places in `rpc/client.go`: `connect()` (line 12), `TryStatus` (line 53), `TryPeers` (line 64), `TryRoute` (line 76).

**File:** `rpc/client.go`

**Implementation steps:**

3a. Add a package-level helper function:

```go
func socketPath(ifname string) string {
    return fmt.Sprintf("/run/hyprspace-rpc.%s.sock", ifname)
}
```

3b. Replace all 4 inline `fmt.Sprintf("/run/hyprspace-rpc.%s.sock", ifname)` calls with `socketPath(ifname)`.

**Side effects:** None — pure refactor, identical behavior.

**Verification:** `go build ./...` — no errors. `go vet ./rpc/...` — clean.

---

### 4. Inconsistent error handling when route RPC fails

**Severity:** Medium — UX bug. When `TryStatus` succeeds but `TryRoute` fails, the status tab is overwritten with an error message, discarding valid peer data. The peers table is not updated at all (shows stale data from last successful poll).

**Root cause:** `cli/tui.go`, `fetchAndUpdate` — the route-failure branch (~line 157) clears and rewrites `statusView` with the route error, and doesn't call `updatePeersTable`, so the peers table retains data from the previous poll cycle.

**File:** `cli/tui.go`, function `fetchAndUpdate`

**Implementation steps:**

Replace the entire route-failure branch:

Current code (~lines 147–159):
```go
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
```

After fix:
```go
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
```

This restructures `fetchAndUpdate` so there's a single `QueueUpdateDraw` call that always updates status + peers (which come from the same RPC call), and conditionally updates routes. The early-return pattern is removed — no error branch clears valid data.

**Side effects:** This changes the control flow of `fetchAndUpdate` significantly. Verify that:
- When both RPC calls succeed: all three tabs update (same as before)
- When status RPC fails: three-way error banner (same as before)
- When status succeeds but routes fail: status + peers update, routes show error (new behavior — previously cleared status)

**Verification:** Test all three scenarios manually. The simplest way to test partial failure is to connect the TUI, then stop the daemon — status should show the error first (status RPC fails). To test route-specific failure, mock would be needed; alternatively, just verify the code path is logically correct.

---

## Minor Issues

---

### 5. `panic(err)` instead of `log.Fatal`

**Severity:** Low — cosmetic, but inconsistent with codebase conventions.

**Root cause:** `cli/tui.go`, line ~130 (the end of `runTUI`):
```go
if err := app.Run(); err != nil {
    panic(err)
}
```

Every other error exit in the `cli` package uses `log.Fatal` (or the `checkErr` helper). `panic` prints a stack trace which is noisy and confusing for CLI users.

**File:** `cli/tui.go`

**Implementation steps:**

5a. Add `"log"` to the import block in `cli/tui.go` (it's already imported in `cli/root.go` but `cli/tui.go` is a separate file in the same package, so it needs its own import).

5b. Replace:
```go
if err := app.Run(); err != nil {
    panic(err)
}
```
with:
```go
if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

**Side effects:** None. `log.Fatal` calls `os.Exit(1)` internally, same final effect as `panic` but without the stack trace.

**Verification:** `go build ./...` — clean.

---

### 6. Poll goroutine posts updates after `app.Stop()`

**Severity:** Low — theoretical race. Even with the context fix from issue #2, between `cancel()` and the goroutine observing `ctx.Done()`, one more `fetchAndUpdate` may execute and call `QueueUpdateDraw` on a stopped `tview.Application`. tview's `QueueUpdateDraw` is documented as a no-op if the app is stopped, so this is safe.

**No code change required.** Documented here for awareness. If future debugging shows panics at shutdown, the fix would be to add a small drain delay in the input capture handler:

```go
// If QueueUpdateDraw panics on stopped app in a future tview version:
case tcell.KeyESC:
    cancel()
    time.Sleep(50 * time.Millisecond) // let goroutine observe cancel
    app.Stop()
    return nil
```

**Do not implement this preemptively** — only add it if a problem is observed.

---

### 7. `parsePeerEntry` is fragile with non-standard input

**Severity:** Low — works correctly today but could break silently if the server format changes.

**Root cause:** `cli/tui.go`, `parsePeerEntry` function parses a formatted string from `StatusReply.NetPeerAddrsCurrent`. The format is produced by `rpc/server.go` in `HyprspaceRPC.Status()`:
```go
fmt.Sprintf("@%s (%s) %s/p2p/%s", p.Name, latency, multiaddr, peerID)
```

The parser assumes:
1. Name contains no spaces (field is delimited by first space)
2. Latency is always parenthesized as the second field
3. Multiaddr never contains `) ` as a substring

All three are true today, but this is an untested implicit protocol.

**File:** `cli/tui.go`, function `parsePeerEntry`

**Implementation steps (choose one):**

**Option A (lower effort — just document the coupling):** Add a comment above `parsePeerEntry` that references the exact `fmt.Sprintf` call in `rpc/server.go` as the source of truth:

```go
// parsePeerEntry extracts name, latency, and multiaddr from a peer entry string.
// Input format: @name (latency) multiaddr/p2p/peerid
// The format is produced by HyprspaceRPC.Status in rpc/server.go:
//   fmt.Sprintf("@%s (%s) %s/p2p/%s", p.Name, latency, multiaddr, peerID)
// If the server format changes, this parser must be updated in lockstep.
```

**Option B (higher effort — more robust):** Add structured fields to `StatusReply` in `rpc/types.go` so the TUI doesn't need to parse formatted strings:

```go
type PeerInfo struct {
    Name     string
    Latency  string
    Multiaddr string
    PeerID   string
}

type StatusReply struct {
    PeerID              string
    SwarmPeersCurrent   int
    NetPeersCurrent     int
    NetPeerAddrsCurrent []string   // kept for backward compat
    NetPeersMax         int
    ListenAddrs         []string
    NetPeers            []PeerInfo // NEW: structured peer data
}
```

Then populate `NetPeers` in the server and use it directly in the TUI's `updatePeersTable`. This eliminates the parsing entirely. However, this changes the RPC type and requires a server-side change — more invasive but more maintainable.

**Recommendation:** Do Option A now (comment-only), file a follow-up issue for Option B if the Peers tab needs richer data later.

**Side effects:** None for Option A. Option B would require updating the server to populate the new field and verifying backward compatibility with existing clients.

---

### 8. No `Register`-style extensibility pattern

**Severity:** Low — by design per the spec. Noted here for future reference.

The current pattern uses a `tabs` slice for tab metadata and individual `update*` functions wired into `fetchAndUpdate`. Adding a 4th tab requires:
1. Add an entry to the `tabs` slice
2. Add a new `updateXxxTab` function
3. Wire it into `fetchAndUpdate`

For 3–4 tabs this is fine. If someone later adds 10+ tabs, the `fetchAndUpdate` function becomes a long if-else chain.

**No change requested.** If extension patterns are needed in the future, the spec's suggested `Register("Name", buildViewFn)` pattern is the right direction — but it would be over-engineering today.

---

## Checklist against spec

| Spec requirement | Status | Notes |
|---|---|---|
| Status tab (PeerID, swarm, VPN, listen addrs) | ✅ | PeerID yellow, relay addrs gray |
| Peers tab (Name, Latency, Multiaddr) | ✅ | Parsed from `NetPeerAddrsCurrent` |
| Routes tab (Network, Target, Relay, Status) | ✅ | Connected green, disconnected red |
| Tab/Shift+Tab navigation | ✅ | |
| q/Esc/Ctrl+C quit | ✅ | Needs context fix (issue #2) |
| 2s poll interval, immediate first fetch | ✅ | |
| RPC failure → error banner, keep polling | ⚠️ | Routes failure clears status tab (issue #4) |
| Empty states for peers/routes | ✅ | |
| Non-fatal RPC wrappers | ✅ | `TryStatus`/`TryRoute` |
| Colorize relay addresses | ✅ | |
| ~200 lines | ❌ | ~250 lines (fine, still small) |

---

## Recommended implementation order

| Order | Issue | Effort | Files touched |
|-------|-------|--------|---------------|
| 1 | FD leak (issue #1) | 3 minutes, 3 lines | `rpc/client.go` |
| 2 | Goroutine leak (issue #2) | 10 minutes, ~10 lines | `cli/tui.go` |
| 3 | Socket path dedup (issue #3) | 5 minutes, 5 lines | `rpc/client.go` |
| 4 | Route error handler (issue #4) | 5 minutes, ~5 lines | `cli/tui.go` |
| 5 | `panic` → `log.Fatal` (issue #5) | 2 minutes, 2 lines | `cli/tui.go` |
| 6 | `parsePeerEntry` comment (issue #7) | 2 minutes, 1 line | `cli/tui.go` |

Issues 1–5 can be implemented independently in any order. Issue 7 (Option A) is independent. Issue 6 and 8 are documentation-only.
