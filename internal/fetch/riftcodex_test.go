package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/database"
)

func TestRiftCodexFetchCardsPaginatesAndMaps(t *testing.T) {
	requestedPages := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cards" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("size") != "100" {
			t.Errorf("size = %q, want 100", r.URL.Query().Get("size"))
		}
		if r.URL.Query().Get("sort") != "public_code" || r.URL.Query().Get("dir") != "1" {
			t.Errorf("sort params = %q, %q; want public_code, 1", r.URL.Query().Get("sort"), r.URL.Query().Get("dir"))
		}
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			t.Errorf("invalid page: %v", err)
			http.Error(w, "bad page", http.StatusBadRequest)
			return
		}
		requestedPages = append(requestedPages, page)
		var item any
		switch page {
		case 1:
			item = riftCodexFixture(
				"Jayce - Defender of Tomorrow",
				"ven-149-166",
				"VEN",
				149,
				"Legend",
				[]string{"Mind", "Body"},
			)
		case 2:
			item = riftCodexFixture(
				"Jayce, Brilliant Inventor (Alternate Art)",
				"ven-068a-166",
				"VEN",
				68,
				"Unit",
				[]string{"Mind"},
			)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{item},
			"total": 2,
			"page":  page,
			"size":  100,
			"pages": 2,
		})
	}))
	defer server.Close()

	client := NewRiftCodexClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	got, err := client.FetchCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("FetchCards() returned %d cards, want 2", len(got))
	}
	if len(requestedPages) != 2 || requestedPages[0] != 1 || requestedPages[1] != 2 {
		t.Fatalf("requested pages = %v, want [1 2]", requestedPages)
	}
	if got[0].ID != "ven-149-166" || got[0].Name != "Jayce, Defender of Tomorrow" ||
		got[0].SetID != "VEN" || got[0].Faction != "Mind, Body" {
		t.Fatalf("first mapped card = %+v", got[0])
	}
	if got[1].Variant != "a" {
		t.Fatalf("alternate-art variant = %q, want a", got[1].Variant)
	}
}

func TestRiftCodexFetchCardsRejectsMalformedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{riftCodexFixture("Broken", "not-an-id", "VEN", 1, "Unit", nil)},
			"total": 1,
			"page":  1,
			"size":  100,
			"pages": 1,
		})
	}))
	defer server.Close()

	client := NewRiftCodexClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	if _, err := client.FetchCards(context.Background()); err == nil {
		t.Fatal("FetchCards() accepted malformed riftbound_id")
	}
}

func TestRiftCodexFetchCardsPrefersCompleteDuplicate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		placeholder := riftCodexFixture("Defender of Tomorrow", "ven-149-166", "VEN", 149, "Legend", nil)
		placeholder["new"] = true
		complete := riftCodexFixture("Jayce - Defender of Tomorrow", "ven-149-166", "VEN", 149, "Legend", nil)
		complete["new"] = false
		complete["tcgplayer_id"] = "706020"
		complete["metadata"].(map[string]any)["clean_name"] = "Jayce Defender of Tomorrow"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{placeholder, complete},
			"total": 2,
			"page":  1,
			"size":  100,
			"pages": 1,
		})
	}))
	defer server.Close()

	client := NewRiftCodexClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	got, err := client.FetchCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Jayce, Defender of Tomorrow" {
		t.Fatalf("FetchCards() = %+v, want complete duplicate", got)
	}
}

func TestCanonicalRiftCodexName(t *testing.T) {
	if got := canonicalRiftCodexName("Jayce - Defender of Tomorrow"); got != "Jayce, Defender of Tomorrow" {
		t.Fatalf("canonicalRiftCodexName() = %q", got)
	}
	if got := canonicalRiftCodexName("Thousand-Tailed Watcher"); got != "Thousand-Tailed Watcher" {
		t.Fatalf("canonicalRiftCodexName() changed an unspaced hyphen: %q", got)
	}
}

func TestRiftCodexMapsExceptionalIDVariants(t *testing.T) {
	tests := []struct {
		id      string
		variant string
	}{
		{id: "ogn-r01-298", variant: ""},
		{id: "sfd-t03", variant: "t"},
		{id: "ven-sp2-006", variant: "sp"},
		{id: "ven-068a-166", variant: "a"},
		{id: "ogn-303*-298", variant: "*"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			rawMap := riftCodexFixture("Fixture", tt.id, "VEN", 1, "Unit", nil)
			encoded, err := json.Marshal(rawMap)
			if err != nil {
				t.Fatal(err)
			}
			var raw riftCodexCard
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			got, err := toRiftCodexCard(raw, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if got.Variant != tt.variant {
				t.Fatalf("variant = %q, want %q", got.Variant, tt.variant)
			}
		})
	}
}

func TestUpdateDBFallsBackAfterPrimaryFailure(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	primary := &fakeProvider{name: "riftcodex", err: errors.New("unavailable")}
	backup := &fakeProvider{name: "riftscribe", cards: []cards.Card{fetchTestCard()}}
	result, err := UpdateDB(context.Background(), db, primary, backup)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "riftscribe" || result.Count != 1 || len(result.Failures) != 1 {
		t.Fatalf("UpdateDB() result = %+v", result)
	}
	if !primary.called || !backup.called {
		t.Fatalf("provider calls: primary=%v backup=%v", primary.called, backup.called)
	}
	source, ok, err := db.GetMeta(context.Background(), "source")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || source != "riftscribe" {
		t.Fatalf("source metadata = %q, %v", source, ok)
	}
}

func TestUpdateDBDoesNotCallBackupAfterPrimarySuccess(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	primary := &fakeProvider{name: "riftcodex", cards: []cards.Card{fetchTestCard()}}
	backup := &fakeProvider{name: "riftscribe", err: errors.New("must not be called")}
	result, err := UpdateDB(context.Background(), db, primary, backup)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "riftcodex" || backup.called {
		t.Fatalf("result = %+v, backup called = %v", result, backup.called)
	}
}

type fakeProvider struct {
	name   string
	cards  []cards.Card
	err    error
	called bool
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) FetchCards(context.Context) ([]cards.Card, error) {
	p.called = true
	return p.cards, p.err
}

func fetchTestCard() cards.Card {
	return cards.Card{
		ID:              "ven-149-166",
		Name:            "Jayce - Defender of Tomorrow",
		SetID:           "VEN",
		CollectorNumber: 149,
		Type:            "Legend",
		UpdatedAt:       time.Now().UTC(),
	}
}

func riftCodexFixture(name, id, setID string, number int, cardType string, domains []string) map[string]any {
	return map[string]any{
		"name":             name,
		"riftbound_id":     id,
		"collector_number": number,
		"attributes": map[string]any{
			"energy": nil,
			"might":  nil,
			"power":  nil,
		},
		"classification": map[string]any{
			"type":   cardType,
			"rarity": "Rare",
			"domain": domains,
		},
		"set": map[string]any{
			"set_id": setID,
		},
		"orientation": "portrait",
		"metadata": map[string]any{
			"updated_on": "2026-07-17T20:38:03.339948+00:00",
		},
	}
}
