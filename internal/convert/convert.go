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

type Resolver struct {
	DB *database.DB
}

func (r *Resolver) ResolveDeck(ctx context.Context, d deck.Deck) (deck.Deck, error) {
	out := deck.Deck{ChosenChampion: d.ChosenChampion}
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
	if d.ChosenChampion != nil {
		card, err := r.DB.GetByRef(ctx, *d.ChosenChampion)
		if err != nil {
			return deck.Deck{}, fmt.Errorf("chosen champion %s: %w", d.ChosenChampion.ShortCode(), err)
		}
		ref := card.Ref()
		out.ChosenChampion = &ref
	}
	return out, nil
}

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
	if strings.Contains(trimmed, "\n") {
		return FormatPiltover
	}
	return FormatNames
}

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
	case FormatNames:
		return formats.ParseNames(text)
	default:
		return deck.Deck{}, fmt.Errorf("unsupported input format %q", from)
	}
}

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
	default:
		return "", fmt.Errorf("unsupported output format %q", to)
	}
}

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

func needsResolve(d deck.Deck) bool {
	for _, e := range d.AllEntries() {
		if e.NameHint != "" && e.Ref.SetID == "" {
			return true
		}
	}
	return false
}
