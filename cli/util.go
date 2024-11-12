package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

func printList(strings []string) {
	for _, ma := range strings {
		fmt.Printf("    %s\n", ma)
	}
}

func printListF(strings []string, handler func(string) string) {
	for _, s := range strings {
		fmt.Printf("    %s\n", handler(s))
	}
}

func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
