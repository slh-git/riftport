# Riftcodex API Reference

Reference for the [Riftcodex](https://riftcodex.com) public HTTP API — an alternative Riftbound card data source.

**Site:** https://riftcodex.com  
**API base URL:** `https://api.riftcodex.com` (not `riftcodex.com/api`)  
**OpenAPI spec:** https://api.riftcodex.com/openapi.json  
**Interactive docs:** https://api.riftcodex.com/docs (Swagger) / https://api.riftcodex.com/redoc  
**Version:** 0.2.0 (as of 2026-07)

> **Homepage caveat:** The landing page example uses `GET https://api.riftcodex.com/api/cards?limit=20` — this is **wrong**. The real path is `GET /cards` with `size` and `page` params (no `/api` prefix, no `limit` param).

---

## Card endpoints

| Endpoint | Response | Purpose |
|----------|----------|---------|
| `GET /cards` | `Page[Card]` | Paginated list — **full `Card` objects** |
| `GET /cards/{id}` | `Card` | Lookup by Riftcodex internal MongoDB ID |
| `GET /cards/riftbound/{id}` | `Card[]` | Lookup by Riftbound ID (e.g. `ogn-001-298`) |
| `GET /cards/tcgplayer/{tcgplayer_id}` | `Card` | Lookup by TCGPlayer product ID |
| `GET /cards/name` | `Page[Card]` | Exact or fuzzy name match |
| `GET /cards/search` | `Page[Card]` | Full-text search over card text |

### Pagination (`GET /cards`)

```text
GET /cards?size=100&page=1
```

| Param | Default | Notes |
|-------|---------|-------|
| `size` | 50 | Page size, **max 100** |
| `page` | 1 | 1-based page number |
| `set_id` | — | Filter by set, e.g. `ogn`, `sfd` (case insensitive) |
| `sort` | — | `name`, `collector_number`, `set_id`, `rarity`, `energy`, etc. |
| `dir` | 1 | `1` ascending, `-1` descending |

Response shape (`Page[Card]`):

```json
{
  "items": [ /* Card[] */ ],
  "total": 1064,
  "page": 1,
  "size": 100,
  "pages": 11
}
```

Unlike RiftScribe, the list endpoint returns the **full card payload** — no separate summary vs detail schema, and no per-card follow-up requests needed.

---

## `Card` schema

Top-level object returned by all card endpoints.

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Riftcodex internal ID (MongoDB ObjectId) |
| `name` | string | Display name, includes variant suffix e.g. "(Alternate Art)" |
| `riftbound_id` | string | Canonical game ID, e.g. `ogn-001-298`, `ogn-303*-298` |
| `tcgplayer_id` | string? | TCGPlayer product ID |
| `collector_number` | int | Number within set |
| `attributes` | object | `energy`, `might`, `power` (nullable ints) |
| `classification` | object | `type`, `supertype`, `rarity`, `domain[]` |
| `text` | object | `rich` (HTML), `plain`, `flavour` |
| `set` | object | `set_id`, `label` |
| `media` | object | `image_url`, `artist`, `accessibility_text` |
| `tags` | string[] | Region/champion tags, e.g. `["Dragon", "Noxus"]` |
| `orientation` | string | `portrait` or `landscape` |
| `metadata` | object | `clean_name`, `updated_on`, `alternate_art`, `overnumbered`, `signature` |

Nested schemas in OpenAPI: `Attributes`, `Classification`, `Text`, `Media`, `Metadata`, `CardSet`.

### Variant encoding

Riftcodex does **not** expose a separate `variant` field. Variants are encoded in `riftbound_id` and/or `metadata` booleans:

| Variant | `riftbound_id` example | `metadata` flags |
|---------|------------------------|------------------|
| Base | `ogn-001-298` | all false |
| Alt art | `ogn-214a-298` | `alternate_art: true` |
| Signature | `ogn-303*-298` | `signature: true` |
| Overnumbered | `ogn-303-298` (same number, different name) | `overnumbered: true` |

`riftbound_id` format: `{set}-{number}[variant]-{set_size}` (lowercase set).

---

## Lookup examples

```bash
# Paginated full card list
curl "https://api.riftcodex.com/cards?set_id=ogn&size=100&page=1"

# By Riftbound ID (returns array — usually 0 or 1 match)
curl "https://api.riftcodex.com/cards/riftbound/ogn-001-298"

# By exact name
curl "https://api.riftcodex.com/cards/name?exact=Blazing%20Scorcher"

# By fuzzy name
curl "https://api.riftcodex.com/cards/name?fuzzy=Blazing"

# Full-text search (searches card text, not just names)
curl "https://api.riftcodex.com/cards/search?query=accelerate&size=5"

# By TCGPlayer ID
curl "https://api.riftcodex.com/cards/tcgplayer/652771"
```

---

## Sets and index endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /sets` | All sets with `set_id`, `card_count`, TCGPlayer/Cardmarket IDs |
| `GET /sets/{id}` | Single set by Riftcodex ID |
| `GET /sets/set-id/{set_id}` | Single set by game set code (e.g. `OGN`) |
| `GET /index/domains` | All domains (Fury, Calm, Chaos, etc.) |
| `GET /index/keywords` | All keywords (Accelerate, Ambush, etc.) |
| `GET /index/rarities` | All rarities |
| `GET /index/card-types` | Unit, Spell, Legend, Rune, etc. |
| `GET /index/card-names` | All distinct card names (~979) |
| `GET /index/tags` | All tags |
| `GET /index/artists` | All artists |
| `GET /index/energy` / `might` / `power` | Stat value indexes |

---

## Other endpoints

| Endpoint | Notes |
|----------|-------|
| `GET /health` | `{"status":"healthy", ...}` |
| `GET /` | `{"message":"Riftbound TCG API is running."}` |
| `GET /metrics` | Prometheus metrics (plain text, not JSON) |

---

## Comparison with RiftScribe

| | RiftScribe | Riftcodex |
|---|-----------|-----------|
| Base URL | `riftscribe.gg/api` | `api.riftcodex.com` |
| Total cards | ~950 | ~1064 |
| List endpoint | `CardSummaryRead` (subset) | Full `Card` |
| Detail endpoint | `GET /api/cards/{id}` → `CardRead` | Same data in list; lookup via `/cards/riftbound/{id}` |
| Pagination | `limit` + `offset`, max 200 | `size` + `page`, max 100 |
| Total count | `X-Total-Count` header | `total` in response body |
| Faction/domain | `faction` (string) | `classification.domain` (array) |
| Variant | explicit `variant` field | encoded in `riftbound_id` + `metadata` |
| Keywords | `keywords[]` on card | `/index/keywords` only; embedded in `text.plain` |
| Images | `cdn.riftscribe.gg` with thumbnails | Riot CDN (`cmsassets.rgpub.io`) |
| Banned status | `is_banned` | **not exposed** |
| TCGPlayer ID | no | `tcgplayer_id` |
| Card text | `description` (plain) | `text.rich` (HTML) + `text.plain` + `text.flavour` |
| Auth | none (public) | none (public) |

---

## Known quirks (2026-07)

- Homepage docs show wrong URL (`/api/cards`) and wrong pagination param (`limit`).
- `limit` query param is **ignored**; use `size`.
- `/cards/search?query=Scorcher` returned 0 results; `query=accelerate` works — search targets card text/keywords, not names. Use `/cards/name` for name lookups.
- `/cards/name?name=...` returns HTTP 500; correct param is `exact=` or `fuzzy=`.
- `/cards/riftbound/ogn-001a-298` returned `[]` when tested — alt-art IDs may not always resolve via this route.

---

## Relevance to riftport

Riftport currently uses RiftScribe (`internal/fetch/riftscribe.go`). Riftcodex is a viable alternative or supplement:

**Advantages for riftport:**
- Full card data in one paginated call (~11 requests at `size=100` for all cards).
- `text.plain` gives ability text without per-card detail fetches.
- `tcgplayer_id` for marketplace integration.
- `classification.domain[]` maps to riftport's `Faction` field (first domain, or join).
- `metadata` booleans + `riftbound_id` can derive riftport's `variant` field.

**Gaps vs current riftport model:**
- No `is_banned` field.
- Variant must be parsed from `riftbound_id` or `metadata`, not a direct field.
- `riftbound_id` includes set-size suffix (`-298`) — differs from riftport short codes (`OGN-001`).
- Images are Riot CDN URLs, not locally cacheable thumbnails.
