// riftcodex.go — RiftCodex API client for the primary update-db source.
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slh/riftport/internal/cards"
)

const defaultRiftCodexBaseURL = "https://api.riftcodex.com"

var riftboundIDPattern = regexp.MustCompile(`(?i)^[^-]+-([a-z]*)(\d+)([a-z*]?)(?:-\d+)?$`)

// RiftCodexClient fetches complete card snapshots from RiftCodex.
type RiftCodexClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewRiftCodexClient returns a client with the production base URL and timeout.
func NewRiftCodexClient() *RiftCodexClient {
	return &RiftCodexClient{
		BaseURL: defaultRiftCodexBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name identifies this provider in update metadata and user-facing output.
func (c *RiftCodexClient) Name() string {
	return "riftcodex"
}

type riftCodexPage struct {
	Items []riftCodexCard `json:"items"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
	Pages int             `json:"pages"`
}

type riftCodexCard struct {
	Name            string  `json:"name"`
	RiftboundID     string  `json:"riftbound_id"`
	TCGPlayerID     *string `json:"tcgplayer_id"`
	CollectorNumber int     `json:"collector_number"`
	Attributes      struct {
		Energy *int `json:"energy"`
		Might  *int `json:"might"`
		Power  *int `json:"power"`
	} `json:"attributes"`
	Classification struct {
		Type   string   `json:"type"`
		Rarity string   `json:"rarity"`
		Domain []string `json:"domain"`
	} `json:"classification"`
	Set struct {
		SetID string `json:"set_id"`
	} `json:"set"`
	Orientation string `json:"orientation"`
	Metadata    struct {
		CleanName *string `json:"clean_name"`
		UpdatedOn *string `json:"updated_on"`
	} `json:"metadata"`
	New *bool `json:"new"`
}

// FetchCards downloads all 1-based pages from RiftCodex.
func (c *RiftCodexClient) FetchCards(ctx context.Context) ([]cards.Card, error) {
	const pageSize = 100
	now := time.Now().UTC()
	var out []cards.Card
	cardIndexes := make(map[string]int)
	cardScores := make(map[string]int)
	rawCount := 0

	for pageNumber := 1; ; pageNumber++ {
		page, err := c.listCards(ctx, pageSize, pageNumber)
		if err != nil {
			return nil, err
		}
		if page.Page != pageNumber {
			return nil, fmt.Errorf("riftcodex returned page %d while requesting page %d", page.Page, pageNumber)
		}
		if page.Pages < 1 || page.Total < 1 || len(page.Items) == 0 {
			return nil, fmt.Errorf("riftcodex returned an empty or invalid card page")
		}
		for _, raw := range page.Items {
			rawCount++
			card, err := toRiftCodexCard(raw, now)
			if err != nil {
				return nil, err
			}
			score := riftCodexCardScore(raw)
			if index, exists := cardIndexes[card.ID]; exists {
				if score > cardScores[card.ID] {
					out[index] = card
					cardScores[card.ID] = score
				}
				continue
			}
			cardIndexes[card.ID] = len(out)
			cardScores[card.ID] = score
			out = append(out, card)
		}
		if pageNumber >= page.Pages {
			if rawCount != page.Total {
				return nil, fmt.Errorf("riftcodex returned %d records, expected %d", rawCount, page.Total)
			}
			return out, nil
		}
	}
}

func riftCodexCardScore(card riftCodexCard) int {
	score := 0
	if card.New == nil || !*card.New {
		score += 4
	}
	if card.Metadata.CleanName != nil && strings.TrimSpace(*card.Metadata.CleanName) != "" {
		score += 2
	}
	if card.TCGPlayerID != nil && strings.TrimSpace(*card.TCGPlayerID) != "" {
		score++
	}
	return score
}

func (c *RiftCodexClient) listCards(ctx context.Context, size, page int) (riftCodexPage, error) {
	base := strings.TrimRight(c.BaseURL, "/") + "/cards"
	values := url.Values{}
	values.Set("size", strconv.Itoa(size))
	values.Set("page", strconv.Itoa(page))
	values.Set("sort", "public_code")
	values.Set("dir", "1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+values.Encode(), nil)
	if err != nil {
		return riftCodexPage{}, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return riftCodexPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return riftCodexPage{}, fmt.Errorf("riftcodex api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var pageResult riftCodexPage
	if err := json.NewDecoder(resp.Body).Decode(&pageResult); err != nil {
		return riftCodexPage{}, fmt.Errorf("decode riftcodex response: %w", err)
	}
	return pageResult, nil
}

func toRiftCodexCard(raw riftCodexCard, fallbackUpdatedAt time.Time) (cards.Card, error) {
	id := strings.ToLower(strings.TrimSpace(raw.RiftboundID))
	match := riftboundIDPattern.FindStringSubmatch(id)
	if match == nil {
		return cards.Card{}, fmt.Errorf("invalid riftcodex riftbound_id %q", raw.RiftboundID)
	}
	if strings.TrimSpace(raw.Name) == "" || strings.TrimSpace(raw.Set.SetID) == "" {
		return cards.Card{}, fmt.Errorf("riftcodex card %q is missing name or set", raw.RiftboundID)
	}

	variantPrefix := strings.ToLower(match[1])
	if variantPrefix == "r" {
		variantPrefix = ""
	}
	variant := variantPrefix + strings.ToLower(match[3])

	updatedAt := fallbackUpdatedAt
	if raw.Metadata.UpdatedOn != nil {
		if parsed, err := time.Parse(time.RFC3339Nano, *raw.Metadata.UpdatedOn); err == nil {
			updatedAt = parsed.UTC()
		}
	}
	return cards.Card{
		ID:              id,
		Name:            canonicalRiftCodexName(raw.Name),
		SetID:           strings.ToUpper(strings.TrimSpace(raw.Set.SetID)),
		CollectorNumber: raw.CollectorNumber,
		Variant:         variant,
		Rarity:          raw.Classification.Rarity,
		Faction:         strings.Join(raw.Classification.Domain, ", "),
		Type:            raw.Classification.Type,
		Orientation:     raw.Orientation,
		Energy:          raw.Attributes.Energy,
		Might:           raw.Attributes.Might,
		Power:           raw.Attributes.Power,
		IsBanned:        false,
		UpdatedAt:       updatedAt,
	}, nil
}

// canonicalRiftCodexName converts RiftCodex's spaced subtitle separator to the
// comma form used by TCG Arena and Piltover exports.
func canonicalRiftCodexName(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), " - ", ", ")
}
