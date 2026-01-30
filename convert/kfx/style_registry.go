package kfx

import (
	"fmt"
	"maps"
	"math"
	"strings"

	"go.uber.org/zap"
)

// StyleRegistry manages style definitions and generates style fragments.
type StyleRegistry struct {
	styles map[string]StyleDef
	order  []string        // Preserve insertion order
	used   map[string]bool // Track which styles are actually used (for BuildFragments)
	usage  map[string]styleUsage

	resolved        map[string]string // signature -> resolved style name
	resolvedCounter int

	tracer *StyleTracer // Optional tracer for debugging style resolution

	externalLinks *ExternalLinkRegistry // Tracks external link URLs -> anchor IDs
}

// NewStyleRegistry creates a new style registry.
func NewStyleRegistry() *StyleRegistry {
	return &StyleRegistry{
		styles:        make(map[string]StyleDef),
		used:          make(map[string]bool),
		usage:         make(map[string]styleUsage),
		resolved:      make(map[string]string),
		externalLinks: NewExternalLinkRegistry(),
		// Start from 55 so the first style is "s1J" (base36(55) == 1J), like KP3 samples.
		resolvedCounter: 54,
	}
}

// SetTracer sets the style tracer for debugging.
func (sr *StyleRegistry) SetTracer(t *StyleTracer) {
	sr.tracer = t
}

// Tracer returns the style tracer, or nil if none is set.
func (sr *StyleRegistry) Tracer() *StyleTracer {
	return sr.tracer
}

// RegisterExternalLink registers an external URL and returns its anchor ID.
// Multiple references to the same URL will share the same anchor ID.
func (sr *StyleRegistry) RegisterExternalLink(url string) string {
	return sr.externalLinks.Register(url)
}

// BuildExternalLinkFragments creates anchor fragments for all registered external URLs.
func (sr *StyleRegistry) BuildExternalLinkFragments() []*Fragment {
	return sr.externalLinks.BuildFragments()
}

// RecomputeUsedStyles scans content fragments and marks which styles are actually referenced.
// This must be called before BuildFragments to ensure only used styles are included in output.
//
// The method recursively scans all fragments looking for $157 (style) symbol references,
// and marks those styles as used. This handles styles in:
//   - Content entries ($146)
//   - Style events ($142)
//   - Nested containers and children
func (sr *StyleRegistry) RecomputeUsedStyles(fragments *FragmentList) {
	// Clear existing usage flags
	sr.used = make(map[string]bool)

	// Scan all fragments for style references
	for _, frag := range fragments.All() {
		sr.scanValueForStyles(frag.Value)
	}
}

// scanValueForStyles recursively scans a value for style symbol references.
func (sr *StyleRegistry) scanValueForStyles(v any) {
	switch val := v.(type) {
	case StructValue:
		sr.scanStructForStyles(val)
	case map[KFXSymbol]any:
		sr.scanStructForStyles(val)
	case []any:
		for _, item := range val {
			sr.scanValueForStyles(item)
		}
	}
}

// scanStructForStyles scans a struct value for style references.
func (sr *StyleRegistry) scanStructForStyles(s map[KFXSymbol]any) {
	// Check for direct style reference ($157)
	if styleVal, ok := s[SymStyle]; ok {
		switch v := styleVal.(type) {
		case SymbolByNameValue:
			// Style stored as string name (before serialization)
			styleName := string(v)
			if styleName != "" {
				sr.used[styleName] = true
			}
		case SymbolValue:
			// Style stored as resolved symbol ID (after serialization)
			// This shouldn't happen in our use case, but handle it for completeness
			styleName := traceSymbolName(KFXSymbol(v))
			if styleName != "" {
				sr.used[styleName] = true
			}
		}
	}

	// Recursively scan all values
	for _, val := range s {
		sr.scanValueForStyles(val)
	}
}

func (sr *StyleRegistry) mergeProperty(dst map[KFXSymbol]any, sym KFXSymbol, val any) {
	sr.mergePropertyWithContext(dst, sym, val, mergeContextInline)
}

func (sr *StyleRegistry) mergePropertyWithContext(dst map[KFXSymbol]any, sym KFXSymbol, val any, ctx mergeContext) {
	if existing, ok := dst[sym]; ok {
		merged, keep := mergeStyleProperty(sym, existing, val, ctx, sr.tracer)
		if keep {
			dst[sym] = merged
		}
		return
	}
	dst[sym] = val
}

func (sr *StyleRegistry) mergePropertiesWithContext(dst map[KFXSymbol]any, src map[KFXSymbol]any, ctx mergeContext) {
	for sym, val := range src {
		sr.mergePropertyWithContext(dst, sym, val, ctx)
	}
}

// Register adds a style to the registry.
// If a style with the same name already exists, the properties are merged,
// with new properties overriding existing ones (CSS cascade behavior).
func (sr *StyleRegistry) Register(def StyleDef) {
	existing, exists := sr.styles[def.Name]
	if !exists {
		sr.order = append(sr.order, def.Name)
		sr.styles[def.Name] = def
		sr.tracer.TraceRegister(def.Name, def.Properties)
		return
	}

	merged := make(map[KFXSymbol]any, len(existing.Properties)+len(def.Properties))
	mergeAllWithRules(merged, existing.Properties, mergeContextInline, sr.tracer)
	// Use mergeContextClassOverride for the new definition so CSS values properly
	// override defaults. With mergeContextInline (allowWritingModeConvert=true),
	// margin-top/bottom use override-maximum which keeps the larger value.
	// But CSS semantics require that later rules override earlier ones regardless
	// of which value is larger. mergeContextClassOverride (allowWritingModeConvert=false)
	// triggers the override rule for margins, matching CSS cascade behavior.
	mergeAllWithRules(merged, def.Properties, mergeContextClassOverride, sr.tracer)

	// Inherit parent from new def if specified
	parent := existing.Parent
	if def.Parent != "" {
		parent = def.Parent
	}

	// Preserve DescendantReplacement flag (true if either has it)
	descReplacement := existing.DescendantReplacement || def.DescendantReplacement

	sr.styles[def.Name] = StyleDef{
		Name:                  def.Name,
		Parent:                parent,
		Properties:            merged,
		DescendantReplacement: descReplacement,
	}
	sr.tracer.TraceRegister(def.Name+" (merged)", merged)
}

// Get returns a style definition by name.
func (sr *StyleRegistry) Get(name string) (StyleDef, bool) {
	def, ok := sr.styles[name]
	return def, ok
}

// IsDescendantReplacement returns true if the named style uses replacement semantics
// for descendant selectors. When true, descendant selectors like "h1--sub" completely
// replace the base class rather than just overriding specific properties.
func (sr *StyleRegistry) IsDescendantReplacement(name string) bool {
	if def, ok := sr.styles[name]; ok {
		return def.DescendantReplacement
	}
	return false
}

// Names returns all registered style names in order.
func (sr *StyleRegistry) Names() []string {
	return sr.order
}

// EnsureBaseStyle ensures a style exists, creating a minimal one if needed.
// Unlike EnsureStyle it does NOT mark the style as used for output.
// This is used for resolving style combinations into KP3-like "s.." styles.
func (sr *StyleRegistry) EnsureBaseStyle(name string) {
	if _, exists := sr.styles[name]; exists {
		return
	}

	// Try to infer parent style from naming conventions
	parent := sr.inferParentStyle(name)

	// Log auto-creation of unknown styles (not defined in CSS)
	sr.tracer.TraceAutoCreate(name, parent)

	// These auto-created styles may be created after sr.PostProcessForKFX() has
	// already run (e.g. from runtime EnsureBaseStyle calls while building content).
	// Apply the same KFX enhancements here so late-created styles match KP3 output.
	def := NewStyle(name).
		Inherit(parent).
		Build()
	def = sr.applyKFXEnhancements(name, def)
	sr.Register(def)
}

// ResolveStyle ensures a style exists, resolves it to a generated name, and tracks usage type.
// Returns the generated style name (like "s1J") that should be used in KFX output.
//
// The usage parameter (styleUsageText, styleUsageImage, styleUsageWrapper) affects how the
// style is post-processed in BuildFragments (e.g., text styles get line-height adjustments).
//
// Note: This does NOT mark the style as "used" for output filtering. Call RecomputeUsedStyles
// before BuildFragments to determine which styles are actually referenced in the final content.
func (sr *StyleRegistry) ResolveStyle(name string, usage styleUsage) string {
	if name == "" {
		return ""
	}
	sr.EnsureBaseStyle(name)

	// Resolve inheritance to get final properties
	def := sr.styles[name]
	resolved := sr.resolveInheritance(def)

	// Filter out height: auto - KP3 never outputs this in styles (it's the implied default).
	// The value may be stored as either KFXSymbol or SymbolValue depending on source.
	props := resolved.Properties
	if h, ok := props[SymHeight]; ok {
		isAuto := false
		switch v := h.(type) {
		case SymbolValue:
			isAuto = KFXSymbol(v) == SymAuto
		case KFXSymbol:
			isAuto = v == SymAuto
		}
		if isAuto {
			// Make a copy to avoid modifying the original
			props = make(map[KFXSymbol]any, len(resolved.Properties))
			maps.Copy(props, resolved.Properties)
			delete(props, SymHeight)
		}
	}

	// Check if we already have a generated name for this property set
	sig := styleSignature(props)
	if genName, ok := sr.resolved[sig]; ok {
		// Don't add styleUsageText to inline-only styles - they should inherit
		// line-height from parent, not get the default line-height added.
		if usage == styleUsageText && sr.hasInlineUsage(genName) {
			// Just mark as used, don't change usage type
		} else {
			sr.usage[genName] = sr.usage[genName] | usage
		}
		return genName
	}

	// Generate a new name and register the resolved style
	genName := sr.nextResolvedStyleName()
	sr.resolved[sig] = genName
	sr.usage[genName] = usage
	sr.Register(StyleDef{Name: genName, Properties: props})
	return genName
}

func (sr *StyleRegistry) hasTextUsage(name string) bool {
	return sr.usage[name]&styleUsageText != 0
}

func (sr *StyleRegistry) hasInlineUsage(name string) bool {
	return sr.usage[name]&styleUsageInline != 0
}

func (sr *StyleRegistry) hasImageUsage(name string) bool {
	return sr.usage[name]&styleUsageImage != 0
}

// GetUsage returns the usage flags for a style. Used when creating style variants
// that should inherit the original style's usage type.
func (sr *StyleRegistry) GetUsage(name string) styleUsage {
	return sr.usage[name]
}

func (sr *StyleRegistry) nextResolvedStyleName() string {
	sr.resolvedCounter++
	return "s" + toBase36(sr.resolvedCounter)
}

// tableElementProperties are properties that KFX requires to be on the table element,
// NOT in the style. KP3 moves these from style to element during table processing.
// See com/amazon/adapter/common/l/a/c/e.java lines 16-18 in KP3 source.
var tableElementProperties = map[KFXSymbol]bool{
	SymTableBorderCollapse:     true,
	SymBorderSpacingHorizontal: true,
	SymBorderSpacingVertical:   true,
}

// TableElementProps holds properties that should be set on the table element,
// extracted from CSS before filtering them from the style.
type TableElementProps struct {
	BorderCollapse    bool
	BorderSpacingH    any // DimensionValue or nil
	BorderSpacingV    any // DimensionValue or nil
	HasBorderCollapse bool
}

// GetTableElementProps extracts table element properties from the resolved "table" style.
// These properties are moved from style to element per KP3 behavior.
// Returns extracted values with defaults for missing properties.
func (sr *StyleRegistry) GetTableElementProps() TableElementProps {
	result := TableElementProps{
		BorderCollapse:    true,                           // default: collapse
		BorderSpacingH:    DimensionValue(0.9, SymUnitPt), // default: 0.9pt (Amazon reference)
		BorderSpacingV:    DimensionValue(0.9, SymUnitPt), // default: 0.9pt
		HasBorderCollapse: false,
	}

	// Get the merged "table" style properties before filtering
	sr.EnsureBaseStyle("table")
	def, exists := sr.styles["table"]
	if !exists {
		return result
	}
	resolved := sr.resolveInheritance(def)

	// Extract border_spacing if present in CSS
	if v, ok := resolved.Properties[SymBorderSpacingHorizontal]; ok {
		result.BorderSpacingH = v
	}
	if v, ok := resolved.Properties[SymBorderSpacingVertical]; ok {
		result.BorderSpacingV = v
	}

	// Extract table_border_collapse if present
	// CSS border-collapse: collapse -> true, separate -> false
	if v, ok := resolved.Properties[SymTableBorderCollapse]; ok {
		result.HasBorderCollapse = true
		if bv, ok := v.(bool); ok {
			result.BorderCollapse = bv
		}
	}

	return result
}

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

// ResolveCoverImageStyle creates a minimal style for cover images in container-type sections.
// Unlike block images, this doesn't include width constraints since the page template
// already defines the container dimensions. Reference KFX cover image style has:
//   - font_size: 1rem (in unit $505)
//   - line_height: 1.0101lh
func (sr *StyleRegistry) ResolveCoverImageStyle() string {
	// Build minimal properties matching KP3 cover image style exactly
	props := map[KFXSymbol]any{
		SymFontSize:   DimensionValue(1, SymUnitRem),                  // font-size: 1rem
		SymLineHeight: DimensionValue(DefaultLineHeightLh, SymUnitLh), // line-height: 1.0101lh
	}

	sig := styleSignature(props)
	if name, ok := sr.resolved[sig]; ok {
		sr.used[name] = true
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	sr.used[name] = true
	sr.Register(StyleDef{Name: name, Properties: props})
	return name
}

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

	// KP3 wrapper variants for nested section titles (section-title--h2..h6)
	if strings.HasPrefix(name, "section-title--h") {
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
// font-size differs from the default (1rem). KP3 uses different formulas based
// on font-size:
//
//   - For font-size < 1rem (e.g., sub/sup): line-height = 1/font-size
//     This keeps absolute line spacing the same as surrounding 1rem text.
//     Example: 0.75rem font-size → 1.33333lh (0.75 * 1.33333 = 1.0 absolute)
//
//   - For font-size >= 1rem (e.g., headings): line-height = 1.0101lh
//     Uses the standard adjustment factor (100/99 ≈ 1.0101).
//
// Note: If line-height is already set (e.g., calculated in ResolveInlineDelta
// for inline elements in non-standard contexts like headings), it is preserved.
// The ratio-based calculation in ResolveInlineDelta is more accurate for those cases.
//
// Vertical margins are recalculated using the line-height adjustment factor.
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

	// KP3 behavior (observed): monospace styles (e.g. <code>/<pre>) are not emitted
	// with font-size below 0.75rem, even when the source CSS uses smaller percent
	// values like 70%.
	//
	// We clamp only monospace here to avoid changing semantics for other small
	// font-size use-cases (sub/sup, small text, etc.).
	if fontSizeVal < 0.75 && isMonospaceFontFamily(props[SymFontFamily]) {
		fontSizeVal = 0.75
		updated[SymFontSize] = DimensionValue(fontSizeVal, SymUnitRem)
	}

	// Calculate line-height based on font-size, but only if not already set.
	// Styles from ResolveInlineDelta may already have ratio-based line-height
	// calculated relative to the parent's font-size, which is more accurate
	// for inline elements in heading contexts.
	var adjustedLh float64
	// For monospace blocks, KP3 uses a slightly different line-height for 0.75rem
	// (observed in reference output: 0.75rem -> 1.33249lh).
	// This also impacts margin scaling for code listings.
	const kp3Monospace075LineHeightLh = 1.33249
	if existingLh, hasLh := props[SymLineHeight]; hasLh {
		// Use existing line-height (already calculated with proper context)
		if lhVal, lhUnit, ok := measureParts(existingLh); ok && lhUnit == SymUnitLh {
			adjustedLh = lhVal
		} else {
			// Fallback: calculate based on font-size
			if fontSizeVal < 1.0 {
				adjustedLh = 1.0 / fontSizeVal
				if isMonospaceFontFamily(updated[SymFontFamily]) && math.Abs(fontSizeVal-0.75) < 1e-9 {
					adjustedLh = kp3Monospace075LineHeightLh
				}
			} else {
				adjustedLh = AdjustedLineHeightLh
			}
			updated[SymLineHeight] = DimensionValue(RoundDecimals(adjustedLh, LineHeightPrecision), SymUnitLh)
		}
	} else {
		// No existing line-height: calculate based on font-size
		if fontSizeVal < 1.0 {
			adjustedLh = 1.0 / fontSizeVal
			if isMonospaceFontFamily(updated[SymFontFamily]) && math.Abs(fontSizeVal-0.75) < 1e-9 {
				adjustedLh = kp3Monospace075LineHeightLh
			}
		} else {
			adjustedLh = AdjustedLineHeightLh
		}
		updated[SymLineHeight] = DimensionValue(RoundDecimals(adjustedLh, LineHeightPrecision), SymUnitLh)
	}

	// Adjust vertical margins using the line-height factor.
	//
	// For most styles, KP3 scales vertical margins down when line-height is adjusted.
	// However, for monospace blocks at 0.75rem (code listings), KP3 keeps the
	// absolute spacing consistent with the ideal 1/font-size line-height and then
	// expresses margins relative to the emitted line-height.
	isMonospace := isMonospaceFontFamily(updated[SymFontFamily])
	if isMonospace && fontSizeVal < 1.0 {
		idealLh := 1.0 / fontSizeVal
		scale := idealLh / adjustedLh
		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom} {
			if margin, ok := updated[sym]; ok {
				if marginVal, marginUnit, ok := measureParts(margin); ok && marginUnit == SymUnitLh {
					adjusted := RoundSignificant(marginVal*scale, SignificantFigures)
					updated[sym] = DimensionValue(adjusted, SymUnitLh)
				}
			}
		}
	} else {
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

func isMonospaceFontFamily(v any) bool {
	fam, ok := v.(string)
	if !ok || fam == "" {
		return false
	}
	return strings.Contains(strings.ToLower(fam), "monospace")
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

// resolveInheritance flattens a style by merging all parent properties.
// Child properties override parent properties. Handles inheritance chains.
// Always returns a new StyleDef with a copied Properties map to prevent
// mutations from affecting the original styles in the registry.
func (sr *StyleRegistry) resolveInheritance(def StyleDef) StyleDef {
	if def.Parent == "" {
		// Even with no inheritance, we must return a copy of Properties
		// to prevent callers from mutating the registry's stored style.
		copied := make(map[KFXSymbol]any, len(def.Properties))
		maps.Copy(copied, def.Properties)
		return StyleDef{
			Name:       def.Name,
			Parent:     def.Parent,
			Properties: copied,
		}
	}

	// Build inheritance chain (child -> parent -> grandparent -> ...)
	chain := []StyleDef{def}
	visited := map[string]bool{def.Name: true}

	current := def
	for current.Parent != "" {
		if visited[current.Parent] {
			// Circular inheritance detected - break the chain
			break
		}
		parent, exists := sr.styles[current.Parent]
		if !exists {
			// Parent not found - stop here
			break
		}
		visited[current.Parent] = true
		chain = append(chain, parent)
		current = parent
	}

	// Merge properties from root ancestor to child (child overrides parent)
	merged := make(map[KFXSymbol]any)
	for i := len(chain) - 1; i >= 0; i-- {
		maps.Copy(merged, chain[i].Properties)
	}

	sr.tracer.TraceInheritance(def.Name, def.Parent, merged)

	return StyleDef{
		Name:       def.Name,
		Parent:     "", // Flattened - no parent needed
		Properties: merged,
	}
}

// RegisterFromCSS adds styles from a parsed CSS stylesheet.
// Later rules override earlier ones for the same style name.
func (sr *StyleRegistry) RegisterFromCSS(styles []StyleDef) {
	for _, def := range styles {
		sr.Register(def)
	}
}

// NewStyleRegistryFromCSS creates a style registry from CSS stylesheet data.
// It starts with default HTML element styles, overlays styles from CSS,
// then applies KFX-specific post-processing for Kindle compatibility.
// Returns the registry and any warnings from CSS parsing/conversion.
func NewStyleRegistryFromCSS(cssData []byte, tracer *StyleTracer, log *zap.Logger) (*StyleRegistry, []string) {
	// Start with HTML element defaults only
	sr := DefaultStyleRegistry()
	sr.SetTracer(tracer)

	mapper := NewStyleMapper(log, tracer)
	warnings := make([]string, 0)

	if len(cssData) > 0 {
		// Parse CSS
		parser := NewParser(log)
		sheet := parser.Parse(cssData)

		// Convert to KFX styles (includes drop cap detection)
		styles, cssWarnings := mapper.MapStylesheet(sheet)
		warnings = append(warnings, cssWarnings...)

		// Register CSS styles (overriding defaults where applicable)
		sr.RegisterFromCSS(styles)

		log.Debug("CSS styles loaded",
			zap.Int("rules", len(sheet.Rules)),
			zap.Int("styles", len(styles)),
			zap.Int("warnings", len(cssWarnings)))
	}

	// Register programmatic descendant selectors.
	// These implement CSS descendant selector semantics (e.g., ".footnote p")
	// that override element defaults for elements inside specific containers.
	// This is needed because CSS class rules like ".footnote { text-indent: 0; }"
	// should not directly apply to child elements - only descendant selectors do.
	sr.registerDescendantSelectors()

	// Apply KFX-specific style adjustments before post-processing.
	// This fixes discrepancies between CSS/EPUB behavior and KFX behavior,
	// such as footnote title margins where the -first/-next variants should
	// inherit from the base footnote-title style rather than override it.
	sr.applyKFXStyleAdjustments()

	// Apply KFX-specific post-processing (layout-hints, yj-break, etc.)
	sr.PostProcessForKFX()

	return sr, warnings
}

// registerDescendantSelectors adds programmatic descendant selectors.
// These implement CSS descendant selector semantics (e.g., ".footnote p")
// that are not expressible in the CSS file but needed for correct KFX output.
//
// In CSS, a rule like ".footnote { text-indent: 0; }" applies to the element
// with class="footnote", not to its children. To affect child paragraphs,
// you need a descendant selector like ".footnote p { text-indent: 0; }".
//
// The style_context.go resolveProperties() function looks up selectors using:
// - "ancestor--descendant" for descendant selectors (CSS: ".ancestor descendant")
// - "parent>child" for direct child selectors (CSS: ".parent > child")
func (sr *StyleRegistry) registerDescendantSelectors() {
	// .footnote > p { text-indent: 0; }
	// Direct child paragraphs of footnote should have no text-indent,
	// overriding p { text-indent: 1em; }. Using direct child selector (>)
	// ensures nested elements like cite inside footnote keep their default indent.
	sr.Register(NewStyle("footnote>p").
		TextIndent(0, SymUnitPercent).
		Build())
}

// applyKFXStyleAdjustments modifies CSS styles for KFX-specific requirements.
// This is called after CSS is loaded but before KFX post-processing.
//
// Unlike registerDescendantSelectors which adds new selector rules, this function
// modifies existing styles to fix discrepancies between CSS/EPUB behavior and KFX behavior.
func (sr *StyleRegistry) applyKFXStyleAdjustments() {
	// Remove vertical margins from footnote-title-first and footnote-title-next.
	//
	// In EPUB, footnote titles use a div wrapper with class "footnote-title" that has
	// margin: 1em 0 0.5em 0, and the p elements inside have class "footnote-title-first"
	// or "footnote-title-next" with margin: 0.2em 0 (for internal spacing).
	//
	// In KFX, there's no div/p hierarchy - styles are applied directly to paragraphs.
	// When both classes are applied ("footnote-title footnote-title-first"), the more
	// specific -first/-next margins (0.2em) override the base margins (1em/0.5em).
	//
	// The reference KP3 output shows footnote title entries with mt=0.833333lh (1em/1.2)
	// and mb=0.416667lh (0.5em/1.2), matching the container margins, not the paragraph margins.
	//
	// By removing vertical margins from -first/-next, they inherit from footnote-title,
	// producing correct output that matches KP3.
	for _, styleName := range []string{"footnote-title-first", "footnote-title-next"} {
		if def, exists := sr.styles[styleName]; exists {
			// Remove vertical margin properties so they inherit from footnote-title
			delete(def.Properties, SymMarginTop)
			delete(def.Properties, SymMarginBottom)
			sr.styles[styleName] = def
		}
	}

	// Inline code styling should not override paragraph alignment.
	// KP3 does not apply code { text-align: left; } to the paragraph when the entire
	// paragraph is a single <code> span and the code style is promoted to block.
	if def, exists := sr.styles["code"]; exists {
		delete(def.Properties, SymTextAlignment)
		sr.styles["code"] = def
	}
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

	// Simple title styles for generated sections (annotation-title, toc-title, footnote-title)
	// These are used directly as content styles without -header suffix
	switch name {
	case "annotation-title", "toc-title", "footnote-title":
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
