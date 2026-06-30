package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/database"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <card-id-or-code>",
		Short: "Show a card from the local database",
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

			query := strings.TrimSpace(args[0])
			ctx := context.Background()

			card, err := db.GetByID(ctx, query)
			if err != nil {
				if ref, refErr := cards.ParseShortCode(query); refErr == nil {
					card, err = db.GetByRef(ctx, ref)
				} else if ref, refErr := cards.ParseTTS(query); refErr == nil {
					card, err = db.GetByRef(ctx, ref)
				}
			}
			if err != nil {
				return fmt.Errorf("card not found: %w", err)
			}

			fmt.Fprintf(os.Stdout, "ID:       %s\n", card.ID)
			fmt.Fprintf(os.Stdout, "Name:     %s\n", card.Name)
			fmt.Fprintf(os.Stdout, "Code:     %s\n", card.Ref().ShortCode())
			fmt.Fprintf(os.Stdout, "Set:      %s\n", card.SetID)
			fmt.Fprintf(os.Stdout, "Number:   %d\n", card.CollectorNumber)
			if card.Variant != "" {
				fmt.Fprintf(os.Stdout, "Variant:  %s\n", card.Variant)
			}
			if card.Type != "" {
				fmt.Fprintf(os.Stdout, "Type:     %s\n", card.Type)
			}
			if card.Faction != "" {
				fmt.Fprintf(os.Stdout, "Faction:  %s\n", card.Faction)
			}
			if card.Rarity != "" {
				fmt.Fprintf(os.Stdout, "Rarity:   %s\n", card.Rarity)
			}
			if card.Energy != nil || card.Might != nil || card.Power != nil {
				fmt.Fprintf(os.Stdout, "Stats:    energy=%s might=%s power=%s\n",
					intOrDash(card.Energy), intOrDash(card.Might), intOrDash(card.Power))
			}
			if card.IsBanned {
				fmt.Fprintf(os.Stdout, "Banned:   yes\n")
			}
			return nil
		},
	}
}

func intOrDash(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}
