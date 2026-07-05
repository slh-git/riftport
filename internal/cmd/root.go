// root.go — Cobra root command and shared CLI helpers.
// Registers update-db, convert, inspect, and search; defines the global --db flag.
package cmd

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var dbPath string

// Execute builds the command tree and runs the CLI.
func Execute() error {
	root := &cobra.Command{
		Use:   "riftport",
		Short: "Riftbound deck converter with local card metadata",
	}
	root.PersistentFlags().StringVar(&dbPath, "db", "", "path to local card database (default: ~/.riftport/cards.db)")

	root.AddCommand(newUpdateDBCmd())
	root.AddCommand(newConvertCmd())
	root.AddCommand(newInspectCmd())
	root.AddCommand(newSearchCmd())
	return root.Execute()
}

// readInput loads deck text from a file path, "-" (stdin), or stdin when no args are given.
func readInput(args []string) (string, error) {
	if len(args) > 0 {
		if args[0] == "-" {
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile("/dev/stdin")
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("no input: provide a file argument or pipe deck text on stdin")
	}
	return string(data), nil
}

// readInteractiveInput prompts on stderr and reads a pasted deck list from the terminal.
func readInteractiveInput() (string, error) {
	fmt.Fprintf(os.Stderr, "Paste your deck list below, then press %s when done:\n\n", finishInteractiveHint())

	reader := io.Reader(os.Stdin)
	if runtime.GOOS != "windows" {
		if tty, err := os.Open("/dev/tty"); err == nil {
			defer tty.Close()
			reader = tty
		}
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	text := string(data)
	if trimInput(text) == "" {
		return "", fmt.Errorf("no input provided")
	}
	return text, nil
}
