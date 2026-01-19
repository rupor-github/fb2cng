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

// RegisterExternalLink registers an external URL and returns its anchor ID.
// Multiple references to the same URL will share the same anchor ID.
func (sr *StyleRegistry) RegisterExternalLink(url string) string {
	return sr.externalLinks.Register(url)
}

// BuildExternalLinkFragments creates anchor fragments for all registered external URLs.
func (sr *StyleRegistry) BuildExternalLinkFragments() []*Fragment {
	return sr.externalLinks.BuildFragments()
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

func (sr *StyleRegistry) mergeProperties(dst map[KFXSymbol]any, src map[KFXSymbol]any) {
	sr.mergePropertiesWithContext(dst, src, mergeContextInline)
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
	mergeAllWithRules(merged, def.Properties, mergeContextInline, sr.tracer)

	// Inherit parent from new def if specified
	parent := existing.Parent
	if def.Parent != "" {
		parent = def.Parent
	}

	sr.styles[def.Name] = StyleDef{
		Name:       def.Name,
		Parent:     parent,
		Properties: merged,
	}
	sr.tracer.TraceRegister(def.Name+" (merged)", merged)
}

// Get returns a style definition by name.
func (sr *StyleRegistry) Get(name string) (StyleDef, bool) {
	def, ok := sr.styles[name]
	return def, ok
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

	sr.Register(NewStyle(name).
		Inherit(parent).
		Build())
}

// EnsureStyle ensures a style exists, resolves it to a generated name, and marks it as used.
// Returns the generated style name (like "s1J") that should be used in the KFX output.
// This is the primary method for single-style usage without merging.
func (sr *StyleRegistry) EnsureStyle(name string) string {
	return sr.MarkUsage(name, styleUsageText)
}

// EnsureStyleNoMark resolves a style name to a generated name but does NOT mark it as used.
// This is used when we need to resolve style names during processing but will mark usage later
// (e.g., after style event segmentation that may deduplicate some events).
func (sr *StyleRegistry) EnsureStyleNoMark(name string) string {
	if name == "" {
		return ""
	}
	sr.EnsureBaseStyle(name)

	// Resolve inheritance to get final properties
	def := sr.styles[name]
	resolved := sr.resolveInheritance(def)

	// Check if we already have a generated name for this property set
	sig := styleSignature(resolved.Properties)
	if genName, ok := sr.resolved[sig]; ok {
		return genName
	}

	// Generate a new name and register the resolved style (but don't mark used)
	genName := sr.nextResolvedStyleName()
	sr.resolved[sig] = genName
	sr.Register(StyleDef{Name: genName, Properties: resolved.Properties})
	return genName
}

// MarkUsage ensures a style exists, resolves it to a generated name, and marks usage.
// Returns the generated style name that should be used in output.
func (sr *StyleRegistry) MarkUsage(name string, usage styleUsage) string {
	if name == "" {
		return ""
	}
	sr.EnsureBaseStyle(name)

	// Resolve inheritance to get final properties
	def := sr.styles[name]
	resolved := sr.resolveInheritance(def)

	// Check if we already have a generated name for this property set
	sig := styleSignature(resolved.Properties)
	if genName, ok := sr.resolved[sig]; ok {
		sr.used[genName] = true
		sr.usage[genName] = sr.usage[genName] | usage
		return genName
	}

	// Generate a new name and register the resolved style
	genName := sr.nextResolvedStyleName()
	sr.resolved[sig] = genName
	sr.used[genName] = true
	sr.usage[genName] = usage
	sr.Register(StyleDef{Name: genName, Properties: resolved.Properties})
	return genName
}

func (sr *StyleRegistry) hasTextUsage(name string) bool {
	return sr.usage[name]&styleUsageText != 0
}

func (sr *StyleRegistry) nextResolvedStyleName() string {
	sr.resolvedCounter++
	return "s" + toBase36(sr.resolvedCounter)
}

// containerStyles are CSS classes that represent structural containers in FB2/EPUB.
// These should NOT contribute margins to child content elements, as their margins
// are meant for the container block itself (in EPUB), not inherited content.
var containerStyles = map[string]bool{
	"section":    true,
	"cite":       true,
	"epigraph":   true,
	"poem":       true,
	"stanza":     true,
	"annotation": true,
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

// ResolveStyle resolves a (possibly multi-part) style spec into a fully-resolved KP3-like style name.
// Later parts override earlier ones. All resolved styles inherit from kfx-unknown as the base.
//
// An optional ElementPosition can be provided for position-based property filtering:
//   - First element: margin-top removed
//   - Last element: margin-bottom removed
//   - First+Last (only element): both margins removed
//   - Middle or no position: no filtering
//
// Elements that don't pass position (tables, TOC entries, style events) are treated as "middle"
// because they either don't participate in vertical margin collapsing or handle spacing differently.
func (sr *StyleRegistry) ResolveStyle(styleSpec string, pos ...ElementPosition) string {
	parts := strings.Fields(styleSpec)
	if len(parts) == 0 {
		return ""
	}

	merged := make(map[KFXSymbol]any)

	// Start with kfx-unknown as the base for all resolved styles
	// This ensures minimal required properties (like line-height: 1lh) are always present
	sr.EnsureBaseStyle("kfx-unknown")
	if unknown, exists := sr.styles["kfx-unknown"]; exists {
		resolved := sr.resolveInheritance(unknown)
		sr.mergeProperties(merged, resolved.Properties)
	}

	// Track margins separately - we may need to use intermediate margins
	// if the final element doesn't define any.
	var lastMargins map[KFXSymbol]any

	// Process parts in order: base element first, then context, then specific class.
	// Later parts override earlier ones using stylelist merge rules, so for "p section section-subtitle":
	// 1. p properties are set (including margins)
	// 2. section properties override p (margins filtered - it's a container)
	// 3. section-subtitle properties override section
	//
	// Container classes (section, cite, etc.) have their margins filtered because
	// in CSS, margins don't inherit from containers to children. Title wrappers
	// (body-title, chapter-title) are NOT containers - they're styling wrappers
	// whose margins should propagate to the header content.
	lastIdx := len(parts) - 1
	for i, part := range parts {
		sr.EnsureBaseStyle(part)
		def := sr.styles[part]
		resolved := sr.resolveInheritance(def)

		// Filter margins from container classes (intermediate or final)
		isContainer := containerStyles[part]
		if i > 0 && isContainer {
			// DON'T save container margins - they should never propagate
			// Merge non-margin properties from containers
			for k, v := range resolved.Properties {
				if k == SymMarginTop || k == SymMarginBottom || k == SymMarginLeft || k == SymMarginRight {
					continue
				}
				sr.mergeProperty(merged, k, v)
			}
		} else if i > 0 && i < lastIdx {
			// Non-container intermediate: track margins for potential use by final element
			for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
				if v, ok := resolved.Properties[sym]; ok {
					if lastMargins == nil {
						lastMargins = make(map[KFXSymbol]any)
					}
					sr.mergeProperty(lastMargins, sym, v)
				}
			}
			// Merge non-margin properties
			for k, v := range resolved.Properties {
				if k == SymMarginTop || k == SymMarginBottom || k == SymMarginLeft || k == SymMarginRight {
					continue
				}
				sr.mergeProperty(merged, k, v)
			}
		} else {
			for k, v := range resolved.Properties {
				sr.mergeProperty(merged, k, v)
			}
		}
	}

	// If the final result has no margins but intermediate parts did, use those
	// This allows title wrappers (body-title, etc.) to provide margins to headers
	if lastMargins != nil {
		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
			if _, hasMargin := merged[sym]; !hasMargin {
				if v, ok := lastMargins[sym]; ok {
					sr.mergeProperty(merged, sym, v)
				}
			}
		}
	}

	// Apply position-based filtering if position was provided
	var position ElementPosition
	var hasPosition bool
	if len(pos) > 0 {
		position = pos[0]
		hasPosition = position.IsFirst || position.IsLast
	}

	if hasPosition {
		filtered, removedProps := filterPropertiesByPositionWithRemoved(merged, position)
		if sr.tracer.IsEnabled() && len(removedProps) > 0 {
			var removedNames []string
			for _, sym := range removedProps {
				removedNames = append(removedNames, traceSymbolNameForStyle(sym))
			}
			posName := positionSuffix(position)[2 : len(positionSuffix(position))-1]
			sr.tracer.TracePositionFilter(styleSpec, posName, removedNames)
		}
		merged = filtered
	}

	// For table styles, remove properties that KFX requires to be on the element, not in the style.
	// KP3 moves these from style to element during table processing.
	if parts[0] == "table" {
		for prop := range tableElementProperties {
			delete(merged, prop)
		}
	}

	sig := styleSignature(merged)
	if name, ok := sr.resolved[sig]; ok {
		sr.used[name] = true
		suffix := ""
		if hasPosition {
			suffix = positionSuffix(position)
		}
		sr.tracer.TraceResolve(styleSpec+suffix, name+" (cached)", merged)
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	sr.used[name] = true
	sr.Register(StyleDef{Name: name, Properties: merged})
	suffix := ""
	if hasPosition {
		suffix = positionSuffix(position)
	}
	sr.tracer.TraceResolve(styleSpec+suffix, name, merged)
	return name
}

// ResolveStyleNoMark resolves a style spec into a generated name but does NOT mark it as used.
// This is used when style names are needed during processing but usage will be marked later
// (e.g., after style event segmentation that may deduplicate some events).
func (sr *StyleRegistry) ResolveStyleNoMark(styleSpec string) string {
	parts := strings.Fields(styleSpec)
	if len(parts) == 0 {
		return ""
	}

	merged := make(map[KFXSymbol]any)

	sr.EnsureBaseStyle("kfx-unknown")
	if unknown, exists := sr.styles["kfx-unknown"]; exists {
		resolved := sr.resolveInheritance(unknown)
		sr.mergeProperties(merged, resolved.Properties)
	}

	var lastMargins map[KFXSymbol]any

	lastIdx := len(parts) - 1
	for i, part := range parts {
		sr.EnsureBaseStyle(part)
		def := sr.styles[part]
		resolved := sr.resolveInheritance(def)

		isContainer := containerStyles[part]
		if i > 0 && isContainer {
			for k, v := range resolved.Properties {
				if k == SymMarginTop || k == SymMarginBottom || k == SymMarginLeft || k == SymMarginRight {
					continue
				}
				sr.mergeProperty(merged, k, v)
			}
		} else if i > 0 && i < lastIdx {
			for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
				if v, ok := resolved.Properties[sym]; ok {
					if lastMargins == nil {
						lastMargins = make(map[KFXSymbol]any)
					}
					sr.mergeProperty(lastMargins, sym, v)
				}
			}
			for k, v := range resolved.Properties {
				if k == SymMarginTop || k == SymMarginBottom || k == SymMarginLeft || k == SymMarginRight {
					continue
				}
				sr.mergeProperty(merged, k, v)
			}
		} else {
			for k, v := range resolved.Properties {
				sr.mergeProperty(merged, k, v)
			}
		}
	}

	if lastMargins != nil {
		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
			if _, hasMargin := merged[sym]; !hasMargin {
				if v, ok := lastMargins[sym]; ok {
					sr.mergeProperty(merged, sym, v)
				}
			}
		}
	}

	// For table styles, remove properties that KFX requires to be on the element, not in the style.
	if parts[0] == "table" {
		for prop := range tableElementProperties {
			delete(merged, prop)
		}
	}

	sig := styleSignature(merged)
	if name, ok := sr.resolved[sig]; ok {
		// Don't mark used - caller will do that if needed
		sr.tracer.TraceResolve(styleSpec, name+" (cached, no-mark)", merged)
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	// Don't mark used - caller will do that if needed
	sr.Register(StyleDef{Name: name, Properties: merged})
	sr.tracer.TraceResolve(styleSpec, name+" (no-mark)", merged)
	return name
}

// positionSuffix returns a descriptive suffix for tracing position-aware resolution.
func positionSuffix(pos ElementPosition) string {
	switch {
	case pos.IsFirst && pos.IsLast:
		return " [first+last]"
	case pos.IsFirst:
		return " [first]"
	case pos.IsLast:
		return " [last]"
	default:
		return " [middle]"
	}
}

// RegisterResolved takes a merged property map, generates a unique style name,
// registers the style, and returns the name. This is used by StyleContext.Resolve
// to register styles built with proper CSS inheritance rules.
// All resolved styles inherit from kfx-unknown as the base.
func (sr *StyleRegistry) RegisterResolved(props map[KFXSymbol]any) string {
	// Start with kfx-unknown as the base, then overlay provided props
	merged := make(map[KFXSymbol]any)
	sr.EnsureBaseStyle("kfx-unknown")
	if unknown, exists := sr.styles["kfx-unknown"]; exists {
		resolved := sr.resolveInheritance(unknown)
		sr.mergeProperties(merged, resolved.Properties)
	}
	sr.mergeProperties(merged, props) // Provided props override kfx-unknown

	sig := styleSignature(merged)
	if name, ok := sr.resolved[sig]; ok {
		sr.used[name] = true
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	sr.used[name] = true
	sr.Register(StyleDef{Name: name, Properties: merged})
	return name
}

// ResolveImageStyle creates a KP3-compatible image style with specific width percentage.
// KP3 calculates image width as percentage of screen width and creates unique styles
// for each distinct width value. This produces styles like:
//
//	.sXX { box-align: center; line-height: 1lh; width: 84.766%; }
//
// screenWidth is the target screen width (e.g., 1280 for Kindle).
func (sr *StyleRegistry) ResolveImageStyle(imageWidth, screenWidth int) string {
	if screenWidth <= 0 {
		screenWidth = 1280 // Default Kindle screen width
	}

	// Calculate width percentage (KP3 uses 3 decimal places)
	widthPercent := float64(imageWidth) / float64(screenWidth) * 100
	return sr.ResolveImagePercentStyle(widthPercent)
}

// ResolveImagePercentStyle creates a KP3-compatible image style with explicit width percent.
func (sr *StyleRegistry) ResolveImagePercentStyle(widthPercent float64) string {
	if widthPercent > 100 {
		widthPercent = 100
	}
	if widthPercent < 0 {
		widthPercent = 0
	}

	props := map[KFXSymbol]any{
		SymBoxAlign:     SymbolValue(SymCenter),                       // box-align: center
		SymSizingBounds: SymbolValue(SymContentBounds),                // sizing_bounds: content_bounds
		SymWidth:        DimensionValue(widthPercent, SymUnitPercent), // width: XX.XXX%
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

// ResolveBlockImageStyle creates a KP3-compatible image style that inherits block-level properties.
// This is used for images that are the sole content of a block element (e.g., subtitle, title).
// The resulting style combines:
//   - Block-level properties from the parent style (margins, break properties, font-weight)
//   - Image-specific properties (width, box-align, baseline-style)
//
// Properties that don't apply to images (like text-indent, text-align) are filtered out.
// text-align: center becomes box-align: center for images.
func (sr *StyleRegistry) ResolveBlockImageStyle(imageWidth, screenWidth int, blockStyle string) string {
	if screenWidth <= 0 {
		screenWidth = 1280
	}

	// Calculate width percentage
	widthPercent := float64(imageWidth) / float64(screenWidth) * 100
	if widthPercent > 100 {
		widthPercent = 100
	}
	if widthPercent < 0 {
		widthPercent = 0
	}

	// Start with block style properties
	props := make(map[KFXSymbol]any)

	// Resolve the block style to get its properties
	if blockStyle != "" {
		for part := range strings.FieldsSeq(blockStyle) {
			sr.EnsureBaseStyle(part)
			if def, exists := sr.styles[part]; exists {
				resolved := sr.resolveInheritance(def)
				for k, v := range resolved.Properties {
					// Filter out properties that don't apply to images
					switch k {
					case SymTextIndent, SymTextAlignment, SymLineHeight:
						// Skip text-specific properties
						continue
					}
					props[k] = v
				}
			}
		}
	}

	// Add/override with image-specific properties
	props[SymBaselineStyle] = SymbolValue(SymCenter)               // baseline-style: center
	props[SymBoxAlign] = SymbolValue(SymCenter)                    // box-align: center
	props[SymWidth] = DimensionValue(widthPercent, SymUnitPercent) // width: XX.XXX%

	// Ensure line-height is present (KP3 requires it)
	props[SymLineHeight] = DimensionValue(1, SymUnitLh) // line-height: 1lh

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

// ResolveInlineImageStyle creates a KP3-compatible inline image style.
// Inline images (images embedded within text paragraphs) use different properties than
// block images. KP3 uses:
//   - baseline-style: center ($44 = $320) - vertical alignment within text
//   - width: X.XXXem ($56 with unit $308) - width in em units
//   - height: X.XXXem ($57 with unit $308) - height in em units
//
// The pixel dimensions are converted to em using 16px as the base (standard browser default).
// Example: 110x23 pixels → width: 6.875em, height: 1.4375em
//
// The caller should also set render: inline ($601 = $283) on the image content entry.
func (sr *StyleRegistry) ResolveInlineImageStyle(imageWidth, imageHeight int) string {
	const baseFontSizePx = 16.0 // Standard em base size

	// Convert pixel dimensions to em (using 16px base)
	widthEm := float64(imageWidth) / baseFontSizePx
	heightEm := float64(imageHeight) / baseFontSizePx

	props := map[KFXSymbol]any{
		SymBaselineStyle: SymbolValue(SymCenter),              // baseline-style: center
		SymWidth:         DimensionValue(widthEm, SymUnitEm),  // width in em
		SymHeight:        DimensionValue(heightEm, SymUnitEm), // height in em
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

// ResolveVignetteImageStyle creates the standard vignette image style (100% width)
// with KP3-compatible top margin.
func (sr *StyleRegistry) ResolveVignetteImageStyle() string {
	return sr.ResolveVignetteImageStyleWithPosition(PositionMiddle())
}

// ResolveVignetteImageStyleWithPosition creates a vignette image style with position filtering.
// First element: margin-top removed; Last element: margin-bottom removed (none present anyway).
// This matches KP3's position-based property filtering.
func (sr *StyleRegistry) ResolveVignetteImageStyleWithPosition(pos ElementPosition) string {
	props := map[KFXSymbol]any{
		SymBoxAlign:     SymbolValue(SymCenter),
		SymSizingBounds: SymbolValue(SymContentBounds),
		SymWidth:        DimensionValue(100, SymUnitPercent),
		SymMarginTop:    DimensionValue(0.697917, SymUnitLh),
	}

	// Apply position-based filtering
	props = FilterPropertiesByPosition(props, pos)

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

// ResolveCoverImageStyle creates a minimal style for cover images in container-type sections.
// Unlike ResolveImageStyle, this doesn't include width constraints since the page template
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
// Unknown styles inherit from "kfx-unknown" (minimal empty style) to avoid unwanted formatting.
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

	// Default to "kfx-unknown" (minimal style) for unknown styles
	// This avoids polluting unknown styles with "p" properties like text-align: justify
	if _, exists := sr.styles["kfx-unknown"]; exists {
		return "kfx-unknown"
	}

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
		if sr.hasTextUsage(name) {
			resolved.Properties = ensureDefaultLineHeight(resolved.Properties)
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
func (sr *StyleRegistry) resolveInheritance(def StyleDef) StyleDef {
	if def.Parent == "" {
		return def
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

	// Seed structural wrappers via stylemap defaults so wrapper classes
	// match KP3 baseline even before user CSS overrides.
	if wrapperStyles, wrapperWarnings := mapDefaultWrapperStyles(mapper); len(wrapperStyles) > 0 {
		sr.RegisterFromCSS(wrapperStyles)
		warnings = append(warnings, wrapperWarnings...)
	}
	if structuralStyles, structuralWarnings := mapStructuralWrappers(mapper); len(structuralStyles) > 0 {
		sr.RegisterFromCSS(structuralStyles)
		warnings = append(warnings, structuralWarnings...)
	}

	// Apply inferred parent relationships based on naming conventions
	// This must happen after all styles are registered but before post-processing
	sr.ApplyInferredParents()

	// Apply KFX-specific post-processing (layout-hints, yj-break, etc.)
	sr.PostProcessForKFX()

	return sr, warnings
}

// DefaultStyleRegistry returns a registry with default HTML element styles for KFX.
// This only includes HTML element selectors (p, h1-h6, code, blockquote, etc.)
// and basic inline styles (strong, em, sub, sup). Class selectors come from CSS.
//
// KFX-specific properties like layout-hints are applied during post-processing,
// not here, to allow CSS to override base styles first.
func DefaultStyleRegistry() *StyleRegistry {
	sr := NewStyleRegistry()

	// ============================================================
	// Minimal fallback style for unknown classes
	// ============================================================

	// "kfx-unknown" is a catch-all base style for classes not defined in CSS.
	// It has minimal properties to avoid polluting derived styles
	// with unwanted formatting (unlike "p" which has text-align: justify, margins, etc.)
	// LineHeight is required to ensure proper text rendering.
	sr.Register(NewStyle("kfx-unknown").
		LineHeight(1.0, SymUnitLh).
		Build())

	// ============================================================
	// Block-level HTML elements
	// ============================================================

	// Base paragraph style - HTML <p> element
	// Amazon reference: only sets display: block (no styling defaults)
	// FB2-specific formatting (text-indent, justify, margins) comes from CSS
	sr.Register(NewStyle("p").
		Build())

	// Heading styles (h1-h6) - HTML heading elements
	// Amazon Java reference: font-size and font-weight only, no text-align
	// Font sizes: h1=2.0em, h2=1.5em, h3=1.17em, h4=1.0em, h5=0.83em, h6=0.67em
	// layout-hints added during post-processing
	sr.Register(NewStyle("h1").
		FontSize(2.0, SymUnitEm).
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("h2").
		FontSize(1.5, SymUnitEm).
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("h3").
		FontSize(1.17, SymUnitEm).
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("h4").
		FontSize(1.0, SymUnitEm).
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("h5").
		FontSize(0.83, SymUnitEm).
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("h6").
		FontSize(0.67, SymUnitEm).
		FontWeight(SymBold).
		Build())

	// Code/preformatted - HTML <code> and <pre> elements
	// Amazon reference for code: font-family: monospace only
	sr.Register(NewStyle("code").
		FontFamily("monospace").
		Build())

	// Amazon reference for pre: font-family: monospace, white-space: pre
	// Note: white-space is handled at content level, not in style
	sr.Register(NewStyle("pre").
		FontFamily("monospace").
		Build())

	// Blockquote - HTML <blockquote> element
	// Amazon reference: margin-left: 40px, margin-right: 40px only
	sr.Register(NewStyle("blockquote").
		MarginLeft(40, SymUnitPx).
		MarginRight(40, SymUnitPx).
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
		MarginTop(0.833, SymUnitLh).
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
	// IMPORTANT: Use rem (not em) to prevent relative merging when nested with
	// other inline styles like link-footnote. Using em causes compounding:
	// sup(0.75em) × link-footnote(0.8em) = 0.6em, which is wrong.
	sr.Register(NewStyle("sub").
		BaselineStyle(SymSubscript).
		FontSize(0.75, SymUnitRem).
		Build())

	sr.Register(NewStyle("sup").
		BaselineStyle(SymSuperscript).
		FontSize(0.75, SymUnitRem).
		Build())

	// Heading-context sub/sup: When sub/sup appears in headings (h1-h6), we apply
	// only the baseline-style without font-size reduction. This matches KP3 behavior
	// where title paragraphs wrapped in <sub>/<sup> render at full title size with
	// just the vertical alignment applied.
	for i := 1; i <= 6; i++ {
		hTag := fmt.Sprintf("h%d", i)
		sr.Register(NewStyle(hTag + "--sub").
			BaselineStyle(SymSubscript).
			Build())
		sr.Register(NewStyle(hTag + "--sup").
			BaselineStyle(SymSuperscript).
			Build())
	}

	// Small text - HTML <small> element
	// Amazon reference: font-size: smaller
	sr.Register(NewStyle("small").
		FontSizeSmaller().
		Build())

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

	// Image container style
	sr.Register(NewStyle("image").
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	return sr
}

// ApplyInferredParents sets up parent relationships for styles based on naming conventions.
// This enables inheritance for suffix variants like "chapter-title-header-first" to inherit
// from "chapter-title-header". Must be called after all styles are registered but before
// any style resolution occurs.
//
// Naming conventions:
//   - "-first", "-next", "-break" suffixes inherit from their base style
//     e.g., "chapter-title-header-first" -> "chapter-title-header"
func (sr *StyleRegistry) ApplyInferredParents() {
	for name, def := range sr.styles {
		// Skip styles that already have an explicit parent
		if def.Parent != "" {
			continue
		}

		// Try to infer parent from naming conventions
		parent := sr.inferParentStyle(name)
		if parent == "" {
			continue
		}

		// Update the style with the inferred parent
		sr.styles[name] = StyleDef{
			Name:       def.Name,
			Parent:     parent,
			Properties: def.Properties,
		}
		sr.tracer.TraceInheritSetup(name, parent)
	}
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
		// KP3 reference: title text styles have margin-top but NOT margin-bottom.
		// The margin-bottom is only on the wrapper container, not the text inside.
		delete(props, SymMarginBottom)
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

	// Note: box_align is NOT used for title wrappers.
	// Reference KFX files rely on text_alignment: center on the content text itself,
	// not box_align on the wrapper container.

	return StyleDef{
		Name:       def.Name,
		Parent:     def.Parent,
		Properties: props,
	}
}

// shouldHaveLayoutHintTitle determines if a style should have layout-hints: [treat_as_title].
// This applies to:
//   - HTML heading elements (h1-h6)
//   - Styles ending with "-title-header" (body-title-header, chapter-title-header, etc.)
//   - Simple title styles for generated sections (annotation-title, toc-title, footnote-title)
//   - Styles named "subtitle" or with "-subtitle" suffix (if centered)
//
// NOTE: Styles with additional suffixes like "-title-header-first", "-title-header-next",
// "-title-header-break", "-title-header-emptyline" should NOT get layout-hints because
// they are used in style_events ($142), not as direct content styles ($157).
// KP3 reference shows layout-hints only on the direct content style, not on style_events styles.
func (sr *StyleRegistry) shouldHaveLayoutHintTitle(name string, props map[KFXSymbol]any) bool {
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

	// Subtitle styles - only apply to centered, bold subtitles
	if name == "subtitle" || strings.HasSuffix(name, "-subtitle") {
		// Check if it's centered (cite-subtitle is left-aligned and shouldn't get layout-hint)
		if align, ok := props[SymTextAlignment]; ok {
			if align == SymbolValue(SymCenter) {
				return true
			}
		}
	}

	return false
}

// shouldHaveBreakInsideAvoid determines if a style should have break-inside: avoid.
// This applies to title wrapper styles to keep titles together.
func (sr *StyleRegistry) shouldHaveBreakInsideAvoid(name string, _ map[KFXSymbol]any) bool {
	// Title wrapper styles
	switch name {
	case "body-title", "chapter-title", "section-title":
		return true
	}
	// Other *-title styles but not *-title-header (those are inline)
	if strings.HasSuffix(name, "-title") && !strings.HasSuffix(name, "-title-header") {
		return true
	}
	return false
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
}
