package kfx

import (
	"fmt"
	"maps"
	"strings"
)

// RegisterResolved takes a merged property map, generates a unique style name,
// registers the style, and returns the name. This is used by StyleContext.Resolve
// to register styles built with proper CSS inheritance rules.
//
// The usage parameter specifies what kind of content uses this style:
//   - styleUsageText: text paragraphs - ensureDefaultLineHeight adds line-height: 1lh in BuildFragments
//   - styleUsageInline: inline style events - no line-height added (inherit from parent block)
//   - styleUsageImage: image styles - line-height handled separately
//   - styleUsageWrapper: wrapper styles - line-height stripped unless break-inside: avoid
//
// The markUsed parameter controls whether the style is marked as used for output.
// Pass markUsed=false when styles may be deduplicated later (e.g., style events).
//
// Standard KFX output filtering is applied:
//   - Removes height: auto (KP3 never outputs this, it's the implied default)
//   - Removes table element properties (these go on the element, not the style)
func (sr *StyleRegistry) RegisterResolved(props map[KFXSymbol]any, usage styleUsage, markUsed bool) string {
	// Make a copy to avoid modifying caller's map
	merged := make(map[KFXSymbol]any, len(props))
	maps.Copy(merged, props)
	if markUsed {
		return sr.registerFilteredStyle(merged, usage)
	}
	return sr.registerFilteredStyleNoMark(merged, usage)
}

// registerFilteredStyle applies standard KFX output filtering, registers the style, and marks it used.
// The usage parameter is recorded so BuildFragments() can apply appropriate post-processing
// (e.g., ensureDefaultLineHeight for text styles).
func (sr *StyleRegistry) registerFilteredStyle(merged map[KFXSymbol]any, usage styleUsage) string {
	return sr.doRegisterFilteredStyle(merged, true, usage)
}

// registerFilteredStyleNoMark applies standard KFX output filtering and registers the style but does NOT mark used.
func (sr *StyleRegistry) registerFilteredStyleNoMark(merged map[KFXSymbol]any, usage styleUsage) string {
	return sr.doRegisterFilteredStyle(merged, false, usage)
}

// doRegisterFilteredStyle is the common implementation for filtered style registration.
// The usage parameter tracks what kind of content uses this style (text, image, wrapper).
func (sr *StyleRegistry) doRegisterFilteredStyle(merged map[KFXSymbol]any, markUsed bool, usage styleUsage) string {
	// KP3 pattern: styles marked as treat_as_title typically do not carry margin-bottom.
	// Spacing is placed on the surrounding wrapper container, not on the title text itself.
	//
	// Important: apply this to resolved (generated) styles too, since title text styles
	// often end up as resolved "s.." names.
	if hints, ok := merged[SymLayoutHints].([]any); ok && containsSymbolAny(hints, SymTreatAsTitle) {
		// KP3 does not put keep-together semantics (break-inside: avoid) on the title text style.
		// It belongs on the wrapper container style.
		delete(merged, SymBreakInside)

		// KP3 also does not carry yj-break-* properties on title text styles.
		// Page-break semantics are represented by wrapper styles and/or section boundaries.
		delete(merged, SymYjBreakBefore)
		delete(merged, SymYjBreakAfter)

		// Exception: footnote-title is used directly on paragraphs (no wrapper), so it needs MB.
		// We detect it by its left alignment (other title headers are centered) and bold.
		isFootnoteTitle := isSymbol(merged[SymTextAlignment], SymLeft) && isSymbol(merged[SymFontWeight], SymBold)
		if !isFootnoteTitle {
			delete(merged, SymMarginBottom)
		}
	}

	// Filter out height: auto - KP3 never outputs this in styles (it's the implied default)
	if h, ok := merged[SymHeight]; ok {
		isAuto := false
		switch v := h.(type) {
		case SymbolValue:
			isAuto = KFXSymbol(v) == SymAuto
		case KFXSymbol:
			isAuto = v == SymAuto
		}
		if isAuto {
			delete(merged, SymHeight)
		}
	}

	// Filter table element properties - KP3 moves these from style to element
	for prop := range tableElementProperties {
		delete(merged, prop)
	}

	// KP3 does not emit break-inside for table styles (even if the source CSS had
	// page-break-inside: avoid on the table rule).
	// Keep-together behavior for titles is handled via wrapper styles.
	if isKP3TableStyle(merged) {
		delete(merged, SymBreakInside)
	}

	sig := styleSignature(merged)
	// Inline-only styles (style events) use a separate signature namespace.
	// This prevents them from being deduplicated with block styles that have
	// the same properties. BuildFragments applies different post-processing:
	// - Block (styleUsageText): ensureDefaultLineHeight adds line-height: 1lh
	// - Inline (styleUsageInline): NO line-height added (inherit from parent)
	// If we reused the same style for both, line-height would "leak" into inline usage.
	if usage == styleUsageInline {
		sig = "inline:" + sig
	}
	if name, ok := sr.resolved[sig]; ok {
		if markUsed {
			sr.used[name] = true
		}
		// Record usage type (OR with existing to accumulate multiple usages)
		if usage != 0 {
			sr.usage[name] = sr.usage[name] | usage
		}
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	if markUsed {
		sr.used[name] = true
	}
	// Record usage type for new style
	if usage != 0 {
		sr.usage[name] = usage
	}
	sr.Register(StyleDef{Name: name, Properties: merged})
	return name
}

// inferParentStyle attempts to determine a parent style based on naming patterns.
// This handles dynamically-created styles like "section-subtitle" -> inherits "subtitle".
//
// Block-level wrapper styles (epigraph, poem, stanza, cite, annotation, footnote, etc.)
// do NOT inherit from "p" to avoid polluting container styles with paragraph properties.
// Unknown styles have no parent - line-height is added in BuildFragments for text usage.
// BuildFragments creates style fragments for all used styles.
// Only styles that were marked via EnsureStyle/ResolveStyle are included.
// Inheritance is resolved (flattened styles).
func (sr *StyleRegistry) BuildFragments() []*Fragment {
	fragments := make([]*Fragment, 0, len(sr.used))
	for _, name := range sr.order {
		if !sr.used[name] {
			continue
		}
		def := sr.styles[name]
		resolved := sr.resolveInheritance(def)
		resolved.Properties = stripZeroMargins(resolved.Properties)
		// Convert any remaining em font-sizes to rem before other adjustments
		resolved.Properties = normalizeFontSizeUnits(resolved.Properties)
		// Replace body font family with "default" (as KP3 does)
		resolved.Properties = sr.normalizeFontFamily(resolved.Properties)
		if sr.hasInlineUsage(name) && !sr.hasTextUsage(name) {
			// Inline-only styles (style events) may need line-height adjustment
			// for sub/sup with different font-size, but should NOT get default
			// line-height added - they inherit from parent.
			// Check this FIRST: if a style is used for both inline AND text,
			// it needs line-height (the text usage takes precedence).
			resolved.Properties = adjustLineHeightForFontSize(resolved.Properties)
		} else if sr.hasTextUsage(name) {
			// Adjust line-height and margins for non-default font-sizes
			// Must be done before ensureDefaultLineHeight to set correct line-height value
			resolved.Properties = adjustLineHeightForFontSize(resolved.Properties)
			resolved.Properties = ensureDefaultLineHeight(resolved.Properties)
			if isSectionTitleHeaderTextStyle(resolved.Properties) {
				resolved.Properties[SymLineHeight] = DimensionValue(RoundDecimals(SectionTitleHeaderLineHeightLh, LineHeightPrecision), SymUnitLh)
			}
		} else if sr.hasImageUsage(name) {
			// KP3 includes line-height: 1lh for standalone block images.
			// Don't strip it, but also don't force default (already set in ResolveBlockImageStyle).
		} else {
			// KP3 wrapper styles with break-inside: avoid retain line-height: 1lh.
			// Only strip line-height from other non-text styles.
			if _, hasBreakInside := resolved.Properties[SymBreakInside]; !hasBreakInside {
				resolved.Properties = stripLineHeight(resolved.Properties)
			}
		}
		fragments = append(fragments, BuildStyle(resolved))
	}
	return fragments
}

// normalizeFontFamily handles font-family for styles when a body font is set.
// KP3 uses "default" in styles to reference the document's default font.
// - If font-family matches body font → replace with "default"
// - If font-family is different (e.g., dropcaps) → keep as-is
// - If no font-family set → add "default" to inherit body font
// If no body font family is set, returns properties unchanged.
func (sr *StyleRegistry) normalizeFontFamily(props map[KFXSymbol]any) map[KFXSymbol]any {
	if sr.bodyFontFamily == "" {
		return props
	}

	bodyFamilyKFX := ToKFXFontFamily(sr.bodyFontFamily)

	if fontFamily, ok := props[SymFontFamily]; ok {
		if familyStr, isString := fontFamily.(string); isString {
			// Check if this is the body font family (nav-prefixed version)
			if familyStr == bodyFamilyKFX {
				props[SymFontFamily] = "default"
			}
			// else: keep non-body font as-is (e.g., nav-dropcaps)
		}
	} else {
		// No font-family set - add "default" to inherit body font
		props[SymFontFamily] = "default"
	}
	return props
}

// DefaultStyleRegistry returns a registry with default HTML element styles for KFX.
// This only includes HTML element selectors (p, h1-h6, code, blockquote, etc.)
// and basic inline styles (strong, em, sub, sup). Class selectors come from CSS.
//
// KFX-specific properties like layout-hints are applied during post-processing,
// not here, to allow CSS to override base styles first.
//
// NOTE: When adding vertical spacing properties (margin-top, margin-bottom,
// padding-top, padding-bottom), use lh units, NOT em units. CSS-parsed styles
// go through unit conversion (em → lh via LineHeightRatio), but styles registered
// here bypass that conversion. Using em units here would result in incorrect
// values compared to CSS-parsed equivalents.
// Example: 1em in CSS → 0.833lh in KFX (1.0 / LineHeightRatio)
func DefaultStyleRegistry() *StyleRegistry {
	sr := NewStyleRegistry()

	// ============================================================
	// Block-level HTML elements
	// ============================================================

	// Empty-line spacer used between block images.
	// KP3 emits this as container($270) with layout: vertical and margin-top set.
	// The actual margin-top is overridden per instance (see AddEmptyLineSpacer).
	sr.Register(NewStyle("emptyline-spacer").
		LineHeight(1, SymUnitLh).
		Build())

	// Base paragraph style - HTML <p> element
	// Amazon reference (stylemap.ion): margin-top: 1em, margin-bottom: 1em
	// Convert to lh units: 1em / 1.2 = 0.833lh
	// FB2-specific formatting (text-indent, justify) comes from CSS
	sr.Register(NewStyle("p").
		MarginTop(0.833333, SymUnitLh).
		MarginBottom(0.833333, SymUnitLh).
		Build())

	// Heading styles (h1-h6) - HTML heading elements
	// Amazon reference (stylemap.ion): font-size, font-weight, margin-top, margin-bottom
	// Font sizes use rem (not em) for KFX output
	// Margins converted from em to lh: em_value / LineHeightRatio (1.2)
	// layout-hints added during post-processing
	// Headings include explicit line-height so that inline contexts (for sub/sup
	// style delta calculations) can inherit it. The value is 1.0101lh (AdjustedLineHeightLh)
	// which is the standard KFX line-height. CSS may override font-size but not line-height,
	// so this base value will be available for inline delta resolution.
	sr.Register(NewStyle("h1").
		FontSize(2.0, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(0.558, SymUnitLh). // 0.67em / 1.2
		MarginBottom(0.558, SymUnitLh).
		Build())

	sr.Register(NewStyle("h2").
		FontSize(1.5, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(0.692, SymUnitLh). // 0.83em / 1.2
		MarginBottom(0.692, SymUnitLh).
		Build())

	sr.Register(NewStyle("h3").
		FontSize(1.17, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(0.833333, SymUnitLh). // 1.0em / 1.2
		MarginBottom(0.833333, SymUnitLh).
		Build())

	sr.Register(NewStyle("h4").
		FontSize(1.0, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(1.108, SymUnitLh). // 1.33em / 1.2
		MarginBottom(1.108, SymUnitLh).
		Build())

	sr.Register(NewStyle("h5").
		FontSize(0.83, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(1.392, SymUnitLh). // 1.67em / 1.2
		MarginBottom(1.392, SymUnitLh).
		Build())

	sr.Register(NewStyle("h6").
		FontSize(0.67, SymUnitRem).
		FontWeight(SymBold).
		LineHeight(AdjustedLineHeightLh, SymUnitLh).
		MarginTop(1.942, SymUnitLh). // 2.33em / 1.2
		MarginBottom(1.942, SymUnitLh).
		Build())

	// Code/preformatted - HTML <code> and <pre> elements
	// Amazon reference for code: font-family: monospace only
	sr.Register(NewStyle("code").
		FontFamily("monospace").
		Build())

	// Amazon reference for pre: font-family: monospace, white-space: pre
	// Amazon reference (stylemap.ion): margin-top: 1em, margin-bottom: 1em
	// Note: white-space is handled at content level, not in style
	sr.Register(NewStyle("pre").
		FontFamily("monospace").
		MarginTop(0.833333, SymUnitLh).
		MarginBottom(0.833333, SymUnitLh).
		Build())

	// Blockquote - HTML <blockquote> element
	// Amazon reference (stylemap.ion): margin-top: 1em, margin-bottom: 1em, margin-left: 40px, margin-right: 40px
	// Vertical margins converted to lh: 1em / 1.2 = 0.833lh
	sr.Register(NewStyle("blockquote").
		MarginTop(0.833333, SymUnitLh).
		MarginBottom(0.833333, SymUnitLh).
		MarginLeft(40, SymUnitPx).
		MarginRight(40, SymUnitPx).
		Build())

	// List elements - HTML <ol> and <ul>
	// From stylemap: margin-top: 1em, margin-bottom: 1em
	// Convert to lh units using LineHeightRatio (1em / 1.2 = 0.833lh)
	// to match KP3's vertical spacing unit preference.
	// list-style is set at content level, not in style
	listMarginLh := 1.0 / LineHeightRatio // 0.8333...

	sr.Register(NewStyle("ol").
		MarginTop(listMarginLh, SymUnitLh).
		MarginBottom(listMarginLh, SymUnitLh).
		Build())

	sr.Register(NewStyle("ul").
		MarginTop(listMarginLh, SymUnitLh).
		MarginBottom(listMarginLh, SymUnitLh).
		Build())

	// Table elements - HTML <table>, <th>, <td>
	// KFX tables require separate styles for container and text elements:
	//   - Container: border, padding, vertical-align (NO text properties)
	//   - Text: text-align only (NO border/padding)
	// These are kept separate from CSS-parsed td/th to avoid property contamination.

	// Table style - applied to the table element ($278)
	// Amazon reference sFE: box-align: center; line-height: 1lh; margin-top: 0.833333lh;
	//   max-width: 100%; min-width: 100%; sizing-bounds: content_bounds; text-indent: 0%; width: 32em
	// The sizing-bounds: content_bounds + yj.table_features: [pan_zoom, scale_fit] enables
	// table scaling to fit within page bounds instead of spanning multiple pages.
	sr.Register(NewStyle("table").
		BoxAlign(SymCenter).
		LineHeight(1, SymUnitLh).
		MarginTop(0.833333, SymUnitLh).
		Width(32, SymUnitEm).
		MinWidth(100, SymUnitPercent).
		MaxWidth(100, SymUnitPercent).
		SizingBounds(SymContentBounds).
		TextIndent(0, SymUnitPercent).
		Build())

	// Cell container style - applied to the cell container ($270)
	// Amazon reference sBP: border-style: solid; border-width: 0.45pt;
	//   padding: 0.416667lh / 1.563%; yj.vertical-align: center
	// Inherits from "td" to pick up CSS properties like background-color.
	sr.Register(NewStyle("td-container").
		Inherit("td").
		BorderStyle(SymSolid).
		BorderWidth(0.45, SymUnitPt).
		PaddingTop(0.416667, SymUnitLh).
		PaddingBottom(0.416667, SymUnitLh).
		PaddingLeft(1.563, SymUnitPercent).
		PaddingRight(1.563, SymUnitPercent).
		YjVerticalAlign(SymCenter).
		Build())

	// Header cell container style - inherits from "th" for CSS properties (background-color, etc.)
	// Local properties provide defaults that CSS can override.
	sr.Register(NewStyle("th-container").
		Inherit("th").
		BorderStyle(SymSolid).
		BorderWidth(0.45, SymUnitPt).
		PaddingTop(0.416667, SymUnitLh).
		PaddingBottom(0.416667, SymUnitLh).
		PaddingLeft(1.563, SymUnitPercent).
		PaddingRight(1.563, SymUnitPercent).
		YjVerticalAlign(SymCenter).
		Build())

	// Cell text style - applied to text inside cell
	// Amazon reference s1F: text-align: left (ONLY text-align, nothing else)
	sr.Register(NewStyle("td-text").
		TextAlign(SymLeft).
		Build())

	// Cell text alignment variants
	sr.Register(NewStyle("td-text-center").
		TextAlign(SymCenter).
		Build())

	sr.Register(NewStyle("td-text-right").
		TextAlign(SymRight).
		Build())

	sr.Register(NewStyle("td-text-justify").
		TextAlign(SymJustify).
		Build())

	// Header cell text style - centered by default (bold applied via style_events)
	sr.Register(NewStyle("th-text").
		TextAlign(SymCenter).
		Build())

	// Header cell text alignment variants
	sr.Register(NewStyle("th-text-center").
		TextAlign(SymCenter).
		Build())

	sr.Register(NewStyle("th-text-left").
		TextAlign(SymLeft).
		Build())

	sr.Register(NewStyle("th-text-right").
		TextAlign(SymRight).
		Build())

	sr.Register(NewStyle("th-text-justify").
		TextAlign(SymJustify).
		Build())

	// Data cell text alignment variants (including explicit left)
	sr.Register(NewStyle("td-text-left").
		TextAlign(SymLeft).
		Build())

	// Table cell image styles with alignment variants
	// th-image uses center by default (header cells are centered)
	sr.Register(NewStyle("th-image").
		BoxAlign(SymCenter).
		Build())
	sr.Register(NewStyle("th-image-center").
		BoxAlign(SymCenter).
		Build())
	sr.Register(NewStyle("th-image-left").
		BoxAlign(SymLeft).
		Build())
	sr.Register(NewStyle("th-image-right").
		BoxAlign(SymRight).
		Build())

	// td-image uses left by default (data cells are left-aligned)
	sr.Register(NewStyle("td-image").
		BoxAlign(SymLeft).
		Build())
	sr.Register(NewStyle("td-image-center").
		BoxAlign(SymCenter).
		Build())
	sr.Register(NewStyle("td-image-left").
		BoxAlign(SymLeft).
		Build())
	sr.Register(NewStyle("td-image-right").
		BoxAlign(SymRight).
		Build())

	// CSS-parsed td/th styles - these exist for CSS compatibility but are NOT
	// used directly for table rendering (td-container/td-text are used instead)
	sr.Register(NewStyle("th").
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("td").
		Build())

	// ============================================================
	// Inline HTML elements
	// ============================================================

	// Strong/bold - HTML <strong> and <b> elements
	sr.Register(NewStyle("strong").
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("b").
		FontWeight(SymBold).
		Build())

	// Emphasis/italic - HTML <em> and <i> elements
	sr.Register(NewStyle("em").
		FontStyle(SymItalic).
		Build())

	sr.Register(NewStyle("i").
		FontStyle(SymItalic).
		Build())

	// Underline - HTML <u> element
	sr.Register(NewStyle("u").
		Underline(true).
		Build())

	// Strikethrough - HTML <s>, <strike>, <del> elements
	sr.Register(NewStyle("s").
		Strikethrough(true).
		Build())

	sr.Register(NewStyle("strike").
		Strikethrough(true).
		Build())

	sr.Register(NewStyle("del").
		Strikethrough(true).
		Build())

	// Subscript and superscript - HTML <sub> and <sup> elements
	// We use baseline_style for vertical-align and 0.75rem for "smaller" font-size.
	// NOTE: We intentionally do NOT set line-height here. Setting line-height: normal
	// causes inconsistent vertical spacing when sub/sup appears in titles or other
	// contexts with specific line-height values. By omitting line-height, the style
	// inherits from its context, maintaining consistent vertical rhythm.
	// KP3 similarly uses explicit line-height values calculated for each context.
	//
	// DescendantReplacement: When sub/sup appears in headings, the heading-context
	// descendant selector (h1--sub, etc.) completely replaces this base style,
	// allowing font-size to be inherited from the heading.
	sr.Register(NewStyle("sub").
		BaselineStyle(SymSubscript).
		FontSize(0.75, SymUnitRem).
		DescendantReplacement().
		Build())

	sr.Register(NewStyle("sup").
		BaselineStyle(SymSuperscript).
		FontSize(0.75, SymUnitRem).
		DescendantReplacement().
		Build())

	// Heading-context sub/sup: When sub/sup appears in headings (h1-h6), we apply
	// baseline-style with a modest font-size reduction (0.9em). This matches KP3
	// behavior where inline <sup>/<sub> in titles are slightly smaller than the
	// heading text but not as small as in normal paragraphs (which use 0.75rem).
	for i := 1; i <= 6; i++ {
		hTag := fmt.Sprintf("h%d", i)
		sr.Register(NewStyle(hTag+"--sub").
			BaselineStyle(SymSubscript).
			FontSize(0.9, SymUnitEm).
			Build())
		sr.Register(NewStyle(hTag+"--sup").
			BaselineStyle(SymSuperscript).
			FontSize(0.9, SymUnitEm).
			Build())
	}

	// Small text - HTML <small> element
	// Amazon reference: font-size: smaller
	// DescendantReplacement: When small appears in headings, the heading-context
	// descendant selector completely replaces this base style, allowing font-size
	// to be inherited from the heading.
	sr.Register(NewStyle("small").
		FontSizeSmaller().
		DescendantReplacement().
		Build())

	// Heading-context small: When <small> appears in headings (h1-h6), we apply
	// no properties, allowing full inheritance from the heading context.
	for i := 1; i <= 6; i++ {
		hTag := fmt.Sprintf("h%d", i)
		sr.Register(NewStyle(hTag + "--small").
			Build())
	}

	// ============================================================
	// FB2-specific inline styles (class names used in default.css)
	// ============================================================

	// Emphasis - FB2 <emphasis> element, maps to .emphasis class
	sr.Register(NewStyle("emphasis").
		FontStyle(SymItalic).
		Build())

	// Strikethrough - FB2 <strikethrough> element, maps to .strikethrough class
	sr.Register(NewStyle("strikethrough").
		Strikethrough(true).
		Build())

	// ============================================================
	// Internal styles (used by generator, not HTML elements)
	// ============================================================

	// Title paragraph immediately following an image-only title paragraph.
	//
	// KP3 special-cases some multi-paragraph titles where the first title line is an
	// inline-image-only paragraph (wrapped as text entry) followed by a real text
	// paragraph. In those cases KP3 emits a slightly larger margin-top on the text
	// paragraph (0.594lh instead of the usual 0.55275lh for 1.25rem title text).
	//
	// This is referenced directly by generator code to avoid changing convert/default.css.
	sr.Register(NewStyle("title-after-image").
		MarginTop(0.5999994, SymUnitLh).
		Build())

	// Image container style
	sr.Register(NewStyle("image").
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	// KP3 uses different wrapper margins for nested section titles depending on depth.
	// Default.css has a single .section-title margin, but KP3 normalizes it into
	// multiple wrapper variants during conversion.
	//
	// These wrappers are referenced directly by generator code; we keep them programmatic
	// to avoid changing convert/default.css.
	for _, tt := range []struct {
		name string
		mt   float64
		mb   float64
	}{
		{name: "section-title--h2", mt: 1.66667, mb: 0.9375},
		{name: "section-title--h3", mt: 1.66667, mb: 1.24688},
		{name: "section-title--h4", mt: 1.66667, mb: 1.56562},
		// KP3 uses the same wrapper margins for deeper levels.
		{name: "section-title--h5", mt: 2.18438, mb: 2.18438},
		{name: "section-title--h6", mt: 2.18438, mb: 2.18438},
	} {
		sr.Register(NewStyle(tt.name).
			BreakInsideAvoid().
			YjBreakAfter(SymAvoid).
			LineHeight(1, SymUnitLh).
			MarginTop(tt.mt, SymUnitLh).
			MarginBottom(tt.mb, SymUnitLh).
			Build())
	}

	// Vignette image style - decorative images in title blocks.
	// Uses 100% width and KP3-compatible margin-top for spacing.
	// Position filtering will remove margin-top for first element in multi-element blocks.
	// Name matches CSS convention: img.image-vignette
	sr.Register(NewStyle("image-vignette").
		BoxAlign(SymCenter).
		SizingBounds(SymContentBounds).
		Width(100, SymUnitPercent).
		MarginTop(0.697917, SymUnitLh). // Matching KP3 reference vignette spacing
		Build())

	// End vignette image style - decorative images at end of chapters/sections.
	// KP3 reference shows mt=1.25lh, mb=1.25lh for section-end vignettes.
	sr.Register(NewStyle("image-vignette-end").
		BoxAlign(SymCenter).
		SizingBounds(SymContentBounds).
		Width(100, SymUnitPercent).
		MarginTop(1.25, SymUnitLh).
		MarginBottom(1.25, SymUnitLh).
		YjBreakBefore(SymAvoid).
		Build())

	return sr
}

// PostProcessForKFX applies Kindle-specific enhancements to styles after CSS conversion.
// This handles KFX-specific properties that don't have direct CSS equivalents or
// need special handling:
//   - layout-hints: [treat_as_title] for headings and title-like styles
//   - yj-break-before/yj-break-after for page break handling
//   - break-inside for keep-together behavior
//
// Note: Drop cap properties are handled during CSS conversion (see Converter.ConvertStylesheet)
// because they require access to the full stylesheet to detect .has-dropcap .dropcap patterns.
func (sr *StyleRegistry) PostProcessForKFX() {
	for name, def := range sr.styles {
		enhanced := sr.applyKFXEnhancements(name, def)
		if len(enhanced.Properties) != len(def.Properties) {
			sr.tracer.TracePostProcess(name, "KFX enhancements applied", enhanced.Properties)
		}
		sr.styles[name] = enhanced
	}
}

// applyKFXEnhancements applies Kindle-specific enhancements to a style definition.
func (sr *StyleRegistry) applyKFXEnhancements(name string, def StyleDef) StyleDef {
	// Make a copy of properties to avoid modifying the original
	props := make(map[KFXSymbol]any, len(def.Properties))
	maps.Copy(props, def.Properties)

	// Apply layout-hints for headings and title-like styles
	if sr.shouldHaveLayoutHintTitle(name, props) {
		if _, exists := props[SymLayoutHints]; !exists {
			props[SymLayoutHints] = []any{SymbolValue(SymTreatAsTitle)}
		}
		// KP3 special case: nested section title header text (font-size: 120% -> 1.125rem)
		// uses a slightly smaller line-height than our generic AdjustedLineHeightLh.
		//
		// In our generator, some of these styles end up as resolved "s.." names, so
		// we match by properties rather than by source CSS class name.
		// (line-height adjustment for 1.125rem title text is handled later in BuildFragments
		// so it can override adjustLineHeightForFontSize/ensureDefaultLineHeight).
		// KP3 reference: title text styles have margin-top but NOT margin-bottom.
		// The margin-bottom is only on the wrapper container, not the text inside.
		// EXCEPTIONS that should KEEP their margin-bottom:
		// 1. Subtitle styles with page-break-after: avoid (spacing with next element)
		// 2. Footnote-title: used directly on paragraphs (no wrapper), needs both margins
		if !sr.isSubtitleWithBreakAfterAvoid(name, props) && name != "footnote-title" {
			delete(props, SymMarginBottom)
		}
	}

	// Convert page-break properties to KFX yj-break properties
	sr.convertPageBreaksToYjBreaks(props)

	// Apply break-inside: avoid for title wrappers
	if sr.shouldHaveBreakInsideAvoid(name, props) {
		if _, exists := props[SymBreakInside]; !exists {
			props[SymBreakInside] = SymbolValue(SymAvoid)
		}
		// KP3 reference shows wrapper styles with break-inside: avoid also include line-height: 1lh
		if _, exists := props[SymLineHeight]; !exists {
			props[SymLineHeight] = DimensionValue(1.0, SymUnitLh)
		}
	}

	// Convert text_color to link styles for link-* classes
	// KFX uses link-unvisited-style and link-visited-style maps containing the color,
	// not direct text_color on the style
	if strings.HasPrefix(name, "link-") {
		if color, hasColor := props[SymTextColor]; hasColor {
			// Create the nested style map with just the color
			linkStyleMap := map[KFXSymbol]any{SymTextColor: color}
			props[SymLinkUnvisitedStyle] = linkStyleMap
			props[SymLinkVisitedStyle] = linkStyleMap
			// Remove direct text_color - it should only be in the link style maps
			delete(props, SymTextColor)
		}
	}

	// Note: box_align is NOT used for title wrappers.
	// Reference KFX files rely on text_alignment: center on the content text itself,
	// not box_align on the wrapper container.

	return StyleDef{
		Name:                  def.Name,
		Parent:                def.Parent,
		Properties:            props,
		DescendantReplacement: def.DescendantReplacement,
	}
}

// shouldHaveLayoutHintTitle determines if a style should have layout-hints: [treat_as_title].
// This applies to:
//   - HTML heading elements (h1-h6)
//   - Styles ending with "-title-header" (body-title-header, chapter-title-header, etc.)
//   - Simple title styles for generated sections (annotation-title, toc-title, footnote-title)
//
// NOTE: Styles with additional suffixes like "-title-header-first", "-title-header-next",
// "-title-header-break", "-title-header-emptyline" should NOT get layout-hints because
// they are used in style_events ($142), not as direct content styles ($157).
// KP3 reference shows layout-hints only on the direct content style, not on style_events styles.
//
// NOTE: Subtitle styles (-subtitle) are NOT treated as titles - they are regular paragraphs
// with special formatting (centered, bold), similar to how EPUB handles them.
func (sr *StyleRegistry) shouldHaveLayoutHintTitle(name string, _ map[KFXSymbol]any) bool {
	// HTML heading elements
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}

	// Title header styles - only the BASE styles, not the suffixed variants used in style_events.
	// Examples that SHOULD match: "body-title-header", "chapter-title-header", "section-title-header"
	// Examples that should NOT match: "chapter-title-header-first", "chapter-title-header-next",
	// "chapter-title-header-break", "chapter-title-header-emptyline"
	if strings.HasSuffix(name, "-title-header") {
		return true
	}

	// Simple title styles for generated sections (annotation-title, toc-title)
	// These are used directly as content styles without -header suffix
	switch name {
	case "annotation-title", "toc-title":
		return true
	}

	return false
}

// shouldHaveBreakInsideAvoid determines if a style should have break-inside: avoid.
// This applies to title WRAPPER styles to keep titles together (e.g., chapter-title).
// It does NOT apply to title CONTENT styles with layout-hints: [treat_as_title]
// (e.g., annotation-title, toc-title, footnote-title, chapter-title-header).
//
// KP3 reference shows:
//   - Wrapper styles have: break-inside: avoid + yj-break-after: avoid (no layout-hints)
//   - Content styles have: layout-hints: [treat_as_title] (no break-inside: avoid)
func (sr *StyleRegistry) shouldHaveBreakInsideAvoid(name string, _ map[KFXSymbol]any) bool {
	// Title wrapper styles - these are containers, not text content
	switch name {
	case "body-title", "chapter-title", "section-title":
		return true
	}

	// KP3 wrapper variants for nested section titles (section-title--h2..h6)
	if strings.HasPrefix(name, "section-title--h") {
		return true
	}

	// Exclude content styles that get layout-hints: [treat_as_title]
	// These are NOT wrappers - they contain the actual title text
	switch name {
	case "annotation-title", "toc-title", "footnote-title":
		return false
	}

	// Other *-title wrapper styles (but not *-title-header which are content styles)
	if strings.HasSuffix(name, "-title") && !strings.HasSuffix(name, "-title-header") {
		return true
	}
	return false
}

// isSubtitleWithBreakAfterAvoid returns true if this is a subtitle style with page-break-after: avoid.
// Such styles should keep their margin-bottom because it's used for spacing with the next element,
// and the element won't participate in sibling margin collapsing.
func (sr *StyleRegistry) isSubtitleWithBreakAfterAvoid(name string, props map[KFXSymbol]any) bool {
	// Only applies to subtitle styles
	if name != "subtitle" && !strings.HasSuffix(name, "-subtitle") {
		return false
	}
	// Check if it has yj-break-after: avoid or the intermediate marker SymKeepLast: avoid
	// (The CSS converter stores SymKeepLast which is converted to yj-break-after during post-processing)
	return isSymbol(props[SymYjBreakAfter], SymAvoid) || isSymbol(props[SymKeepLast], SymAvoid)
}

// convertPageBreaksToYjBreaks converts CSS page-break properties to KFX yj-break properties.
// The CSS converter sets SymKeepFirst/SymKeepLast as intermediate markers.
// This function converts them to proper yj-break-* properties and also handles
// title wrapper styles that need yj-break-after: avoid.
// The intermediate markers are removed after conversion since KP3 doesn't output them.
func (sr *StyleRegistry) convertPageBreaksToYjBreaks(props map[KFXSymbol]any) {
	// Convert SymKeepFirst (from page-break-before) to yj-break-before
	if keepFirst, ok := props[SymKeepFirst]; ok {
		if _, exists := props[SymYjBreakBefore]; !exists {
			switch v := keepFirst.(type) {
			case SymbolValue:
				props[SymYjBreakBefore] = v
			case KFXSymbol:
				props[SymYjBreakBefore] = SymbolValue(v)
			}
		}
		delete(props, SymKeepFirst)
	}

	// Convert SymKeepLast (from page-break-after) to yj-break-after
	if keepLast, ok := props[SymKeepLast]; ok {
		if _, exists := props[SymYjBreakAfter]; !exists {
			switch v := keepLast.(type) {
			case SymbolValue:
				props[SymYjBreakAfter] = v
			case KFXSymbol:
				props[SymYjBreakAfter] = SymbolValue(v)
			}
		}
		delete(props, SymKeepLast)
	}

	// KP3 pattern for break properties:
	// Pattern 1: break-inside: avoid + yj-break-after: avoid (no yj-break-before)
	// Pattern 2: yj-break-before: auto + yj-break-after: avoid (no break-inside)
	// These are mutually exclusive. Remove yj-break-before when break-inside: avoid exists.
	if _, hasBreakInside := props[SymBreakInside]; hasBreakInside {
		// When break-inside: avoid is present, KP3 does NOT output yj-break-before.
		// The break-inside alone handles keeping content together without a page break.
		delete(props, SymYjBreakBefore)
	}

	// KP3 reference output does not use yj-break-before: always in styles.
	// Page breaks for "always" are represented via section/storyline boundaries.
	if v, ok := props[SymYjBreakBefore]; ok && isSymbol(v, SymAlways) {
		delete(props, SymYjBreakBefore)
	}
}
