package pdf

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"strings"
	"sync"

	foliFont "github.com/carlos7ags/folio/font"
)

// Embedded font data (gzip-compressed TTF).
// Literata — serif (SIL OFL, https://github.com/googlefonts/literata)
//
//go:embed fonts/Literata-Regular.ttf.gz
var literataRegularGZ []byte

//go:embed fonts/Literata-Bold.ttf.gz
var literataBoldGZ []byte

//go:embed fonts/Literata-Italic.ttf.gz
var literataItalicGZ []byte

//go:embed fonts/Literata-BoldItalic.ttf.gz
var literataBoldItalicGZ []byte

// Noto Sans — sans-serif (SIL OFL, https://github.com/notofonts/latin-greek-cyrillic)
//
//go:embed fonts/NotoSans-Regular.ttf.gz
var notoSansRegularGZ []byte

//go:embed fonts/NotoSans-Bold.ttf.gz
var notoSansBoldGZ []byte

//go:embed fonts/NotoSans-Italic.ttf.gz
var notoSansItalicGZ []byte

//go:embed fonts/NotoSans-BoldItalic.ttf.gz
var notoSansBoldItalicGZ []byte

// Noto Sans Mono — monospace (SIL OFL, https://github.com/notofonts/latin-greek-cyrillic)
// No italic variants available; regular is reused for italic.
//
//go:embed fonts/NotoSansMono-Regular.ttf.gz
var notoMonoRegularGZ []byte

//go:embed fonts/NotoSansMono-Bold.ttf.gz
var notoMonoBoldGZ []byte

// builtinFamily groups the four style variants of an embedded font family.
type builtinFamily struct {
	Regular *foliFont.EmbeddedFont
	Bold    *foliFont.EmbeddedFont
	Italic  *foliFont.EmbeddedFont
	BoldIt  *foliFont.EmbeddedFont
}

func (f *builtinFamily) match(bold, italic bool) *foliFont.EmbeddedFont {
	switch {
	case bold && italic:
		return f.BoldIt
	case bold:
		return f.Bold
	case italic:
		return f.Italic
	default:
		return f.Regular
	}
}

var (
	builtinOnce  sync.Once
	builtinSerif builtinFamily
	builtinSans  builtinFamily
	builtinMono  builtinFamily
)

func initBuiltinFonts() {
	builtinOnce.Do(func() {
		builtinSerif = builtinFamily{
			Regular: mustParseGZ(literataRegularGZ),
			Bold:    mustParseGZ(literataBoldGZ),
			Italic:  mustParseGZ(literataItalicGZ),
			BoldIt:  mustParseGZ(literataBoldItalicGZ),
		}
		builtinSans = builtinFamily{
			Regular: mustParseGZ(notoSansRegularGZ),
			Bold:    mustParseGZ(notoSansBoldGZ),
			Italic:  mustParseGZ(notoSansItalicGZ),
			BoldIt:  mustParseGZ(notoSansBoldItalicGZ),
		}
		monoRegular := mustParseGZ(notoMonoRegularGZ)
		monoBold := mustParseGZ(notoMonoBoldGZ)
		builtinMono = builtinFamily{
			Regular: monoRegular,
			Bold:    monoBold,
			Italic:  monoRegular, // no italic variant available
			BoldIt:  monoBold,    // no italic variant available
		}
	})
}

func mustParseGZ(data []byte) *foliFont.EmbeddedFont {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		panic("builtin font decompress: " + err.Error())
	}
	ttf, err := io.ReadAll(r)
	if err != nil {
		panic("builtin font decompress: " + err.Error())
	}
	face, err := foliFont.ParseFont(ttf)
	if err != nil {
		panic("builtin font parse: " + err.Error())
	}
	return foliFont.NewEmbeddedFont(face)
}

// builtinFont returns the embedded font matching the style's font-family
// and bold/italic flags.
func builtinFont(style resolvedStyle) *foliFont.EmbeddedFont {
	initBuiltinFonts()
	family := builtinFamilyFor(style.FontFamily)
	return family.match(style.Bold, style.Italic)
}

func builtinFamilyFor(fontFamily string) *builtinFamily {
	name := strings.ToLower(strings.TrimSpace(fontFamily))
	switch {
	case strings.Contains(name, "monospace") || strings.Contains(name, "courier") || name == "mono":
		return &builtinMono
	case strings.Contains(name, "sans"):
		return &builtinSans
	default:
		return &builtinSerif
	}
}
