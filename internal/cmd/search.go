// search.go — search command implementation.
// Queries the local FTS5 index and prints matching cards.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/slh/riftport/internal/database"
	"github.com/spf13/cobra"
)

// newSearchCmd creates the search subcommand with --limit flag.
func newSearchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the local card index",
		Args:  cobra.ExactArgs(1),
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

			results, err := db.Search(context.Background(), args[0], limit)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "no matches")
				return nil
			}
			for _, r := range results {
				c := r.Card
				fmt.Fprintf(os.Stdout, "%s  %s  [%s] %s\n", c.Ref().ShortCode(), c.Name, c.SetID, c.Type)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	return cmd
}
