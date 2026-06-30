package formats

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/convert/deckcode"
	"github.com/slh/riftport/internal/deck"
)

var (
	ttsTokenPattern = regexp.MustCompile(`^[A-Z]{3}-\d{1,3}-\d+$`)
	nameLinePattern = regexp.MustCompile(`^(\d+)\s+(.+)$`)
)

func ParseTTS(text string) (deck.Deck, error) {
	tokens := strings.Fields(strings.TrimSpace(text))
	if len(tokens) == 0 {
		return deck.Deck{}, fmt.Errorf("empty TTS deck")
	}
	var entries []deck.Entry
	for _, token := range tokens {
		ref, err := cards.ParseTTS(token)
		if err != nil {
			return deck.Deck{}, err
		}
		entries = append(entries, deck.Entry{Ref: ref, Count: 1, Section: deck.SectionMain})
	}
	return collapseEntries(entries), nil
}

func FormatTTS(d deck.Deck) string {
	var tokens []string
	for _, e := range d.FlatMain() {
		for i := 0; i < e.Count; i++ {
			tokens = append(tokens, e.Ref.TTSToken())
		}
	}
	return strings.Join(tokens, " ")
}

func ParsePixelborn(text string) (deck.Deck, error) {
	text = strings.TrimSpace(text)
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return deck.Deck{}, fmt.Errorf("decode pixelborn: %w", err)
	}
	parts := strings.Split(string(raw), "$")
	var entries []deck.Entry
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ref, err := cards.ParseTTS(part)
		if err != nil {
			return deck.Deck{}, err
		}
		entries = append(entries, deck.Entry{Ref: ref, Count: 1, Section: deck.SectionMain})
	}
	return collapseEntries(entries), nil
}

func FormatPixelborn(d deck.Deck) string {
	var parts []string
	for _, e := range d.FlatMain() {
		for i := 0; i < e.Count; i++ {
			parts = append(parts, e.Ref.TTSToken())
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(parts, "$")))
	return encoded
}

func ParseDeckCode(text string) (deck.Deck, error) {
	return deckcode.Decode(strings.TrimSpace(text))
}

func FormatDeckCode(d deck.Deck) (string, error) {
	return deckcode.Encode(d)
}

func ParseNames(text string) (deck.Deck, error) {
	lines := strings.Split(text, "\n")
	var entries []deck.Entry
	var section deck.Section
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if sec, ok := parseSectionHeader(line); ok {
			section = sec
			continue
		}
		m := nameLinePattern.FindStringSubmatch(line)
		if m == nil {
			return deck.Deck{}, fmt.Errorf("invalid name line: %q", line)
		}
		count := 0
		fmt.Sscanf(m[1], "%d", &count)
		name := strings.TrimSpace(m[2])
		entries = append(entries, deck.Entry{
			NameHint: name,
			Count:    count,
			Section:  section,
		})
	}
	return namesToDeck(entries)
}

func namesToDeck(entries []deck.Entry) (deck.Deck, error) {
	d := deck.Deck{}
	for _, e := range entries {
		if e.NameHint == "" {
			return deck.Deck{}, fmt.Errorf("entry missing card name")
		}
		switch e.Section {
		case deck.SectionSideboard:
			d.Sideboard = append(d.Sideboard, e)
		default:
			d.Main = append(d.Main, e)
		}
	}
	if len(d.Main) == 0 && len(entries) > 0 {
		d.Main = entries
	}
	return d, nil
}

func FormatNames(d deck.Deck, names func(cards.Ref) (string, error)) (string, error) {
	var b strings.Builder
	if hasSections(d.Main) {
		if err := writeNameSection(&b, "Legend", filterSection(d.Main, deck.SectionLegend), names); err != nil {
			return "", err
		}
		if err := writeNameSection(&b, "Main Deck", filterSection(d.Main, deck.SectionMain), names); err != nil {
			return "", err
		}
		if err := writeNameSection(&b, "Battlefields", filterSection(d.Main, deck.SectionBattlefield), names); err != nil {
			return "", err
		}
		if err := writeNameSection(&b, "Runes", filterSection(d.Main, deck.SectionRune), names); err != nil {
			return "", err
		}
	} else if err := writeNameSection(&b, "", d.Main, names); err != nil {
		return "", err
	}
	if err := writeNameSection(&b, "Sideboard", d.Sideboard, names); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func ParsePiltover(text string) (deck.Deck, error) {
	lines := strings.Split(text, "\n")
	var entries []deck.Entry
	var section deck.Section
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if sec, ok := parseSectionHeader(line); ok {
			section = sec
			continue
		}
		m := nameLinePattern.FindStringSubmatch(line)
		if m == nil {
			return deck.Deck{}, fmt.Errorf("invalid piltover line: %q", line)
		}
		count := 0
		fmt.Sscanf(m[1], "%d", &count)
		name := strings.TrimSpace(m[2])
		entries = append(entries, deck.Entry{
			NameHint: name,
			Count:    count,
			Section:  section,
		})
	}
	return namesToDeck(entries)
}

func FormatPiltover(d deck.Deck, names func(cards.Ref) (string, error)) (string, error) {
	return FormatNames(d, names)
}

func parseSectionHeader(line string) (deck.Section, bool) {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch lower {
	case "legend", "legends":
		return deck.SectionLegend, true
	case "main deck", "main":
		return deck.SectionMain, true
	case "battlefields", "battlefield":
		return deck.SectionBattlefield, true
	case "runes", "rune deck", "rune pool":
		return deck.SectionRune, true
	case "sideboard":
		return deck.SectionSideboard, true
	default:
		return deck.SectionUnknown, false
	}
}

func writeNameSection(b *strings.Builder, title string, entries []deck.Entry, names func(cards.Ref) (string, error)) error {
	if len(entries) == 0 {
		return nil
	}
	if title != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(title)
		b.WriteString("\n")
	}
	for _, e := range entries {
		name, err := names(e.Ref)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%d %s\n", e.Count, name)
	}
	return nil
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

func hasSections(entries []deck.Entry) bool {
	for _, e := range entries {
		switch e.Section {
		case deck.SectionLegend, deck.SectionBattlefield, deck.SectionRune, deck.SectionSideboard:
			return true
		}
	}
	return false
}

func collapseEntries(entries []deck.Entry) deck.Deck {
	type key struct {
		ref cards.Ref
	}
	counts := map[key]int{}
	order := []key{}
	for _, e := range entries {
		k := key{ref: e.Ref}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	var main []deck.Entry
	for _, k := range order {
		main = append(main, deck.Entry{Ref: k.ref, Count: counts[k], Section: deck.SectionMain})
	}
	return deck.Deck{Main: main}
}

func LooksLikeDeckCode(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 8 {
		return false
	}
	for _, ch := range text {
		if strings.IndexByte("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", byte(ch)) < 0 {
			return false
		}
	}
	return strings.HasPrefix(strings.ToUpper(text), "CI")
}

func LooksLikePixelborn(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 16 {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(text)
	return err == nil && !LooksLikeDeckCode(text)
}

func LooksLikeTTS(text string) bool {
	tokens := strings.Fields(strings.TrimSpace(text))
	if len(tokens) == 0 {
		return false
	}
	for _, t := range tokens {
		if !ttsTokenPattern.MatchString(strings.ToUpper(t)) {
			return false
		}
	}
	return true
}
