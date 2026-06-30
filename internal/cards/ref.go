package cards

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ref identifies a card by set, collector number, and optional variant suffix.
type Ref struct {
	SetID           string
	CollectorNumber int
	Variant         string // "", "a", "s", "*", "b"
	IsRune          bool
}

var (
	ttsPattern      = regexp.MustCompile(`^([A-Z]{3})-(\d{1,3})-(\d+)$`)
	shortPattern    = regexp.MustCompile(`^([A-Z]{3})-((?:R)?\d{1,3})([a-z*]?)$`)
	nameLinePattern = regexp.MustCompile(`^(\d+)\s+(.+)$`)
)

// ParseShortCode parses codes like OGN-265, OGN-007a, OGN-R01.
func ParseShortCode(code string) (Ref, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	m := shortPattern.FindStringSubmatch(code)
	if m == nil {
		return Ref{}, fmt.Errorf("invalid card code: %q", code)
	}
	ref := Ref{SetID: m[1]}
	numPart := m[2]
	variant := m[3]
	if strings.HasPrefix(numPart, "R") {
		ref.IsRune = true
		n, err := strconv.Atoi(numPart[1:])
		if err != nil {
			return Ref{}, fmt.Errorf("invalid rune number in %q", code)
		}
		ref.CollectorNumber = n
	} else {
		n, err := strconv.Atoi(numPart)
		if err != nil {
			return Ref{}, fmt.Errorf("invalid collector number in %q", code)
		}
		ref.CollectorNumber = n
	}
	ref.Variant = variant
	return ref, nil
}

// ParseTTS parses Tabletop Simulator tokens like OGN-265-1.
func ParseTTS(token string) (Ref, error) {
	token = strings.TrimSpace(strings.ToUpper(token))
	m := ttsPattern.FindStringSubmatch(token)
	if m == nil {
		return Ref{}, fmt.Errorf("invalid TTS token: %q", token)
	}
	num, err := strconv.Atoi(m[2])
	if err != nil {
		return Ref{}, err
	}
	art, err := strconv.Atoi(m[3])
	if err != nil {
		return Ref{}, err
	}
	ref := Ref{SetID: m[1], CollectorNumber: num}
	if art > 1 {
		ref.Variant = "a"
	}
	return ref, nil
}

// ShortCode returns the canonical short code (OGN-265, OGN-007a, OGN-R01).
func (r Ref) ShortCode() string {
	var num string
	if r.IsRune {
		num = fmt.Sprintf("R%02d", r.CollectorNumber)
	} else {
		num = fmt.Sprintf("%03d", r.CollectorNumber)
	}
	return fmt.Sprintf("%s-%s%s", r.SetID, num, r.Variant)
}

// TTSToken returns a TTS export token (OGN-265-1).
func (r Ref) TTSToken() string {
	art := 1
	if r.Variant == "a" {
		art = 2
	}
	return fmt.Sprintf("%s-%03d-%d", r.SetID, r.CollectorNumber, art)
}

// LookupKey returns the database lookup key used for set+number+variant.
func (r Ref) LookupKey() string {
	return strings.ToLower(fmt.Sprintf("%s-%d-%s", r.SetID, r.CollectorNumber, r.Variant))
}
