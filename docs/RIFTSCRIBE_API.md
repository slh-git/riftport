# RiftScribe API Reference

Reference for the [RiftScribe](https://riftscribe.gg) public HTTP API used by `riftport update-db`.

**OpenAPI spec:** https://riftscribe.gg/openapi.json  
**Base URL:** `https://riftscribe.gg/api`

---

## Card endpoints

| Endpoint | Response schema | Purpose |
|----------|-----------------|---------|
| `GET /api/cards` | `CardSummaryRead[]` | Paginated card list (summary fields only) |
| `GET /api/cards/{card_id}` | **`CardRead`** | Full card detail for one card |
| `GET /api/cards/search` | `CardSearchResult[]` | Lightweight search / autocomplete |
| `GET /api/cards/filters` | `CardFiltersRead` | Available filter metadata |

### Pagination (`GET /api/cards`)

```text
GET /api/cards?limit=200&offset=0
```

- `limit` — page size (riftport uses 200)
- `offset` — zero-based offset
- `X-Total-Count` response header — total number of cards

---

## `CardRead` vs `CardSummaryRead`

**`CardRead` is only returned by the single-card endpoint:**

```text
GET /api/cards/{card_id}
```

Example:

```bash
curl "https://riftscribe.gg/api/cards/ogn-001-298"
```

`GET /api/cards` returns **`CardSummaryRead`**, which is a subset of `CardRead`. Fields present on `CardRead` but **not** on the list endpoint:

| Field | Description |
|-------|-------------|
| `description` | Card ability text |
| `flavor_text` | Flavor text |
| `art` | Artwork details (image URLs, artist) |
| `keywords` | e.g. `["accelerate"]` |
| `tags` | Categorization tags |
| `prev_card_id` | Previous card in set (by collector number) |
| `next_card_id` | Next card in set (by collector number) |

Both schemas share: `id`, `name`, `set_id`, `collector_number`, `variant`, `rarity`, `faction`, `type`, `orientation`, `stats`, `image`, `image_thumb`, `image_blur_data_url`, `is_banned`.

There is **no bulk `CardRead` endpoint** (no `?full=true` or similar). To fetch full detail for every card:

1. Paginate `GET /api/cards` to collect IDs (~950 cards as of 2026-07).
2. Call `GET /api/cards/{card_id}` once per card.

---

## `CardSearchResult`

Returned by `GET /api/cards/search`. Minimal fields for typeahead:

- `card_id`, `name`, `type`, `set_id`, `thumbnail_url`, `is_banned`

---

## Schema definitions

Schemas live in the OpenAPI document, not a separate runtime endpoint:

```bash
curl -s https://riftscribe.gg/openapi.json | jq '.components.schemas.CardRead'
```

Relevant schema names:

- `CardRead` — full card detail
- `CardSummaryRead` — list view
- `CardSearchResult` — search results
- `CardStatsRead` — `energy`, `might`, `power`
- `CardArtRead` — artwork sub-object on `CardRead`
- `ImageThumbnails` — `small`, `medium`, `large` thumbnail URLs

---

## What riftport uses today

`internal/fetch/riftscribe.go` paginates `GET /api/cards` and maps `CardSummaryRead` fields into `cards.Card`. It does **not** call `/api/cards/{card_id}`, so description, keywords, flavor text, and art metadata are not stored locally.
