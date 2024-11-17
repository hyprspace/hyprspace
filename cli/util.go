package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

func printListF(strings []string, handler func(string) string) {
	for _, s := range strings {
		fmt.Printf("    %s\n", handler(s))
	}
}

func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
