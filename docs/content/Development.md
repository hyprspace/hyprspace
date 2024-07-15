# Development

## Developer Environment

Hyprspace is built with [Nix](https://nixos.org). The Hyprspace flake includes a devShell with all the tools needed for development.

To use it, simply run:

```shell-session
$ nix develop
```

You can also use [direnv](https://direnv.net).

```shell-session
$ direnv allow  
```

## Building

To build Hyprspace for testing during development, you first need to generate the [[config-schema]] code. This is always done automatically upon entering the devShell. If you made changes to the config schema, you can regenerate the Go code (requires Nix):

```shell-session
$ go generate ./schema
```

Then you can build the binary as usual:

```shell-session
$ go build
```
