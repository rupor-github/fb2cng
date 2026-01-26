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
// where an element sits within its parent container.
//
// For title blocks (wrappers containing vignettes and text), KP3 uses inverted
// margin-top logic where spacing between elements comes from margin-top on
// following elements rather than margin-bottom on preceding elements:
//   - First element: LOSES margin-top (nothing precedes it)
//   - Non-first elements: KEEPS margin-top (creates spacing after preceding element)
//   - margin-bottom is not filtered (all elements keep it)
//
// Reference: com/amazon/yj/i/b/c/a.java and com/amazon/yj/i/b/d.java
type ElementPosition struct {
	IsFirst    bool // First element in container
	IsLast     bool // Last element in container
	TitleBlock bool // Title block context - uses inverted margin-top logic
}

// PositionFirstAndLast returns a position for a single element in its container.
// Both first and last - no filtering applied (keeps all margins).
func PositionFirstAndLast() ElementPosition {
	return ElementPosition{IsFirst: true, IsLast: true}
}

// PositionFirst returns a position for the first of multiple elements.
func PositionFirst() ElementPosition {
	return ElementPosition{IsFirst: true, IsLast: false}
}

// PositionMiddle returns a position for a middle element (not first, not last).
func PositionMiddle() ElementPosition {
	return ElementPosition{IsFirst: false, IsLast: false}
}

// PositionLast returns a position for the last of multiple elements.
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

// String returns a human-readable position name for debugging/tracing.
func (p ElementPosition) String() string {
	if p.IsFirst && p.IsLast {
		return "only"
	}
	if p.IsFirst {
		return "first"
	}
	if p.IsLast {
		return "last"
	}
	return "middle"
}

// StyleDef defines a KFX style with its properties.
type StyleDef struct {
	Name       string            // Style name (becomes local symbol)
	Parent     string            // Parent style name (for inheritance)
	Properties map[KFXSymbol]any // KFX property symbol -> value

	// DescendantReplacement marks this style as using "replacement" semantics for
	// descendant selectors. When true, if a descendant selector like "h1--sub" exists,
	// it completely replaces the base class (e.g., "sub") rather than just overriding
	// specific properties. This is used for styles like sub/sup/small where the
	// heading-context version should inherit font-size from the heading rather than
	// using the base class's explicit font-size.
	DescendantReplacement bool
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
// Values should be pre-rounded to the appropriate precision before calling this function.
// See SignificantFigures, LineHeightPrecision, WidthPercentPrecision in kp3_units.go.
func formatNumber(v float64) string {
	// Round to SignificantFigures as a safety net (values should already be rounded)
	v = RoundSignificant(v, SignificantFigures)

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
	name                  string
	parent                string
	props                 map[KFXSymbol]any
	descendantReplacement bool
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

// DescendantReplacement marks this style as using replacement semantics for
// descendant selectors. When a descendant selector like "h1--sub" exists for
// this style, it completely replaces the base class rather than just overriding.
// Use this for styles with explicit font-size that should inherit from context
// when inside headings (sub, sup, small).
func (sb *StyleBuilder) DescendantReplacement() *StyleBuilder {
	sb.descendantReplacement = true
	return sb
}

// FontSize sets the font size.
func (sb *StyleBuilder) FontSize(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymFontSize] = DimensionValue(value, unit)
	return sb
}

// FontSizeSmaller sets font-size to 'smaller' (0.8333em = 5/6).
// This matches Amazon's CSS conversion: font-size: smaller -> 0.8333em.
func (sb *StyleBuilder) FontSizeSmaller() *StyleBuilder {
	sb.props[SymFontSize] = DimensionValue(0.8333333333333334, SymUnitEm)
	return sb
}

// FontSizeLarger sets font-size to 'larger' (1.2em).
// This matches Amazon's CSS conversion: font-size: larger -> 1.2em.
func (sb *StyleBuilder) FontSizeLarger() *StyleBuilder {
	sb.props[SymFontSize] = DimensionValue(1.2, SymUnitEm)
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

// LineHeightNormal sets line-height to 'normal' (keyword, not dimension).
// Used for elements like sub/sup where Amazon uses line-height: normal.
func (sb *StyleBuilder) LineHeightNormal() *StyleBuilder {
	sb.props[SymLineHeight] = SymbolValue(SymNormal)
	return sb
}

// WhiteSpaceNowrap sets white-space to 'nowrap' to prevent line wrapping.
// This matches Amazon's CSS conversion: white-space: nowrap -> $716: $715.
func (sb *StyleBuilder) WhiteSpaceNowrap() *StyleBuilder {
	sb.props[SymWhiteSpace] = SymbolValue(SymNowrap)
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

// MinWidth sets the min_width property.
func (sb *StyleBuilder) MinWidth(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMinWidth] = DimensionValue(value, unit)
	return sb
}

// MaxWidth sets the max_width property.
func (sb *StyleBuilder) MaxWidth(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymMaxWidth] = DimensionValue(value, unit)
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

// PaddingTop sets the top padding.
func (sb *StyleBuilder) PaddingTop(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymPaddingTop] = DimensionValue(value, unit)
	return sb
}

// PaddingBottom sets the bottom padding.
func (sb *StyleBuilder) PaddingBottom(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymPaddingBottom] = DimensionValue(value, unit)
	return sb
}

// PaddingLeft sets the left padding.
func (sb *StyleBuilder) PaddingLeft(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymPaddingLeft] = DimensionValue(value, unit)
	return sb
}

// PaddingRight sets the right padding.
func (sb *StyleBuilder) PaddingRight(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymPaddingRight] = DimensionValue(value, unit)
	return sb
}

// BorderStyle sets the border style (solid, none, etc.).
func (sb *StyleBuilder) BorderStyle(style KFXSymbol) *StyleBuilder {
	sb.props[SymBorderStyle] = SymbolValue(style)
	return sb
}

// BorderWidth sets the border width.
func (sb *StyleBuilder) BorderWidth(value float64, unit KFXSymbol) *StyleBuilder {
	sb.props[SymBorderWeight] = DimensionValue(value, unit)
	return sb
}

// YjVerticalAlign sets the yj.vertical_align property for table cell vertical alignment.
// Use SymCenter for vertical centering within table cells.
func (sb *StyleBuilder) YjVerticalAlign(align KFXSymbol) *StyleBuilder {
	sb.props[SymYjVerticalAlign] = SymbolValue(align)
	return sb
}

// Build creates the StyleDef.
func (sb *StyleBuilder) Build() StyleDef {
	return StyleDef{
		Name:                  sb.name,
		Parent:                sb.parent,
		Properties:            sb.props,
		DescendantReplacement: sb.descendantReplacement,
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
