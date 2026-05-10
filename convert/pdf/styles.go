package pdf

import (
	"slices"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

const (
	pdfPointsPerInch  = 72.0
	pdfCSSPixelsPerIn = 96.0
	pdfMinBlockWidth  = 12.0

	pdfBaseFontSize            = 10.5
	pdfBaseLineHeight          = 13.4
	pdfBodyIndent              = 14.0
	pdfParagraphSpaceAfter     = pdfBaseFontSize * 0.3
	pdfHeadingBaseFontSize     = 16.0
	pdfHeadingMinFontSize      = 11.0
	pdfHeadingLineHeightFactor = 1.25
	pdfHeadingSpaceBefore      = 10.0
	pdfHeadingSpaceAfter       = 8.0
	pdfSubtitleFontSize        = 11.0
	pdfSubtitleLineHeight      = 14.0
	pdfSubtitleSpaceBefore     = 6.0
	pdfSubtitleSpaceAfter      = 5.0
	pdfVerseLineHeight         = 13.2
	pdfVerseSpaceAfter         = 2.0
	pdfTextAuthorFontSize      = 10.0
	pdfTextAuthorLineHeight    = 12.5
	pdfTextAuthorSpaceAfter    = 4.0
	pdfImageSpace              = 6.0
	pdfTOCIndentPerDepth       = 12.0
	pdfTOCSpaceAfter           = 1.5
	pdfDefaultKeepLines        = 2
	pdfSingleKeepLine          = 1

	pdfStyleParagraph          = "p"
	pdfStyleChapterTitleHeader = "chapter-title-header"
	pdfStyleSectionTitleHeader = "section-title-header"
	pdfStyleSubtitle           = "section-subtitle"
	pdfStyleVerse              = "verse"
	pdfStyleTextAuthor         = "text-author"
	pdfStyleImage              = "image"
	pdfStyleTOCItem            = "toc-item"
	pdfStyleTOCTitle           = "toc-title"
	pdfStyleAnnotationTitle    = "annotation-title"
	pdfStyleEmptyLine          = "emptyline"
)

type pdfBlockResolvedStyle struct {
	Paragraph         paragraphStyle
	SpaceBefore       float64
	SpaceAfter        float64
	MarginLeft        float64
	MarginRight       float64
	KeepTogether      bool
	KeepWithNextLines int
	Orphans           int
	Widows            int
}

type pdfStyleResolver struct {
	styles map[string]pdfBlockResolvedStyle
	tracer *pdfStyleTracer
}

type pdfDebugResolvedStyle struct {
	Name              string  `json:"name"`
	FontSize          float64 `json:"font_size"`
	LineHeight        float64 `json:"line_height"`
	FirstLineIndent   float64 `json:"first_line_indent,omitempty"`
	TextAlign         string  `json:"text_align"`
	SpaceBefore       float64 `json:"space_before,omitempty"`
	SpaceAfter        float64 `json:"space_after,omitempty"`
	MarginLeft        float64 `json:"margin_left,omitempty"`
	MarginRight       float64 `json:"margin_right,omitempty"`
	KeepTogether      bool    `json:"keep_together,omitempty"`
	KeepWithNextLines int     `json:"keep_with_next_lines,omitempty"`
	Orphans           int     `json:"orphans,omitempty"`
	Widows            int     `json:"widows,omitempty"`
}

func (a textAlign) String() string {
	switch a {
	case textAlignLeft:
		return "left"
	case textAlignCenter:
		return "center"
	case textAlignRight:
		return "right"
	case textAlignJustify:
		return "justify"
	default:
		return "unknown"
	}
}

func newPDFStyleResolver(book *fb2.FictionBook, log *zap.Logger, tracers ...*pdfStyleTracer) *pdfStyleResolver {
	if log == nil {
		log = zap.NewNop()
	}
	var tracer *pdfStyleTracer
	if len(tracers) > 0 {
		tracer = tracers[0]
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles(), tracer: tracer}
	resolver.traceDefaults()
	if book == nil {
		return resolver
	}
	parser := css.NewParser(log)
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		resolver.applyStylesheet(parser.Parse([]byte(stylesheet.Data), "pdf stylesheet"))
	}
	return resolver
}

func defaultPDFStyles() map[string]pdfBlockResolvedStyle {
	styles := map[string]pdfBlockResolvedStyle{
		pdfStyleParagraph: {
			Paragraph:  paragraphStyle{FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify},
			SpaceAfter: pdfParagraphSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleChapterTitleHeader: headingPDFStyle(1),
		pdfStyleSectionTitleHeader: headingPDFStyle(2),
		pdfStyleSubtitle: {
			Paragraph:         paragraphStyle{FontSize: pdfSubtitleFontSize, LineHeight: pdfSubtitleLineHeight, Align: textAlignCenter},
			SpaceBefore:       pdfSubtitleSpaceBefore,
			SpaceAfter:        pdfSubtitleSpaceAfter,
			KeepTogether:      true,
			KeepWithNextLines: pdfSingleKeepLine,
		},
		pdfStyleVerse: {
			Paragraph:  paragraphStyle{FontSize: pdfBaseFontSize, LineHeight: pdfVerseLineHeight, Align: textAlignLeft},
			SpaceAfter: pdfVerseSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleTextAuthor: {
			Paragraph:  paragraphStyle{FontSize: pdfTextAuthorFontSize, LineHeight: pdfTextAuthorLineHeight, Align: textAlignRight},
			SpaceAfter: pdfTextAuthorSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleImage: {
			Paragraph:    paragraphStyle{FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight},
			SpaceBefore:  pdfImageSpace,
			SpaceAfter:   pdfImageSpace,
			KeepTogether: true,
		},
		pdfStyleTOCItem: {
			Paragraph:  paragraphStyle{FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft},
			SpaceAfter: pdfTOCSpaceAfter,
			Orphans:    pdfSingleKeepLine,
			Widows:     pdfSingleKeepLine,
		},
		pdfStyleTOCTitle:        headingPDFStyle(1),
		pdfStyleAnnotationTitle: headingPDFStyle(1),
		pdfStyleEmptyLine: {
			Paragraph: paragraphStyle{FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight},
		},
	}
	return styles
}

func (r *pdfStyleResolver) traceDefaults() {
	if r == nil || r.tracer == nil {
		return
	}
	names := make([]string, 0, len(r.styles))
	for name := range r.styles {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		r.tracer.traceDefault(name, r.styles[name])
	}
}

func headingPDFStyle(depth int) pdfBlockResolvedStyle {
	fontSize := max(pdfHeadingBaseFontSize-float64(depth-1), pdfHeadingMinFontSize)
	return pdfBlockResolvedStyle{
		Paragraph:         paragraphStyle{FontSize: fontSize, LineHeight: fontSize * pdfHeadingLineHeightFactor, Align: textAlignCenter},
		SpaceBefore:       pdfHeadingSpaceBefore,
		SpaceAfter:        pdfHeadingSpaceAfter,
		KeepTogether:      true,
		KeepWithNextLines: pdfDefaultKeepLines,
	}
}

func (r *pdfStyleResolver) styleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfStyleNameForBlock(block)
	style, ok := r.styles[name]
	if !ok {
		style = r.styles[pdfStyleParagraph]
	}
	fallback := r.styles[pdfStyleParagraph]
	for _, class := range strings.Fields(block.StyleClasses) {
		classStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		style = mergePDFStyleOverrides(style, classStyle, fallback)
	}
	if block.Kind == pdfBlockTOCEntry {
		style.Paragraph.FirstLineIndent = max(float64(block.Depth-1)*pdfTOCIndentPerDepth, 0)
	}
	return style
}

func mergePDFStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if override.Paragraph.FontSize != fallback.Paragraph.FontSize {
		base.Paragraph.FontSize = override.Paragraph.FontSize
	}
	if override.Paragraph.LineHeight != fallback.Paragraph.LineHeight {
		base.Paragraph.LineHeight = override.Paragraph.LineHeight
	}
	if override.Paragraph.FirstLineIndent != fallback.Paragraph.FirstLineIndent {
		base.Paragraph.FirstLineIndent = override.Paragraph.FirstLineIndent
	}
	if override.Paragraph.Align != fallback.Paragraph.Align {
		base.Paragraph.Align = override.Paragraph.Align
	}
	if override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
	}
	if override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
	}
	if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
	}
	if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
	}
	if override.KeepTogether != fallback.KeepTogether {
		base.KeepTogether = override.KeepTogether
	}
	if override.KeepWithNextLines != fallback.KeepWithNextLines {
		base.KeepWithNextLines = override.KeepWithNextLines
	}
	if override.Orphans != fallback.Orphans {
		base.Orphans = override.Orphans
	}
	if override.Widows != fallback.Widows {
		base.Widows = override.Widows
	}
	return base
}

func pdfStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	return newPDFStyleResolver(nil, nil).styleForBlock(block)
}

func pdfStyleNameForBlock(block pdfTextBlock) string {
	if block.StyleName != "" {
		return block.StyleName
	}
	if block.Kind == pdfBlockHeading {
		return pdfHeadingStyleName(block.Depth)
	}
	return pdfStyleNameForKind(block.Kind)
}

func pdfStyleNameForKind(kind pdfBlockKind) string {
	switch kind {
	case pdfBlockParagraph:
		return pdfStyleParagraph
	case pdfBlockHeading:
		return pdfStyleChapterTitleHeader
	case pdfBlockSubtitle:
		return pdfStyleSubtitle
	case pdfBlockPoem:
		return pdfStyleVerse
	case pdfBlockTextAuthor:
		return pdfStyleTextAuthor
	case pdfBlockImage:
		return pdfStyleImage
	case pdfBlockTOCEntry:
		return pdfStyleTOCItem
	case pdfBlockEmptyLine:
		return pdfStyleEmptyLine
	default:
		return pdfStyleParagraph
	}
}

func pdfHeadingStyleName(depth int) string {
	if depth <= 1 {
		return pdfStyleChapterTitleHeader
	}
	return pdfStyleSectionTitleHeader
}

func blockContentWidth(contentWidth float64, style pdfBlockResolvedStyle) float64 {
	return max(contentWidth-style.MarginLeft-style.MarginRight, pdfMinBlockWidth)
}

func (r *pdfStyleResolver) applyStylesheet(sheet *css.Stylesheet) {
	if sheet == nil {
		return
	}
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			r.applyRule(*item.Rule)
		case item.MediaBlock != nil:
			matched := item.MediaBlock.Query.Evaluate(true, true)
			r.tracer.traceMedia(item.MediaBlock.Query.Raw, matched, len(item.MediaBlock.Rules))
			if !matched {
				continue
			}
			for _, rule := range item.MediaBlock.Rules {
				r.applyRule(rule)
			}
		}
	}
}

func (r *pdfStyleResolver) applyRule(rule css.Rule) {
	mapped := pdfSelectorStyleNames(rule.Selector)
	if len(mapped) == 0 {
		r.tracer.traceIgnoredRule(rule.Selector, rule.Properties)
		return
	}
	r.tracer.traceRule(rule.Selector, rule.Properties, mapped)
	for _, name := range mapped {
		style, ok := r.styles[name]
		if !ok {
			style = r.styles[pdfStyleParagraph]
		}
		before := style
		applyPDFStyleProperties(&style, rule.Properties)
		r.tracer.traceStyleUpdate(name, before, style)
		r.styles[name] = style
	}
}

func pdfSelectorStyleNames(sel css.Selector) []string {
	if sel.Class != "" {
		return []string{sel.Class}
	}
	switch strings.ToLower(sel.Element) {
	case "p":
		return []string{pdfStyleParagraph}
	case "h1":
		return []string{pdfStyleChapterTitleHeader, pdfStyleTOCTitle, pdfStyleAnnotationTitle}
	case "h2", "h3", "h4", "h5", "h6":
		return []string{pdfStyleSectionTitleHeader}
	case "img":
		return []string{pdfStyleImage}
	default:
		return nil
	}
}

func applyPDFStyleProperties(style *pdfBlockResolvedStyle, props map[string]css.Value) {
	if style == nil {
		return
	}
	if value, ok := props["font-size"]; ok {
		if points, ok := pdfCSSFontSizePoints(value, style.Paragraph.FontSize); ok {
			ratio := points / style.Paragraph.FontSize
			style.Paragraph.FontSize = points
			style.Paragraph.LineHeight *= ratio
		}
	}
	if value, ok := props["line-height"]; ok {
		if points, ok := pdfCSSLineHeightPoints(value, style.Paragraph.FontSize); ok {
			style.Paragraph.LineHeight = points
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		lower := strings.ToLower(name)
		if lower != "font-size" && lower != "line-height" {
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
			}
		case "margin-bottom":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.SpaceAfter = points
			}
		case "margin-left":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.MarginLeft = points
			}
		case "margin-right":
			if points, ok := pdfCSSLengthPoints(value, style.Paragraph.FontSize); ok {
				style.MarginRight = points
			}
		case "page-break-inside", "break-inside":
			if cssKeyword(value) == "avoid" {
				style.KeepTogether = true
			}
		case "page-break-after", "break-after":
			if cssKeyword(value) == "avoid" && style.KeepWithNextLines == 0 {
				style.KeepWithNextLines = 1
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

func applyPDFMarginShorthand(style *pdfBlockResolvedStyle, value css.Value) {
	tokens := strings.Fields(value.Raw)
	if len(tokens) == 0 && value.Raw == "" {
		tokens = []string{formatCSSValue(value)}
	}
	if len(tokens) == 0 {
		return
	}
	values := make([]float64, 0, len(tokens))
	for _, token := range tokens {
		points, ok := pdfCSSLengthPoints(parsePDFCSSValueToken(token), style.Paragraph.FontSize)
		if !ok {
			return
		}
		values = append(values, points)
	}
	var top, right, bottom, left float64
	switch len(values) {
	case 1:
		top, right, bottom, left = values[0], values[0], values[0], values[0]
	case 2:
		top, right, bottom, left = values[0], values[1], values[0], values[1]
	case 3:
		top, right, bottom, left = values[0], values[1], values[2], values[1]
	default:
		top, right, bottom, left = values[0], values[1], values[2], values[3]
	}
	style.SpaceBefore = top
	style.SpaceAfter = bottom
	style.MarginLeft = left
	style.MarginRight = right
}

func pdfCSSFontSizePoints(value css.Value, current float64) (float64, bool) {
	if value.Unit == "%" {
		return pdfBaseFontSize * value.Value / 100, true
	}
	return pdfCSSLengthPoints(value, current)
}

func pdfCSSLineHeightPoints(value css.Value, fontSize float64) (float64, bool) {
	if value.Unit == "" && value.IsNumeric() {
		return fontSize * value.Value, true
	}
	if value.Unit == "%" {
		return fontSize * value.Value / 100, true
	}
	return pdfCSSLengthPoints(value, fontSize)
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

func pdfCSSPositiveInt(value css.Value) (int, bool) {
	if !value.IsNumeric() || value.Value < 1 {
		return 0, false
	}
	return int(value.Value), true
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

func (r *pdfStyleResolver) debugStyles() []pdfDebugResolvedStyle {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.styles))
	for name := range r.styles {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]pdfDebugResolvedStyle, 0, len(names))
	for _, name := range names {
		style := r.styles[name]
		out = append(out, pdfDebugResolvedStyle{
			Name:              name,
			FontSize:          style.Paragraph.FontSize,
			LineHeight:        style.Paragraph.LineHeight,
			FirstLineIndent:   style.Paragraph.FirstLineIndent,
			TextAlign:         style.Paragraph.Align.String(),
			SpaceBefore:       style.SpaceBefore,
			SpaceAfter:        style.SpaceAfter,
			MarginLeft:        style.MarginLeft,
			MarginRight:       style.MarginRight,
			KeepTogether:      style.KeepTogether,
			KeepWithNextLines: style.KeepWithNextLines,
			Orphans:           style.Orphans,
			Widows:            style.Widows,
		})
	}
	return out
}
