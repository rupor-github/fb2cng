package pdf

import (
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

type pdfFontRegistry struct {
	families map[string]pdfEmbeddedFontFamily
	log      *zap.Logger
}

type pdfEmbeddedFontFamily struct {
	faces map[pdfFontVariant]*builtinFontFace
}

type pdfFontVariant struct {
	Bold   bool
	Italic bool
}

func newPDFFontRegistry(book *fb2.FictionBook, log *zap.Logger) *pdfFontRegistry {
	if log == nil {
		log = zap.NewNop()
	}
	registry := &pdfFontRegistry{families: make(map[string]pdfEmbeddedFontFamily), log: log.Named("pdf-fonts")}
	if book == nil {
		return registry
	}

	parser := css.NewParser(log)
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		registry.addStylesheetFonts(stylesheet, parser.Parse([]byte(stylesheet.Data), "pdf font stylesheet"))
	}
	return registry
}

func (r *pdfFontRegistry) addStylesheetFonts(stylesheet *fb2.Stylesheet, parsed *css.Stylesheet) {
	if r == nil || stylesheet == nil || parsed == nil {
		return
	}
	resources := make(map[string]*fb2.StylesheetResource)
	for i := range stylesheet.Resources {
		resource := &stylesheet.Resources[i]
		resources[resource.OriginalURL] = resource
	}

	for _, fontFace := range parsed.FontFaces() {
		family := strings.Trim(strings.TrimSpace(fontFace.Family), `"'`)
		if family == "" {
			continue
		}
		url := pdfFontFaceURL(fontFace.Src)
		if url == "" {
			continue
		}
		resource := resources[url]
		if resource == nil || len(resource.Data) == 0 || !pdfFontResourceMIMEType(resource.MimeType) {
			continue
		}
		variant := pdfFontVariant{Bold: pdfFontFaceBold(fontFace.Weight), Italic: pdfFontFaceItalic(fontFace.Style)}
		familyKey := strings.ToLower(family)
		embeddedFamily := r.families[familyKey]
		if embeddedFamily.faces == nil {
			embeddedFamily.faces = make(map[pdfFontVariant]*builtinFontFace)
		}
		if embeddedFamily.faces[variant] != nil {
			continue
		}
		face, err := loadRawFont(family+"-embedded", resource.Data, false, variant.Italic)
		if err != nil {
			r.log.Warn("Skipping unsupported PDF @font-face resource", zap.String("family", family), zap.String("url", url), zap.Error(err))
			continue
		}
		embeddedFamily.faces[variant] = face
		r.families[familyKey] = embeddedFamily
	}
}

func (r *pdfFontRegistry) fontForKey(key pdfFontKey) (*builtinFontFace, error) {
	if r != nil {
		if face := r.embeddedFont(key); face != nil {
			return face, nil
		}
	}
	return builtinFont(key.Family, key.Bold, key.Italic)
}

func (r *pdfFontRegistry) embeddedFont(key pdfFontKey) *builtinFontFace {
	if r == nil || len(r.families) == 0 {
		return nil
	}
	family := r.families[strings.ToLower(strings.TrimSpace(key.Family))]
	if len(family.faces) == 0 {
		return nil
	}
	variant := pdfFontVariant{Bold: key.Bold, Italic: key.Italic}
	if face := family.faces[variant]; face != nil {
		return face
	}
	fallbacks := []pdfFontVariant{
		{Bold: key.Bold},
		{Italic: key.Italic},
		{},
	}
	for _, fallback := range fallbacks {
		if face := family.faces[fallback]; face != nil {
			return face
		}
	}
	return nil
}

func pdfFontFaceURL(src string) string {
	start := strings.Index(strings.ToLower(src), "url(")
	if start < 0 {
		return ""
	}
	start += len("url(")
	end := strings.Index(src[start:], ")")
	if end < 0 {
		return ""
	}
	url := strings.TrimSpace(src[start : start+end])
	return strings.Trim(url, `"'`)
}

func pdfFontResourceMIMEType(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasPrefix(mimeType, "font/") ||
		strings.HasPrefix(mimeType, "application/font-") ||
		strings.HasPrefix(mimeType, "application/x-font-") ||
		mimeType == "application/vnd.ms-fontobject" ||
		mimeType == "application/octet-stream"
}

func pdfFontFaceBold(weight string) bool {
	weight = strings.ToLower(strings.TrimSpace(weight))
	switch weight {
	case "bold", "bolder":
		return true
	case "", "normal", "regular", "lighter":
		return false
	}
	value, err := strconv.Atoi(weight)
	return err == nil && value >= 600
}

func pdfFontFaceItalic(style string) bool {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "italic", "oblique":
		return true
	default:
		return false
	}
}
