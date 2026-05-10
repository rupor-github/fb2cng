package pdf

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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

type builtinFontFace struct {
	PostScriptName string
	Data           []byte
	Font           *sfnt.Font
	UnitsPerEm     int
	Ascent         int
	Descent        int
	CapHeight      int
	BBox           [4]int
	Flags          int
	ItalicAngle    int
}

type builtinFamily struct {
	Regular *builtinFontFace
	Bold    *builtinFontFace
	Italic  *builtinFontFace
	BoldIt  *builtinFontFace
}

func (f *builtinFamily) match(bold, italic bool) *builtinFontFace {
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
	builtinErr   error
	builtinSerif builtinFamily
	builtinSans  builtinFamily
	builtinMono  builtinFamily
)

func builtinFont(fontFamily string, bold, italic bool) (*builtinFontFace, error) {
	if err := initBuiltinFonts(); err != nil {
		return nil, err
	}
	return builtinFamilyFor(fontFamily).match(bold, italic), nil
}

func initBuiltinFonts() error {
	builtinOnce.Do(func() {
		builtinSerif, builtinErr = loadBuiltinFamily("Literata", false, literataRegularGZ, literataBoldGZ, literataItalicGZ, literataBoldItalicGZ)
		if builtinErr != nil {
			return
		}
		builtinSans, builtinErr = loadBuiltinFamily("NotoSans", false, notoSansRegularGZ, notoSansBoldGZ, notoSansItalicGZ, notoSansBoldItalicGZ)
		if builtinErr != nil {
			return
		}

		regular, err := loadBuiltinFont("NotoSansMono-Regular", notoMonoRegularGZ, true, false)
		if err != nil {
			builtinErr = err
			return
		}
		bold, err := loadBuiltinFont("NotoSansMono-Bold", notoMonoBoldGZ, true, false)
		if err != nil {
			builtinErr = err
			return
		}
		builtinMono = builtinFamily{
			Regular: regular,
			Bold:    bold,
			Italic:  regular,
			BoldIt:  bold,
		}
	})
	return builtinErr
}

func loadBuiltinFamily(name string, fixedPitch bool, regularGZ, boldGZ, italicGZ, boldItalicGZ []byte) (builtinFamily, error) {
	regular, err := loadBuiltinFont(name+"-Regular", regularGZ, fixedPitch, false)
	if err != nil {
		return builtinFamily{}, err
	}
	bold, err := loadBuiltinFont(name+"-Bold", boldGZ, fixedPitch, false)
	if err != nil {
		return builtinFamily{}, err
	}
	italic, err := loadBuiltinFont(name+"-Italic", italicGZ, fixedPitch, true)
	if err != nil {
		return builtinFamily{}, err
	}
	boldItalic, err := loadBuiltinFont(name+"-BoldItalic", boldItalicGZ, fixedPitch, true)
	if err != nil {
		return builtinFamily{}, err
	}
	return builtinFamily{Regular: regular, Bold: bold, Italic: italic, BoldIt: boldItalic}, nil
}

func loadBuiltinFont(label string, gzData []byte, fixedPitch, italic bool) (*builtinFontFace, error) {
	data, err := gunzipFont(gzData)
	if err != nil {
		return nil, fmt.Errorf("load builtin font %s: %w", label, err)
	}
	face, err := loadRawFont(label, data, fixedPitch, italic)
	if err != nil {
		return nil, fmt.Errorf("load builtin font %s: %w", label, err)
	}
	return face, nil
}

func loadRawFont(label string, data []byte, fixedPitch, italic bool) (*builtinFontFace, error) {
	parsed, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}

	units := int(parsed.UnitsPerEm())
	ppem := fixed.I(units)
	metrics, err := parsed.Metrics(nil, ppem, font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("read metrics: %w", err)
	}
	bounds, err := parsed.Bounds(nil, ppem, font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("read bounds: %w", err)
	}

	postScriptName, err := parsed.Name(nil, sfnt.NameIDPostScript)
	if err != nil || postScriptName == "" {
		postScriptName = label
	}

	flags := 1 << 5 // Nonsymbolic.
	if fixedPitch {
		flags |= 1
	}
	if strings.Contains(strings.ToLower(postScriptName), "literata") {
		flags |= 1 << 1 // Serif.
	}
	if italic {
		flags |= 1 << 6
	}

	return &builtinFontFace{
		PostScriptName: sanitizePDFName(postScriptName),
		Data:           data,
		Font:           parsed,
		UnitsPerEm:     units,
		Ascent:         metrics.Ascent.Round(),
		Descent:        -metrics.Descent.Round(),
		CapHeight:      metrics.CapHeight.Round(),
		BBox: [4]int{
			bounds.Min.X.Round(),
			bounds.Min.Y.Round(),
			bounds.Max.X.Round(),
			bounds.Max.Y.Round(),
		},
		Flags:       flags,
		ItalicAngle: 0,
	}, nil
}

func gunzipFont(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
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

func sanitizePDFName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "FBCFont"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "FBCFont"
	}
	return b.String()
}
