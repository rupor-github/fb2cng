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

	Color     layout.Color
	HasColor  bool
	Underline bool
	Strike    bool

	BaselineShift float64

	MarginTop    float64
	MarginRight  float64
	MarginBottom float64
	MarginLeft   float64

	PaddingTop    float64
	PaddingRight  float64
	PaddingBottom float64
	PaddingLeft   float64

	Background *layout.Color
	Border     layout.Border
	HasBorder  bool

	KeepTogether bool
	Hidden       bool
	WhiteSpace   string
	WidthPercent float64
}

func defaultResolvedStyle() resolvedStyle {
	return resolvedStyle{
		FontFamily: "serif",
		FontSize:   defaultFontSizePt,
		LineHeight: 1.2,
		Align:      layout.AlignLeft,
		Color:      layout.ColorBlack,
		HasColor:   true,
	}
}

func (s resolvedStyle) inheritedOnly() resolvedStyle {
	return resolvedStyle{
		FontFamily: s.FontFamily,
		FontSize:   s.FontSize,
		LineHeight: s.LineHeight,
		Bold:       s.Bold,
		Italic:     s.Italic,
		Align:      s.Align,
		TextIndent: s.TextIndent,
		Color:      s.Color,
		HasColor:   s.HasColor,
		Underline:  s.Underline,
		Strike:     s.Strike,
		WhiteSpace: s.WhiteSpace,
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

	classList := splitClasses(classes)
	current := styleScope{Tag: tag, Classes: classList}
	matched := sr.matchRules(sr.rules, current, ancestors)
	for _, rule := range matched {
		sr.applyRule(&style, parent, rule)
	}

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
		applyBoxShorthand(val, style.FontSize, &style.MarginTop, &style.MarginRight, &style.MarginBottom, &style.MarginLeft)
	}
	if val, ok := props["padding"]; ok {
		applyBoxShorthand(val, style.FontSize, &style.PaddingTop, &style.PaddingRight, &style.PaddingBottom, &style.PaddingLeft)
	}
	if val, ok := props["border"]; ok {
		if border, ok := parseBorder(val.Raw); ok {
			style.Border = border
			style.HasBorder = true
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
		if size, ok := parseLength(val, parent.FontSize, defaultFontSizePt); ok && size > 0 {
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
	if val, ok := props["display"]; ok {
		style.Hidden = strings.EqualFold(valueKeyword(val), "none")
	}
	if val, ok := props["page-break-inside"]; ok {
		style.KeepTogether = strings.EqualFold(valueKeyword(val), "avoid")
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
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginRight = measure
		}
	}
	if val, ok := props["margin-bottom"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginBottom = measure
		}
	}
	if val, ok := props["margin-left"]; ok {
		if measure, ok := parseLength(val, style.FontSize, style.FontSize); ok {
			style.MarginLeft = measure
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
		return value * 72.0 / defaultScreenDPI, true
	case "em":
		return value * fontSize, true
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
