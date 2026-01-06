package kfx

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

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

// DimensionValue creates a dimension value with unit.
// Example: DimensionValue(1.2, SymUnitRatio) -> {$307: 1.2, $306: $310}
func DimensionValue(value float64, unit KFXSymbol) StructValue {
	return NewStruct().
		SetFloat(SymValue, value). // $307 = value
		SetSymbol(SymUnit, unit)   // $306 = unit
}

// DimensionValueKPV creates a KPV-compatible dimension value.
// Uses string representation for the value to match KPV output format.
// KPV uses formats like "3.125" for percent and "2.5d-1" for 0.25lh.
func DimensionValueKPV(value float64, unit KFXSymbol) StructValue {
	return NewStruct().
		SetString(SymValue, formatKPVNumber(value)). // $307 = value as string
		SetSymbol(SymUnit, unit)                     // $306 = unit
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
	// For values < 1, KPV often uses d-1 notation (e.g., "2.5d-1" for 0.25)
	// But we'll use simple format for now as it's also valid
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

// Register adds a style to the registry.
// If a style with the same name already exists, the properties are merged,
// with new properties overriding existing ones (CSS cascade behavior).
func (sr *StyleRegistry) Register(def StyleDef) {
	existing, exists := sr.styles[def.Name]
	if !exists {
		sr.order = append(sr.order, def.Name)
		sr.styles[def.Name] = def
		return
	}

	// Merge properties: existing properties are preserved, new ones override
	merged := make(map[KFXSymbol]any, len(existing.Properties)+len(def.Properties))
	for k, v := range existing.Properties {
		merged[k] = v
	}
	for k, v := range def.Properties {
		merged[k] = v
	}

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

// ResolveStyle resolves a (possibly multi-part) style spec into a fully-resolved KPV-like style name.
// Later parts override earlier ones.
func (sr *StyleRegistry) ResolveStyle(styleSpec string) string {
	parts := strings.Fields(styleSpec)
	if len(parts) == 0 {
		return ""
	}

	merged := make(map[KFXSymbol]any)
	for _, part := range parts {
		sr.EnsureBaseStyle(part)
		def := sr.styles[part]
		resolved := sr.resolveInheritance(def)
		maps.Copy(merged, resolved.Properties)
	}

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

// inferParentStyle attempts to determine a parent style based on naming patterns.
// This handles dynamically-created styles like "section-subtitle" -> inherits "subtitle".
func (sr *StyleRegistry) inferParentStyle(name string) string {
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

	// Default to "p" (base paragraph style) for unknown styles
	if _, exists := sr.styles["p"]; exists {
		return "p"
	}

	return ""
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
		for sym, val := range chain[i].Properties {
			merged[sym] = val
		}
	}

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
// It starts with default styles, then overlays styles from the CSS.
// Returns the registry and any warnings from CSS parsing/conversion.
func NewStyleRegistryFromCSS(cssData []byte, log *zap.Logger) (*StyleRegistry, []string) {
	// Start with defaults
	sr := DefaultStyleRegistry()

	if len(cssData) == 0 {
		return sr, nil
	}

	// Parse CSS
	parser := NewParser(log)
	sheet := parser.Parse(cssData)

	// Convert to KFX styles
	converter := NewConverter(log)
	styles, warnings := converter.ConvertStylesheet(sheet)

	// Register CSS styles (overriding defaults where applicable)
	sr.RegisterFromCSS(styles)

	log.Debug("CSS styles loaded",
		zap.Int("rules", len(sheet.Rules)),
		zap.Int("styles", len(styles)),
		zap.Int("warnings", len(warnings)))

	return sr, warnings
}

// DefaultStyleRegistry returns a registry with default FB2-to-KFX styles.
// These styles map FB2 semantic elements to KFX formatting.
// Uses KPV-compatible units:
//   - % (SymUnitPercent) for text-indent
//   - lh (SymUnitLh) for margins (line-height units)
//   - rem (SymUnitRem) for font sizes in inline styles
//   - Always includes line-height: 1lh on block-level styles
func DefaultStyleRegistry() *StyleRegistry {
	sr := NewStyleRegistry()

	// Base paragraph style - matches CSS "p { }" selector
	// KPV uses: text-indent: 3.125%, line-height: 1lh, text-align: justify
	sr.Register(NewStyle("p").
		LineHeight(1.0, SymUnitLh).
		TextIndent(3.125, SymUnitPercent).
		TextAlign(SymJustify).
		MarginTop(0, SymUnitLh).
		MarginBottom(0.25, SymUnitLh).
		MarginLeft(0, SymUnitLh).
		MarginRight(0, SymUnitLh).
		Build())

	// Code style - matches CSS "code { }" selector
	sr.Register(NewStyle("code").
		Inherit("p").
		FontFamily("monospace").
		FontSize(0.7, SymUnitLh).
		TextAlign(SymStart).
		TextIndent(0, SymUnitPercent).
		Build())

	// Image style - matches CSS ".image { }"
	sr.Register(NewStyle("image").
		TextAlign(SymCenter).
		Build())

	// Heading styles (h1-h6) - KPV uses rem for font-size
	sr.Register(NewStyle("h1").
		FontSize(1.4, SymUnitLh).
		Build())

	sr.Register(NewStyle("h2").
		FontSize(1.2, SymUnitLh).
		Build())

	sr.Register(NewStyle("h3").
		FontSize(1.2, SymUnitLh).
		Build())

	sr.Register(NewStyle("h4").
		FontSize(1.2, SymUnitLh).
		Build())

	sr.Register(NewStyle("h5").
		FontSize(1.2, SymUnitLh).
		Build())

	sr.Register(NewStyle("h6").
		FontSize(1.2, SymUnitLh).
		Build())

	// has-dropcap style for paragraphs with drop caps
	// KPV uses dropcap-chars and dropcap-lines properties
	sr.Register(NewStyle("has-dropcap").
		Inherit("p").
		Dropcap(1, 3).
		TextIndent(0, SymUnitPercent).
		MarginBottom(0.333333, SymUnitLh).
		Build())

	// Epigraph style - uses percentage-based margins like KPV
	sr.Register(NewStyle("epigraph").
		Inherit("p").
		FontStyle(SymItalic).
		TextIndent(0, SymUnitPercent).
		MarginLeftPercent(9.375).
		MarginTop(0.833333, SymUnitLh).
		MarginBottom(0.25, SymUnitLh).
		Build())

	// Text author style (for epigraphs, poems, cites)
	// KPV uses yj-break-before: avoid to keep author with preceding content
	sr.Register(NewStyle("text-author").
		Inherit("p").
		TextAlign(SymEnd).
		FontStyle(SymItalic).
		FontWeight(SymBold).
		TextIndent(0, SymUnitPercent).
		YjBreakBefore(SymAvoid).
		Build())

	// Subtitle style - matches CSS ".subtitle { }" and ".section-subtitle { }"
	sr.Register(NewStyle("subtitle").
		Inherit("p").
		FontWeight(SymBold).
		TextAlign(SymCenter).
		TextIndent(0, SymUnitPercent).
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		Build())

	// Context-specific subtitle styles - matches EPUB's "context-subtitle" pattern
	sr.Register(NewStyle("section-subtitle").
		Inherit("subtitle").
		Build())

	sr.Register(NewStyle("cite-subtitle").
		Inherit("p").
		TextAlign(SymStart).
		MarginTop(0.5, SymUnitEm).
		MarginBottom(0.5, SymUnitEm).
		Build())

	sr.Register(NewStyle("annotation-subtitle").
		Inherit("subtitle").
		Build())

	sr.Register(NewStyle("epigraph-subtitle").
		Inherit("subtitle").
		Build())

	// Empty line / blank space style - matches CSS ".emptyline { }"
	sr.Register(NewStyle("emptyline").
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		MarginLeft(1.0, SymUnitEm).
		MarginRight(1.0, SymUnitEm).
		Render(SymBlock).
		Build())

	// Poem styles - inherits from p for line-height
	sr.Register(NewStyle("poem").
		Inherit("p").
		MarginLeft(2.0, SymUnitEm).
		TextIndent(0, SymUnitPercent).
		MarginTop(1.0, SymUnitLh).
		MarginBottom(1.0, SymUnitLh).
		Build())

	sr.Register(NewStyle("poem-title").
		Inherit("poem").
		FontWeight(SymBold).
		TextAlign(SymCenter).
		Build())

	// Poem subtitle - matches EPUB's ".poem-subtitle { }"
	sr.Register(NewStyle("poem-subtitle").
		Inherit("subtitle").
		MarginLeft(2.0, SymUnitEm).
		Build())

	sr.Register(NewStyle("stanza").
		Inherit("poem").
		MarginTop(0.5, SymUnitLh).
		Build())

	// Stanza title - matches EPUB's "stanza-title" class
	sr.Register(NewStyle("stanza-title").
		Inherit("poem").
		FontWeight(SymBold).
		TextAlign(SymCenter).
		Build())

	// Stanza subtitle - matches EPUB's ".stanza-subtitle { }"
	sr.Register(NewStyle("stanza-subtitle").
		Inherit("subtitle").
		MarginLeft(2.0, SymUnitEm).
		Build())

	sr.Register(NewStyle("verse").
		Inherit("p").
		MarginTop(0.25, SymUnitEm).
		MarginBottom(0.25, SymUnitEm).
		MarginLeft(2.0, SymUnitEm).
		TextIndent(0, SymUnitPercent).
		Build())

	// Citation style
	sr.Register(NewStyle("cite").
		Inherit("p").
		MarginLeft(2.0, SymUnitEm).
		MarginRight(2.0, SymUnitEm).
		TextIndent(0, SymUnitPercent).
		FontStyle(SymItalic).
		Build())

	// Table style
	sr.Register(NewStyle("table").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		MarginTop(1.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		MarginLeftAuto().
		MarginRightAuto().
		KeepTogether().
		Width(1.0, SymUnitLh).
		Build())

	// Inline styles for text formatting
	sr.Register(NewStyle("strong").
		FontWeight(SymBold).
		Build())

	sr.Register(NewStyle("emphasis").
		FontStyle(SymItalic).
		Build())

	sr.Register(NewStyle("strikethrough").
		Strikethrough(true).
		Build())

	// Subscript and superscript styles - KPV uses baseline-style
	sr.Register(NewStyle("sub").
		BaselineStyle(SymSubscript).
		FontSize(0.75, SymUnitEm).
		Build())

	sr.Register(NewStyle("sup").
		BaselineStyle(SymSuperscript).
		FontSize(0.75, SymUnitEm).
		Build())

	// Footnote style
	sr.Register(NewStyle("footnote").
		Inherit("p").
		FontSize(0.85, SymUnitEm).
		Build())

	// Footnote title style
	sr.Register(NewStyle("footnote-title").
		Inherit("p").
		FontWeight(SymBold).
		Build())

	// Annotation style
	sr.Register(NewStyle("annotation").
		Inherit("p").
		FontStyle(SymItalic).
		TextIndent(0, SymUnitPercent).
		MarginBottom(1.0, SymUnitLh).
		Build())

	// Annotation page title
	sr.Register(NewStyle("annotation-title").
		Inherit("h2").
		Build())

	// TOC page styles - matches CSS ".toc-title { }" and toc elements
	sr.Register(NewStyle("toc-title").
		Inherit("h2").
		Build())
	sr.Register(NewStyle("toc-item").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		MarginLeft(1.0, SymUnitEm).
		Build())

	// Body/chapter/section title wrappers - matches CSS ".body-title { }", ".chapter-title { }", ".section-title { }"
	// Uses KPV-compatible break properties: yj-break-after: avoid, yj-break-before: auto
	sr.Register(NewStyle("body-title").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		MarginTop(2.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		YjBreakAfter(SymAvoid).
		YjBreakBefore(SymAuto).
		Build())

	sr.Register(NewStyle("chapter-title").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		MarginTop(2.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		YjBreakAfter(SymAvoid).
		YjBreakBefore(SymAuto).
		Build())

	sr.Register(NewStyle("section-title").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		MarginTop(2.0, SymUnitEm).
		MarginBottom(1.0, SymUnitEm).
		YjBreakAfter(SymAvoid).
		YjBreakBefore(SymAuto).
		Build())

	// Title header styles - matches CSS ".body-title-header { }", ".chapter-title-header { }", ".section-title-header { }"
	sr.Register(NewStyle("body-title-header").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		TextAlign(SymCenter).
		FontWeight(SymBold).
		YjBreakAfter(SymAvoid).
		Build())

	sr.Register(NewStyle("chapter-title-header").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		TextAlign(SymCenter).
		FontWeight(SymBold).
		YjBreakAfter(SymAvoid).
		Build())

	sr.Register(NewStyle("section-title-header").
		Inherit("p").
		TextIndent(0, SymUnitPercent).
		TextAlign(SymCenter).
		FontWeight(SymBold).
		YjBreakAfter(SymAvoid).
		Build())

	// Date style - matches CSS ".date { }"
	sr.Register(NewStyle("date").
		Inherit("p").
		TextAlign(SymEnd).
		TextIndent(0, SymUnitPercent).
		MarginTop(0.5, SymUnitEm).
		MarginBottom(0.5, SymUnitEm).
		Build())

	// Link styles - matches CSS ".link-external { }", ".link-internal { }", ".link-footnote { }"
	sr.Register(NewStyle("link-external").
		Build())

	sr.Register(NewStyle("link-internal").
		Build())

	// Footnote link style - KPV uses baseline-style: superscript, font-size in rem
	// We use baseline-style instead of baseline-shift for KPV compatibility
	sr.Register(NewStyle("link-footnote").
		FontStyle(SymNormal).
		FontSize(0.75, SymUnitRem).
		BaselineStyle(SymSuperscript).
		LineHeight(1.33, SymUnitLh).
		Build())

	sr.Register(NewStyle("link-backlink").
		FontWeight(SymBold).
		Build())

	// Vignette image style
	sr.Register(NewStyle("vignette").
		Inherit("image").
		MarginTop(0.5, SymUnitEm).
		MarginBottom(0.5, SymUnitEm).
		Build())

	return sr
}
