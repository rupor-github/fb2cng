package kfx

import (
	"maps"
	"math"
	"strings"
)

// inferParentStyle attempts to determine a parent style based on naming patterns.
// This handles dynamically-created styles like "section-subtitle" -> inherits "subtitle".
//
// Block-level wrapper styles (epigraph, poem, stanza, cite, annotation, footnote, etc.)
// do NOT inherit from "p" to avoid polluting container styles with paragraph properties.
// Unknown styles have no parent - line-height is added in BuildFragments for text usage.
func (sr *StyleRegistry) inferParentStyle(name string) string {
	// Block-level container styles should NOT inherit from anything
	// These are wrappers that correspond to EPUB <div class="..."> elements
	if isBlockStyleName(name) {
		return ""
	}

	// Pattern 1: Paragraph variants inherit from their base style
	// "chapter-title-header-first" -> "chapter-title-header"
	// "body-title-header-next" -> "body-title-header"
	variantSuffixes := []string{"-first", "-next", "-break"}
	for _, suffix := range variantSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			baseName := name[:len(name)-len(suffix)] // Strip suffix to get base
			// Don't inherit from block containers
			if isBlockStyleName(baseName) {
				continue
			}
			if _, exists := sr.styles[baseName]; exists {
				return baseName
			}
		}
	}

	// Pattern 2: Suffix-named styles can inherit from a base style named after the suffix
	// "section-subtitle" -> "subtitle" (if subtitle style exists)
	// "custom-subtitle" -> "subtitle" (if subtitle style exists)
	// This provides a fallback inheritance for styles that follow the X-suffix naming pattern
	baseSuffixes := []string{"-subtitle"}
	for _, suffix := range baseSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			baseName := suffix[1:] // "subtitle" from "-subtitle"
			if _, exists := sr.styles[baseName]; exists {
				return baseName
			}
		}
	}

	// No parent - line-height will be added in BuildFragments for text styles
	return ""
}

// isBlockStyleName returns true if the style name represents a block-level container.
// Block containers wrap content and should not inherit paragraph text properties.
// This matches EPUB's <div class="..."> elements vs <p> or <span> elements.
func isBlockStyleName(name string) bool {
	// Exact matches for known block wrapper names from EPUB generation
	switch name {
	case "epigraph", "poem", "stanza", "cite", "annotation", "footnote",
		"section", "image", "vignette", "emptyline",
		"body-title", "chapter-title", "section-title",
		"footnote-body", "main-body", "other-body",
		"poem-title", "stanza-title", "footnote-title", "toc-title":
		return true
	}

	// KP3 wrapper variants for nested section titles (section-title-wrap-h2..h6)
	if strings.HasPrefix(name, "section-title-wrap-h") {
		return true
	}

	// Vignette-level selector anchors for nested section titles.
	// These are internal wrapper classes used only for descendant image rules.
	if strings.HasPrefix(name, "section-title-vig-h") {
		return true
	}

	// Vignette position variants (vignette-chapter-title-top, etc.)
	if strings.HasPrefix(name, "vignette-") {
		return true
	}

	return false
}

func stripZeroMargins(props map[KFXSymbol]any) map[KFXSymbol]any {
	if len(props) == 0 {
		return props
	}
	var trimmed map[KFXSymbol]any
	for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
		val, ok := props[sym]
		if !ok {
			continue
		}
		if isZeroMeasureValue(val) {
			if trimmed == nil {
				trimmed = make(map[KFXSymbol]any, len(props))
				maps.Copy(trimmed, props)
			}
			delete(trimmed, sym)
		}
	}
	if trimmed == nil {
		return props
	}
	return trimmed
}

func ensureDefaultLineHeight(props map[KFXSymbol]any) map[KFXSymbol]any {
	if _, ok := props[SymLineHeight]; ok {
		return props
	}
	updated := make(map[KFXSymbol]any, len(props)+1)
	maps.Copy(updated, props)
	updated[SymLineHeight] = DimensionValue(DefaultLineHeightLh, SymUnitLh)
	return updated
}

func stripLineHeight(props map[KFXSymbol]any) map[KFXSymbol]any {
	if len(props) == 0 {
		return props
	}
	if _, ok := props[SymLineHeight]; !ok {
		return props
	}
	updated := make(map[KFXSymbol]any, len(props))
	maps.Copy(updated, props)
	delete(updated, SymLineHeight)
	return updated
}

// adjustLineHeightForFontSize adjusts line-height and vertical margins when
// font-size differs from the default (1rem). KP3 uses different strategies based
// on font-size:
//
//   - For font-size < 1rem (e.g., sub/sup, code): line-height = 1lh (default).
//     Vertical margins are scaled by 1/font-size to preserve absolute spacing.
//     Example: 0.7rem font-size, margin 0.5lh → 0.5/0.7 = 0.714286lh
//
//   - For font-size >= 1rem (e.g., headings): line-height = 1.0101lh
//     Uses the standard adjustment factor (100/99 ≈ 1.0101).
//     Vertical margins are divided by the adjusted line-height.
//
// Note: If line-height is already set (e.g., calculated in ResolveInlineDelta
// for inline elements in non-standard contexts like headings), it is preserved.
// The ratio-based calculation in ResolveInlineDelta is more accurate for those cases.
func adjustLineHeightForFontSize(props map[KFXSymbol]any) map[KFXSymbol]any {
	// Check if font-size exists and differs from default (1rem)
	fontSize, ok := props[SymFontSize]
	if !ok {
		return props
	}

	fontSizeVal, fontSizeUnit, ok := measureParts(fontSize)
	if !ok {
		return props
	}

	// Only adjust if font-size is in rem and differs from 1.0
	if fontSizeUnit != SymUnitRem || math.Abs(fontSizeVal-1.0) < 1e-9 {
		return props
	}

	updated := make(map[KFXSymbol]any, len(props))
	maps.Copy(updated, props)

	if fontSizeVal < 1.0 {
		// Small font-size: KP3 sets line-height to 1lh explicitly.
		// This preserves the default line spacing for inline elements (sub/sup, code)
		// so they don't affect surrounding line rhythm.
		// If line-height was already set by ResolveInlineDelta, preserve it as-is.
		if _, hasLh := props[SymLineHeight]; !hasLh {
			updated[SymLineHeight] = DimensionValue(DefaultLineHeightLh, SymUnitLh)
		}

		// Scale vertical margins by 1/font-size to preserve absolute spacing.
		// Example: code at 0.7rem with margin 0.5lh → 0.5/0.7 = 0.714286lh
		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom} {
			if margin, ok := updated[sym]; ok {
				if marginVal, marginUnit, ok := measureParts(margin); ok && marginUnit == SymUnitLh {
					adjusted := RoundSignificant(marginVal/fontSizeVal, SignificantFigures)
					updated[sym] = DimensionValue(adjusted, SymUnitLh)
				}
			}
		}
	} else {
		// Large font-size: line-height = 1.0101lh, margins divided by line-height.
		// Only set if not already present (ResolveInlineDelta may have set it).
		adjustedLh := AdjustedLineHeightLh
		if existingLh, hasLh := props[SymLineHeight]; hasLh {
			if lhVal, lhUnit, ok := measureParts(existingLh); ok && lhUnit == SymUnitLh {
				adjustedLh = lhVal
			} else {
				updated[SymLineHeight] = DimensionValue(RoundDecimals(adjustedLh, LineHeightPrecision), SymUnitLh)
			}
		} else {
			updated[SymLineHeight] = DimensionValue(RoundDecimals(adjustedLh, LineHeightPrecision), SymUnitLh)
		}

		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom} {
			if margin, ok := updated[sym]; ok {
				if marginVal, marginUnit, ok := measureParts(margin); ok && marginUnit == SymUnitLh {
					adjusted := RoundSignificant(marginVal/adjustedLh, SignificantFigures)
					updated[sym] = DimensionValue(adjusted, SymUnitLh)
				}
			}
		}
	}

	return updated
}

func containsSymbolAny(list []any, expected KFXSymbol) bool {
	for _, v := range list {
		if sym, ok := symbolIDFromAny(v); ok && sym == expected {
			return true
		}
	}
	return false
}

// isKP3TableStyle returns true for the special table wrapper style that KP3 emits.
//
// In KP3 output, the table style has sizing-bounds: content_bounds and width: 32em,
// but does NOT include break-inside even if the source CSS had page-break-inside: avoid.
func isKP3TableStyle(props map[KFXSymbol]any) bool {
	if props == nil {
		return false
	}
	if !isSymbol(props[SymSizingBounds], SymContentBounds) {
		return false
	}
	// width: 32em
	v, ok := props[SymWidth]
	if !ok {
		return false
	}
	widthVal, widthUnit, ok := measureParts(v)
	return ok && widthUnit == SymUnitEm && widthVal == 32
}

func isSectionTitleHeaderTextStyle(props map[KFXSymbol]any) bool {
	if props == nil {
		return false
	}
	// Needs to be title-like.
	hints, ok := props[SymLayoutHints].([]any)
	if !ok || !containsSymbolAny(hints, SymTreatAsTitle) {
		return false
	}
	// Note: We intentionally allow break-inside: avoid here. Our generator sometimes
	// emits treat_as_title on styles that also carry break-inside (KP3 does not, but
	// we still want the correct line-height).
	// In reference output, nested section title headers use this font size.
	fs, ok := props[SymFontSize]
	if !ok {
		return false
	}
	fsVal, fsUnit, ok := measureParts(fs)
	if !ok || fsUnit != SymUnitRem || math.Abs(fsVal-1.125) >= 1e-9 {
		return false
	}
	// And they are centered/bold.
	if !isSymbol(props[SymTextAlignment], SymCenter) {
		return false
	}
	if !isSymbol(props[SymFontWeight], SymBold) {
		return false
	}
	return true
}

// normalizeFontSizeUnits converts font-size from em to rem for final KFX output.
// During style merging, em units enable relative multiplication (e.g., 0.75rem * 0.8em = 0.6rem).
// KFX output requires rem units, so we convert any remaining em values here.
// An em value at this point means it wasn't merged with a rem value, so we treat 1em = 1rem.
func normalizeFontSizeUnits(props map[KFXSymbol]any) map[KFXSymbol]any {
	fontSize, ok := props[SymFontSize]
	if !ok {
		return props
	}

	fontSizeVal, fontSizeUnit, ok := measureParts(fontSize)
	if !ok || fontSizeUnit != SymUnitEm {
		return props
	}

	// Convert em to rem (1em = 1rem at the base level)
	updated := make(map[KFXSymbol]any, len(props))
	maps.Copy(updated, props)
	updated[SymFontSize] = DimensionValue(fontSizeVal, SymUnitRem)
	return updated
}

func isZeroMeasureValue(val any) bool {
	v, _, ok := measureParts(val)
	if !ok {
		return false
	}
	return math.Abs(v) < 1e-9
}
