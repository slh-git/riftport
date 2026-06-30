package convert

type Format string

const (
	FormatAuto      Format = "auto"
	FormatNames     Format = "names"
	FormatTTS       Format = "tts"
	FormatPixelborn Format = "pixelborn"
	FormatPiltover  Format = "piltover"
	FormatDeckCode  Format = "deckcode"
)

func (f Format) Valid() bool {
	switch f {
	case FormatAuto, FormatNames, FormatTTS, FormatPixelborn, FormatPiltover, FormatDeckCode:
		return true
	default:
		return false
	}
}
