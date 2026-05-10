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
	pdfStyleAnnotation         = "annotation"
	pdfStyleAnnotationTitle    = "annotation-title"
	pdfStyleAnnotationSubtitle = "annotation-subtitle"
	pdfStylePoem               = "poem"
	pdfStylePoemSubtitle       = "poem-subtitle"
	pdfStyleStanzaSubtitle     = "stanza-subtitle"
	pdfStyleEpigraph           = "epigraph"
	pdfStyleEpigraphSubtitle   = "epigraph-subtitle"
	pdfStyleCite               = "cite"
	pdfStyleCiteSubtitle       = "cite-subtitle"
	pdfStyleTable              = "table"
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
	PageBreakBefore   bool
	PageBreakAfter    bool
	Hidden            bool
	Orphans           int
	Widows            int
}

type pdfStyleResolver struct {
	styles map[string]pdfBlockResolvedStyle
	tracer *pdfStyleTracer
}

type pdfDebugResolvedStyle struct {
	Name              string  `json:"name"`
	FontFamily        string  `json:"font_family,omitempty"`
	FontWeight        string  `json:"font_weight,omitempty"`
	FontStyle         string  `json:"font_style,omitempty"`
	FontSize          float64 `json:"font_size"`
	LineHeight        float64 `json:"line_height"`
	FirstLineIndent   float64 `json:"first_line_indent,omitempty"`
	TextAlign         string  `json:"text_align"`
	Color             string  `json:"color,omitempty"`
	Underline         bool    `json:"underline,omitempty"`
	Strikethrough     bool    `json:"strikethrough,omitempty"`
	SpaceBefore       float64 `json:"space_before,omitempty"`
	SpaceAfter        float64 `json:"space_after,omitempty"`
	MarginLeft        float64 `json:"margin_left,omitempty"`
	MarginRight       float64 `json:"margin_right,omitempty"`
	Hyphenation       string  `json:"hyphenation,omitempty"`
	KeepTogether      bool    `json:"keep_together,omitempty"`
	KeepWithNextLines int     `json:"keep_with_next_lines,omitempty"`
	PageBreakBefore   bool    `json:"page_break_before,omitempty"`
	PageBreakAfter    bool    `json:"page_break_after,omitempty"`
	Hidden            bool    `json:"hidden,omitempty"`
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
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfParagraphSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleChapterTitleHeader: headingPDFStyle(1),
		pdfStyleSectionTitleHeader: headingPDFStyle(2),
		pdfStyleSubtitle: {
			Paragraph:         paragraphStyle{FontFamily: "serif", Bold: true, FontSize: pdfSubtitleFontSize, LineHeight: pdfSubtitleLineHeight, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore:       pdfSubtitleSpaceBefore,
			SpaceAfter:        pdfSubtitleSpaceAfter,
			KeepTogether:      true,
			KeepWithNextLines: pdfSingleKeepLine,
		},
		pdfStyleVerse: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfVerseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfVerseSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleTextAuthor: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfTextAuthorFontSize, LineHeight: pdfTextAuthorLineHeight, Align: textAlignRight, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTextAuthorSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleImage: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore:  pdfImageSpace,
			SpaceAfter:   pdfImageSpace,
			KeepTogether: true,
		},
		pdfStyleTOCItem: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTOCSpaceAfter,
			Orphans:    pdfSingleKeepLine,
			Widows:     pdfSingleKeepLine,
		},
		pdfStyleTOCTitle:        headingPDFStyle(1),
		pdfStyleAnnotationTitle: headingPDFStyle(1),
		pdfStyleEmptyLine: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
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
		Paragraph:         paragraphStyle{FontFamily: "serif", Bold: true, FontSize: fontSize, LineHeight: fontSize * pdfHeadingLineHeightFactor, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
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
	if override.Paragraph.FontFamily != fallback.Paragraph.FontFamily {
		base.Paragraph.FontFamily = override.Paragraph.FontFamily
	}
	if override.Paragraph.Bold != fallback.Paragraph.Bold {
		base.Paragraph.Bold = override.Paragraph.Bold
	}
	if override.Paragraph.Italic != fallback.Paragraph.Italic {
		base.Paragraph.Italic = override.Paragraph.Italic
	}
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
	if override.Paragraph.Color != fallback.Paragraph.Color {
		base.Paragraph.Color = override.Paragraph.Color
	}
	if override.Paragraph.Underline != fallback.Paragraph.Underline {
		base.Paragraph.Underline = override.Paragraph.Underline
	}
	if override.Paragraph.Strikethrough != fallback.Paragraph.Strikethrough {
		base.Paragraph.Strikethrough = override.Paragraph.Strikethrough
	}
	if override.Paragraph.Hyphenation != fallback.Paragraph.Hyphenation {
		base.Paragraph.Hyphenation = override.Paragraph.Hyphenation
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
	if override.PageBreakBefore != fallback.PageBreakBefore {
		base.PageBreakBefore = override.PageBreakBefore
	}
	if override.PageBreakAfter != fallback.PageBreakAfter {
		base.PageBreakAfter = override.PageBreakAfter
	}
	if override.Hidden != fallback.Hidden {
		base.Hidden = override.Hidden
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
	case "table":
		return []string{pdfStyleTable}
	default:
		return nil
	}
}

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
	if value, ok := props["text-decoration"]; ok {
		applyPDFTextDecoration(style, value)
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
		if lower != "font-family" && lower != "font-weight" && lower != "font-style" && lower != "color" && lower != "text-decoration" && lower != "font-size" && lower != "line-height" {
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

func pdfHyphenationString(mode paragraphHyphenation) string {
	switch mode {
	case paragraphHyphenationNone:
		return "none"
	case paragraphHyphenationManual:
		return "manual"
	case paragraphHyphenationAuto:
		return "auto"
	default:
		return "auto"
	}
}

func pdfCSSFontWeightString(bold bool) string {
	if bold {
		return "bold"
	}
	return "normal"
}

func pdfCSSFontStyleString(italic bool) string {
	if italic {
		return "italic"
	}
	return "normal"
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
			FontFamily:        normalizedPDFFontFamily(style.Paragraph.FontFamily),
			FontWeight:        pdfCSSFontWeightString(style.Paragraph.Bold),
			FontStyle:         pdfCSSFontStyleString(style.Paragraph.Italic),
			FontSize:          style.Paragraph.FontSize,
			LineHeight:        style.Paragraph.LineHeight,
			FirstLineIndent:   style.Paragraph.FirstLineIndent,
			TextAlign:         style.Paragraph.Align.String(),
			Color:             style.Paragraph.Color.String(),
			Underline:         style.Paragraph.Underline,
			Strikethrough:     style.Paragraph.Strikethrough,
			Hyphenation:       pdfHyphenationString(style.Paragraph.Hyphenation),
			SpaceBefore:       style.SpaceBefore,
			SpaceAfter:        style.SpaceAfter,
			MarginLeft:        style.MarginLeft,
			MarginRight:       style.MarginRight,
			KeepTogether:      style.KeepTogether,
			KeepWithNextLines: style.KeepWithNextLines,
			PageBreakBefore:   style.PageBreakBefore,
			PageBreakAfter:    style.PageBreakAfter,
			Hidden:            style.Hidden,
			Orphans:           style.Orphans,
			Widows:            style.Widows,
		})
	}
	return out
}
