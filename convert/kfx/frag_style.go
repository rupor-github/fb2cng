package kfx

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
	"go.uber.org/zap"
)

// Style fragment ($157) generation for KFX.
// Styles define formatting properties applied to content elements.
// Each style is referenced by name (symbol) in content entries.
//
// KPV-compatible approach: Generate unique styles for each unique property combination.
// Style names are short auto-generated identifiers like "s1", "s2", etc.
// This matches how Kindle Previewer generates styles from EPUB.

// StyleDef defines a KFX style with its properties.
type StyleDef struct {
	Name       string            // Style name (becomes local symbol)
	Parent     string            // Parent style name (for inheritance)
	Properties map[KFXSymbol]any // KFX property symbol -> value
}

// DimensionValue creates a KPV-compatible dimension value with unit.
// Example: DimensionValue(1.2, SymUnitRatio) -> {$307: "1.2", $306: $310}
func DimensionValue(value float64, unit KFXSymbol) StructValue {
	// KPV uses Ion decimals (not strings) for $307.
	dec := ion.MustParseDecimal(formatKPVNumber(value))
	return NewStruct().
		Set(SymValue, dec). // $307 = Ion decimal
		SetSymbol(SymUnit, unit)
}

// DimensionValueKPV creates a KPV-compatible dimension value.
// Uses string representation for the value to match KPV output format.
// KPV uses formats like "3.125" for percent and "2.5d-1" for 0.25lh.
func DimensionValueKPV(value float64, unit KFXSymbol) StructValue {
	return DimensionValue(value, unit)
}

// formatKPVNumber formats a number in KPV's style.
// KPV uses scientific notation "d-1" for small decimals.
func formatKPVNumber(v float64) string {
	if v == 0 {
		return "0."
	}
	if v == 1 {
		return "1."
	}

	// KPV generally emits integers with a trailing dot (e.g. "100.").
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d.", int64(v))
	}

	// KPV typically uses "d" scientific notation for values < 1 (e.g. 0.25 -> "2.5d-1").
	av := v
	if av < 0 {
		av = -av
	}
	if av > 0 && av < 1 {
		exp := 0
		mant := v
		for {
			am := mant
			if am < 0 {
				am = -am
			}
			if am >= 1 {
				break
			}
			mant *= 10
			exp--
			if exp < -12 {
				break
			}
		}
		return fmt.Sprintf("%gd%d", mant, exp)
	}

	return fmt.Sprintf("%g", v)
}

func styleSignature(props map[KFXSymbol]any) string {
	keys := make([]int, 0, len(props))
	for k := range props {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	var b strings.Builder
	for _, ki := range keys {
		k := KFXSymbol(ki)
		b.WriteString(strconv.Itoa(ki))
		b.WriteByte('=')
		b.WriteString(encodeStyleValue(props[k]))
		b.WriteByte(';')
	}
	return b.String()
}

func encodeStyleValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "n"
	case string:
		return "s:" + x
	case bool:
		if x {
			return "b:1"
		}
		return "b:0"
	case int:
		return "i:" + strconv.FormatInt(int64(x), 10)
	case int64:
		return "i:" + strconv.FormatInt(x, 10)
	case float64:
		return "f:" + strconv.FormatFloat(x, 'g', -1, 64)
	case *ion.Decimal:
		return "dec:" + x.String()
	case KFXSymbol:
		return "sym:" + strconv.Itoa(int(x))
	case SymbolValue:
		return "sym:" + strconv.Itoa(int(x))
	case SymbolByNameValue:
		return "symname:" + string(x)
	case StructValue:
		keys := make([]int, 0, len(x))
		for k := range x {
			keys = append(keys, int(k))
		}
		sort.Ints(keys)
		var b strings.Builder
		b.WriteString("{")
		for _, ki := range keys {
			k := KFXSymbol(ki)
			b.WriteString(strconv.Itoa(ki))
			b.WriteByte(':')
			b.WriteString(encodeStyleValue(x[k]))
			b.WriteByte(',')
		}
		b.WriteString("}")
		return b.String()
	default:
		return fmt.Sprintf("%T:%v", v, v)
	}
}

// StyleBuilder helps construct style definitions.
type StyleBuilder struct {
	name   string
	parent string
	props  map[KFXSymbol]any
}

// NewStyle creates a new style builder.
func NewStyle(name string) *StyleBuilder {
	return &StyleBuilder{
		name:  name,
		props: make(map[KFXSymbol]any),
	}
}

// Inherit sets the parent style for inheritance.
func (sb *StyleBuilder) Inherit(parentName string) *StyleBuilder {
	sb.parent = parentName
	return sb
}

// FontSize sets the font size.
func (sb *StyleBuilder) FontSize(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymFontSize] = DimensionValue(value, unit)
	return sb
}

// FontWeight sets the font weight (SymBold, SymNormal, etc.).
func (sb *StyleBuilder) FontWeight(weight KFXSymbol) *StyleBuilder {
	sb.props[SymFontWeight] = SymbolValue(weight)
	return sb
}

// FontStyle sets the font style (SymItalic, SymNormal, etc.).
func (sb *StyleBuilder) FontStyle(style KFXSymbol) *StyleBuilder {
	sb.props[SymFontStyle] = SymbolValue(style)
	return sb
}

// LineHeight sets the line height.
func (sb *StyleBuilder) LineHeight(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymLineHeight] = DimensionValue(value, unit)
	return sb
}

// TextAlign sets the text alignment.
func (sb *StyleBuilder) TextAlign(align KFXSymbol) *StyleBuilder {
	sb.props[SymTextAlignment] = SymbolValue(align)
	return sb
}

// TextIndent sets the first-line text indent.
func (sb *StyleBuilder) TextIndent(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymTextIndent] = DimensionValue(value, unit)
	return sb
}

// MarginTop sets the top margin.
func (sb *StyleBuilder) MarginTop(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMarginTop] = DimensionValue(value, unit)
	return sb
}

// MarginBottom sets the bottom margin.
func (sb *StyleBuilder) MarginBottom(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMarginBottom] = DimensionValue(value, unit)
	return sb
}

// MarginLeft sets the left margin.
func (sb *StyleBuilder) MarginLeft(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMarginLeft] = DimensionValue(value, unit)
	return sb
}

// MarginRight sets the right margin.
func (sb *StyleBuilder) MarginRight(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMarginRight] = DimensionValue(value, unit)
	return sb
}

// SpaceBefore sets space before the element.
func (sb *StyleBuilder) SpaceBefore(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymSpaceBefore] = DimensionValue(value, unit)
	return sb
}

// SpaceAfter sets space after the element.
func (sb *StyleBuilder) SpaceAfter(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymSpaceAfter] = DimensionValue(value, unit)
	return sb
}

// BaselineStyle sets the baseline style (superscript, subscript).
func (sb *StyleBuilder) BaselineStyle(style KFXSymbol) *StyleBuilder {
	sb.props[SymBaselineStyle] = SymbolValue(style)
	return sb
}

// BaselineShift sets the baseline shift value.
func (sb *StyleBuilder) BaselineShift(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymBaselineShift] = DimensionValue(value, unit)
	return sb
}

// FontFamily sets the font family.
func (sb *StyleBuilder) FontFamily(family string) *StyleBuilder {
	sb.props[SymFontFamily] = family
	return sb
}

// Underline sets the underline property.
func (sb *StyleBuilder) Underline(enabled bool) *StyleBuilder {
	sb.props[SymUnderline] = enabled
	return sb
}

// Strikethrough sets the strikethrough property.
func (sb *StyleBuilder) Strikethrough(enabled bool) *StyleBuilder {
	sb.props[SymStrikethrough] = enabled
	return sb
}

// Render sets the render mode (block, inline).
func (sb *StyleBuilder) Render(mode KFXSymbol) *StyleBuilder {
	sb.props[SymRender] = SymbolValue(mode)
	return sb
}

// Width sets the width property.
func (sb *StyleBuilder) Width(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymWidth] = DimensionValue(value, unit)
	return sb
}

// KeepTogether sets orphans/widows to avoid.
func (sb *StyleBuilder) KeepTogether() *StyleBuilder {
	sb.props[SymKeepFirst] = SymbolValue(SymAvoid)
	sb.props[SymKeepLast] = SymbolValue(SymAvoid)
	return sb
}

// BreakInsideAvoid sets break-inside to avoid (KPV style).
func (sb *StyleBuilder) BreakInsideAvoid() *StyleBuilder {
	sb.props[SymBreakInside] = SymbolValue(SymAvoid)
	return sb
}

// YjBreakBefore sets yj-break-before property for KPV compatibility.
func (sb *StyleBuilder) YjBreakBefore(mode KFXSymbol) *StyleBuilder {
	sb.props[SymYjBreakBefore] = SymbolValue(mode)
	return sb
}

// YjBreakAfter sets yj-break-after property for KPV compatibility.
func (sb *StyleBuilder) YjBreakAfter(mode KFXSymbol) *StyleBuilder {
	sb.props[SymYjBreakAfter] = SymbolValue(mode)
	return sb
}

// Dropcap sets drop cap properties.
func (sb *StyleBuilder) Dropcap(chars, lines int) *StyleBuilder {
	sb.props[SymDropcapChars] = chars
	sb.props[SymDropcapLines] = lines
	return sb
}

// MarginLeftAuto sets left margin to auto (for centering).
func (sb *StyleBuilder) MarginLeftAuto() *StyleBuilder {
	sb.props[SymMarginLeft] = SymbolValue(SymAuto)
	return sb
}

// MarginRightAuto sets right margin to auto (for centering).
func (sb *StyleBuilder) MarginRightAuto() *StyleBuilder {
	sb.props[SymMarginRight] = SymbolValue(SymAuto)
	return sb
}

// MarginLeftPercent sets left margin in percentage.
func (sb *StyleBuilder) MarginLeftPercent(value float64) *StyleBuilder {
	sb.props[SymMarginLeft] = DimensionValue(value, SymUnitPercent)
	return sb
}

// MarginRightPercent sets right margin in percentage.
func (sb *StyleBuilder) MarginRightPercent(value float64) *StyleBuilder {
	sb.props[SymMarginRight] = DimensionValue(value, SymUnitPercent)
	return sb
}

// LayoutHintTitle sets layout-hints to [treat_as_title] for KPV compatibility.
// This is critical for proper title rendering on Kindle devices.
func (sb *StyleBuilder) LayoutHintTitle() *StyleBuilder {
	sb.props[SymLayoutHints] = []any{SymbolValue(SymTreatAsTitle)}
	return sb
}

// BoxAlign sets the box_align property for block-level centering.
// Use SymCenter for centering blocks within their container.
func (sb *StyleBuilder) BoxAlign(align KFXSymbol) *StyleBuilder {
	sb.props[SymBoxAlign] = SymbolValue(align)
	return sb
}

// SizingBounds sets the sizing_bounds property.
// Use SymContentBounds for content-based sizing.
func (sb *StyleBuilder) SizingBounds(bounds KFXSymbol) *StyleBuilder {
	sb.props[SymSizingBounds] = SymbolValue(bounds)
	return sb
}

// Build creates the StyleDef.
func (sb *StyleBuilder) Build() StyleDef {
	return StyleDef{
		Name:       sb.name,
		Parent:     sb.parent,
		Properties: sb.props,
	}
}

// BuildStyle creates a $157 style fragment from a StyleDef.
// Fragment naming uses the actual style name from the stylesheet (e.g., "body", "emphasis").
// Unlike other fragment types, style names are semantic identifiers from the source document
// rather than auto-generated sequential names, preserving the original style semantics.
func BuildStyle(def StyleDef) *Fragment {
	style := NewStruct().
		Set(SymStyleName, SymbolByName(def.Name)) // $173 = style_name as symbol

	// TODO: parent_style ($158) is valid but not commonly used in KFX files
	// generated by Kindle Previewer. For compatibility, we skip it.

	// Add all properties
	for sym, val := range def.Properties {
		style.Set(sym, val)
	}

	return &Fragment{
		FType:   SymStyle,
		FIDName: def.Name,
		Value:   style,
	}
}

// StyleRegistry manages style definitions and generates style fragments.
type StyleRegistry struct {
	styles map[string]StyleDef
	order  []string        // Preserve insertion order
	used   map[string]bool // Track which styles are actually used (for BuildFragments)

	resolved        map[string]string // signature -> resolved style name
	resolvedCounter int

	tracer *StyleTracer // Optional tracer for debugging style resolution
}

// NewStyleRegistry creates a new style registry.
func NewStyleRegistry() *StyleRegistry {
	return &StyleRegistry{
		styles:   make(map[string]StyleDef),
		used:     make(map[string]bool),
		resolved: make(map[string]string),
		// Start from 55 so the first style is "s1J" (base36(55) == 1J), like KPV samples.
		resolvedCounter: 54,
	}
}

// SetTracer sets the style tracer for debugging.
func (sr *StyleRegistry) SetTracer(t *StyleTracer) {
	sr.tracer = t
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

	// Merge properties: existing properties are preserved, new ones override
	merged := make(map[KFXSymbol]any, len(existing.Properties)+len(def.Properties))
	maps.Copy(merged, existing.Properties)
	maps.Copy(merged, def.Properties)

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
// This is used for resolving style combinations into KPV-like "s.." styles.
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

// EnsureStyle ensures a style exists and marks it as used for output.
// This is used for tests and for any code paths that want to reference a style
// by its semantic name.
func (sr *StyleRegistry) EnsureStyle(name string) {
	sr.used[name] = true
	sr.EnsureBaseStyle(name)
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

// ResolveStyle resolves a (possibly multi-part) style spec into a fully-resolved KPV-like style name.
// Later parts override earlier ones.
func (sr *StyleRegistry) ResolveStyle(styleSpec string) string {
	parts := strings.Fields(styleSpec)
	if len(parts) == 0 {
		return ""
	}

	merged := make(map[KFXSymbol]any)
	// Track margins separately - we may need to use intermediate margins
	// if the final element doesn't define any.
	var lastMargins map[KFXSymbol]any

	// Process parts in order: base element first, then context, then specific class.
	// Later parts override earlier ones (via maps.Copy), so for "p section section-subtitle":
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
				merged[k] = v
			}
		} else if i > 0 && i < lastIdx {
			// Non-container intermediate: track margins for potential use by final element
			for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
				if v, ok := resolved.Properties[sym]; ok {
					if lastMargins == nil {
						lastMargins = make(map[KFXSymbol]any)
					}
					lastMargins[sym] = v
				}
			}
			// Merge non-margin properties
			for k, v := range resolved.Properties {
				if k == SymMarginTop || k == SymMarginBottom || k == SymMarginLeft || k == SymMarginRight {
					continue
				}
				merged[k] = v
			}
		} else {
			maps.Copy(merged, resolved.Properties)
		}
	}

	// If the final result has no margins but intermediate parts did, use those
	// This allows title wrappers (body-title, etc.) to provide margins to headers
	if lastMargins != nil {
		for _, sym := range []KFXSymbol{SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight} {
			if _, hasMargin := merged[sym]; !hasMargin {
				if v, ok := lastMargins[sym]; ok {
					merged[sym] = v
				}
			}
		}
	}

	sig := styleSignature(merged)
	if name, ok := sr.resolved[sig]; ok {
		sr.used[name] = true
		sr.tracer.TraceResolve(styleSpec, name+" (cached)", merged)
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	sr.used[name] = true
	sr.Register(StyleDef{Name: name, Properties: merged})
	sr.tracer.TraceResolve(styleSpec, name, merged)
	return name
}

// RegisterResolved takes a merged property map, generates a unique style name,
// registers the style, and returns the name. This is used by StyleContext.Resolve
// to register styles built with proper CSS inheritance rules.
func (sr *StyleRegistry) RegisterResolved(props map[KFXSymbol]any) string {
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

// ResolveImageStyle creates a KPV-compatible image style with specific width percentage.
// KPV calculates image width as percentage of screen width and creates unique styles
// for each distinct width value. This produces styles like:
//
//	.sXX { box-align: center; line-height: 1lh; width: 84.766%; }
//
// screenWidth is the target screen width (e.g., 1280 for Kindle).
func (sr *StyleRegistry) ResolveImageStyle(imageWidth, screenWidth int) string {
	if screenWidth <= 0 {
		screenWidth = 1280 // Default Kindle screen width
	}

	// Calculate width percentage (KPV uses 3 decimal places)
	widthPercent := float64(imageWidth) / float64(screenWidth) * 100
	if widthPercent > 100 {
		widthPercent = 100
	}

	// Build properties matching KPV image style output
	props := map[KFXSymbol]any{
		SymBoxAlign:   SymbolValue(SymCenter),                       // box-align: center
		SymLineHeight: DimensionValue(1, SymUnitRatio),              // line-height: 1lh
		SymWidth:      DimensionValue(widthPercent, SymUnitPercent), // width: XX.XXX%
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

// ResolveCoverImageStyle creates a minimal style for cover images in container-type sections.
// Unlike ResolveImageStyle, this doesn't include width constraints since the page template
// already defines the container dimensions. Reference KFX cover image style has:
//   - font_size: 1rem (in unit $505)
//   - line_height: 1.0101lh
func (sr *StyleRegistry) ResolveCoverImageStyle() string {
	// Build minimal properties matching KPV cover image style exactly
	props := map[KFXSymbol]any{
		SymFontSize:   DimensionValue(1, SymUnitRem),     // font-size: 1rem
		SymLineHeight: DimensionValue(1.0101, SymUnitLh), // line-height: 1.0101lh
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

	// Check for suffix patterns: "xxx-subtitle" -> "subtitle", "xxx-title" -> base title style
	suffixes := []string{"-subtitle", "-title", "-header", "-first", "-next", "-break"}
	for _, suffix := range suffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			baseName := suffix[1:] // Remove leading dash
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

	if len(cssData) == 0 {
		return sr, nil
	}

	// Parse CSS
	parser := NewParser(log)
	sheet := parser.Parse(cssData)

	// Convert to KFX styles (includes drop cap detection)
	converter := NewConverter(log)
	converter.SetTracer(tracer)
	styles, warnings := converter.ConvertStylesheet(sheet)

	// Register CSS styles (overriding defaults where applicable)
	sr.RegisterFromCSS(styles)

	// Apply KFX-specific post-processing (layout-hints, yj-break, etc.)
	sr.PostProcessForKFX()

	log.Debug("CSS styles loaded",
		zap.Int("rules", len(sheet.Rules)),
		zap.Int("styles", len(styles)),
		zap.Int("warnings", len(warnings)))

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
	// KPV uses: text-indent: 3.125%, line-height: 1lh, text-align: justify
	sr.Register(NewStyle("p").
		LineHeight(1.0, SymUnitLh).
		TextIndent(3.125, SymUnitPercent).
		TextAlign(SymJustify).
		MarginBottom(0.25, SymUnitLh).
		// Note: margin-top, margin-left, margin-right omitted - KPV doesn't include zero margins
		Build())

	// Heading styles (h1-h6) - HTML heading elements
	// Minimal styles for combining with title header classes
	// Margins and line-height omitted - those come from the header class CSS
	// layout-hints added during post-processing
	sr.Register(NewStyle("h1").
		FontSize(1.5, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("h2").
		FontSize(1.25, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("h3").
		FontSize(1.125, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("h4").
		FontSize(1.0, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("h5").
		FontSize(1.0, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("h6").
		FontSize(1.0, SymUnitRem).
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		Build())

	// Code/preformatted - HTML <code> and <pre> elements
	sr.Register(NewStyle("code").
		FontFamily("monospace").
		FontSize(0.875, SymUnitEm).
		TextAlign(SymLeft).
		TextIndent(0, SymUnitPercent).
		Build())

	sr.Register(NewStyle("pre").
		FontFamily("monospace").
		FontSize(0.875, SymUnitEm).
		TextAlign(SymLeft).
		TextIndent(0, SymUnitPercent).
		LineHeight(1.0, SymUnitLh).
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		Build())

	// Blockquote - HTML <blockquote> element
	sr.Register(NewStyle("blockquote").
		MarginLeft(2.0, SymUnitEm).
		MarginRight(2.0, SymUnitEm).
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		TextIndent(0, SymUnitPercent).
		Build())

	// Table elements - HTML <table>, <th>, <td>
	sr.Register(NewStyle("table").
		TextIndent(0, SymUnitPercent).
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		MarginLeftAuto().
		MarginRightAuto().
		Build())

	sr.Register(NewStyle("th").
		FontWeight(SymBold).
		TextAlign(SymCenter).
		Build())

	sr.Register(NewStyle("td").
		TextAlign(SymLeft).
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
	// KPV uses baseline-style for these
	sr.Register(NewStyle("sub").
		BaselineStyle(SymSubscript).
		FontSize(0.75, SymUnitEm).
		Build())

	sr.Register(NewStyle("sup").
		BaselineStyle(SymSuperscript).
		FontSize(0.75, SymUnitEm).
		Build())

	// Small text - HTML <small> element
	sr.Register(NewStyle("small").
		FontSize(0.875, SymUnitEm).
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
	}

	// Convert page-break properties to KFX yj-break properties
	sr.convertPageBreaksToYjBreaks(name, props)

	// Apply break-inside: avoid for title wrappers
	if sr.shouldHaveBreakInsideAvoid(name, props) {
		if _, exists := props[SymBreakInside]; !exists {
			props[SymBreakInside] = SymbolValue(SymAvoid)
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
//   - Styles with "-title-header" suffix (body-title-header, chapter-title-header, etc.)
//   - Styles named "subtitle" or with "-subtitle" suffix (if centered)
func (sr *StyleRegistry) shouldHaveLayoutHintTitle(name string, props map[KFXSymbol]any) bool {
	// HTML heading elements
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}

	// Title header styles
	if strings.HasSuffix(name, "-title-header") {
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
// The intermediate markers are removed after conversion since KPV doesn't output them.
func (sr *StyleRegistry) convertPageBreaksToYjBreaks(name string, props map[KFXSymbol]any) {
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

	// For title wrappers, ensure yj-break-after: avoid to keep title with content
	if sr.isTitleWrapper(name) {
		if _, exists := props[SymYjBreakAfter]; !exists {
			props[SymYjBreakAfter] = SymbolValue(SymAvoid)
		}
		if _, exists := props[SymYjBreakBefore]; !exists {
			props[SymYjBreakBefore] = SymbolValue(SymAuto)
		}
	}
}

// isTitleWrapper checks if a style name represents a title wrapper element.
func (sr *StyleRegistry) isTitleWrapper(name string) bool {
	switch name {
	case "body-title", "chapter-title", "section-title":
		return true
	}
	return false
}
