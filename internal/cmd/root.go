// root.go — Cobra root command and shared CLI helpers.
// Registers update-db, convert, inspect, and search; defines the global --db flag.
package cmd

import (
	"fmt"
	"os"

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
