// tcgarena.go — TCG Arena positional deck list parser and writer.
// Line order: legend, chosen champion, battlefields, runes, main deck, sideboard.
package formats

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/deck"
)

const (
	tcgArenaBattlefieldCount = 3
	tcgArenaRuneTotal        = 12
)

var tcgArenaHeaderPattern = regexp.MustCompile(`(?i)^(intcg\s+arena|tcg-?arena)$`)

// ParseTCGArena parses a TCG Arena positional deck list into a deck.Deck.
func ParseTCGArena(text string) (deck.Deck, error) {
	lines := splitNonEmptyLines(text)
	if len(lines) == 0 {
		return deck.Deck{}, fmt.Errorf("empty tcgarena deck")
	}

	idx := 0
	if tcgArenaHeaderPattern.MatchString(lines[0]) {
		idx = 1
	}

	var mainLines []string
	var sideLines []string
	inSideboard := false
	for i := idx; i < len(lines); i++ {
		if isSideboardHeader(lines[i]) {
			inSideboard = true
			continue
		}
		if inSideboard {
			sideLines = append(sideLines, lines[i])
		} else {
			mainLines = append(mainLines, lines[i])
		}
	}
	if !inSideboard {
		return deck.Deck{}, fmt.Errorf("tcgarena deck missing sideboard section")
	}

	entries, err := parseNameLines(mainLines)
	if err != nil {
		return deck.Deck{}, err
	}
	sideEntries, err := parseNameLines(sideLines)
	if err != nil {
		return deck.Deck{}, err
	}

	d, err := assignTCGArenaSections(entries)
	if err != nil {
		return deck.Deck{}, err
	}
	for _, e := range sideEntries {
		e.Section = deck.SectionSideboard
		d.Sideboard = append(d.Sideboard, e)
	}
	return d, nil
}

func assignTCGArenaSections(entries []deck.Entry) (deck.Deck, error) {
	minLines := 2 + tcgArenaBattlefieldCount + 1
	if len(entries) < minLines {
		return deck.Deck{}, fmt.Errorf("tcgarena deck too short: expected at least %d card lines before sideboard", minLines)
	}

	d := deck.Deck{}

	if entries[0].Count != 1 {
		return deck.Deck{}, fmt.Errorf("tcgarena legend must be count 1, got %d", entries[0].Count)
	}
	entries[0].Section = deck.SectionLegend
	d.Main = append(d.Main, entries[0])

	if entries[1].Count != 1 {
		return deck.Deck{}, fmt.Errorf("tcgarena chosen champion must be count 1, got %d", entries[1].Count)
	}
	d.ChosenChampionHint = entries[1].NameHint

	for i := 2; i < 2+tcgArenaBattlefieldCount; i++ {
		entries[i].Section = deck.SectionBattlefield
		d.Main = append(d.Main, entries[i])
	}

	runeStart := 2 + tcgArenaBattlefieldCount
	runeTotal := 0
	runeEnd := runeStart
	for runeEnd < len(entries) && runeTotal < tcgArenaRuneTotal {
		runeTotal += entries[runeEnd].Count
		entries[runeEnd].Section = deck.SectionRune
		d.Main = append(d.Main, entries[runeEnd])
		runeEnd++
	}
	if runeTotal != tcgArenaRuneTotal {
		return deck.Deck{}, fmt.Errorf("tcgarena runes must total %d, got %d", tcgArenaRuneTotal, runeTotal)
	}

	for i := runeEnd; i < len(entries); i++ {
		entries[i].Section = deck.SectionMain
		d.Main = append(d.Main, entries[i])
	}

	return d, nil
}

// FormatTCGArena renders a deck as a TCG Arena positional list.
func FormatTCGArena(d deck.Deck, names func(cards.Ref) (string, error)) (string, error) {
	var b strings.Builder
	b.WriteString("intcg arena\n")

	legends := filterSection(d.Main, deck.SectionLegend)
	if len(legends) != 1 {
		return "", fmt.Errorf("tcgarena output requires exactly 1 legend")
	}
	if err := writeNameLines(&b, legends, names); err != nil {
		return "", err
	}

	if d.ChosenChampion == nil {
		return "", fmt.Errorf("tcgarena output requires a chosen champion")
	}
	champName, err := names(*d.ChosenChampion)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "1 %s\n", champName)

	for _, section := range []deck.Section{deck.SectionBattlefield, deck.SectionRune, deck.SectionMain} {
		if err := writeNameLines(&b, filterSection(d.Main, section), names); err != nil {
			return "", err
		}
	}

	b.WriteString("Sideboard:\n")
	if err := writeNameLines(&b, d.Sideboard, names); err != nil {
		return "", err
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

// LooksLikeTCGArena heuristically detects TCG Arena positional deck lists.
func LooksLikeTCGArena(text string) bool {
	lines := splitNonEmptyLines(text)
	if len(lines) == 0 {
		return false
	}
	if tcgArenaHeaderPattern.MatchString(lines[0]) {
		return true
	}

	hasSideboard := false
	hasPiltoverHeaders := false
	for _, line := range lines {
		if isSideboardHeader(line) {
			hasSideboard = true
		}
		if sec, ok := parseSectionHeader(line); ok {
			switch sec {
			case deck.SectionLegend, deck.SectionChampion, deck.SectionMain, deck.SectionBattlefield, deck.SectionRune:
				hasPiltoverHeaders = true
			}
		}
	}
	return hasSideboard && !hasPiltoverHeaders
}
