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

func (c Card) Ref() Ref {
	return Ref{
		SetID:           c.SetID,
		CollectorNumber: c.CollectorNumber,
		Variant:         c.Variant,
		IsRune:          c.Type == "Rune",
	}
}
