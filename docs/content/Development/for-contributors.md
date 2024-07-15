# For Contributors

This page contains some information you might find useful when contributing to Hyprspace.

## Canonical Repository Location

Development canonically takes place at https://github.com/hyprspace/hyprspace. Feel free to open issues and PRs there.

## Continuous Integration

Hyprspace uses [Hercules CI](https://hercules-ci.com/github/hyprspace/hyprspace) to verify code functionality and quality. You can also run all the builds and checks locally by running:

```shell-session
$ nix flake check
```

## Code Formatting

Hyprspace's codebase consists mainly of Go and Nix code. `go fmt` and nixfmt (RFC Style) are used respectively to format the code for those languages. Formatting is enforced via CI.

Formatters are available in the [[Development|devShell]].

