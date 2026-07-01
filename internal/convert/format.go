// format.go — deck format identifier enum.
// Used by convert and CLI --from / --to flags.
package convert

// Format names a supported deck text encoding.
type Format string

const (
	FormatAuto      Format = "auto"
	FormatNames     Format = "names"
	FormatTTS       Format = "tts"
	FormatPixelborn Format = "pixelborn"
	FormatPiltover  Format = "piltover"
	FormatTCGArena  Format = "tcgarena"
	FormatDeckCode  Format = "deckcode"
)

// Valid reports whether f is a known format string.
func (f Format) Valid() bool {
	switch f {
	case FormatAuto, FormatNames, FormatTTS, FormatPixelborn, FormatPiltover, FormatTCGArena, FormatDeckCode:
		return true
	default:
		return false
	}
}
