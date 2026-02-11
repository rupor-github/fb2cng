package kfx

import (
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

// FontInfo contains metadata about embedded fonts for use in KFX generation.
// It tracks the body font family name and all font resources needed.
type FontInfo struct {
	// BodyFontFamily is the font-family name used in the body rule (e.g., "paragraph").
	// This becomes the document_data $11 (font_family) value with "nav-" prefix.
	// Empty string means no body font was specified.
	BodyFontFamily string

	// AllFonts contains all font resources indexed by their CSS font-family name.
	// Each family can have multiple resources for different weight/style combinations.
	// This includes both body fonts and non-body fonts (like dropcap).
	AllFonts map[string][]fontResource
}

// HasBodyFont returns true if a body font family is specified.
func (fi *FontInfo) HasBodyFont() bool {
	return fi != nil && fi.BodyFontFamily != ""
}

// HasFonts returns true if there are any embedded fonts.
func (fi *FontInfo) HasFonts() bool {
	return fi != nil && len(fi.AllFonts) > 0
}

// GetKFXFontFamily returns the KFX font family name used in document_data.
// Returns empty string if no body font is configured.
func (fi *FontInfo) GetKFXFontFamily() string {
	if fi == nil || !fi.HasBodyFont() {
		return ""
	}
	return ToKFXFontFamily(fi.BodyFontFamily)
}

// ToKFXFontFamily converts a CSS font-family name to KFX format.
// This adds the "nav-" prefix that KP3 uses.
// e.g., "paragraph" -> "nav-paragraph", "dropcaps" -> "nav-dropcaps"
func ToKFXFontFamily(cssFamily string) string {
	if cssFamily == "" {
		return ""
	}
	// Strip quotes if present
	family := strings.Trim(cssFamily, `"'`)
	if family == "" {
		return ""
	}
	// Add nav- prefix
	return "nav-" + family
}

// ToKFXFontFamilyFromCSS converts a CSS font-family value to the string KP3 emits
// in KFX styles.
//
// KP3 prefixes embedded font family names with "nav-" (see ToKFXFontFamily), but
// keeps CSS generic families (e.g. monospace/serif) unprefixed.
//
// It also accepts a font stack and uses the first family.
func ToKFXFontFamilyFromCSS(cssFontFamily string) string {
	cssFontFamily = strings.TrimSpace(cssFontFamily)
	if cssFontFamily == "" {
		return ""
	}

	// Use first family from stack: "MyFont", serif -> "MyFont"
	if idx := strings.Index(cssFontFamily, ","); idx >= 0 {
		cssFontFamily = cssFontFamily[:idx]
		cssFontFamily = strings.TrimSpace(cssFontFamily)
	}

	// Unquote if present
	family := strings.Trim(cssFontFamily, `"'`)
	family = strings.TrimSpace(family)
	if family == "" {
		return ""
	}

	lower := strings.ToLower(family)
	if lower == "default" {
		return "default"
	}
	if strings.HasPrefix(lower, "nav-") {
		// Avoid double-prefixing if author CSS already uses nav-*.
		return family
	}

	// CSS generic family keywords must remain unprefixed to match KP3 output.
	switch lower {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui",
		"ui-serif", "ui-sans-serif", "ui-monospace", "ui-rounded",
		"emoji", "math", "fangsong":
		return lower
	}

	return ToKFXFontFamily(family)
}

// fontResource represents a single font file resource.
type fontResource struct {
	Data     []byte
	MimeType string
	Style    KFXSymbol // SymNormal or SymItalic
	Weight   KFXSymbol // SymNormal, SymBold, SymSemibold, etc.
	OrigURL  string    // Original URL from CSS for deduplication
}

// BuildFontInfo extracts embedded font information from stylesheets.
// It parses @font-face declarations and the body font-family rule.
// All fonts are collected into AllFonts, and the body font family is identified.
func BuildFontInfo(stylesheets []fb2.Stylesheet, parsedCSS *css.Stylesheet, log *zap.Logger) *FontInfo {
	info := &FontInfo{
		AllFonts: make(map[string][]fontResource),
	}

	// No parsed CSS means no @font-face declarations
	if parsedCSS == nil {
		return info
	}

	// Build a map from original URL to resource data (from fb2 stylesheet resources)
	urlToResource := make(map[string]*fb2.StylesheetResource)
	for i := range stylesheets {
		for j := range stylesheets[i].Resources {
			res := &stylesheets[i].Resources[j]
			urlToResource[res.OriginalURL] = res
		}
	}

	// Process @font-face declarations from parsed CSS - collect ALL fonts
	for _, ff := range parsedCSS.FontFaces {
		if ff.Family == "" || ff.Src == "" {
			continue
		}

		// Extract URL from src (e.g., 'url("path/to/font.ttf")' or 'url(path/to/font.ttf)')
		url := extractURLFromSrc(ff.Src)
		if url == "" {
			continue
		}

		// Find the resource data
		res := urlToResource[url]
		if res == nil || len(res.Data) == 0 {
			continue
		}

		// Only include font MIME types
		if !isFontMIMEType(res.MimeType) {
			continue
		}

		// Normalize weight and style to KFX symbols
		weight := normalizeWeightToSymbol(ff.Weight)
		style := normalizeStyleToSymbol(ff.Style)

		// Store under lowercase family name for case-insensitive lookup
		familyKey := strings.ToLower(strings.Trim(ff.Family, `"'`))
		info.AllFonts[familyKey] = append(info.AllFonts[familyKey], fontResource{
			Data:     res.Data,
			MimeType: res.MimeType,
			Style:    style,
			Weight:   weight,
			OrigURL:  url,
		})
	}

	// Find body font-family from CSS rules
	rules := flattenStylesheetForKFX(parsedCSS)
	for _, rule := range rules {
		if rule.Selector.Element == "body" && rule.Selector.Class == "" {
			if val, ok := rule.Properties["font-family"]; ok {
				family := extractFontFamilyName(val)
				if family != "" {
					// Only use as body font if we have resources for it
					familyKey := strings.ToLower(family)
					if resources, hasResources := info.AllFonts[familyKey]; hasResources {
						info.BodyFontFamily = family
						if log != nil {
							log.Debug("Detected body font rule",
								zap.String("selector", "body"),
								zap.String("font-family", family),
								zap.Int("variants", len(resources)))
						}
					}
				}
			}
			break
		}
	}

	return info
}

// extractURLFromSrc extracts the URL from a CSS src property value.
// Handles: url("path"), url('path'), url(path)
func extractURLFromSrc(src string) string {
	// Find url( and extract content
	start := strings.Index(strings.ToLower(src), "url(")
	if start < 0 {
		return ""
	}
	start += 4 // skip "url("

	// Find the closing )
	end := strings.Index(src[start:], ")")
	if end < 0 {
		return ""
	}
	end += start

	url := strings.TrimSpace(src[start:end])

	// Remove quotes if present
	url = strings.Trim(url, `"'`)

	return url
}

// extractFontFamilyName extracts the font family name from a CSS value.
// Handles quoted and unquoted names, returns first font in stack.
func extractFontFamilyName(val css.Value) string {
	if val.Keyword != "" {
		// Remove quotes and get first family from comma-separated list
		family := val.Keyword
		if idx := strings.Index(family, ","); idx > 0 {
			family = family[:idx]
		}
		family = strings.Trim(strings.TrimSpace(family), `"'`)
		return family
	}
	return ""
}

// normalizeWeightToSymbol converts CSS font-weight string to KFX symbol.
// This handles @font-face descriptor values like "bold", "700", etc.
func normalizeWeightToSymbol(weight string) KFXSymbol {
	// Use ConvertFontWeight with a css.Value
	val := css.Value{Keyword: strings.TrimSpace(weight)}

	// Try as keyword first
	if sym, ok := ConvertFontWeight(val); ok {
		return sym
	}

	// Try parsing as number
	weight = strings.TrimSpace(weight)
	if w, err := strconv.Atoi(weight); err == nil {
		val = css.Value{Value: float64(w)}
		if sym, ok := ConvertFontWeight(val); ok {
			return sym
		}
	}

	return SymNormal
}

// normalizeStyleToSymbol converts CSS font-style string to KFX symbol.
// This handles @font-face descriptor values like "italic", "oblique", etc.
func normalizeStyleToSymbol(style string) KFXSymbol {
	val := css.Value{Keyword: strings.TrimSpace(style)}
	if sym, ok := ConvertFontStyle(val); ok {
		return sym
	}
	return SymNormal
}

// isFontMIMEType returns true if the MIME type indicates a font.
func isFontMIMEType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "font/") ||
		strings.HasPrefix(mimeType, "application/font-") ||
		strings.HasPrefix(mimeType, "application/x-font-") ||
		mimeType == "application/vnd.ms-fontobject"
}

// BuildFontFragments creates the font ($262) and bcRawFont ($418) fragments.
// Returns nil slices if no fonts are available.
//
// KFX font structure (per KP3 reference):
//   - font ($262): ONE fragment PER font variant (not just one total).
//     Each bcRawFont resource needs a corresponding font($262) declaration.
//     Structure: {$11: "nav-family", $12: symbol(style), $13: symbol(weight), $15: symbol($350), $165: "resource/rsrcXXX"}
//   - bcRawFont ($418): Raw font file data blobs for ALL font variants (body + dropcap, etc.)
//
// KP3 uses "nav-{family}" naming convention for the KFX font family name.
func BuildFontFragments(fontInfo *FontInfo, startIndex int) ([]*Fragment, []*Fragment) {
	if fontInfo == nil || !fontInfo.HasFonts() {
		return nil, nil
	}

	var fontFrags []*Fragment
	var rawFrags []*Fragment

	// Track unique resources (by URL) to avoid duplicates
	seen := make(map[string]bool)
	idx := startIndex

	// Create bcRawFont ($418) and font ($262) for ALL font resources
	// Each font variant needs both a raw data blob and a font declaration
	for family, resources := range fontInfo.AllFonts {
		kfxFontFamily := ToKFXFontFamily(family)

		for _, res := range resources {
			if seen[res.OrigURL] {
				continue
			}
			seen[res.OrigURL] = true

			location := makeResourceLocation(idx)

			// Create bcRawFont fragment ($418) - raw font data
			rawFrag := &Fragment{
				FType:   SymRawFont,
				FIDName: location,
				Value:   RawValue(res.Data),
			}
			rawFrags = append(rawFrags, rawFrag)

			// Create font fragment ($262) - font declaration that references the raw data
			// This is required for the bcRawFont to be considered "referenced" by the validator
			fontFrag := buildFontFragment(kfxFontFamily, res.Style, res.Weight, location)
			fontFrags = append(fontFrags, fontFrag)

			idx++
		}
	}

	if len(rawFrags) == 0 {
		return nil, nil
	}

	return fontFrags, rawFrags
}

// buildFontFragment creates a single font ($262) fragment.
// Structure: {$11: "font-family", $12: symbol(style), $13: symbol(weight), $15: symbol($350), $165: "location"}
func buildFontFragment(family string, style, weight KFXSymbol, location string) *Fragment {
	fontData := NewStruct()
	fontData.Set(SymFontFamily, family)
	fontData.Set(SymFontStyle, SymbolValue(style))
	fontData.Set(SymFontWeight, SymbolValue(weight))
	fontData.Set(SymFontStretch, SymbolValue(SymNormal)) // Always normal for now
	fontData.Set(SymLocation, location)

	return NewRootFragment(SymFont, fontData)
}
