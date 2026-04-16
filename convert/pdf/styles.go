package pdf

import (
	"sort"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"
	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

const defaultFontSizePt = 12.0

type styleScope struct {
	Tag     string
	Classes []string
}

type styleSpecificity struct {
	Classes  int
	Elements int
}

type compiledRule struct {
	Selector    css.Selector
	Properties  map[string]css.Value
	Order       int
	Specificity styleSpecificity
}

type pseudoContent struct {
	Before string
	After  string
}

type resolvedStyle struct {
	FontFamily string
	FontSize   float64
	LineHeight float64
	Bold       bool
	Italic     bool

	Align      layout.Align
	TextIndent float64

	Color         layout.Color
	HasColor      bool
	Underline     bool
	Strike        bool
	LetterSpacing float64
	Hyphens       string // "none", "manual", "auto", or "" (inherit)

	BaselineShift float64

	MarginTop       float64
	MarginRight     float64
	MarginBottom    float64
	MarginLeft      float64
	MarginLeftAuto  bool // true when margin-left: auto was set
	MarginRightAuto bool // true when margin-right: auto was set

	PaddingTop    float64
	PaddingRight  float64
	PaddingBottom float64
	PaddingLeft   float64

	Background *layout.Color
	Border     layout.Border
	HasBorder  bool

	KeepTogether bool
	BreakBefore  string // "always", "avoid", "" (auto/unset)
	BreakAfter   string // "always", "avoid", "" (auto/unset)
	Hidden       bool
	WhiteSpace   string
	WidthPercent float64

	MinWidth  cssDimension
	MaxWidth  cssDimension
	MinHeight cssDimension
	MaxHeight cssDimension

	Orphans int // min lines at bottom of page before break (CSS default 2)
	Widows  int // min lines at top of page after break (CSS default 2)
}

// cssDimension holds a CSS length that may be either an absolute value (points)
// or a percentage of the container.  Zero-value means unset.
type cssDimension struct {
	Pt      float64 // absolute value in points (used when Percent == 0)
	Percent float64 // percentage value (0–100); when > 0, Pt is ignored
}

func defaultResolvedStyle() resolvedStyle {
	return resolvedStyle{
		FontFamily: "serif",
		FontSize:   defaultFontSizePt,
		LineHeight: 1.2,
		Align:      layout.AlignLeft,
		Color:      layout.ColorBlack,
		HasColor:   true,
		Orphans:    2,
		Widows:     2,
	}
}

func (s resolvedStyle) inheritedOnly() resolvedStyle {
	return resolvedStyle{
		FontFamily:    s.FontFamily,
		FontSize:      s.FontSize,
		LineHeight:    s.LineHeight,
		Bold:          s.Bold,
		Italic:        s.Italic,
		Align:         s.Align,
		TextIndent:    s.TextIndent,
		Color:         s.Color,
		HasColor:      s.HasColor,
		Underline:     s.Underline,
		Strike:        s.Strike,
		LetterSpacing: s.LetterSpacing,
		Hyphens:       s.Hyphens,
		WhiteSpace:    s.WhiteSpace,
		Orphans:       s.Orphans,
		Widows:        s.Widows,
	}
}

type styleResolver struct {
	rules       []compiledRule
	pseudoRules []compiledRule
	log         *zap.Logger
}

func buildCombinedStylesheet(stylesheets []fb2.Stylesheet, log *zap.Logger) *css.Stylesheet {
	if log == nil {
		log = zap.NewNop()
	}

	var combinedCSS []byte
	for _, sheet := range stylesheets {
		if sheet.Type != "" && sheet.Type != "text/css" {
			continue
		}
		if sheet.Data == "" {
			continue
		}
		combinedCSS = append(combinedCSS, []byte(sheet.Data)...)
		combinedCSS = append(combinedCSS, '\n')
	}

	if len(combinedCSS) == 0 {
		return nil
	}

	return css.NewParser(log).Parse(combinedCSS, "combined stylesheets")
}

func newStyleResolverFromParsed(sheet *css.Stylesheet, log *zap.Logger) *styleResolver {
	if log == nil {
		log = zap.NewNop()
	}

	sr := &styleResolver{log: log.Named("pdf-css")}
	if sheet == nil {
		return sr
	}

	order := 0
	for _, rule := range flattenStylesheetForPDF(sheet) {
		compiled := compiledRule{
			Selector:    rule.Selector,
			Properties:  rule.Properties,
			Order:       order,
			Specificity: selectorSpecificity(rule.Selector),
		}
		order++
		if rule.Selector.Pseudo == css.PseudoNone {
			sr.rules = append(sr.rules, compiled)
			continue
		}
		sr.pseudoRules = append(sr.pseudoRules, compiled)
	}

	return sr
}

func flattenStylesheetForPDF(sheet *css.Stylesheet) []css.Rule {
	if sheet == nil {
		return nil
	}

	var rules []css.Rule
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			rules = append(rules, *item.Rule)
		case item.MediaBlock != nil:
			if item.MediaBlock.Query.Evaluate(false, false) {
				rules = append(rules, item.MediaBlock.Rules...)
			}
		}
	}
	return rules
}

func (sr *styleResolver) Resolve(tag, classes string, ancestors []styleScope, parent resolvedStyle) resolvedStyle {
	style := parent.inheritedOnly()
	if style.FontSize <= 0 {
		style = defaultResolvedStyle()
	}

	// Apply UA (User Agent) font defaults BEFORE CSS rules.
	// These provide browser-like defaults for HTML tags (bold for <strong>,
	// italic for <em>, monospace for <code>, heading font-sizes, etc.).
	// CSS rules applied below will override these if they set the same properties.
	applyUAFontDefaults(&style, tag)

	classList := splitClasses(classes)
	current := styleScope{Tag: tag, Classes: classList}
	matched := sr.matchRules(sr.rules, current, ancestors)

	// Scan matched CSS rules to determine which margin and baseline properties
	// are explicitly set. UA defaults for these will not override CSS values.
	cssFlags := scanCSSOverrides(matched)

	for _, rule := range matched {
		sr.applyRule(&style, parent, rule)
	}

	// Apply UA margin and baseline defaults AFTER CSS rules, only for
	// properties not explicitly set by any matched CSS rule. Margins are
	// computed from the final font-size (which CSS may have changed).
	applyUAPostCSSDefaults(&style, tag, cssFlags)

	// Convert margin: auto to alignment (KFX MarginAutoTransformer).
	resolveMarginAuto(&style)

	if style.FontFamily == "" {
		style.FontFamily = parent.FontFamily
		if style.FontFamily == "" {
			style.FontFamily = defaultResolvedStyle().FontFamily
		}
	}
	if style.FontSize <= 0 {
		style.FontSize = parent.FontSize
		if style.FontSize <= 0 {
			style.FontSize = defaultFontSizePt
		}
	}
	if style.LineHeight <= 0 {
		style.LineHeight = parent.LineHeight
		if style.LineHeight <= 0 {
			style.LineHeight = 1.2
		}
	}
	if !style.HasColor {
		style.Color = parent.Color
		style.HasColor = parent.HasColor
		if !style.HasColor {
			style.Color = layout.ColorBlack
			style.HasColor = true
		}
	}

	return style
}

// cssOverrideFlags tracks which properties were explicitly set by matched CSS rules.
// Used to avoid overriding user CSS with UA (User Agent) defaults.
type cssOverrideFlags struct {
	marginTop     bool
	marginRight   bool
	marginBottom  bool
	marginLeft    bool
	verticalAlign bool
}

// scanCSSOverrides scans matched CSS rules to determine which margin and baseline
// properties are explicitly set. Returns flags indicating which properties were found.
func scanCSSOverrides(matched []compiledRule) cssOverrideFlags {
	var flags cssOverrideFlags
	for _, rule := range matched {
		if _, ok := rule.Properties["margin"]; ok {
			flags.marginTop = true
			flags.marginRight = true
			flags.marginBottom = true
			flags.marginLeft = true
		}
		if _, ok := rule.Properties["margin-top"]; ok {
			flags.marginTop = true
		}
		if _, ok := rule.Properties["margin-right"]; ok {
			flags.marginRight = true
		}
		if _, ok := rule.Properties["margin-bottom"]; ok {
			flags.marginBottom = true
		}
		if _, ok := rule.Properties["margin-left"]; ok {
			flags.marginLeft = true
		}
		if _, ok := rule.Properties["vertical-align"]; ok {
			flags.verticalAlign = true
		}
	}
	return flags
}

// applyUAFontDefaults applies HTML User Agent font defaults for the given tag.
// These are applied BEFORE CSS rules so that CSS can override them.
// Matches the KFX DefaultStyleRegistry() font properties.
func applyUAFontDefaults(style *resolvedStyle, tag string) {
	switch tag {
	case "strong", "b":
		style.Bold = true
	case "em", "i":
		style.Italic = true
	case "del", "s", "strike":
		style.Strike = true
	case "u":
		style.Underline = true
	case "sub":
		style.FontSize = 0.75 * defaultFontSizePt // 0.75rem = 9pt
	case "sup":
		style.FontSize = 0.75 * defaultFontSizePt // 0.75rem = 9pt
	case "code", "pre":
		style.FontFamily = "monospace"
	case "th":
		style.Bold = true
	case "h1":
		style.FontSize = 1.5 * defaultFontSizePt // 18pt
		style.Bold = true
	case "h2":
		style.FontSize = 1.5 * defaultFontSizePt // 18pt
		style.Bold = true
	case "h3":
		style.FontSize = 1.17 * defaultFontSizePt // 14.04pt
		style.Bold = true
	case "h4":
		style.FontSize = 1.0 * defaultFontSizePt // 12pt
		style.Bold = true
	case "h5":
		style.FontSize = 0.83 * defaultFontSizePt // 9.96pt
		style.Bold = true
	case "h6":
		style.FontSize = 0.67 * defaultFontSizePt // 8.04pt
		style.Bold = true
	}
}

// applyUAPostCSSDefaults applies HTML User Agent margin and baseline defaults
// AFTER CSS rules. Only sets properties not explicitly overridden by any matched
// CSS rule. Vertical margins use the CSS UA stylesheet values (in em, multiplied
// by the element's final font-size). This matches the KFX DefaultStyleRegistry():
//
//   - p:          margin 1em 0         (KFX: 0.833333lh ≈ 1em/1.2)
//   - h1:         margin 0.67em 0      (KFX: 0.558lh)
//   - h2:         margin 0.83em 0      (KFX: 0.692lh)
//   - h3:         margin 1.0em 0       (KFX: 0.833333lh)
//   - h4:         margin 1.33em 0      (KFX: 1.108lh)
//   - h5:         margin 1.67em 0      (KFX: 1.392lh)
//   - h6:         margin 2.33em 0      (KFX: 1.942lh)
//   - blockquote: margin 1em 40px      (KFX: 0.833333lh + 40px)
//   - pre:        margin 1em 0         (KFX: 0.833333lh)
func applyUAPostCSSDefaults(style *resolvedStyle, tag string, flags cssOverrideFlags) {
	var marginTopEm, marginBottomEm float64
	var hasVerticalMargins bool
	var marginLeftPx, marginRightPx float64
	var hasHorizontalMargins bool

	switch tag {
	case "p":
		marginTopEm, marginBottomEm = 1.0, 1.0
		hasVerticalMargins = true
	case "h1":
		marginTopEm, marginBottomEm = 0.67, 0.67
		hasVerticalMargins = true
	case "h2":
		marginTopEm, marginBottomEm = 0.83, 0.83
		hasVerticalMargins = true
	case "h3":
		marginTopEm, marginBottomEm = 1.0, 1.0
		hasVerticalMargins = true
	case "h4":
		marginTopEm, marginBottomEm = 1.33, 1.33
		hasVerticalMargins = true
	case "h5":
		marginTopEm, marginBottomEm = 1.67, 1.67
		hasVerticalMargins = true
	case "h6":
		marginTopEm, marginBottomEm = 2.33, 2.33
		hasVerticalMargins = true
	case "blockquote":
		marginTopEm, marginBottomEm = 1.0, 1.0
		hasVerticalMargins = true
		marginLeftPx, marginRightPx = 40, 40
		hasHorizontalMargins = true
	case "pre":
		marginTopEm, marginBottomEm = 1.0, 1.0
		hasVerticalMargins = true
	}

	if hasVerticalMargins {
		fontSize := style.FontSize
		if fontSize <= 0 {
			fontSize = defaultFontSizePt
		}
		if !flags.marginTop {
			style.MarginTop = marginTopEm * fontSize
		}
		if !flags.marginBottom {
			style.MarginBottom = marginBottomEm * fontSize
		}
	}
	if hasHorizontalMargins {
		if !flags.marginLeft {
			style.MarginLeft = CSSPxToPt(marginLeftPx)
		}
		if !flags.marginRight {
			style.MarginRight = CSSPxToPt(marginRightPx)
		}
	}

	// Baseline shift for sub/sup using final font-size.
	// Only applied if CSS didn't explicitly set vertical-align.
	if !flags.verticalAlign {
		switch tag {
		case "sub":
			style.BaselineShift = -style.FontSize * 0.2
		case "sup":
			style.BaselineShift = style.FontSize * 0.33
		}
	}
}

func (sr *styleResolver) ResolvePseudo(tag, classes string, ancestors []styleScope, parent resolvedStyle) pseudoContent {
	matched := sr.matchRules(sr.pseudoRules, styleScope{Tag: tag, Classes: splitClasses(classes)}, ancestors)
	var out pseudoContent
	for _, rule := range matched {
		content, ok := rule.Properties["content"]
		if !ok {
			continue
		}
		text := extractPseudoText(content)
		if text == "" {
			continue
		}
		switch rule.Selector.Pseudo {
		case css.PseudoBefore:
			out.Before = text
		case css.PseudoAfter:
			out.After = text
		}
	}
	_ = parent
	return out
}

func (sr *styleResolver) matchRules(rules []compiledRule, current styleScope, ancestors []styleScope) []compiledRule {
	matched := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		if selectorMatches(rule.Selector, current, ancestors) {
			matched = append(matched, rule)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		left, right := matched[i], matched[j]
		if left.Specificity.Classes != right.Specificity.Classes {
			return left.Specificity.Classes < right.Specificity.Classes
		}
		if left.Specificity.Elements != right.Specificity.Elements {
			return left.Specificity.Elements < right.Specificity.Elements
		}
		return left.Order < right.Order
	})
	return matched
}

func selectorSpecificity(sel css.Selector) styleSpecificity {
	var spec styleSpecificity
	for current := &sel; current != nil; current = current.Ancestor {
		if current.Class != "" {
			spec.Classes++
		}
		if current.Element != "" {
			spec.Elements++
		}
	}
	return spec
}

func selectorMatches(sel css.Selector, current styleScope, ancestors []styleScope) bool {
	if !selectorPartMatches(sel, current) {
		return false
	}
	if sel.Ancestor == nil {
		return true
	}
	return ancestorSelectorMatches(sel.Ancestor, ancestors)
}

func ancestorSelectorMatches(sel *css.Selector, ancestors []styleScope) bool {
	if sel == nil {
		return true
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if !selectorPartMatches(*sel, ancestors[i]) {
			continue
		}
		if ancestorSelectorMatches(sel.Ancestor, ancestors[:i]) {
			return true
		}
	}
	return false
}

func selectorPartMatches(sel css.Selector, current styleScope) bool {
	if sel.Element != "" && !strings.EqualFold(sel.Element, current.Tag) {
		return false
	}
	if sel.Class != "" && !scopeHasClass(current, sel.Class) {
		return false
	}
	return sel.Element != "" || sel.Class != ""
}

func scopeHasClass(scope styleScope, class string) bool {
	for _, candidate := range scope.Classes {
		if candidate == class {
			return true
		}
	}
	return false
}

func splitClasses(classes string) []string {
	if classes == "" {
		return nil
	}
	return strings.Fields(classes)
}

func (sr *styleResolver) applyRule(style *resolvedStyle, parent resolvedStyle, rule compiledRule) {
	props := rule.Properties
	if len(props) == 0 {
		return
	}

	if val, ok := props["margin"]; ok {
		applyMarginShorthand(val, style.FontSize, style)
	}
	if val, ok := props["padding"]; ok {
		applyBoxShorthand(val, style.FontSize, &style.PaddingTop, &style.PaddingRight, &style.PaddingBottom, &style.PaddingLeft)
	}
	if val, ok := props["border"]; ok {
		if border, ok := parseBorder(val.Raw); ok {
			style.Border = border
			style.HasBorder = true
		}
		raw := strings.ToLower(val.Raw)
		for _, unsupported := range []string{"groove", "ridge", "inset", "outset"} {
			if strings.Contains(raw, unsupported) {
				sr.log.Debug("Ignoring unsupported border-style value",
					zap.String("property", "border"),
					zap.String("unsupported", unsupported),
					zap.String("value", val.Raw),
				)
			}
		}
	}
	if val, ok := props["background"]; ok {
		if color, ok := parseColorValue(val); ok {
			style.Background = &color
		}
	}

	if val, ok := props["font-family"]; ok {
		style.FontFamily = normalizeFontFamilyValue(val)
	}
	if val, ok := props["font-size"]; ok {
		if size, ok := parseFontSize(val, parent.FontSize); ok && size > 0 {
			style.FontSize = size
		}
	}
	if val, ok := props["font-weight"]; ok {
		style.Bold = parseBold(val)
	}
	if val, ok := props["font-style"]; ok {
		style.Italic = parseItalic(val)
	}
	if val, ok := props["line-height"]; ok {
		if lh, ok := parseLineHeight(val, style.FontSize); ok && lh > 0 {
			style.LineHeight = lh
		}
	}
	if val, ok := props["text-align"]; ok {
		style.Align = parseAlign(val)
	}
	if val, ok := props["text-indent"]; ok {
		if indent, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.TextIndent = indent
		}
	}
	if val, ok := props["color"]; ok {
		if color, ok := parseColorValue(val); ok {
			style.Color = color
			style.HasColor = true
		}
	}
	if val, ok := props["text-decoration"]; ok {
		style.Underline, style.Strike = parseTextDecoration(val)
		if strings.Contains(strings.ToLower(val.Raw), "overline") {
			sr.log.Debug("Ignoring unsupported text-decoration value",
				zap.String("property", "text-decoration"),
				zap.String("unsupported", "overline"),
				zap.String("value", val.Raw),
			)
		}
	}
	if val, ok := props["vertical-align"]; ok {
		style.BaselineShift = parseBaselineShift(val, style.FontSize)
	}
	if val, ok := props["white-space"]; ok {
		style.WhiteSpace = strings.ToLower(strings.TrimSpace(valueKeyword(val)))
	}
	if val, ok := props["width"]; ok {
		if val.Unit == "%" {
			style.WidthPercent = val.Value
		}
	}
	if val, ok := props["min-width"]; ok {
		style.MinWidth = parseDimension(val, style.FontSize)
	}
	if val, ok := props["max-width"]; ok {
		style.MaxWidth = parseDimension(val, style.FontSize)
	}
	if val, ok := props["min-height"]; ok {
		style.MinHeight = parseDimension(val, style.FontSize)
	}
	if val, ok := props["max-height"]; ok {
		style.MaxHeight = parseDimension(val, style.FontSize)
	}
	if val, ok := props["display"]; ok {
		style.Hidden = strings.EqualFold(valueKeyword(val), "none")
	}
	if val, ok := props["page-break-inside"]; ok {
		style.KeepTogether = strings.EqualFold(valueKeyword(val), "avoid")
	}
	if val, ok := props["letter-spacing"]; ok {
		keyword := strings.ToLower(strings.TrimSpace(valueKeyword(val)))
		if keyword == "normal" {
			style.LetterSpacing = 0
		} else if ls, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.LetterSpacing = ls
		}
	}
	if val, ok := props["hyphens"]; ok {
		style.Hyphens = parseHyphensKeyword(val)
	}
	if val, ok := props["-webkit-hyphens"]; ok {
		if style.Hyphens == "" { // don't override unprefixed
			style.Hyphens = parseHyphensKeyword(val)
		}
	}
	if val, ok := props["page-break-before"]; ok {
		style.BreakBefore = parseBreakKeyword(val)
	}
	if val, ok := props["break-before"]; ok {
		style.BreakBefore = parseBreakKeyword(val)
	}
	if val, ok := props["page-break-after"]; ok {
		style.BreakAfter = parseBreakKeyword(val)
	}
	if val, ok := props["break-after"]; ok {
		style.BreakAfter = parseBreakKeyword(val)
	}
	if val, ok := props["orphans"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(val.Raw)); err == nil && n >= 1 {
			style.Orphans = n
		}
	}
	if val, ok := props["widows"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(val.Raw)); err == nil && n >= 1 {
			style.Widows = n
		}
	}
	if val, ok := props["background-color"]; ok {
		if color, ok := parseColorValue(val); ok {
			style.Background = &color
		}
	}

	if val, ok := props["margin-top"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginTop = measure
		}
	}
	if val, ok := props["margin-right"]; ok {
		raw := strings.TrimSpace(strings.ToLower(val.Raw))
		if raw == "auto" {
			style.MarginRightAuto = true
			style.MarginRight = 0
		} else if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginRight = measure
			style.MarginRightAuto = false
		}
	}
	if val, ok := props["margin-bottom"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginBottom = measure
		}
	}
	if val, ok := props["margin-left"]; ok {
		raw := strings.TrimSpace(strings.ToLower(val.Raw))
		if raw == "auto" {
			style.MarginLeftAuto = true
			style.MarginLeft = 0
		} else if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginLeft = measure
			style.MarginLeftAuto = false
		}
	}

	if val, ok := props["padding-top"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.PaddingTop = measure
		}
	}
	if val, ok := props["padding-right"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.PaddingRight = measure
		}
	}
	if val, ok := props["padding-bottom"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.PaddingBottom = measure
		}
	}
	if val, ok := props["padding-left"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.PaddingLeft = measure
		}
	}

	// Log CSS properties that we received but do not handle.
	sr.logUnhandledProperties(props)
}

// handledCSSProperties is the set of CSS properties consumed by applyRule.
// Properties not in this set trigger a debug log when encountered.
var handledCSSProperties = map[string]bool{
	"margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true,
	"padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true,
	"border": true, "background": true, "background-color": true,
	"font-family": true, "font-size": true, "font-weight": true, "font-style": true,
	"line-height": true, "text-align": true, "text-indent": true,
	"color": true, "text-decoration": true, "vertical-align": true,
	"white-space": true, "width": true, "display": true,
	"page-break-inside": true, "page-break-before": true, "page-break-after": true,
	"break-before": true, "break-after": true,
	"orphans": true, "widows": true,
	"letter-spacing": true, "hyphens": true, "-webkit-hyphens": true,
	"min-width": true, "max-width": true, "min-height": true, "max-height": true,
	// Handled elsewhere (font resolution).
	"src": true, "font-display": true, "unicode-range": true,
	// CSS properties that are valid but have no effect in PDF generation.
	"content": true, "height": true,
}

func (sr *styleResolver) logUnhandledProperties(props map[string]css.Value) {
	for name, val := range props {
		if handledCSSProperties[name] {
			continue
		}
		sr.log.Debug("Ignoring unsupported CSS property",
			zap.String("property", name),
			zap.String("value", val.Raw),
		)
	}
}

func applyBoxShorthand(val css.Value, fontSize float64, top, right, bottom, left *float64) {
	parts := strings.Fields(val.Raw)
	if len(parts) == 0 {
		return
	}
	values := make([]float64, 0, len(parts))
	for _, part := range parts {
		parsed, ok := parseRawLength(part, fontSize, fontSize)
		if !ok {
			return
		}
		values = append(values, parsed)
	}
	if len(values) == 1 {
		*top, *right, *bottom, *left = values[0], values[0], values[0], values[0]
		return
	}
	if len(values) == 2 {
		*top, *right, *bottom, *left = values[0], values[1], values[0], values[1]
		return
	}
	if len(values) == 3 {
		*top, *right, *bottom, *left = values[0], values[1], values[2], values[1]
		return
	}
	*top, *right, *bottom, *left = values[0], values[1], values[2], values[3]
}

func parseLength(val css.Value, fontSize float64, percentBase float64) (float64, bool) {
	if val.Raw == "" {
		return 0, false
	}
	return parseRawLength(val.Raw, fontSize, percentBase)
}

func parseRawLength(raw string, fontSize float64, percentBase float64) (float64, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, false
	}
	if raw == "0" || raw == "0px" || raw == "0pt" || raw == "0em" || raw == "0rem" || raw == "0%" {
		return 0, true
	}
	if raw == "auto" {
		return 0, false
	}

	number, unit := splitNumericUnit(raw)
	if number == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, false
	}
	if fontSize <= 0 {
		fontSize = defaultFontSizePt
	}
	if percentBase <= 0 {
		percentBase = fontSize
	}

	switch unit {
	case "", "pt":
		return value, true
	case "px":
		return CSSPxToPt(value), true
	case "em":
		return value * fontSize, true
	case "ex":
		return value * 0.44 * fontSize, true // x-height ≈ 0.44em for most fonts
	case "rem":
		return value * defaultFontSizePt, true
	case "%":
		return percentBase * value / 100.0, true
	case "in":
		return value * 72.0, true
	case "cm":
		return value * 72.0 / 2.54, true
	case "mm":
		return value * 72.0 / 25.4, true
	default:
		return 0, false
	}
}

func splitNumericUnit(raw string) (string, string) {
	idx := 0
	if idx < len(raw) && (raw[idx] == '+' || raw[idx] == '-') {
		idx++
	}
	hasDigits := false
	for idx < len(raw) && raw[idx] >= '0' && raw[idx] <= '9' {
		idx++
		hasDigits = true
	}
	if idx < len(raw) && raw[idx] == '.' {
		idx++
		for idx < len(raw) && raw[idx] >= '0' && raw[idx] <= '9' {
			idx++
			hasDigits = true
		}
	}
	if !hasDigits {
		return "", ""
	}
	return raw[:idx], raw[idx:]
}

func parseLineHeight(val css.Value, fontSize float64) (float64, bool) {
	keyword := strings.ToLower(strings.TrimSpace(valueKeyword(val)))
	if keyword == "normal" {
		return 1.2, true
	}
	if val.Unit == "" && val.IsNumeric() {
		return val.Value, true
	}
	if size, ok := parseLength(val, fontSize, fontSize); ok && fontSize > 0 {
		return size / fontSize, true
	}
	return 0, false
}

func parseAlign(val css.Value) layout.Align {
	switch strings.ToLower(strings.TrimSpace(valueKeyword(val))) {
	case "center":
		return layout.AlignCenter
	case "right":
		return layout.AlignRight
	case "justify":
		return layout.AlignJustify
	default:
		return layout.AlignLeft
	}
}

func parseBold(val css.Value) bool {
	keyword := strings.ToLower(strings.TrimSpace(valueKeyword(val)))
	switch keyword {
	case "bold", "bolder", "600", "700", "800", "900":
		return true
	case "normal", "lighter", "100", "200", "300", "400", "500":
		return false
	}
	if val.IsNumeric() {
		return val.Value >= 600
	}
	return false
}

func parseItalic(val css.Value) bool {
	keyword := strings.ToLower(strings.TrimSpace(valueKeyword(val)))
	return keyword == "italic" || keyword == "oblique"
}

func parseTextDecoration(val css.Value) (underline, strike bool) {
	raw := strings.ToLower(strings.TrimSpace(val.Raw))
	if raw == "none" {
		return false, false
	}
	underline = strings.Contains(raw, "underline")
	strike = strings.Contains(raw, "line-through")
	return underline, strike
}

func parseBaselineShift(val css.Value, fontSize float64) float64 {
	switch strings.ToLower(strings.TrimSpace(valueKeyword(val))) {
	case "super":
		return fontSize * 0.33
	case "sub":
		return -fontSize * 0.2
	default:
		return 0
	}
}

func normalizeFontFamilyValue(val css.Value) string {
	if val.Keyword != "" {
		return strings.TrimSpace(val.Keyword)
	}
	return strings.TrimSpace(val.Raw)
}

func extractPseudoText(val css.Value) string {
	text := valueKeyword(val)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, `"'`)
	return text
}

func valueKeyword(val css.Value) string {
	if val.Keyword != "" {
		return val.Keyword
	}
	return val.Raw
}

func parseColorValue(val css.Value) (layout.Color, bool) {
	return parseColor(strings.TrimSpace(valueKeyword(val)))
}

func parseColor(raw string) (layout.Color, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || strings.HasPrefix(raw, "url(") {
		return layout.ColorBlack, false
	}
	if strings.HasPrefix(raw, "#") {
		hex := strings.TrimPrefix(raw, "#")
		if len(hex) == 3 {
			hex = strings.Repeat(string(hex[0]), 2) + strings.Repeat(string(hex[1]), 2) + strings.Repeat(string(hex[2]), 2)
		}
		if len(hex) == 6 {
			return layout.Hex(hex), true
		}
		return layout.ColorBlack, false
	}

	switch raw {
	case "black":
		return layout.ColorBlack, true
	case "white":
		return layout.ColorWhite, true
	case "red":
		return layout.ColorRed, true
	case "green":
		return layout.ColorGreen, true
	case "blue":
		return layout.ColorBlue, true
	case "gray", "grey":
		return layout.ColorGray, true
	case "lightgray", "lightgrey":
		return layout.ColorLightGray, true
	case "darkgray", "darkgrey":
		return layout.ColorDarkGray, true
	case "yellow":
		return layout.ColorYellow, true
	case "cyan":
		return layout.ColorCyan, true
	case "magenta":
		return layout.ColorMagenta, true
	case "orange":
		return layout.ColorOrange, true
	case "navy":
		return layout.ColorNavy, true
	case "maroon":
		return layout.ColorMaroon, true
	case "purple":
		return layout.ColorPurple, true
	case "teal":
		return layout.ColorTeal, true
	default:
		return layout.ColorBlack, false
	}
}

func parseBorder(raw string) (layout.Border, bool) {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(raw)))
	if len(tokens) == 0 {
		return layout.Border{}, false
	}

	border := layout.DefaultBorder()
	hasWidth := false
	hasColor := false
	hasStyle := false

	for _, token := range tokens {
		if width, ok := parseRawLength(token, defaultFontSizePt, defaultFontSizePt); ok {
			border.Width = width
			hasWidth = true
			continue
		}
		switch token {
		case "solid":
			border.Style = layout.BorderSolid
			hasStyle = true
		case "dashed":
			border.Style = layout.BorderDashed
			hasStyle = true
		case "dotted":
			border.Style = layout.BorderDotted
			hasStyle = true
		case "double":
			border.Style = layout.BorderDouble
			hasStyle = true
		case "none":
			border.Style = layout.BorderNone
			border.Width = 0
			hasStyle = true
		default:
			if color, ok := parseColor(token); ok {
				border.Color = color
				hasColor = true
			}
		}
	}

	if !hasWidth && !hasColor && !hasStyle {
		return layout.Border{}, false
	}
	return border, true
}

// parseFontSize handles font-size CSS values including keywords.
// Relative keywords (smaller/larger) scale relative to parentFontSize.
// Absolute keywords (xx-small through xxx-large) use the CSS absolute
// size scale relative to defaultFontSizePt ("medium").
func parseFontSize(val css.Value, parentFontSize float64) (float64, bool) {
	keyword := strings.ToLower(strings.TrimSpace(valueKeyword(val)))
	switch keyword {
	case "smaller":
		return parentFontSize * (5.0 / 6.0), true // KFX: 0.8333em
	case "larger":
		return parentFontSize * 1.2, true // KFX: 1.2em
	case "xx-small":
		return defaultFontSizePt * (3.0 / 5.0), true
	case "x-small":
		return defaultFontSizePt * (3.0 / 4.0), true
	case "small":
		return defaultFontSizePt * (8.0 / 9.0), true
	case "medium":
		return defaultFontSizePt, true
	case "large":
		return defaultFontSizePt * (6.0 / 5.0), true
	case "x-large":
		return defaultFontSizePt * (3.0 / 2.0), true
	case "xx-large":
		return defaultFontSizePt * 2.0, true
	case "xxx-large":
		return defaultFontSizePt * 3.0, true
	default:
		return parseLength(val, parentFontSize, defaultFontSizePt)
	}
}

// parseDimension parses a CSS length value into a cssDimension.
// Percentage values are stored in Percent; all other lengths in Pt.
func parseDimension(val css.Value, fontSize float64) cssDimension {
	if val.Unit == "%" && val.IsNumeric() {
		return cssDimension{Percent: val.Value}
	}
	if pts, ok := parseLength(val, fontSize, fontSize); ok && pts > 0 {
		return cssDimension{Pt: pts}
	}
	return cssDimension{}
}

// parseHyphensKeyword extracts the hyphens value from a CSS property.
// Returns "none", "manual", "auto", or "" for unrecognised values.
func parseHyphensKeyword(val css.Value) string {
	switch strings.ToLower(strings.TrimSpace(valueKeyword(val))) {
	case "none":
		return "none"
	case "manual":
		return "manual"
	case "auto":
		return "auto"
	default:
		return ""
	}
}

// parseBreakKeyword extracts a page-break / break-before/after value.
// Returns "always" or "avoid"; everything else (auto, unrecognised) returns "".
func parseBreakKeyword(val css.Value) string {
	switch strings.ToLower(strings.TrimSpace(valueKeyword(val))) {
	case "always", "page", "left", "right", "recto", "verso":
		return "always"
	case "avoid":
		return "avoid"
	default:
		return ""
	}
}

// applyMarginShorthand parses the CSS margin shorthand, treating "auto" as
// zero length while recording which sides are auto for later resolution.
// Unlike the generic applyBoxShorthand, this does not abort when a part
// is "auto".
func applyMarginShorthand(val css.Value, fontSize float64, style *resolvedStyle) {
	parts := strings.Fields(val.Raw)
	if len(parts) == 0 {
		return
	}

	type marginPart struct {
		value float64
		auto  bool
		ok    bool
	}
	parse := func(s string) marginPart {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "auto" {
			return marginPart{value: 0, auto: true, ok: true}
		}
		v, ok := parseRawLength(s, fontSize, fontSize)
		return marginPart{value: v, ok: ok}
	}

	var top, right, bottom, left marginPart
	switch len(parts) {
	case 1:
		v := parse(parts[0])
		if !v.ok {
			return
		}
		top, right, bottom, left = v, v, v, v
	case 2:
		v0, v1 := parse(parts[0]), parse(parts[1])
		if !v0.ok || !v1.ok {
			return
		}
		top, bottom = v0, v0
		right, left = v1, v1
	case 3:
		v0, v1, v2 := parse(parts[0]), parse(parts[1]), parse(parts[2])
		if !v0.ok || !v1.ok || !v2.ok {
			return
		}
		top, right, bottom, left = v0, v1, v2, v1
	default:
		v0, v1, v2, v3 := parse(parts[0]), parse(parts[1]), parse(parts[2]), parse(parts[3])
		if !v0.ok || !v1.ok || !v2.ok || !v3.ok {
			return
		}
		top, right, bottom, left = v0, v1, v2, v3
	}

	style.MarginTop = top.value
	style.MarginRight = right.value
	style.MarginBottom = bottom.value
	style.MarginLeft = left.value
	style.MarginLeftAuto = left.auto
	style.MarginRightAuto = right.auto
}

// resolveMarginAuto converts margin-left/right: auto into text alignment,
// matching the KFX MarginAutoTransformer behavior:
//
//   - Both left and right auto → center
//   - Only left auto → right-align
//   - Only right auto → left-align
//   - top/bottom auto → 0 (already handled during parsing)
func resolveMarginAuto(style *resolvedStyle) {
	if !style.MarginLeftAuto && !style.MarginRightAuto {
		return
	}
	if style.MarginLeftAuto && style.MarginRightAuto {
		style.Align = layout.AlignCenter
	} else if style.MarginLeftAuto {
		style.Align = layout.AlignRight
	} else {
		style.Align = layout.AlignLeft
	}
	style.MarginLeft = 0
	style.MarginRight = 0
	style.MarginLeftAuto = false
	style.MarginRightAuto = false
}
