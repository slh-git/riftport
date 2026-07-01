// convert.go — deck conversion pipeline orchestration.
// Detects format, parses to deck.Deck, resolves names via DB, and renders output.
package convert

import (
	"context"
	"fmt"
	"strings"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/convert/formats"
	"github.com/slh/riftport/internal/database"
	"github.com/slh/riftport/internal/deck"
)

// Resolver maps unresolved card name hints to cards.Ref using the local database.
type Resolver struct {
	DB *database.DB
}

// ResolveDeck resolves every name-based entry in main and sideboard.
func (r *Resolver) ResolveDeck(ctx context.Context, d deck.Deck) (deck.Deck, error) {
	out := deck.Deck{ChosenChampion: d.ChosenChampion, ChosenChampionHint: d.ChosenChampionHint}
	for _, e := range d.Main {
		resolved, err := r.resolveEntry(ctx, e)
		if err != nil {
			return deck.Deck{}, err
		}
		out.Main = append(out.Main, resolved)
	}
	for _, e := range d.Sideboard {
		resolved, err := r.resolveEntry(ctx, e)
		if err != nil {
			return deck.Deck{}, err
		}
		out.Sideboard = append(out.Sideboard, resolved)
	}
	if d.ChosenChampionHint != "" {
		card, err := r.DB.GetByName(ctx, d.ChosenChampionHint)
		if err != nil {
			return deck.Deck{}, fmt.Errorf("chosen champion %q: %w", d.ChosenChampionHint, err)
		}
		ref := card.Ref()
		out.ChosenChampion = &ref
		out.ChosenChampionHint = ""
	}
	if d.ChosenChampion != nil && out.ChosenChampion == nil {
		card, err := r.DB.GetByRef(ctx, *d.ChosenChampion)
		if err != nil {
			return deck.Deck{}, fmt.Errorf("chosen champion %s: %w", d.ChosenChampion.ShortCode(), err)
		}
		ref := card.Ref()
		out.ChosenChampion = &ref
	}
	return out, nil
}

// resolveEntry resolves one entry by name lookup or returns it unchanged if already coded.
func (r *Resolver) resolveEntry(ctx context.Context, e deck.Entry) (deck.Entry, error) {
	if e.NameHint == "" && e.Ref.SetID != "" {
		return e, nil
	}
	if e.NameHint == "" {
		return deck.Entry{}, fmt.Errorf("unresolved deck entry")
	}
	card, err := r.DB.GetByName(ctx, e.NameHint)
	if err != nil {
		return deck.Entry{}, fmt.Errorf("card %q: %w", e.NameHint, err)
	}
	return deck.Entry{
		Ref:     card.Ref(),
		Count:   e.Count,
		Section: e.Section,
	}, nil
}

// DetectFormat guesses the input format from deck text heuristics.
func DetectFormat(text string) Format {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return FormatNames
	}
	if formats.LooksLikeDeckCode(trimmed) {
		return FormatDeckCode
	}
	if formats.LooksLikePixelborn(trimmed) {
		return FormatPixelborn
	}
	if formats.LooksLikeTTS(trimmed) {
		return FormatTTS
	}
	if formats.LooksLikeTCGArena(trimmed) {
		return FormatTCGArena
	}
	if strings.Contains(trimmed, "\n") {
		return FormatPiltover
	}
	return FormatNames
}

// Parse converts raw deck text in the given format into a deck.Deck.
func Parse(from Format, text string) (deck.Deck, error) {
	switch from {
	case FormatAuto:
		from = DetectFormat(text)
		return Parse(from, text)
	case FormatTTS:
		return formats.ParseTTS(text)
	case FormatPixelborn:
		return formats.ParsePixelborn(text)
	case FormatDeckCode:
		return formats.ParseDeckCode(text)
	case FormatPiltover:
		return formats.ParsePiltover(text)
	case FormatTCGArena:
		return formats.ParseTCGArena(text)
	case FormatNames:
		return formats.ParseNames(text)
	default:
		return deck.Deck{}, fmt.Errorf("unsupported input format %q", from)
	}
}

// Render writes a deck.Deck to the target format string.
func Render(ctx context.Context, to Format, d deck.Deck, db *database.DB) (string, error) {
	switch to {
	case FormatTTS:
		return formats.FormatTTS(d), nil
	case FormatPixelborn:
		return formats.FormatPixelborn(d), nil
	case FormatDeckCode:
		return formats.FormatDeckCode(d)
	case FormatPiltover, FormatNames:
		if db == nil {
			return "", fmt.Errorf("local card database required for %s output", to)
		}
		nameFn := func(ref cards.Ref) (string, error) {
			card, err := db.GetByRef(ctx, ref)
			if err != nil {
				return "", fmt.Errorf("lookup %s: %w", ref.ShortCode(), err)
			}
			return card.Name, nil
		}
		if to == FormatPiltover {
			return formats.FormatPiltover(d, nameFn)
		}
		return formats.FormatNames(d, nameFn)
	case FormatTCGArena:
		if db == nil {
			return "", fmt.Errorf("local card database required for %s output", to)
		}
		nameFn := func(ref cards.Ref) (string, error) {
			card, err := db.GetByRef(ctx, ref)
			if err != nil {
				return "", fmt.Errorf("lookup %s: %w", ref.ShortCode(), err)
			}
			return card.Name, nil
		}
		return formats.FormatTCGArena(d, nameFn)
	default:
		return "", fmt.Errorf("unsupported output format %q", to)
	}
}

// Convert runs the full parse → resolve → render pipeline.
func Convert(ctx context.Context, db *database.DB, from, to Format, text string) (string, error) {
	parsed, err := Parse(from, text)
	if err != nil {
		return "", err
	}
	if needsResolve(parsed) {
		if db == nil {
			return "", fmt.Errorf("local card database required to resolve card names")
		}
		resolver := Resolver{DB: db}
		parsed, err = resolver.ResolveDeck(ctx, parsed)
		if err != nil {
			return "", err
		}
	}
	return Render(ctx, to, parsed, db)
}

// needsResolve reports whether parsed deck entries require name-to-code DB lookup.
func needsResolve(d deck.Deck) bool {
	if d.ChosenChampionHint != "" {
		return true
	}
	for _, e := range d.AllEntries() {
		if e.NameHint != "" && e.Ref.SetID == "" {
			return true
		}
	}
	return false
}
