package deck

import "github.com/slh/riftport/internal/cards"

type Section string

const (
	SectionUnknown    Section = ""
	SectionLegend     Section = "legend"
	SectionMain       Section = "main"
	SectionBattlefield Section = "battlefield"
	SectionRune       Section = "rune"
	SectionSideboard  Section = "sideboard"
)

type Entry struct {
	Ref      cards.Ref
	NameHint string // populated when parsing name-based formats
	Count    int
	Section  Section
}

type Deck struct {
	Main           []Entry
	Sideboard      []Entry
	ChosenChampion *cards.Ref
}

func (d Deck) AllEntries() []Entry {
	out := make([]Entry, 0, len(d.Main)+len(d.Sideboard))
	out = append(out, d.Main...)
	out = append(out, d.Sideboard...)
	return out
}

func (d Deck) FlatMain() []Entry {
	if len(d.Main) > 0 {
		return d.Main
	}
	return d.AllEntries()
}
