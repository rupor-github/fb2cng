package pdf

import (
	"fmt"
	"image"
	"image/draw"
	"strings"

	foliFont "github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"go.uber.org/zap"

	"fbc/content"
	"fbc/css"
	"fbc/fb2"
	imgutil "fbc/utils/images"
)

type fontVariant struct {
	Embedded *foliFont.EmbeddedFont
	Weight   string
	Style    string
}

type fontRegistry struct {
	families map[string][]fontVariant
	fallback *foliFont.EmbeddedFont
	log      *zap.Logger
}

func newFontRegistry(stylesheets []fb2.Stylesheet, parsed *css.Stylesheet, log *zap.Logger) *fontRegistry {
	if log == nil {
		log = zap.NewNop()
	}

	fr := &fontRegistry{
		families: make(map[string][]fontVariant),
		log:      log.Named("pdf-fonts"),
	}

	if parsed == nil {
		return fr
	}

	resources := make(map[string]*fb2.StylesheetResource)
	for i := range stylesheets {
		for j := range stylesheets[i].Resources {
			res := &stylesheets[i].Resources[j]
			resources[res.OriginalURL] = res
		}
	}

	for _, ff := range parsed.FontFaces() {
		family := normalizeFontFamilyName(ff.Family)
		if family == "" || ff.Src == "" {
			continue
		}

		url := extractURLFromCSSSrc(ff.Src)
		if url == "" {
			continue
		}

		res := resources[url]
		if res == nil || len(res.Data) == 0 {
			continue
		}

		face, err := foliFont.ParseFont(res.Data)
		if err != nil {
			fr.log.Debug("Skipping unloadable font-face resource",
				zap.String("family", family),
				zap.String("url", url),
				zap.Error(err))
			continue
		}

		fr.families[family] = append(fr.families[family], fontVariant{
			Embedded: foliFont.NewEmbeddedFont(face),
			Weight:   normalizeFontWeight(ff.Weight),
			Style:    normalizeFontStyle(ff.Style),
		})
	}

	return fr
}

func (fr *fontRegistry) resolve(style resolvedStyle, text string) (*foliFont.Standard, *foliFont.EmbeddedFont) {
	if fr != nil {
		if ef := fr.matchEmbedded(style); ef != nil {
			return nil, ef
		}
	}

	std := resolveStandardFont(style)
	if foliFont.CanEncodeWinAnsi(text) || fr == nil {
		return std, nil
	}

	if fr.fallback == nil {
		fr.fallback = loadFallbackEmbeddedFont(fr.log)
	}
	if fr.fallback != nil {
		return nil, fr.fallback
	}

	return std, nil
}

func (fr *fontRegistry) matchEmbedded(style resolvedStyle) *foliFont.EmbeddedFont {
	if fr == nil {
		return nil
	}

	family := normalizeFontFamilyName(style.FontFamily)
	variants := fr.families[family]
	if len(variants) == 0 {
		return nil
	}

	weight := "normal"
	if style.Bold {
		weight = "bold"
	}
	italic := "normal"
	if style.Italic {
		italic = "italic"
	}

	for _, variant := range variants {
		if variant.Weight == weight && variant.Style == italic {
			return variant.Embedded
		}
	}
	for _, variant := range variants {
		if variant.Style == italic {
			return variant.Embedded
		}
	}
	for _, variant := range variants {
		if variant.Weight == weight {
			return variant.Embedded
		}
	}
	return variants[0].Embedded
}

func normalizeFontFamilyName(family string) string {
	family = strings.TrimSpace(family)
	if family == "" {
		return ""
	}
	if idx := strings.IndexByte(family, ','); idx >= 0 {
		family = strings.TrimSpace(family[:idx])
	}
	family = strings.Trim(family, `"'`)
	return strings.ToLower(strings.TrimSpace(family))
}

func normalizeFontWeight(weight string) string {
	weight = strings.TrimSpace(strings.ToLower(weight))
	switch weight {
	case "", "normal", "400", "500":
		return "normal"
	case "bold", "bolder", "600", "700", "800", "900":
		return "bold"
	default:
		return weight
	}
}

func normalizeFontStyle(style string) string {
	style = strings.TrimSpace(strings.ToLower(style))
	switch style {
	case "italic", "oblique":
		return "italic"
	default:
		return "normal"
	}
}

func extractURLFromCSSSrc(src string) string {
	start := strings.Index(strings.ToLower(src), "url(")
	if start < 0 {
		return ""
	}
	start += 4
	end := strings.Index(src[start:], ")")
	if end < 0 {
		return ""
	}
	end += start
	url := strings.TrimSpace(src[start:end])
	return strings.Trim(url, `"'`)
}

func resolveStandardFont(style resolvedStyle) *foliFont.Standard {
	family := mapToStandardFamily(normalizeFontFamilyName(style.FontFamily))
	switch family {
	case "courier":
		switch {
		case style.Bold && style.Italic:
			return foliFont.CourierBoldOblique
		case style.Bold:
			return foliFont.CourierBold
		case style.Italic:
			return foliFont.CourierOblique
		default:
			return foliFont.Courier
		}
	case "times":
		switch {
		case style.Bold && style.Italic:
			return foliFont.TimesBoldItalic
		case style.Bold:
			return foliFont.TimesBold
		case style.Italic:
			return foliFont.TimesItalic
		default:
			return foliFont.TimesRoman
		}
	default:
		switch {
		case style.Bold && style.Italic:
			return foliFont.HelveticaBoldOblique
		case style.Bold:
			return foliFont.HelveticaBold
		case style.Italic:
			return foliFont.HelveticaOblique
		default:
			return foliFont.Helvetica
		}
	}
}

func loadFallbackEmbeddedFont(log *zap.Logger) *foliFont.EmbeddedFont {
	paths := []string{
		"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
	}
	for _, path := range paths {
		face, err := foliFont.LoadFont(path)
		if err == nil {
			return foliFont.NewEmbeddedFont(face)
		}
		if log != nil {
			log.Debug("Unable to load fallback font", zap.String("path", path), zap.Error(err))
		}
	}
	return nil
}

func mapToStandardFamily(family string) string {
	switch {
	case strings.Contains(family, "courier") || strings.Contains(family, "monospace") || family == "mono":
		return "courier"
	case strings.Contains(family, "times") || (strings.Contains(family, "serif") && !strings.Contains(family, "sans")):
		return "times"
	default:
		return "helvetica"
	}
}

func newPDFImage(img *fb2.BookImage) (*folioimage.Image, error) {
	if img == nil || len(img.Data) == 0 {
		return nil, fmt.Errorf("empty image")
	}

	mimeType := strings.ToLower(img.MimeType)
	switch mimeType {
	case "image/jpeg":
		return folioimage.NewJPEG(img.Data)
	case "image/png":
		return folioimage.NewPNG(img.Data)
	case "image/gif":
		return folioimage.NewGIF(img.Data)
	case "image/webp":
		return folioimage.NewWebP(img.Data)
	case "image/tiff":
		return folioimage.NewTIFF(img.Data)
	case "image/svg+xml":
		raster, err := imgutil.RasterizeSVGToImage(img.Data, 0, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("rasterize svg: %w", err)
		}
		rgba, ok := toRGBA(raster)
		if !ok {
			return nil, fmt.Errorf("convert svg raster to rgba")
		}
		return folioimage.NewFromGoImage(rgba), nil
	default:
		return nil, fmt.Errorf("unsupported image mime type %q", img.MimeType)
	}
}

func imageByID(c *content.Content, href string) (*fb2.BookImage, string) {
	if c == nil {
		return nil, ""
	}
	id := strings.TrimPrefix(strings.TrimSpace(href), "#")
	if id == "" {
		return nil, ""
	}
	img := c.ImagesIndex[id]
	if img != nil {
		c.TrackImageUsage(id)
	}
	return img, id
}

func toRGBA(img image.Image) (*image.RGBA, bool) {
	if img == nil {
		return nil, false
	}
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba, true
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba, true
}
