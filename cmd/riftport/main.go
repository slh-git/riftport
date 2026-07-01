// main.go — CLI binary entrypoint for riftport.
// Delegates all command handling to internal/cmd and exits with status 1 on error.
package main

import (
	"fmt"
	"os"

	"github.com/slh/riftport/internal/cmd"
)

// main runs the riftport CLI and prints errors to stderr before exiting.
func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
