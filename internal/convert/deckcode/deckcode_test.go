package deckcode_test

import (
	"testing"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/convert/deckcode"
	"github.com/slh/riftport/internal/deck"
)

func TestDeckCodeRoundTrip(t *testing.T) {
	original := deck.Deck{
		Main: []deck.Entry{
			{Ref: mustRef("OGN-265"), Count: 1},
			{Ref: mustRef("OGN-246"), Count: 1},
			{Ref: mustRef("OGN-245"), Count: 3},
		},
	}
	code, err := deckcode.Encode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := deckcode.Decode(code)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Main) != 3 {
		t.Fatalf("expected 3 main entries, got %d", len(decoded.Main))
	}
}

func mustRef(code string) cards.Ref {
	ref, err := cards.ParseShortCode(code)
	if err != nil {
		panic(err)
	}
	return ref
}
