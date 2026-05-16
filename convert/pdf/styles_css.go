package pdf

import (
	"slices"
	"strconv"
	"strings"

	"fbc/css"
)

func applyPDFStyleProperties(style *pdfBlockResolvedStyle, props map[string]css.Value) {
	if style == nil {
		return
	}
	if value, ok := props["font-family"]; ok {
		if family, ok := pdfCSSFontFamily(value); ok {
			style.Paragraph.FontFamily = family
		}
	}
	if value, ok := props["font-weight"]; ok {
		if bold, ok := pdfCSSFontWeightBold(value); ok {
			style.Paragraph.Bold = bold
		}
	}
	if value, ok := props["font-style"]; ok {
		if italic, ok := pdfCSSFontStyleItalic(value); ok {
			style.Paragraph.Italic = italic
		}
	}
	if value, ok := props["color"]; ok {
		if color, ok := pdfCSSColor(value); ok {
			style.Paragraph.Color = color
		}
	}
	if value, ok := props["background-color"]; ok {
		if color, ok := pdfCSSColor(value); ok {
			style.BackgroundColor = color
			style.HasBackground = true
		}
	}
	if value, ok := props["background"]; ok {
		if color, ok := pdfCSSColor(value); ok {
			style.BackgroundColor = color
			style.HasBackground = true
		}
	}
	if value, ok := props["border"]; ok {
		applyPDFBorderShorthand(style, value)
	}
	if value, ok := props["border-width"]; ok {
		if width, ok := pdfCSSBorderWidth(value, style.Paragraph.FontSize); ok {
			style.BorderWidth = width
			style.HasBorder = width > 0
		}
	}
	if value, ok := props["border-color"]; ok {
		if color, ok := pdfCSSColor(value); ok {
			style.BorderColor = color
			style.HasBorder = style.BorderWidth > 0
		}
	}
	if value, ok := props["border-style"]; ok {
		applyPDFBorderStyle(style, value)
	}
	if value, ok := props["text-decoration"]; ok {
		applyPDFTextDecoration(style, value)
	}
	if value, ok := props["font-size"]; ok {
		if points, ok := pdfCSSFontSizePoints(value, style.Paragraph.FontSize); ok {
			style.Paragraph.FontSize = points
		}
	}
	if value, ok := props["line-height"]; ok {
		if points, ok := pdfCSSLineHeightPoints(value, style.Paragraph.FontSize); ok {
			style.Paragraph.LineHeight = points
			style.Paragraph.LineHeightExplicit = true
		}
	}
	if value, ok := props["letter-spacing"]; ok {
		if points, ok := pdfCSSLetterSpacingPoints(value, style.Paragraph.FontSize); ok {
			style.Paragraph.LetterSpacing = points
		}
	}
	if value, ok := props["vertical-align"]; ok {
		if align, ok := pdfCSSVerticalAlign(value); ok {
			style.Paragraph.VerticalAlign = align
		}
	}
	if value, ok := props["white-space"]; ok {
		switch cssKeyword(value) {
		case "pre", "pre-wrap", "break-spaces":
			style.Paragraph.PreserveSpace = true
		case "normal", "nowrap", "pre-line":
			style.Paragraph.PreserveSpace = false
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		lower := strings.ToLower(name)
		if lower != "font-family" && lower != "font-weight" && lower != "font-style" && lower != "color" && lower != "background-color" && lower != "background" && lower != "border" && lower != "border-width" && lower != "border-color" && lower != "border-style" && lower != "text-decoration" && lower != "font-size" && lower != "line-height" && lower != "letter-spacing" && lower != "vertical-align" && lower != "white-space" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		value := props[name]
		switch strings.ToLower(name) {
		case "text-indent":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.Paragraph.FirstLineIndent = points
			}
		case "text-align":
			if align, ok := pdfCSSTextAlign(value); ok {
				style.Paragraph.Align = align
			}
		case "margin":
			applyPDFMarginShorthand(style, value)
		case "margin-top":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.SpaceBefore = points
				style.HasSpaceBefore = true
			}
		case "margin-bottom":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.SpaceAfter = points
				style.HasSpaceAfter = true
			}
		case "margin-left":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.MarginLeft = points
			}
		case "margin-right":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.MarginRight = points
			}
		case "padding":
			applyPDFPaddingShorthand(style, value)
		case "padding-top":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.PaddingTop = points
			}
		case "padding-right":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.PaddingRight = points
			}
		case "padding-bottom":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.PaddingBottom = points
			}
		case "padding-left":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.PaddingLeft = points
			}
		case "width":
			if cssKeyword(value) == "auto" {
				style.Width = pdfBlockLength{}
				style.HasWidth = false
				continue
			}
			if length, ok := pdfCSSBlockLength(value, style.Paragraph.FontSize); ok {
				style.Width = length
				style.HasWidth = true
			}
		case "min-width":
			if cssKeyword(value) == "auto" {
				style.MinWidth = pdfBlockLength{}
				style.HasMinWidth = false
				continue
			}
			if length, ok := pdfCSSBlockLength(value, style.Paragraph.FontSize); ok {
				style.MinWidth = length
				style.HasMinWidth = true
			}
		case "max-width":
			if cssKeyword(value) == "none" {
				style.MaxWidth = pdfBlockLength{}
				style.HasMaxWidth = false
				continue
			}
			if length, ok := pdfCSSBlockLength(value, style.Paragraph.FontSize); ok {
				style.MaxWidth = length
				style.HasMaxWidth = true
			}
		case "page-break-inside", "break-inside":
			if pdfCSSAvoidPageBreakKeyword(value) {
				style.KeepTogether = true
			}
		case "page-break-before", "break-before":
			if pdfCSSForcedPageBreakKeyword(value) {
				style.PageBreakBefore = true
			}
		case "page-break-after", "break-after":
			switch {
			case pdfCSSForcedPageBreakKeyword(value):
				style.PageBreakAfter = true
			case pdfCSSAvoidPageBreakKeyword(value) && style.KeepWithNextLines == 0:
				style.KeepWithNextLines = 1
			}
		case "hyphens", "-webkit-hyphens":
			if hyphenation, ok := pdfCSSHyphenation(value); ok {
				style.Paragraph.Hyphenation = hyphenation
			}
		case "display":
			if cssKeyword(value) == "none" {
				style.Hidden = true
			}
		case "orphans":
			if count, ok := pdfCSSPositiveInt(value); ok {
				style.Orphans = count
			}
		case "widows":
			if count, ok := pdfCSSPositiveInt(value); ok {
				style.Widows = count
			}
		}
	}
}

func applyPDFBorderShorthand(style *pdfBlockResolvedStyle, value css.Value) {
	tokens := strings.Fields(formatCSSValue(value))
	if len(tokens) == 0 {
		return
	}
	style.BorderWidth = 1
	style.BorderColor = pdfColor{}
	style.HasBorder = true
	for _, token := range tokens {
		parsed := parsePDFCSSValueToken(token)
		if width, ok := pdfCSSBorderWidth(parsed, style.Paragraph.FontSize); ok {
			style.BorderWidth = width
			style.HasBorder = width > 0
			continue
		}
		if color, ok := pdfCSSColor(parsed); ok {
			style.BorderColor = color
			continue
		}
		applyPDFBorderStyle(style, parsed)
	}
}

func applyPDFBorderStyle(style *pdfBlockResolvedStyle, value css.Value) {
	switch cssKeyword(value) {
	case "none", "hidden":
		style.HasBorder = false
	case "solid", "dotted", "dashed", "double":
		if style.BorderWidth <= 0 {
			style.BorderWidth = 1
		}
		style.HasBorder = true
	}
}

func pdfCSSBorderWidth(value css.Value, fontSize float64) (float64, bool) {
	switch cssKeyword(value) {
	case "thin":
		return 0.5, true
	case "medium":
		return 1, true
	case "thick":
		return 2, true
	}
	return pdfCSSLengthPoints(value, fontSize)
}

func applyPDFTextDecoration(style *pdfBlockResolvedStyle, value css.Value) {
	decorations := strings.Fields(strings.ToLower(formatCSSValue(value)))
	if len(decorations) == 0 {
		decorations = []string{cssKeyword(value)}
	}
	for _, decoration := range decorations {
		switch strings.TrimSpace(decoration) {
		case "none":
			style.Paragraph.Underline = false
			style.Paragraph.Strikethrough = false
		case "underline":
			style.Paragraph.Underline = true
		case "line-through":
			style.Paragraph.Strikethrough = true
		}
	}
}

func applyPDFMarginShorthand(style *pdfBlockResolvedStyle, value css.Value) {
	top, right, bottom, left, ok := pdfCSSBoxShorthand(value, style.Paragraph.FontSize)
	if !ok {
		return
	}
	style.SpaceBefore = top
	style.HasSpaceBefore = true
	style.SpaceAfter = bottom
	style.HasSpaceAfter = true
	style.MarginLeft = left
	style.MarginRight = right
}

func applyPDFPaddingShorthand(style *pdfBlockResolvedStyle, value css.Value) {
	top, right, bottom, left, ok := pdfCSSBoxShorthand(value, style.Paragraph.FontSize)
	if !ok {
		return
	}
	style.PaddingTop = top
	style.PaddingRight = right
	style.PaddingBottom = bottom
	style.PaddingLeft = left
}

func pdfCSSBoxShorthand(value css.Value, fontSize float64) (float64, float64, float64, float64, bool) {
	tokens := strings.Fields(value.Raw)
	if len(tokens) == 0 && value.Raw == "" {
		tokens = []string{formatCSSValue(value)}
	}
	if len(tokens) == 0 {
		return 0, 0, 0, 0, false
	}
	values := make([]float64, 0, len(tokens))
	for _, token := range tokens {
		points, ok := pdfCSSLengthPoints(parsePDFCSSValueToken(token), fontSize)
		if !ok {
			return 0, 0, 0, 0, false
		}
		values = append(values, points)
	}
	switch len(values) {
	case 1:
		return values[0], values[0], values[0], values[0], true
	case 2:
		return values[0], values[1], values[0], values[1], true
	case 3:
		return values[0], values[1], values[2], values[1], true
	default:
		return values[0], values[1], values[2], values[3], true
	}
}

func pdfCSSFontFamily(value css.Value) (string, bool) {
	raw := strings.TrimSpace(formatCSSValue(value))
	if raw == "" {
		return "", false
	}
	first, _, _ := strings.Cut(raw, ",")
	first = strings.TrimSpace(first)
	first = strings.Trim(first, `"'`)
	if first == "" {
		return "", false
	}
	return first, true
}

func pdfCSSFontWeightBold(value css.Value) (bool, bool) {
	keyword := cssKeyword(value)
	switch keyword {
	case "normal", "regular":
		return false, true
	case "bold", "bolder":
		return true, true
	case "lighter":
		return false, true
	}
	if value.IsNumeric() {
		return value.Value >= 600, true
	}
	return false, false
}

func pdfCSSFontStyleItalic(value css.Value) (bool, bool) {
	switch cssKeyword(value) {
	case "normal":
		return false, true
	case "italic", "oblique":
		return true, true
	default:
		return false, false
	}
}

func pdfCSSFontSizePoints(value css.Value, current float64) (float64, bool) {
	if value.Unit == "%" {
		return pdfBaseFontSize * value.Value / 100, true
	}
	return pdfCSSLengthPoints(value, current)
}

func pdfCSSLineHeightPoints(value css.Value, fontSize float64) (float64, bool) {
	if cssKeyword(value) == "normal" {
		return fontSize * pdfNormalLineHeightFactor, true
	}
	if value.Unit == "" && value.IsNumeric() {
		return fontSize * value.Value, true
	}
	if value.Unit == "%" {
		return fontSize * value.Value / 100, true
	}
	return pdfCSSLengthPoints(value, fontSize)
}

func pdfCSSLetterSpacingPoints(value css.Value, fontSize float64) (float64, bool) {
	if cssKeyword(value) == "normal" {
		return 0, true
	}
	return pdfCSSLengthPoints(value, fontSize)
}

func pdfCSSBlockLength(value css.Value, fontSize float64) (pdfBlockLength, bool) {
	if cssKeyword(value) == "auto" {
		return pdfBlockLength{}, false
	}
	if value.IsNumeric() && value.Unit == "%" {
		return pdfBlockLength{Value: value.Value, Percent: true}, true
	}
	points, ok := pdfCSSLengthPoints(value, fontSize)
	if !ok {
		return pdfBlockLength{}, false
	}
	return pdfBlockLength{Value: points}, true
}

func pdfCSSLengthPoints(value css.Value, fontSize float64) (float64, bool) {
	if !value.IsNumeric() {
		return 0, false
	}
	switch strings.ToLower(value.Unit) {
	case "", "pt":
		return value.Value, true
	case "px":
		return value.Value * pdfPointsPerInch / pdfCSSPixelsPerIn, true
	case "em":
		return value.Value * fontSize, true
	case "rem":
		return value.Value * pdfBaseFontSize, true
	case "%":
		return value.Value * fontSize / 100, true
	case "in":
		return value.Value * pdfPointsPerInch, true
	case "cm":
		return value.Value * pdfPointsPerInch / 2.54, true
	case "mm":
		return value.Value * pdfPointsPerInch / 25.4, true
	default:
		return 0, false
	}
}

func pdfCSSTextAlign(value css.Value) (textAlign, bool) {
	switch cssKeyword(value) {
	case "left", "start":
		return textAlignLeft, true
	case "right", "end":
		return textAlignRight, true
	case "center":
		return textAlignCenter, true
	case "justify":
		return textAlignJustify, true
	default:
		return textAlignLeft, false
	}
}

func pdfCSSVerticalAlign(value css.Value) (textVerticalAlign, bool) {
	switch cssKeyword(value) {
	case "baseline":
		return textVerticalAlignBaseline, true
	case "sub":
		return textVerticalAlignSub, true
	case "super", "sup":
		return textVerticalAlignSuper, true
	default:
		return textVerticalAlignBaseline, false
	}
}

func pdfCSSPositiveInt(value css.Value) (int, bool) {
	if !value.IsNumeric() || value.Value < 1 {
		return 0, false
	}
	return int(value.Value), true
}

func pdfCSSForcedPageBreakKeyword(value css.Value) bool {
	switch cssKeyword(value) {
	case "always", "page", "left", "right":
		return true
	default:
		return false
	}
}

func pdfCSSAvoidPageBreakKeyword(value css.Value) bool {
	switch cssKeyword(value) {
	case "avoid", "avoid-page":
		return true
	default:
		return false
	}
}

func pdfCSSHyphenation(value css.Value) (paragraphHyphenation, bool) {
	switch cssKeyword(value) {
	case "none":
		return paragraphHyphenationNone, true
	case "manual":
		return paragraphHyphenationManual, true
	case "auto":
		return paragraphHyphenationAuto, true
	default:
		return paragraphHyphenationAuto, false
	}
}

func parsePDFCSSValueToken(token string) css.Value {
	token = strings.TrimSpace(token)
	if token == "" {
		return css.Value{}
	}
	valueEnd := 0
	for valueEnd < len(token) {
		ch := token[valueEnd]
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '+' {
			valueEnd++
			continue
		}
		break
	}
	if valueEnd == 0 {
		return css.Value{Raw: token, Keyword: strings.ToLower(token)}
	}
	number, err := strconv.ParseFloat(token[:valueEnd], 64)
	if err != nil {
		return css.Value{Raw: token}
	}
	return css.Value{Raw: token, Value: number, Unit: strings.ToLower(token[valueEnd:])}
}

func cssKeyword(value css.Value) string {
	if value.Keyword != "" {
		return strings.ToLower(value.Keyword)
	}
	return strings.ToLower(strings.TrimSpace(value.Raw))
}

func formatCSSValue(value css.Value) string {
	if value.Raw != "" {
		return value.Raw
	}
	if value.Keyword != "" {
		return value.Keyword
	}
	return strconv.FormatFloat(value.Value, 'f', -1, 64) + value.Unit
}
