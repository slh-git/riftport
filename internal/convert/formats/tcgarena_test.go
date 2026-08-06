package formats_test

import (
	"strings"
	"testing"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/convert/formats"
	"github.com/slh/riftport/internal/deck"
)

const teemoTCGArenaDeck = `intcg arena 
1 Teemo, Swift Scout
1 Teemo, Strategist
1 Grove of the God-Willow
1 Startipped Peak
1 The Arena's Greatest
6 Mind Rune
6 Chaos Rune
3 Consult the Past
3 Sprite Call
3 Stacked Deck
3 Switcheroo
2 Rebuke
2 Guerilla Warfare
1 Ride the Wind
1 Bone Skewer
3 Tideturner
3 Windsinger
2 Sneaky Deckhand
1 Baron Nashor
2 Teemo, Strategist
3 Nocturne, Horrifying
3 Teemo, Scout
1 Evelynn, Entrancing
3 Sprite Fountain
Sideboard:
3 Turn to Dust
2 Existential Dread
1 Singularity
1 Thousand-Tailed Watcher
1 Downwell`

const teemoPiltoverDeck = `Legend:
1 Teemo, Swift Scout

Champion:
1 Teemo, Strategist

MainDeck:
3 Consult the Past
3 Sprite Call
3 Stacked Deck
3 Tideturner
3 Windsinger
3 Switcheroo
3 Sprite Fountain
2 Rebuke
2 Sneaky Deckhand
2 Guerilla Warfare
1 Ride the Wind
1 Bone Skewer
1 Baron Nashor
2 Teemo, Strategist
3 Nocturne, Horrifying
3 Teemo, Scout
1 Evelynn, Entrancing

Battlefields:
1 Grove of the God-Willow
1 Startipped Peak
1 The Arena's Greatest

Runes:
6 Mind Rune
6 Chaos Rune

Sideboard:
3 Turn to Dust
2 Existential Dread
1 Singularity
1 Thousand-Tailed Watcher
1 Downwell`

func TestParseTCGArena_TeemoDeck(t *testing.T) {
	d, err := formats.ParseTCGArena(teemoTCGArenaDeck)
	if err != nil {
		t.Fatal(err)
	}

	if d.ChosenChampionHint != "Teemo, Strategist" {
		t.Fatalf("chosen champion hint = %q, want Teemo, Strategist", d.ChosenChampionHint)
	}

	legend := filterSection(d.Main, deck.SectionLegend)
	if len(legend) != 1 || legend[0].NameHint != "Teemo, Swift Scout" {
		t.Fatalf("legend = %+v", legend)
	}

	battlefields := filterSection(d.Main, deck.SectionBattlefield)
	if len(battlefields) != 3 {
		t.Fatalf("battlefields count = %d, want 3", len(battlefields))
	}

	runes := filterSection(d.Main, deck.SectionRune)
	runeTotal := 0
	for _, e := range runes {
		runeTotal += e.Count
	}
	if runeTotal != 12 {
		t.Fatalf("rune total = %d, want 12", runeTotal)
	}

	mainStrat := findMainCount(d.Main, "Teemo, Strategist")
	if mainStrat != 2 {
		t.Fatalf("main Teemo, Strategist count = %d, want 2", mainStrat)
	}

	if len(d.Sideboard) != 5 {
		t.Fatalf("sideboard lines = %d, want 5", len(d.Sideboard))
	}
}

func TestParsePiltover_TeemoDeckWithColons(t *testing.T) {
	d, err := formats.ParsePiltover(teemoPiltoverDeck)
	if err != nil {
		t.Fatal(err)
	}

	if d.ChosenChampionHint != "Teemo, Strategist" {
		t.Fatalf("chosen champion hint = %q, want Teemo, Strategist", d.ChosenChampionHint)
	}

	mainStrat := findMainCount(d.Main, "Teemo, Strategist")
	if mainStrat != 2 {
		t.Fatalf("main Teemo, Strategist count = %d, want 2", mainStrat)
	}
}

func TestFormatTCGArena_RoundTripStructure(t *testing.T) {
	parsed, err := formats.ParseTCGArena(teemoTCGArenaDeck)
	if err != nil {
		t.Fatal(err)
	}

	ref := cards.Ref{SetID: "OGN", CollectorNumber: 99}
	champRef := ref

	names := map[string]cards.Ref{
		"Teemo, Swift Scout": {SetID: "OGN", CollectorNumber: 1},
		"Teemo, Strategist":  champRef,
	}
	resolved := deck.Deck{
		ChosenChampion: &champRef,
	}
	for _, e := range parsed.Main {
		r, ok := names[e.NameHint]
		if !ok {
			r = cards.Ref{SetID: "OGN", CollectorNumber: 100 + len(resolved.Main)}
			names[e.NameHint] = r
		}
		resolved.Main = append(resolved.Main, deck.Entry{
			Ref:     r,
			Count:   e.Count,
			Section: e.Section,
		})
	}
	for _, e := range parsed.Sideboard {
		r, ok := names[e.NameHint]
		if !ok {
			r = cards.Ref{SetID: "OGN", CollectorNumber: 200 + len(resolved.Sideboard)}
			names[e.NameHint] = r
		}
		resolved.Sideboard = append(resolved.Sideboard, deck.Entry{
			Ref:     r,
			Count:   e.Count,
			Section: deck.SectionSideboard,
		})
	}

	nameFn := func(r cards.Ref) (string, error) {
		for name, ref := range names {
			if ref == r {
				return name, nil
			}
		}
		return "", nil
	}

	out, err := formats.FormatTCGArena(resolved, nameFn)
	if err != nil {
		t.Fatal(err)
	}

	lines := nonEmpty(out)
	if lines[0] != "intcg arena" {
		t.Fatalf("first line = %q, want intcg arena", lines[0])
	}
	if lines[1] != "1 Teemo, Swift Scout" {
		t.Fatalf("legend line = %q", lines[1])
	}
	if lines[2] != "1 Teemo, Strategist" {
		t.Fatalf("champion line = %q", lines[2])
	}
	if !strings.Contains(out, "Sideboard:") {
		t.Fatal("missing Sideboard: header")
	}

	reparsed, err := formats.ParseTCGArena(out)
	if err != nil {
		t.Fatal(err)
	}
	if reparsed.ChosenChampionHint != "Teemo, Strategist" {
		t.Fatalf("reparsed champion = %q", reparsed.ChosenChampionHint)
	}
	if findMainCount(reparsed.Main, "Teemo, Strategist") != 2 {
		t.Fatal("reparsed main Teemo, Strategist count != 2")
	}
}

func TestLooksLikeTCGArena(t *testing.T) {
	if !formats.LooksLikeTCGArena(teemoTCGArenaDeck) {
		t.Fatal("expected tcgarena deck to match LooksLikeTCGArena")
	}
	if formats.LooksLikeTCGArena(teemoPiltoverDeck) {
		t.Fatal("piltover deck should not match LooksLikeTCGArena")
	}
}

func TestFormatPiltoverCanonicalSections(t *testing.T) {
	legend := cards.Ref{SetID: "VEN", CollectorNumber: 149}
	champion := cards.Ref{SetID: "VEN", CollectorNumber: 68}
	main := cards.Ref{SetID: "VEN", CollectorNumber: 1}
	battlefield := cards.Ref{SetID: "VEN", CollectorNumber: 2}
	rune := cards.Ref{SetID: "OGN", CollectorNumber: 1, IsRune: true}
	sideboard := cards.Ref{SetID: "VEN", CollectorNumber: 3}
	names := map[cards.Ref]string{
		legend:      "Jayce - Defender of Tomorrow",
		champion:    "Jayce, Brilliant Inventor",
		main:        "Promising Future",
		battlefield: "Sigil of the Storm",
		rune:        "Body Rune",
		sideboard:   "Turn to Dust",
	}
	d := deck.Deck{
		ChosenChampion: &champion,
		Main: []deck.Entry{
			{Ref: legend, Count: 1, Section: deck.SectionLegend},
			{Ref: main, Count: 3, Section: deck.SectionMain},
			{Ref: battlefield, Count: 1, Section: deck.SectionBattlefield},
			{Ref: rune, Count: 9, Section: deck.SectionRune},
		},
		Sideboard: []deck.Entry{
			{Ref: sideboard, Count: 2, Section: deck.SectionSideboard},
		},
	}
	nameFn := func(ref cards.Ref) (string, error) {
		return names[ref], nil
	}

	got, err := formats.FormatPiltover(d, nameFn)
	if err != nil {
		t.Fatal(err)
	}
	want := `Legend:
1 Jayce - Defender of Tomorrow

Champion:
1 Jayce, Brilliant Inventor

MainDeck:
3 Promising Future

Battlefields:
1 Sigil of the Storm

Runes:
9 Body Rune

Sideboard:
2 Turn to Dust`
	if got != want {
		t.Fatalf("FormatPiltover() =\n%s\nwant:\n%s", got, want)
	}
}

func filterSection(entries []deck.Entry, section deck.Section) []deck.Entry {
	var out []deck.Entry
	for _, e := range entries {
		if e.Section == section {
			out = append(out, e)
		}
	}
	return out
}

func findMainCount(entries []deck.Entry, name string) int {
	for _, e := range entries {
		if e.Section == deck.SectionMain && e.NameHint == name {
			return e.Count
		}
	}
	return 0
}

func nonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
