package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slh/riftport/internal/cards"
)

func TestGetByNameNormalizesProviderPunctuation(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	card := testCard("ven-149-166", "Jayce - Defender of Tomorrow", "VEN", 149)
	if err := db.UpsertCard(context.Background(), card); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetByName(context.Background(), "Jayce, Defender of Tomorrow")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != card.ID {
		t.Fatalf("GetByName() ID = %q, want %q", got.ID, card.ID)
	}
}

func TestOpenBackfillsLegacyNameKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacySchema := strings.Replace(schema, "\tname_key TEXT NOT NULL,\n", "", 1)
	if _, err := raw.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		INSERT INTO cards (
			id, name, set_id, collector_number, variant, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "ven-149-166", "Jayce - Defender of Tomorrow", "VEN", 149, "", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.GetByName(context.Background(), "Jayce, Defender of Tomorrow"); err != nil {
		t.Fatalf("lookup after migration: %v", err)
	}
}

func TestReplaceCardsReplacesSnapshotAndMetadata(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	oldCard := testCard("ogn-001-298", "Old Card", "OGN", 1)
	if err := db.UpsertCard(ctx, oldCard); err != nil {
		t.Fatal(err)
	}
	newCard := testCard("ven-149-166", "Jayce - Defender of Tomorrow", "VEN", 149)
	if err := db.ReplaceCards(ctx, []cards.Card{newCard}, map[string]string{"source": "riftcodex"}); err != nil {
		t.Fatal(err)
	}

	count, err := db.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}
	if _, err := db.GetByID(ctx, oldCard.ID); err != sql.ErrNoRows {
		t.Fatalf("old card lookup error = %v, want sql.ErrNoRows", err)
	}
	source, ok, err := db.GetMeta(ctx, "source")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || source != "riftcodex" {
		t.Fatalf("source metadata = %q, %v; want riftcodex, true", source, ok)
	}
	results, err := db.Search(ctx, "Defender", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Card.ID != newCard.ID {
		t.Fatalf("Search() = %+v, want new card", results)
	}

	if err := db.ReplaceCards(ctx, nil, nil); err == nil {
		t.Fatal("ReplaceCards() with empty snapshot succeeded")
	}
	count, err = db.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("Count() after rejected replacement = %d, want 1", count)
	}
}

func testCard(id, name, setID string, collectorNumber int) cards.Card {
	return cards.Card{
		ID:              id,
		Name:            name,
		SetID:           setID,
		CollectorNumber: collectorNumber,
		Type:            "Unit",
		UpdatedAt:       time.Now().UTC(),
	}
}
