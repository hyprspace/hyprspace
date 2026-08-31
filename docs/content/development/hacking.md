# Hacking

## Developer Environment

Hyprspace is built with [Nix](https://nixos.org). The Hyprspace flake includes a devShell with all the tools needed for development.

To use it, simply run:

```shell-session
nix develop
```

You can also use [direnv](https://direnv.net).

```shell-session
direnv allow  
```

## Building

To build Hyprspace for testing during development, you first need to generate the [configuration schema](config-schema.html) code. This is always done automatically upon entering the devShell. If you made changes to the config schema, you can regenerate the Go code (requires Nix):

```shell-session
go generate ./schema
```

Then you can build the binary as usual:

```shell-session
go build
```

## Android Mobile Bindings

The Android .aar binding in `mobile/` is built with [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile).

### Prerequisites

- Go 1.24+ with gomobile installed: `go install golang.org/x/mobile/cmd/gomobile@latest`
- Android SDK (`ANDROID_HOME` pointing to the SDK root)
- Android NDK (`ANDROID_NDK_HOME` pointing to the NDK version directory)
- A JDK with `javac` on `$PATH` (bundled with Android Studio)

### Build

```shell-session
export ANDROID_HOME=/path/to/Android/Sdk
export ANDROID_NDK_HOME=/path/to/Android/Sdk/ndk/30.0.14904198
export PATH="/path/to/jdk/bin:$PATH"
gomobile bind -ldflags "-checklinkname=0" -javapkg hyprspace -target=android -androidapi 26 -o hyprspace.aar ./mobile
```

> [!WARNING]
> The `-ldflags "-checklinkname=0"` flag is required because a transitive dependency
> (`github.com/wlynxg/anet`) uses `//go:linkname` to reference Go's internal
> `net.zoneCache` type, which was restricted in Go 1.23+.
> See [DECISION-ANDROID-GOMOBILE.md](../../decisions/DECISION-ANDROID-GOMOBILE.md) for details.

### Why gomobile needs this flag

`wlynxg/anet` provides Android-compatible replacements for `net.Interfaces()` and
`net.InterfaceAddrs()` that bypass Android's NETLINK permission restrictions. The package
uses `//go:linkname` to reach into Go internals (`net.zoneCache`). Since Go 1.23,
`//go:linkname` references are validated by the linker and fail because `zoneCache`
is no longer part of the public API contract.

This dependency was already present before mobile bindings — it's pulled in by
`libp2p`, `go-libp2p-kad-dht`, `boxo`, and several `pion/*` packages. It only becomes
a build failure during `gomobile bind` because gomobile compiles with the `android`
build tag, which enables `anet`'s Android-specific file that uses `linkname`.

Without this flag, gomobile fails with:

```
link: github.com/wlynxg/anet: invalid reference to net.zoneCache
```

**Decision rationale:** Disabling linker name checks is safe here because:

- `anet` is an indirect dependency we don't control
- The linker only validates the *target* of the linkname, not the *source* — so our
code can't craft arbitrary linknames. The risk is that future Go versions may silently
break the internal `zoneCache` structure, which would only surface at runtime.
- No maintained alternative exists.
- We pin the Go toolchain version, so this won't regress silently.
