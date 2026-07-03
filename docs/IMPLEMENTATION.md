# Riftport — Architecture Reference

Technical reference for repository layout, file responsibilities, data flow, and dependencies.

---

## Commands

```text
riftport update-db   → fetch card metadata into ~/.riftport/cards.db
riftport convert     → transform deck text between formats (offline after update-db)
riftport inspect     → print one card from local DB
riftport search      → fuzzy search local FTS index
```

### Supported deck formats

| Format | Example |
|--------|---------|
| `names` | `3 Seal of Unity` |
| `piltover` | Sectioned list (Legend, Main Deck, Battlefields, Runes, Sideboard) |
| `tts` | `OGN-265-1 OGN-246-1` |
| `pixelborn` | Base64 of `$`-separated TTS tokens |
| `deckcode` | `CIAAAAAAAA…` (Piltover Archive / RiftMana share codes) |

---

## Repository layout

```text
riftport/
├── cmd/riftport/main.go          CLI entrypoint
├── internal/
│   ├── cmd/                      Cobra commands
│   ├── cards/                    Card model and code parsing
│   ├── database/                 SQLite + FTS5
│   ├── deck/                     Canonical deck representation
│   ├── fetch/                    Remote card acquisition (update-db only)
│   └── convert/
│       ├── convert.go            Conversion orchestration
│       ├── format.go             Format enum
│       ├── formats/              Per-format parsers and writers
│       └── deckcode/             Share-code codec
├── docs/IMPLEMENTATION.md        This file
├── README.md                     User-facing documentation
├── go.mod / go.sum
└── .gitignore
```

---

## File reference

### `cmd/riftport/main.go`

**Purpose:** Binary entrypoint. Delegates to `internal/cmd.Execute()` and exits non-zero on error.

| Function | Description |
|----------|-------------|
| `main` | Runs the CLI; prints errors to stderr |

---

### `internal/cmd/root.go`

**Purpose:** Root Cobra command, global `--db` flag, and shared stdin/file input helper.

| Function | Description |
|----------|-------------|
| `Execute` | Builds root command, registers subcommands, runs Cobra |
| `readInput` | Reads deck text from file arg, `-`, or stdin |

---

### `internal/cmd/update_db.go`

**Purpose:** `riftport update-db` — the only command that uses the network.

| Function | Description |
|----------|-------------|
| `newUpdateDBCmd` | Creates Cobra command; opens DB, calls RiftScribe client, prints card count |

---

### `internal/cmd/convert.go`

**Purpose:** `riftport convert` — format transformation with optional DB for name resolution.

| Function | Description |
|----------|-------------|
| `newConvertCmd` | Creates Cobra command with `--from` / `--to` flags |
| `needsDB` | Returns true when input or output requires local card names |

---

### `internal/cmd/inspect.go`

**Purpose:** `riftport inspect <id-or-code>` — offline single-card lookup.

| Function | Description |
|----------|-------------|
| `newInspectCmd` | Creates Cobra command; tries ID, short code, then TTS token lookup |
| `intOrDash` | Formats optional int stat for display |

---

### `internal/cmd/search.go`

**Purpose:** `riftport search <query>` — offline FTS search.

| Function | Description |
|----------|-------------|
| `newSearchCmd` | Creates Cobra command with `--limit` flag |

---

### `internal/cards/ref.go`

**Purpose:** Card reference parsing and string formatting (short codes and TTS tokens).

| Function | Description |
|----------|-------------|
| `ParseShortCode` | Parses `OGN-265`, `OGN-007a`, `OGN-R01` |
| `ParseTTS` | Parses `OGN-265-1` (art index → variant) |
| `Ref.ShortCode` | Renders canonical short code |
| `Ref.TTSToken` | Renders TTS export token |
| `Ref.LookupKey` | Lowercase DB lookup key |

---

### `internal/cards/card.go`

**Purpose:** Locally stored card metadata struct.

| Function | Description |
|----------|-------------|
| `Card.Ref` | Converts stored card to a `cards.Ref` |

---

### `internal/deck/deck.go`

**Purpose:** Canonical in-memory deck model used by all format parsers and writers.

| Type / Function | Description |
|-----------------|-------------|
| `Section` | Legend, main, battlefield, rune, sideboard |
| `Entry` | Card ref or name hint, count, section |
| `Deck` | Main list, sideboard, optional chosen champion |
| `Deck.AllEntries` | Main + sideboard combined |
| `Deck.FlatMain` | Main deck or all entries if main is empty |

---

### `internal/database/db.go`

**Purpose:** SQLite persistence, schema migration, card CRUD, and FTS5 search index.

| Function | Description |
|----------|-------------|
| `DefaultPath` | Returns `~/.riftport/cards.db` |
| `Open` | Opens/creates DB, runs migrations |
| `DB.Close` | Closes connection |
| `DB.migrate` | Applies schema (cards, meta, cards_fts) |
| `DB.UpsertCard` | Insert or update one card |
| `DB.RebuildSearchIndex` | Rebuilds FTS5 from cards table |
| `DB.SetMeta` / `GetMeta` | Key-value metadata (last_updated, source) |
| `DB.Count` | Total card count |
| `DB.GetByID` | Lookup by full ID (`ogn-265-298`) |
| `DB.GetByRef` | Lookup by set + number + variant |
| `DB.GetByName` | Exact case-insensitive name match |
| `DB.Search` | FTS5 trigram search with rank |
| `DB.scanCard` | Scans one row into `cards.Card` |
| `ftsQuery` | Escapes query for FTS phrase match |
| `nullString`, `nullInt`, `boolInt`, `intPtr` | SQL null helpers |

---

### `internal/fetch/riftscribe.go`

**Purpose:** RiftScribe HTTP client used exclusively by `update-db`.

| Function | Description |
|----------|-------------|
| `NewClient` | Client with 30s timeout and default API base URL |
| `Client.UpdateDB` | Paginates `/api/cards`, upserts all, rebuilds index, sets meta |
| `Client.listCards` | GET one page; reads `X-Total-Count` header |
| `toCard` | Maps API JSON to `cards.Card` |

---

### `internal/convert/format.go`

**Purpose:** Deck format identifier enum and validation.

| Function | Description |
|----------|-------------|
| `Format.Valid` | Returns true for known format strings |

---

### `internal/convert/convert.go`

**Purpose:** Conversion pipeline orchestration: detect, parse, resolve names, render.

| Function | Description |
|----------|-------------|
| `Resolver.ResolveDeck` | Resolves all name hints via local DB |
| `Resolver.resolveEntry` | Resolves one entry by name or passes through code ref |
| `DetectFormat` | Auto-detects deckcode, pixelborn, TTS, or name/sectioned text |
| `Parse` | Dispatches to format-specific parser |
| `Render` | Dispatches to format-specific writer |
| `Convert` | Full pipeline: parse → resolve if needed → render |
| `needsResolve` | True when entries have name hints without card refs |

---

### `internal/convert/formats/formats.go`

**Purpose:** Per-format parse and format functions plus auto-detect heuristics.

| Function | Description |
|----------|-------------|
| `ParseTTS` / `FormatTTS` | Tabletop Simulator space-separated tokens |
| `ParsePixelborn` / `FormatPixelborn` | Base64 `$`-separated TTS tokens |
| `ParseDeckCode` / `FormatDeckCode` | Delegates to `deckcode` package |
| `ParseNames` / `FormatNames` | Quantity + name lines |
| `ParsePiltover` / `FormatPiltover` | Sectioned name lists |
| `namesToDeck` | Splits entries into main vs sideboard |
| `parseSectionHeader` | Recognizes Legend, Main Deck, Runes, etc. |
| `writeNameSection` | Writes one section of a name list |
| `filterSection` | Filters entries by section |
| `hasSections` | True if deck uses legend/battlefield/rune sections |
| `collapseEntries` | Merges duplicate card refs with counts |
| `LooksLikeDeckCode` | Heuristic: base32 chars, starts with `CI` |
| `LooksLikePixelborn` | Heuristic: valid base64, not deckcode |
| `LooksLikeTTS` | Heuristic: all tokens match TTS pattern |

---

### `internal/convert/deckcode/deckcode.go`

**Purpose:** Share-code codec for `CI…` deck strings (base32 + varint, v1–v4).

| Function | Description |
|----------|-------------|
| `Decode` | Base32 decode → varint parse → `deck.Deck` |
| `Encode` | `deck.Deck` → varint bytes → base32 string |
| `decodeSection` | Decodes main (counts 1–12) or sideboard (1–3) |
| `decodeChampion` | Decodes optional chosen champion (v3+) |
| `encodeSection` | Encodes a deck section with count grouping |
| `groupBySetVariant` | Groups cards for efficient encoding |
| `encodeChampion` | Encodes chosen champion bytes |
| `entriesToCounted` | Converts deck entries to counted cards |
| `toDeck` | Builds `deck.Deck` from decoded counts |
| `base32Encode` / `base32Decode` | Custom base32 alphabet (RFC-style) |
| `varintTranslator.popByte` / `popVarint` | Reads bytes and varints from buffer |
| `encodeVarint` | Writes variable-length integer |

---

### `internal/convert/deckcode/deckcode_test.go`

**Purpose:** Round-trip test for deck code encode/decode.

| Function | Description |
|----------|-------------|
| `TestDeckCodeRoundTrip` | Encodes a sample deck and verifies decode entry count |
| `mustRef` | Test helper: parse short code or panic |

---

### Other files

| File | Purpose |
|------|---------|
| `README.md` | User-facing install, usage, architecture overview |
| `go.mod` / `go.sum` | Go module definition and dependency checksums |
| `.gitignore` | Ignores built binary, SQLite files, `.DS_Store` |

---

## Data flow

```text
update-db:
  RiftScribe API → fetch.Client → database.UpsertCard → RebuildSearchIndex → ~/.riftport/cards.db

convert (name-based):
  stdin/file → formats.Parse* → Resolver (DB lookup) → formats.Format* → stdout

convert (code-based):
  stdin/file → formats.Parse* → formats.Format* → stdout   (no DB required)

inspect / search:
  argv → database.Get* / Search → stdout
```

---

## Dependencies

| Package | Use |
|---------|-----|
| `github.com/spf13/cobra` | CLI framework |
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO) |

---

## What was intentionally not built

- No automatic background sync
- No Riot API integration (manual `update-db` only)
- No GUI or web server
- No deck validation against format rules (legends, rune counts, etc.)
