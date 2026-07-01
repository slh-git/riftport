// deck.go — canonical in-memory deck representation.
// All format parsers produce and all format writers consume this structure.
package deck

import "github.com/slh/riftport/internal/cards"

// Section identifies which part of a deck an entry belongs to.
type Section string

const (
	SectionUnknown     Section = ""
	SectionLegend      Section = "legend"
	SectionChampion    Section = "champion"
	SectionMain        Section = "main"
	SectionBattlefield Section = "battlefield"
	SectionRune        Section = "rune"
	SectionSideboard   Section = "sideboard"
)

// Entry is one line in a deck: a card reference or unresolved name hint plus count.
type Entry struct {
	Ref      cards.Ref
	NameHint string // populated when parsing name-based formats
	Count    int
	Section  Section
}

// Deck holds main and sideboard entries plus an optional chosen champion.
type Deck struct {
	Main                 []Entry
	Sideboard            []Entry
	ChosenChampion       *cards.Ref
	ChosenChampionHint   string // unresolved champion name from tcgarena/piltover text
}

// AllEntries returns main and sideboard entries combined.
func (d Deck) AllEntries() []Entry {
	out := make([]Entry, 0, len(d.Main)+len(d.Sideboard))
	out = append(out, d.Main...)
	out = append(out, d.Sideboard...)
	return out
}

// FlatMain returns the main deck, or all entries if main is empty.
func (d Deck) FlatMain() []Entry {
	if len(d.Main) > 0 {
		return d.Main
	}
	return d.AllEntries()
}
