// update_db.go — update-db command implementation.
// The only CLI command that performs network I/O; refreshes local card metadata.
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/slh/riftport/internal/database"
	"github.com/slh/riftport/internal/fetch"
	"github.com/spf13/cobra"
)

// newUpdateDBCmd creates the update-db subcommand.
func newUpdateDBCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update-db",
		Short: "Download or refresh local card metadata",
		Long: `Acquire card metadata from the configured remote source and store it locally.

Conversion commands do not call the network. Run update-db manually whenever you
want fresher card data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = database.DefaultPath()
			}
			db, err := database.Open(path)
			if err != nil {
				return err
			}
			defer db.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			result, err := fetch.UpdateDB(
				ctx,
				db,
				fetch.NewRiftCodexClient(),
				fetch.NewRiftScribeClient(),
			)
			if err != nil {
				return err
			}
			for _, failure := range result.Failures {
				fmt.Fprintf(os.Stderr, "warning: %s unavailable: %v\n", failure.Source, failure.Err)
			}
			fmt.Fprintf(os.Stdout, "updated %d cards from %s in %s\n", result.Count, result.Source, path)
			return nil
		},
	}
}
