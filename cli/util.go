package cli

import "fmt"

func printList(strings []string) {
	for _, ma := range strings {
		fmt.Printf("    %s\n", ma)
	}
}
