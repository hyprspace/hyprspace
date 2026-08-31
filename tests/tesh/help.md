# tesh: Testable CLI help examples for hyprspace

These tests verify that the CLI help output remains consistent.

## Main help

```console tesh-session="main-help" tesh-exitcodes="1"
$ hyprspace --help
USAGE: hyprspace CMD [OPTIONS]
...
DESCRIPTION: Hyprspace Distributed Network
...
COMMANDS:
...
    NAME     ALIAS  DESCRIPTION
    help     ?      Get help with a specific subcommand
    init     i      Initialize An Interface Config
    peers           List peer connections
    route    r      Control routing
    status   s      Display Hyprspace daemon status
    up       up     Create and Bring Up a Hyprspace Interface.
    version         Print the version of this command
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Help without subcommand (error)

```console tesh-session="help-no-arg" tesh-exitcodes="1"
$ hyprspace help
Error: Missing argument(s)
...
USAGE: hyprspace help [OPTIONS] <Subcommand>
...
DESCRIPTION: Get help with a specific subcommand
...
ARGUMENTS:
...
    NAME        TYPE    DESCRIPTION
    Subcommand  STRING  Command to get help for
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Init help

```console tesh-session="init-help"
$ hyprspace help init
USAGE: hyprspace init [OPTIONS]
...
DESCRIPTION: Initialize An Interface Config
...
INIT FLAGS:
...
    NAME    ARG     DESCRIPTION
    --name  STRING  Name to include in the suggested peer entry.
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Up help

```console tesh-session="up-help"
$ hyprspace help up
USAGE: hyprspace up [OPTIONS]
...
DESCRIPTION: Create and Bring Up a Hyprspace Interface.
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Status help

```console tesh-session="status-help"
$ hyprspace help status
USAGE: hyprspace status [OPTIONS]
...
DESCRIPTION: Display Hyprspace daemon status
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Peers help

```console tesh-session="peers-help"
$ hyprspace help peers
USAGE: hyprspace peers [OPTIONS]
...
DESCRIPTION: List peer connections
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```

## Route help

```console tesh-session="route-help"
$ hyprspace help route
USAGE: hyprspace route [OPTIONS] <Action> [Args...]
...
DESCRIPTION: Control routing
...
ARGUMENTS:
...
    NAME    TYPE      DESCRIPTION
    Action  STRING
    Args    []STRING
...
GLOBAL FLAGS:
...
    NAME             ARG     DESCRIPTION
    -c, --config     STRING  Specify a custom config path.
    -i, --interface  STRING  Interface name.
```
