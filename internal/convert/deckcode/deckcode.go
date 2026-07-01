// deckcode.go — Piltover Archive share-code encoder and decoder.
// Implements base32 + varint deck codes (CI…) per @piltoverarchive/riftbound-deck-codes.
package deckcode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/deck"
)

const (
	formatID      = 1
	maxVersion    = 4
	base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

var (
	setMap = map[string]byte{
		"OGN": 0,
		"OGS": 1,
		"ARC": 2,
		"SFD": 3,
		"UNL": 4,
	}
	variantMap = map[string]byte{
		"":  0,
		"a": 1,
		"s": 2,
		"*": 2,
		"b": 3,
	}
	revSetMap     map[byte]string
	revVariantMap map[byte]string
)

func init() {
	revSetMap = make(map[byte]string, len(setMap))
	for k, v := range setMap {
		revSetMap[v] = k
	}
	revVariantMap = make(map[byte]string, len(variantMap))
	for k, v := range variantMap {
		if _, ok := revVariantMap[v]; !ok {
			revVariantMap[v] = k
		}
	}
}

type countedCard struct {
	Ref   cards.Ref
	Count int
}

// Decode parses a share code string into a deck.Deck.
func Decode(code string) (deck.Deck, error) {
	code = strings.TrimSpace(code)
	bytes, err := base32Decode(code)
	if err != nil {
		return deck.Deck{}, err
	}
	tr := newVarintTranslator(bytes)
	formatVersion, err := tr.popByte()
	if err != nil {
		return deck.Deck{}, err
	}
	format := formatVersion >> 4
	version := formatVersion & 0x0f
	if format != formatID {
		return deck.Deck{}, fmt.Errorf("unsupported deck code format %d", format)
	}
	if version > maxVersion {
		return deck.Deck{}, fmt.Errorf("unsupported deck code version %d", version)
	}

	mainCards, err := decodeSection(tr, 12, version)
	if err != nil {
		return deck.Deck{}, err
	}
	var sideCards []countedCard
	if version >= 2 {
		sideCards, err = decodeSection(tr, 3, version)
		if err != nil {
			return deck.Deck{}, err
		}
	}
	var champion *cards.Ref
	if version >= 3 {
		hasChampion, err := tr.popByte()
		if err != nil {
			return deck.Deck{}, err
		}
		if hasChampion == 1 {
			ref, err := decodeChampion(tr, version)
			if err != nil {
				return deck.Deck{}, err
			}
			champion = &ref
		}
	}

	return toDeck(mainCards, sideCards, champion), nil
}

// Encode serializes a deck.Deck into a share code string.
func Encode(d deck.Deck) (string, error) {
	main := entriesToCounted(d.Main)
	side := entriesToCounted(d.Sideboard)
	version := byte(3)
	for _, c := range append(main, side...) {
		if c.Ref.IsRune {
			version = 4
			break
		}
	}
	if d.ChosenChampion != nil && d.ChosenChampion.IsRune {
		version = 4
	}

	var out []byte
	out = append(out, (formatID<<4)|version)
	out = append(out, encodeSection(main, 12, version)...)
	out = append(out, encodeSection(side, 3, version)...)
	if d.ChosenChampion != nil {
		out = append(out, 1)
		out = append(out, encodeChampion(*d.ChosenChampion, version)...)
	} else {
		out = append(out, 0)
	}
	return base32Encode(out), nil
}

// decodeSection decodes main (maxCount 12) or sideboard (maxCount 3) card groups.
func decodeSection(tr *varintTranslator, maxCount int, version byte) ([]countedCard, error) {
	var result []countedCard
	for count := maxCount; count >= 1; count-- {
		numGroups, err := tr.popVarint()
		if err != nil {
			return nil, err
		}
		for g := 0; g < numGroups; g++ {
			numCards, err := tr.popVarint()
			if err != nil {
				return nil, err
			}
			setID, err := tr.popByte()
			if err != nil {
				return nil, err
			}
			variantID, err := tr.popByte()
			if err != nil {
				return nil, err
			}
			setCode, ok := revSetMap[setID]
			if !ok {
				return nil, fmt.Errorf("unknown set id %d", setID)
			}
			variant := revVariantMap[variantID]
			for i := 0; i < numCards; i++ {
				ref := cards.Ref{SetID: setCode, Variant: variant}
				if version >= 4 {
					isRune, err := tr.popByte()
					if err != nil {
						return nil, err
					}
					num, err := tr.popVarint()
					if err != nil {
						return nil, err
					}
					if isRune == 1 {
						ref.IsRune = true
						ref.CollectorNumber = num
					} else {
						ref.CollectorNumber = num
					}
				} else {
					num, err := tr.popVarint()
					if err != nil {
						return nil, err
					}
					ref.CollectorNumber = num
				}
				result = append(result, countedCard{Ref: ref, Count: count})
			}
		}
	}
	return result, nil
}

// decodeChampion decodes the optional chosen champion field (format v3+).
func decodeChampion(tr *varintTranslator, version byte) (cards.Ref, error) {
	setID, err := tr.popByte()
	if err != nil {
		return cards.Ref{}, err
	}
	variantID, err := tr.popByte()
	if err != nil {
		return cards.Ref{}, err
	}
	setCode, ok := revSetMap[setID]
	if !ok {
		return cards.Ref{}, fmt.Errorf("unknown champion set id %d", setID)
	}
	ref := cards.Ref{SetID: setCode, Variant: revVariantMap[variantID]}
	if version >= 4 {
		isRune, err := tr.popByte()
		if err != nil {
			return cards.Ref{}, err
		}
		num, err := tr.popVarint()
		if err != nil {
			return cards.Ref{}, err
		}
		if isRune == 1 {
			ref.IsRune = true
		}
		ref.CollectorNumber = num
	} else {
		num, err := tr.popVarint()
		if err != nil {
			return cards.Ref{}, err
		}
		ref.CollectorNumber = num
	}
	return ref, nil
}

type setVariantGroup struct {
	setID   byte
	variant byte
	numbers []string
}

// encodeSection encodes card groups by copy count, set, and variant.
func encodeSection(cardsList []countedCard, maxCount int, version byte) []byte {
	var out []byte
	for count := maxCount; count >= 1; count-- {
		var bucket []countedCard
		for _, c := range cardsList {
			if c.Count == count {
				bucket = append(bucket, c)
			}
		}
		groups := groupBySetVariant(bucket)
		out = append(out, encodeVarint(len(groups))...)
		for _, g := range groups {
			out = append(out, encodeVarint(len(g.numbers))...)
			out = append(out, g.setID, g.variant)
			for _, num := range g.numbers {
				if version >= 4 {
					if strings.HasPrefix(num, "R") {
						out = append(out, 1)
						n, _ := strconv.Atoi(num[1:])
						out = append(out, encodeVarint(n)...)
					} else {
						out = append(out, 0)
						n, _ := strconv.Atoi(num)
						out = append(out, encodeVarint(n)...)
					}
				} else {
					n, _ := strconv.Atoi(strings.TrimPrefix(num, "R"))
					out = append(out, encodeVarint(n)...)
				}
			}
		}
	}
	return out
}

// groupBySetVariant groups cards for compact varint encoding.
func groupBySetVariant(items []countedCard) []setVariantGroup {
	m := map[string]*setVariantGroup{}
	for _, item := range items {
		code := item.Ref.ShortCode()
		parts := strings.SplitN(code, "-", 2)
		setCode := parts[0]
		rest := parts[1]
		match := strings.TrimSuffix(rest, item.Ref.Variant)
		key := setCode + "|" + item.Ref.Variant
		if m[key] == nil {
			m[key] = &setVariantGroup{
				setID:   setMap[setCode],
				variant: variantMap[item.Ref.Variant],
			}
		}
		m[key].numbers = append(m[key].numbers, match)
	}
	groups := make([]setVariantGroup, 0, len(m))
	for _, g := range m {
		sort.Slice(g.numbers, func(i, j int) bool {
			return g.numbers[i] < g.numbers[j]
		})
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].setID != groups[j].setID {
			return groups[i].setID < groups[j].setID
		}
		return groups[i].variant < groups[j].variant
	})
	return groups
}

// encodeChampion writes the chosen champion bytes into the code payload.
func encodeChampion(ref cards.Ref, version byte) []byte {
	var out []byte
	out = append(out, setMap[ref.SetID], variantMap[ref.Variant])
	if version >= 4 {
		if ref.IsRune {
			out = append(out, 1)
		} else {
			out = append(out, 0)
		}
		out = append(out, encodeVarint(ref.CollectorNumber)...)
	} else {
		out = append(out, encodeVarint(ref.CollectorNumber)...)
	}
	return out
}

// entriesToCounted converts deck entries to internal counted-card slices.
func entriesToCounted(entries []deck.Entry) []countedCard {
	out := make([]countedCard, 0, len(entries))
	for _, e := range entries {
		out = append(out, countedCard{Ref: e.Ref, Count: e.Count})
	}
	return out
}

// toDeck builds a deck.Deck from decoded counted cards and optional champion.
func toDeck(main, side []countedCard, champion *cards.Ref) deck.Deck {
	d := deck.Deck{ChosenChampion: champion}
	for _, c := range main {
		d.Main = append(d.Main, deck.Entry{Ref: c.Ref, Count: c.Count, Section: deck.SectionMain})
	}
	for _, c := range side {
		d.Sideboard = append(d.Sideboard, deck.Entry{Ref: c.Ref, Count: c.Count, Section: deck.SectionSideboard})
	}
	return d
}

// base32Encode encodes bytes using the deck-code base32 alphabet.
func base32Encode(data []byte) string {
	var result strings.Builder
	var buffer uint
	bitsLeft := 0
	for _, b := range data {
		buffer = (buffer << 8) | uint(b)
		bitsLeft += 8
		for bitsLeft >= 5 {
			bitsLeft -= 5
			result.WriteByte(base32Alphabet[(buffer>>bitsLeft)&0x1f])
		}
	}
	if bitsLeft > 0 {
		buffer <<= 5 - bitsLeft
		result.WriteByte(base32Alphabet[buffer&0x1f])
	}
	return result.String()
}

// base32Decode decodes a deck code string into raw bytes.
func base32Decode(s string) ([]byte, error) {
	var out []byte
	var buffer uint
	bitsLeft := 0
	for _, ch := range strings.ToUpper(s) {
		idx := strings.IndexByte(base32Alphabet, byte(ch))
		if idx < 0 {
			return nil, fmt.Errorf("invalid deck code character %q", ch)
		}
		buffer = (buffer << 5) | uint(idx)
		bitsLeft += 5
		for bitsLeft >= 8 {
			bitsLeft -= 8
			out = append(out, byte((buffer>>bitsLeft)&0xff))
		}
	}
	return out, nil
}

type varintTranslator struct {
	data []byte
}

// newVarintTranslator creates a byte reader for deck code payloads.
func newVarintTranslator(data []byte) *varintTranslator {
	return &varintTranslator{data: append([]byte(nil), data...)}
}

// popByte reads and consumes one byte from the translator buffer.
func (v *varintTranslator) popByte() (byte, error) {
	if len(v.data) == 0 {
		return 0, fmt.Errorf("unexpected end of deck code bytes")
	}
	b := v.data[0]
	v.data = v.data[1:]
	return b, nil
}

// popVarint reads and consumes one variable-length integer.
func (v *varintTranslator) popVarint() (int, error) {
	if len(v.data) == 0 {
		return 0, fmt.Errorf("no bytes available to read varint")
	}
	result := 0
	shift := 0
	consumed := 0
	for i, b := range v.data {
		consumed = i + 1
		result |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			v.data = v.data[consumed:]
			return result, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("invalid varint in deck code")
}

// encodeVarint writes a variable-length integer as bytes.
func encodeVarint(value int) []byte {
	if value == 0 {
		return []byte{0}
	}
	var out []byte
	for value != 0 {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}
