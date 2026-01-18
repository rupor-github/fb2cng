package kfx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
)

// Style fragment ($157) generation for KFX.
// Styles define formatting properties applied to content elements.
// Each style is referenced by name (symbol) in content entries.
//
// KP3-compatible approach: Generate unique styles for each unique property combination.
// Style names are short auto-generated identifiers like "s1", "s2", etc.
// This matches how Kindle Previewer generates styles from EPUB.

// ElementPosition tracks an element's position within its container.
// KP3 (Kindle Previewer 3) applies position-based CSS property filtering based on
// where an element sits within its parent container. This affects margin collapsing
// and break behavior at container boundaries.
//
// From KP3 reference (com/amazon/yj/style/merger/e/e.java):
//   - First element: margin-top, padding-top, break-before, clear REMOVED
//   - Last element: margin-bottom, padding-bottom, break-after REMOVED
//   - Middle elements: all properties KEPT
//   - Single element (first+last): both sets of properties REMOVED
type ElementPosition struct {
	IsFirst bool // First element in container - removes top margin/padding/break-before
	IsLast  bool // Last element in container - removes bottom margin/padding/break-after
}

// PositionFirstAndLast returns a position for a single element in its container.
// Both first and last filtering applies - removes both top and bottom margins.
func PositionFirstAndLast() ElementPosition {
	return ElementPosition{IsFirst: true, IsLast: true}
}

// PositionFirst returns a position for the first of multiple elements.
// Only first-element filtering applies - removes top margin but keeps bottom.
func PositionFirst() ElementPosition {
	return ElementPosition{IsFirst: true, IsLast: false}
}

// PositionMiddle returns a position for a middle element (not first, not last).
// No filtering - all properties are kept.
func PositionMiddle() ElementPosition {
	return ElementPosition{IsFirst: false, IsLast: false}
}

// PositionLast returns a position for the last of multiple elements.
// Only last-element filtering applies - removes bottom margin but keeps top.
func PositionLast() ElementPosition {
	return ElementPosition{IsFirst: false, IsLast: true}
}

// PositionFromIndex calculates element position given index and total count.
// index is 0-based, count is total number of elements in the container.
func PositionFromIndex(index, count int) ElementPosition {
	if count <= 0 {
		return PositionMiddle() // Defensive: treat as middle if invalid
	}
	if count == 1 {
		return PositionFirstAndLast()
	}
	switch index {
	case 0:
		return PositionFirst()
	case count - 1:
		return PositionLast()
	default:
		return PositionMiddle()
	}
}

// FilterPropertiesByPosition applies KP3's position-based property selection.
// This selectively excludes margin/break properties based on element position in container.
//
// KP3 behavior observed from reference output:
//   - First element in container: excludes margin-top (no space above needed at container start)
//   - Last element in container: excludes margin-bottom (no space below needed at container end)
//   - First+Last (only element): excludes BOTH margins
//   - Middle elements: keeps all margins
//
// If the input props is nil, returns nil.
// Returns a new map with selected properties (original is not modified).
func FilterPropertiesByPosition(props map[KFXSymbol]any, pos ElementPosition) map[KFXSymbol]any {
	filtered, _ := filterPropertiesByPositionWithRemoved(props, pos)
	return filtered
}

// filterPropertiesByPositionWithRemoved is the internal implementation that also returns
// which properties were excluded. This is used for tracing/debugging.
func filterPropertiesByPositionWithRemoved(props map[KFXSymbol]any, pos ElementPosition) (map[KFXSymbol]any, []KFXSymbol) {
	if len(props) == 0 {
		return props, nil
	}

	// Middle elements (neither first nor last): no exclusions
	if !pos.IsFirst && !pos.IsLast {
		return props, nil
	}

	// Build set of properties to exclude based on position
	toExclude := make(map[KFXSymbol]bool)

	// First elements exclude margin-top (no space above needed at container start)
	if pos.IsFirst {
		toExclude[SymMarginTop] = true
	}

	// Last elements exclude margin-bottom (no space below needed at container end)
	if pos.IsLast {
		toExclude[SymMarginBottom] = true
	}

	// Check which properties actually exist and will be excluded
	var excludedProps []KFXSymbol
	for sym := range toExclude {
		if _, exists := props[sym]; exists {
			excludedProps = append(excludedProps, sym)
		}
	}
	if len(excludedProps) == 0 {
		return props, nil
	}

	// Create copy without excluded properties
	filtered := make(map[KFXSymbol]any, len(props))
	for sym, val := range props {
		if !toExclude[sym] {
			filtered[sym] = val
		}
	}
	return filtered, excludedProps
}

// traceSymbolNameForStyle returns a human-readable name for a KFX symbol used in style tracing.
func traceSymbolNameForStyle(sym KFXSymbol) string {
	if name, ok := yjSymbolNames[sym]; ok {
		return name
	}
	return fmt.Sprintf("$%d", sym)
}

// StyleDef defines a KFX style with its properties.
type StyleDef struct {
	Name       string            // Style name (becomes local symbol)
	Parent     string            // Parent style name (for inheritance)
	Properties map[KFXSymbol]any // KFX property symbol -> value
}

type styleUsage uint8

const (
	styleUsageText styleUsage = 1 << iota
	styleUsageImage
	styleUsageWrapper
)

// DimensionValue creates a KP3-compatible dimension value with unit.
// Example: DimensionValue(1.2, SymUnitRatio) -> {$307: "1.2", $306: $310}
func DimensionValue(value float64, unit KFXSymbol) StructValue {
	// KP3 uses Ion decimals (not strings) for $307.
	dec := ion.MustParseDecimal(formatNumber(value))
	return NewStruct().
		Set(SymValue, dec). // $307 = Ion decimal
		SetSymbol(SymUnit, unit)
}

// formatNumber formats a number in KP3's style.
// KP3 uses scientific notation "d-1" for small decimals.
// KP3 uses at most 3 significant decimal digits - more precision causes rendering issues.
// See DecimalPrecision in kpv_units.go for details.
func formatNumber(v float64) string {
	// Round to DecimalPrecision decimal places to match KP3 precision
	v = RoundDecimal(v)

	if v == 0 {
		return "0."
	}
	if v == 1 {
		return "1."
	}

	// KP3 generally emits integers with a trailing dot (e.g. "100.").
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d.", int64(v))
	}

	// KP3 typically uses "d" scientific notation for values < 1 (e.g. 0.25 -> "2.5d-1").
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

// BreakInsideAvoid sets break-inside to avoid (KP3 style).
func (sb *StyleBuilder) BreakInsideAvoid() *StyleBuilder {
	sb.props[SymBreakInside] = SymbolValue(SymAvoid)
	return sb
}

// YjBreakBefore sets yj-break-before property for KP3 compatibility.
func (sb *StyleBuilder) YjBreakBefore(mode KFXSymbol) *StyleBuilder {
	sb.props[SymYjBreakBefore] = SymbolValue(mode)
	return sb
}

// YjBreakAfter sets yj-break-after property for KP3 compatibility.
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

// LayoutHintTitle sets layout-hints to [treat_as_title] for KP3 compatibility.
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
