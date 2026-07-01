// convert.go — convert command implementation.
// Reads deck text, transforms between formats, and writes the result to stdout.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/slh/riftport/internal/convert"
	"github.com/slh/riftport/internal/database"
	"github.com/spf13/cobra"
)

// newConvertCmd creates the convert subcommand with --from and --to flags.
func newConvertCmd() *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "convert [file|-]",
		Short: "Transform deck text between supported formats",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readInput(args)
			if err != nil {
				return err
			}
			fromFmt := convert.Format(strings.ToLower(from))
			toFmt := convert.Format(strings.ToLower(to))
			if !fromFmt.Valid() {
				return fmt.Errorf("unknown --from format %q", from)
			}
			if !toFmt.Valid() || toFmt == convert.FormatAuto {
				return fmt.Errorf("unknown --to format %q", to)
			}

			path := dbPath
			if path == "" {
				path = database.DefaultPath()
			}
			var db *database.DB
			if needsDB(fromFmt, toFmt, input) {
				db, err = database.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				n, err := db.Count(cmd.Context())
				if err != nil {
					return err
				}
				if n == 0 {
					return fmt.Errorf("card database is empty at %s; run `riftport update-db` first", path)
				}
			}

			out, err := convert.Convert(context.Background(), db, fromFmt, toFmt, input)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, out)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "auto", "input format: auto, names, tts, pixelborn, piltover, tcgarena, deckcode")
	cmd.Flags().StringVar(&to, "to", "names", "output format: names, tts, pixelborn, piltover, tcgarena, deckcode")
	return cmd
}

// needsDB reports whether the conversion requires a populated local card database.
func needsDB(from, to convert.Format, input string) bool {
	if from == convert.FormatNames || from == convert.FormatPiltover || from == convert.FormatTCGArena {
		return true
	}
	if to == convert.FormatNames || to == convert.FormatPiltover || to == convert.FormatTCGArena {
		return true
	}
	if from == convert.FormatAuto {
		detected := convert.DetectFormat(input)
		if detected == convert.FormatNames || detected == convert.FormatPiltover || detected == convert.FormatTCGArena {
			return true
		}
	}
	return false
}
