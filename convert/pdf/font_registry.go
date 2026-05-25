package pdf

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

type pdfFontRegistry struct {
	families                map[string]pdfEmbeddedFontFamily
	log                     *zap.Logger
	missingGlyphLogSeen     map[pdfMissingGlyphLogKey]bool
	missingGlyphLogMu       sync.Mutex
	fontFallbackLogSeen     map[pdfFontFallbackLogKey]bool
	fontFallbackLogSeenLock sync.Mutex
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
	registry := &pdfFontRegistry{
		families:            make(map[string]pdfEmbeddedFontFamily),
		log:                 log.Named("pdf-fonts"),
		missingGlyphLogSeen: make(map[pdfMissingGlyphLogKey]bool),
		fontFallbackLogSeen: make(map[pdfFontFallbackLogKey]bool),
	}
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
			r.log.Warn("Skipping unsupported PDF @font-face resource",
				zap.String("family", family),
				zap.String("url", url),
				zap.String("reason", "font_parse_failed"),
				zap.Error(err))
			continue
		}
		program, err := pdfFontProgram(face.Data)
		if err != nil {
			r.log.Warn("Skipping unsupported PDF @font-face resource",
				zap.String("family", family),
				zap.String("url", url),
				zap.String("font", face.PostScriptName),
				zap.String("reason", "unsupported_outline_tables"),
				zap.Error(err))
			continue
		}
		r.logPDFStylesheetFontSupport(family, url, variant, face, program)
		r.logPDFFontEmbeddingRestrictions(family, url, face)
		embeddedFamily.faces[variant] = face
		r.families[familyKey] = embeddedFamily
	}
}

func (r *pdfFontRegistry) logPDFStylesheetFontSupport(family string, url string, variant pdfFontVariant, face *builtinFontFace, program pdfFontProgramInfo) {
	if r == nil || r.log == nil || face == nil {
		return
	}
	subsettingStatus := pdfFontSubsettingStatus(face, program)
	fields := []zap.Field{
		zap.String("family", family),
		zap.String("url", url),
		zap.Bool("bold", variant.Bold),
		zap.Bool("italic", variant.Italic),
		zap.String("font", face.PostScriptName),
		zap.String("outline_kind", program.OutlineKind),
		zap.String("pdf_cid_font_subtype", string(program.CIDFontSubtype)),
		zap.String("pdf_embedded_font_file", program.FontFileKey),
		zap.Bool("opentype_shaping_supported", face.TextFace != nil),
		zap.Bool("pdf_embedding_supported", true),
		zap.Bool("pdf_subsetting_supported", subsettingStatus == "compact_truetype"),
		zap.String("pdf_subsetting_status", subsettingStatus),
	}
	limitations := pdfFontSupportLimitations(face, program)
	if len(limitations) != 0 {
		fields = append(fields, zap.Strings("limitations", limitations))
		r.log.Warn("Loaded PDF @font-face resource with limitations", fields...)
		return
	}
	r.log.Info("Loaded PDF @font-face resource", fields...)
}

func pdfFontSubsettingStatus(face *builtinFontFace, program pdfFontProgramInfo) string {
	switch {
	case face == nil:
		return "unknown"
	case program.TrueTypeOutlines && allowPDFTrueTypeSubsetting(face):
		return "compact_truetype"
	case program.TrueTypeOutlines:
		return "disabled_by_font_fs_type"
	case program.OutlineKind == "opentype_cff", program.OutlineKind == "opentype_cff2":
		return "not_supported_for_cff"
	default:
		return "not_supported"
	}
}

func pdfFontSupportLimitations(face *builtinFontFace, program pdfFontProgramInfo) []string {
	limitations := make([]string, 0, 3)
	if face != nil && !allowPDFTrueTypeSubsetting(face) && program.TrueTypeOutlines {
		limitations = append(limitations, "font_disallows_subsetting_full_font_will_be_embedded")
	}
	if program.OutlineKind == "opentype_cff" {
		limitations = append(limitations, "cff_subsetting_not_implemented_full_font_will_be_embedded")
	}
	if program.OutlineKind == "opentype_cff2" {
		limitations = append(limitations,
			"cff2_subsetting_not_implemented_full_font_will_be_embedded",
			"cff2_pdf_viewer_compatibility_not_fully_validated")
	}
	return limitations
}

func (r *pdfFontRegistry) logPDFFontEmbeddingRestrictions(family string, url string, face *builtinFontFace) {
	if r == nil || r.log == nil || face == nil || face.EmbeddingFSType == 0 {
		return
	}
	r.log.Warn("PDF font has embedding restrictions",
		zap.String("family", family),
		zap.String("url", url),
		zap.String("font", face.PostScriptName),
		zap.String("fs_type", fmt.Sprintf("0x%04X", face.EmbeddingFSType)),
		zap.Bool("restricted_license_embedding", face.EmbeddingFSType&0x0002 != 0),
		zap.Bool("preview_and_print_embedding", face.EmbeddingFSType&0x0004 != 0),
		zap.Bool("editable_embedding", face.EmbeddingFSType&0x0008 != 0),
		zap.Bool("no_subsetting", face.EmbeddingFSType&0x0100 != 0),
		zap.Bool("bitmap_embedding_only", face.EmbeddingFSType&0x0200 != 0))
}

func (r *pdfFontRegistry) fontForKey(key pdfFontKey) (*builtinFontFace, error) {
	if r != nil {
		if pdfFontKeyIsBuiltinSymbolFallback(key) {
			face, err := builtinFont(key.Family, key.Bold, key.Italic)
			return r.fontFaceWithLogger(face, key), err
		}
		if face, selected, ok := r.embeddedFont(key); ok {
			r.logPDFEmbeddedFontVariantFallback(key, selected, face)
			return r.fontFaceWithLogger(face, key), nil
		}
		face, err := builtinFont(key.Family, key.Bold, key.Italic)
		if err == nil {
			r.logPDFBuiltinFontFamilyFallback(key, face)
		}
		return r.fontFaceWithLogger(face, key), err
	}
	return fontForKey(nil, key)
}

type pdfFontFallbackLogKey struct {
	Kind            string
	Family          string
	RequestedBold   bool
	RequestedItalic bool
	SelectedBold    bool
	SelectedItalic  bool
}

func (r *pdfFontRegistry) fontFaceWithLogger(face *builtinFontFace, key pdfFontKey) *builtinFontFace {
	if r == nil {
		return face
	}
	return pdfFontFaceWithLogger(face, r.log, key, r.missingGlyphLogSeen, &r.missingGlyphLogMu)
}

func (r *pdfFontRegistry) logPDFEmbeddedFontVariantFallback(key pdfFontKey, selected pdfFontVariant, face *builtinFontFace) {
	requested := pdfFontVariant{Bold: key.Bold, Italic: key.Italic}
	if r == nil || r.log == nil || requested == selected {
		return
	}
	logKey := pdfFontFallbackLogKey{
		Kind:            "variant",
		Family:          normalizedPDFFontFamily(key.Family),
		RequestedBold:   key.Bold,
		RequestedItalic: key.Italic,
		SelectedBold:    selected.Bold,
		SelectedItalic:  selected.Italic,
	}
	if r.pdfFontFallbackAlreadyLogged(logKey) {
		return
	}
	r.log.Warn("Using fallback PDF font face for missing variant",
		zap.String("font_family", logKey.Family),
		zap.Bool("requested_bold", key.Bold),
		zap.Bool("requested_italic", key.Italic),
		zap.Bool("selected_bold", selected.Bold),
		zap.Bool("selected_italic", selected.Italic),
		zap.String("font", face.PostScriptName))
}

func (r *pdfFontRegistry) logPDFBuiltinFontFamilyFallback(key pdfFontKey, face *builtinFontFace) {
	if r == nil || r.log == nil || pdfFontFamilyIsBuiltinAlias(key.Family) {
		return
	}
	logKey := pdfFontFallbackLogKey{
		Kind:            "family",
		Family:          normalizedPDFFontFamily(key.Family),
		RequestedBold:   key.Bold,
		RequestedItalic: key.Italic,
	}
	if r.pdfFontFallbackAlreadyLogged(logKey) {
		return
	}
	r.log.Warn("Using fallback PDF font family for missing family",
		zap.String("font_family", logKey.Family),
		zap.Bool("requested_bold", key.Bold),
		zap.Bool("requested_italic", key.Italic),
		zap.String("font", face.PostScriptName))
}

func (r *pdfFontRegistry) pdfFontFallbackAlreadyLogged(key pdfFontFallbackLogKey) bool {
	if r == nil || r.fontFallbackLogSeen == nil {
		return false
	}
	r.fontFallbackLogSeenLock.Lock()
	defer r.fontFallbackLogSeenLock.Unlock()
	if r.fontFallbackLogSeen[key] {
		return true
	}
	r.fontFallbackLogSeen[key] = true
	return false
}

func pdfFontFamilyIsBuiltinAlias(family string) bool {
	name := strings.ToLower(strings.TrimSpace(family))
	switch {
	case name == "", name == "serif", name == "sans-serif", name == "sans", name == "monospace", name == "mono", name == "courier":
		return true
	case name == strings.ToLower(pdfBuiltinFontFamilyMath), name == strings.ToLower(pdfBuiltinFontFamilySymbols), name == strings.ToLower(pdfBuiltinFontFamilySymbols2):
		return true
	case strings.Contains(name, "noto serif"), strings.Contains(name, "noto sans"), strings.Contains(name, "noto sans mono"):
		return true
	default:
		return false
	}
}

func (r *pdfFontRegistry) embeddedFont(key pdfFontKey) (*builtinFontFace, pdfFontVariant, bool) {
	if r == nil || len(r.families) == 0 {
		return nil, pdfFontVariant{}, false
	}
	family := r.families[strings.ToLower(strings.TrimSpace(key.Family))]
	if len(family.faces) == 0 {
		return nil, pdfFontVariant{}, false
	}
	variant := pdfFontVariant{Bold: key.Bold, Italic: key.Italic}
	if face := family.faces[variant]; face != nil {
		return face, variant, true
	}
	fallbacks := []pdfFontVariant{
		{Bold: key.Bold},
		{Italic: key.Italic},
		{},
	}
	for _, fallback := range fallbacks {
		if face := family.faces[fallback]; face != nil {
			return face, fallback, true
		}
	}
	return nil, pdfFontVariant{}, false
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
