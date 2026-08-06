// riftscribe.go — RiftScribe API client for update-db.
// Paginates the public /api/cards endpoint and returns a complete card snapshot.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/slh/riftport/internal/cards"
)

const defaultBaseURL = "https://riftscribe.gg/api"

// RiftScribeClient fetches card data from the RiftScribe HTTP API.
type RiftScribeClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewRiftScribeClient returns a client with the default API base URL and timeout.
func NewRiftScribeClient() *RiftScribeClient {
	return &RiftScribeClient{
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type apiCard struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	SetID           string    `json:"set_id"`
	CollectorNumber int       `json:"collector_number"`
	Variant         string    `json:"variant"`
	Rarity          *string   `json:"rarity"`
	Faction         *string   `json:"faction"`
	Type            *string   `json:"type"`
	Orientation     *string   `json:"orientation"`
	Stats           *apiStats `json:"stats"`
	IsBanned        bool      `json:"is_banned"`
}

type apiStats struct {
	Energy *int `json:"energy"`
	Might  *int `json:"might"`
	Power  *int `json:"power"`
}

// Name identifies this provider in update metadata and user-facing output.
func (c *RiftScribeClient) Name() string {
	return "riftscribe"
}

// FetchCards downloads a complete card snapshot.
func (c *RiftScribeClient) FetchCards(ctx context.Context) ([]cards.Card, error) {
	offset := 0
	limit := 200
	now := time.Now().UTC()
	var out []cards.Card

	for {
		batch, countHeader, err := c.listCards(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, raw := range batch {
			out = append(out, toCard(raw, now))
		}
		offset += len(batch)
		if countHeader > 0 && offset >= countHeader {
			break
		}
		if len(batch) < limit {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("riftscribe returned no cards")
	}
	return out, nil
}

// listCards fetches one page of cards from the API.
func (c *RiftScribeClient) listCards(ctx context.Context, limit, offset int) ([]apiCard, int, error) {
	url := fmt.Sprintf("%s/cards?limit=%d&offset=%d", strings.TrimRight(c.BaseURL, "/"), limit, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	total := 0
	if h := resp.Header.Get("X-Total-Count"); h != "" {
		total, _ = strconv.Atoi(h)
	}
	var batch []apiCard
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, 0, err
	}
	return batch, total, nil
}

// toCard maps API JSON into a cards.Card with normalized set ID.
func toCard(raw apiCard, now time.Time) cards.Card {
	c := cards.Card{
		ID:              raw.ID,
		Name:            raw.Name,
		SetID:           strings.ToUpper(raw.SetID),
		CollectorNumber: raw.CollectorNumber,
		Variant:         raw.Variant,
		IsBanned:        raw.IsBanned,
		UpdatedAt:       now,
	}
	if raw.Rarity != nil {
		c.Rarity = *raw.Rarity
	}
	if raw.Faction != nil {
		c.Faction = *raw.Faction
	}
	if raw.Type != nil {
		c.Type = *raw.Type
	}
	if raw.Orientation != nil {
		c.Orientation = *raw.Orientation
	}
	if raw.Stats != nil {
		c.Energy = raw.Stats.Energy
		c.Might = raw.Stats.Might
		c.Power = raw.Stats.Power
	}
	return c
}
