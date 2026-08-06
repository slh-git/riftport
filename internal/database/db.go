// db.go — SQLite card database, schema migration, and FTS5 search index.
// Stores card metadata locally for offline convert, inspect, and search.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/slh/riftport/internal/cards"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS cards (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	name_key TEXT NOT NULL,
	set_id TEXT NOT NULL,
	collector_number INTEGER NOT NULL,
	variant TEXT NOT NULL DEFAULT '',
	rarity TEXT,
	faction TEXT,
	type TEXT,
	orientation TEXT,
	energy INTEGER,
	might INTEGER,
	power INTEGER,
	is_banned INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_cards_set_num ON cards(set_id, collector_number, variant);

CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS cards_fts USING fts5(
	card_id UNINDEXED,
	name,
	set_id,
	type,
	faction,
	tokenize='trigram'
);
`

// DB wraps a SQLite connection and card/search operations.
type DB struct {
	sql *sql.DB
}

// DefaultPath returns the default database file path (~/.riftport/cards.db).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "riftport.db"
	}
	return filepath.Join(home, ".riftport", "cards.db")
}

// Open opens or creates the database at path and applies migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.sql.Close()
}

// migrate applies the cards, meta, and FTS5 schema if not present.
func (db *DB) migrate() error {
	if _, err := db.sql.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	hasNameKey, err := db.hasColumn("cards", "name_key")
	if err != nil {
		return fmt.Errorf("inspect schema: %w", err)
	}
	if !hasNameKey {
		if _, err := db.sql.Exec(`ALTER TABLE cards ADD COLUMN name_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add cards.name_key: %w", err)
		}
	}
	if err := db.backfillNameKeys(); err != nil {
		return fmt.Errorf("backfill cards.name_key: %w", err)
	}
	if _, err := db.sql.Exec(`CREATE INDEX IF NOT EXISTS idx_cards_name_key ON cards(name_key)`); err != nil {
		return fmt.Errorf("index cards.name_key: %w", err)
	}
	return nil
}

// UpsertCard inserts or updates one card record.
func (db *DB) UpsertCard(ctx context.Context, c cards.Card) error {
	return upsertCard(ctx, db.sql, c)
}

type cardExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func upsertCard(ctx context.Context, execer cardExecer, c cards.Card) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO cards (
			id, name, name_key, set_id, collector_number, variant,
			rarity, faction, type, orientation,
			energy, might, power, is_banned, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			name_key=excluded.name_key,
			set_id=excluded.set_id,
			collector_number=excluded.collector_number,
			variant=excluded.variant,
			rarity=excluded.rarity,
			faction=excluded.faction,
			type=excluded.type,
			orientation=excluded.orientation,
			energy=excluded.energy,
			might=excluded.might,
			power=excluded.power,
			is_banned=excluded.is_banned,
			updated_at=excluded.updated_at
	`,
		c.ID, c.Name, normalizeNameKey(c.Name), c.SetID, c.CollectorNumber, c.Variant,
		nullString(c.Rarity), nullString(c.Faction), nullString(c.Type), nullString(c.Orientation),
		nullInt(c.Energy), nullInt(c.Might), nullInt(c.Power), boolInt(c.IsBanned), c.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// ReplaceCards atomically replaces all cards, rebuilds search, and updates metadata.
func (db *DB) ReplaceCards(ctx context.Context, snapshot []cards.Card, metadata map[string]string) error {
	if len(snapshot) == 0 {
		return fmt.Errorf("refusing to replace cards with an empty snapshot")
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM cards_fts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cards`); err != nil {
		return err
	}
	for _, card := range snapshot {
		if err := upsertCard(ctx, tx, card); err != nil {
			return fmt.Errorf("upsert card %q: %w", card.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cards_fts(card_id, name, set_id, type, faction)
		SELECT id, name, set_id, COALESCE(type, ''), COALESCE(faction, '') FROM cards
	`); err != nil {
		return err
	}
	for key, value := range metadata {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO meta(key, value) VALUES(?, ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value
		`, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RebuildSearchIndex repopulates the FTS5 table from the cards table.
func (db *DB) RebuildSearchIndex(ctx context.Context) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM cards_fts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cards_fts(card_id, name, set_id, type, faction)
		SELECT id, name, set_id, COALESCE(type, ''), COALESCE(faction, '') FROM cards
	`); err != nil {
		return err
	}
	return tx.Commit()
}

// SetMeta stores a key-value metadata entry (e.g. last_updated, source).
func (db *DB) SetMeta(ctx context.Context, key, value string) error {
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	return err
}

// GetMeta retrieves a metadata value; the bool is false when the key is missing.
func (db *DB) GetMeta(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := db.sql.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Count returns the total number of cards in the database.
func (db *DB) Count(ctx context.Context) (int, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards`).Scan(&n)
	return n, err
}

// GetByID looks up a card by full database ID (case-insensitive).
func (db *DB) GetByID(ctx context.Context, id string) (cards.Card, error) {
	return db.scanCard(db.sql.QueryRowContext(ctx, `
		SELECT id, name, set_id, collector_number, variant,
			rarity, faction, type, orientation,
			energy, might, power, is_banned, updated_at
		FROM cards WHERE lower(id) = lower(?)
	`, id))
}

// GetByRef looks up a card by set, collector number, and variant.
func (db *DB) GetByRef(ctx context.Context, ref cards.Ref) (cards.Card, error) {
	return db.scanCard(db.sql.QueryRowContext(ctx, `
		SELECT id, name, set_id, collector_number, variant,
			rarity, faction, type, orientation,
			energy, might, power, is_banned, updated_at
		FROM cards
		WHERE upper(set_id) = upper(?) AND collector_number = ? AND variant = ?
		LIMIT 1
	`, ref.SetID, ref.CollectorNumber, ref.Variant))
}

// GetByName looks up a card by exact case-insensitive name match.
func (db *DB) GetByName(ctx context.Context, name string) (cards.Card, error) {
	trimmed := strings.TrimSpace(name)
	return db.scanCard(db.sql.QueryRowContext(ctx, `
		SELECT id, name, set_id, collector_number, variant,
			rarity, faction, type, orientation,
			energy, might, power, is_banned, updated_at
		FROM cards
		WHERE name_key = ?
		ORDER BY CASE WHEN lower(name) = lower(?) THEN 0 ELSE 1 END,
			variant, set_id, collector_number
		LIMIT 1
	`, normalizeNameKey(trimmed), trimmed))
}

// SearchResult pairs a card with its FTS relevance rank.
type SearchResult struct {
	Card cards.Card
	Rank float64
}

// Search performs a trigram FTS5 query and returns ranked results.
func (db *DB) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	rows, err := db.sql.QueryContext(ctx, `
		SELECT c.id, c.name, c.set_id, c.collector_number, c.variant,
			c.rarity, c.faction, c.type, c.orientation,
			c.energy, c.might, c.power, c.is_banned, c.updated_at,
			fts.rank
		FROM cards_fts fts
		JOIN cards c ON c.id = fts.card_id
		WHERE cards_fts MATCH ?
		ORDER BY fts.rank
		LIMIT ?
	`, ftsQuery(q), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		var c cards.Card
		var rarity, faction, typ, orientation sql.NullString
		var energy, might, power sql.NullInt64
		var banned int
		var updated string
		var rank float64
		if err := rows.Scan(
			&c.ID, &c.Name, &c.SetID, &c.CollectorNumber, &c.Variant,
			&rarity, &faction, &typ, &orientation,
			&energy, &might, &power, &banned, &updated, &rank,
		); err != nil {
			return nil, err
		}
		c.Rarity = rarity.String
		c.Faction = faction.String
		c.Type = typ.String
		c.Orientation = orientation.String
		c.Energy = intPtr(energy)
		c.Might = intPtr(might)
		c.Power = intPtr(power)
		c.IsBanned = banned == 1
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, SearchResult{Card: c, Rank: rank})
	}
	return out, rows.Err()
}

// scanCard reads one cards row from a sql.Row into a cards.Card.
func (db *DB) scanCard(row *sql.Row) (cards.Card, error) {
	var c cards.Card
	var rarity, faction, typ, orientation sql.NullString
	var energy, might, power sql.NullInt64
	var banned int
	var updated string
	err := row.Scan(
		&c.ID, &c.Name, &c.SetID, &c.CollectorNumber, &c.Variant,
		&rarity, &faction, &typ, &orientation,
		&energy, &might, &power, &banned, &updated,
	)
	if err != nil {
		return cards.Card{}, err
	}
	c.Rarity = rarity.String
	c.Faction = faction.String
	c.Type = typ.String
	c.Orientation = orientation.String
	c.Energy = intPtr(energy)
	c.Might = intPtr(might)
	c.Power = intPtr(power)
	c.IsBanned = banned == 1
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return c, nil
}

// ftsQuery wraps a user query as an FTS5 phrase search.
func ftsQuery(q string) string {
	escaped := strings.ReplaceAll(q, `"`, `""`)
	return `"` + escaped + `"`
}

// nullString converts a Go string to sql.NullString for optional DB columns.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullInt converts an optional int pointer to sql.NullInt64.
func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// boolInt converts a bool to SQLite integer 0/1.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intPtr converts sql.NullInt64 to an optional int pointer.
func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func (db *DB) hasColumn(table, column string) (bool, error) {
	rows, err := db.sql.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (db *DB) backfillNameKeys() error {
	rows, err := db.sql.Query(`SELECT id, name FROM cards WHERE name_key = ''`)
	if err != nil {
		return err
	}
	var pending [][2]string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, [2]string{id, normalizeNameKey(name)})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range pending {
		if _, err := db.sql.Exec(`UPDATE cards SET name_key = ? WHERE id = ?`, item[1], item[0]); err != nil {
			return err
		}
	}
	return nil
}

// normalizeNameKey removes provider-specific punctuation and spacing from names.
func normalizeNameKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
