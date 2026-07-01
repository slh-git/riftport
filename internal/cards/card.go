// card.go — locally stored card metadata model.
// Represents one card row from the SQLite database.
package cards

import "time"

// Card is locally stored card metadata.
type Card struct {
	ID              string
	Name            string
	SetID           string
	CollectorNumber int
	Variant         string
	Rarity          string
	Faction         string
	Type            string
	Orientation     string
	Energy          *int
	Might           *int
	Power           *int
	IsBanned        bool
	UpdatedAt       time.Time
}

// Ref converts stored card fields into a cards.Ref for deck conversion.
func (c Card) Ref() Ref {
	return Ref{
		SetID:           c.SetID,
		CollectorNumber: c.CollectorNumber,
		Variant:         c.Variant,
		IsRune:          c.Type == "Rune",
	}
}
