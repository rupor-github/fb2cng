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
	pdfPointsPerInch     = 72.0
	pdfCSSPixelsPerIn    = 96.0
	pdfKP3ContentWidthPx = 512.0
	pdfMinBlockWidth     = 12.0

	pdfBaseFontSize               = 10.5
	pdfBaseLineHeight             = 13.4
	pdfBodyIndent                 = 14.0
	pdfParagraphSpaceAfter        = pdfBaseFontSize * 0.3
	pdfHeadingBaseFontSize        = 16.0
	pdfHeadingMinFontSize         = 11.0
	pdfHeadingLineHeightFactor    = 1.25
	pdfHeadingSpaceBefore         = 10.0
	pdfHeadingSpaceAfter          = 8.0
	pdfTitleFirstSpaceBefore      = pdfBaseLineHeight * 1.7
	pdfTitleAfterImageSpaceBefore = pdfBaseLineHeight * 1.5
	pdfSubtitleFontSize           = 11.0
	pdfSubtitleLineHeight         = 14.0
	pdfSubtitleSpaceBefore        = 6.0
	pdfSubtitleSpaceAfter         = 5.0
	pdfVerseLineHeight            = 13.2
	pdfVerseSpaceAfter            = 2.0
	pdfTextAuthorFontSize         = 10.0
	pdfTextAuthorLineHeight       = 12.5
	pdfTextAuthorSpaceAfter       = 4.0
	pdfTOCIndentPerDepth          = 12.0
	pdfTOCSpaceAfter              = 1.5
	pdfDefaultKeepLines           = 2
	pdfSingleKeepLine             = 1

	pdfStyleParagraph          = "p"
	pdfStyleBodyTitle          = "body-title"
	pdfStyleChapterTitle       = "chapter-title"
	pdfStyleSectionTitle       = "section-title"
	pdfStyleSectionTitleH2     = "section-title-h2"
	pdfStyleBodyTitleHeader    = "body-title-header"
	pdfStyleChapterTitleHeader = "chapter-title-header"
	pdfStyleSectionTitleHeader = "section-title-header"
	pdfStyleSubtitle           = "section-subtitle"
	pdfStyleVerse              = "verse"
	pdfStyleTextAuthor         = "text-author"
	pdfStyleImage              = "image"
	pdfStyleHeadingImage       = "heading-image"
	pdfStyleTOCItem            = "toc-item"
	pdfStyleTOCTitle           = "toc-title"
	pdfStyleAnnotation         = "annotation"
	pdfStyleAnnotationTitle    = "annotation-title"
	pdfStyleAnnotationSubtitle = "annotation-subtitle"
	pdfStyleFootnote           = "footnote"
	pdfStyleFootnoteTitle      = "footnote-title"
	pdfStylePoem               = "poem"
	pdfStylePoemTitle          = "poem-title"
	pdfStylePoemSubtitle       = "poem-subtitle"
	pdfStyleStanza             = "stanza"
	pdfStyleStanzaTitle        = "stanza-title"
	pdfStyleStanzaSubtitle     = "stanza-subtitle"
	pdfStyleEpigraph           = "epigraph"
	pdfStyleEpigraphSubtitle   = "epigraph-subtitle"
	pdfStyleCite               = "cite"
	pdfStyleCiteSubtitle       = "cite-subtitle"
	pdfStyleTable              = "table"
	pdfStyleCode               = "code"
	pdfStyleEmptyLine          = "emptyline"
	pdfStyleLinkExternal       = "link-external"
	pdfStyleLinkInternal       = "link-internal"
	pdfStyleLinkFootnote       = "link-footnote"
	pdfStyleLinkTOC            = "link-toc"
	pdfStyleTitleAfterImage    = "title-after-image"
	pdfStyleHTML               = "__html__"
	pdfStyleBody               = "__body__"
	pdfStylePage               = "__page__"
)

type pdfBlockResolvedStyle struct {
	Paragraph         paragraphStyle
	SpaceBefore       float64
	HasSpaceBefore    bool
	SpaceAfter        float64
	HasSpaceAfter     bool
	MarginLeft        float64
	MarginRight       float64
	PaddingTop        float64
	PaddingRight      float64
	PaddingBottom     float64
	PaddingLeft       float64
	Width             pdfBlockLength
	HasWidth          bool
	MinWidth          pdfBlockLength
	HasMinWidth       bool
	MaxWidth          pdfBlockLength
	HasMaxWidth       bool
	BackgroundColor   pdfColor
	HasBackground     bool
	BorderWidth       float64
	BorderColor       pdfColor
	HasBorder         bool
	KeepTogether      bool
	KeepWithNextLines int
	PageBreakBefore   bool
	PageBreakAfter    bool
	Hidden            bool
	Orphans           int
	Widows            int
}

type pdfBlockLength struct {
	Value   float64
	Percent bool
}

type pdfStyleResolver struct {
	styles   map[string]pdfBlockResolvedStyle
	defaults map[string]pdfBlockResolvedStyle
	tracer   *pdfStyleTracer
}

type pdfDebugResolvedStyle struct {
	Name              string  `json:"name"`
	FontFamily        string  `json:"font_family,omitempty"`
	FontWeight        string  `json:"font_weight,omitempty"`
	FontStyle         string  `json:"font_style,omitempty"`
	FontSize          float64 `json:"font_size"`
	LineHeight        float64 `json:"line_height"`
	LetterSpacing     float64 `json:"letter_spacing,omitempty"`
	FirstLineIndent   float64 `json:"first_line_indent,omitempty"`
	TextAlign         string  `json:"text_align"`
	VerticalAlign     string  `json:"vertical_align,omitempty"`
	Color             string  `json:"color,omitempty"`
	Underline         bool    `json:"underline,omitempty"`
	Strikethrough     bool    `json:"strikethrough,omitempty"`
	PreserveSpace     bool    `json:"preserve_space,omitempty"`
	SpaceBefore       float64 `json:"space_before,omitempty"`
	SpaceAfter        float64 `json:"space_after,omitempty"`
	MarginLeft        float64 `json:"margin_left,omitempty"`
	MarginRight       float64 `json:"margin_right,omitempty"`
	PaddingTop        float64 `json:"padding_top,omitempty"`
	PaddingRight      float64 `json:"padding_right,omitempty"`
	PaddingBottom     float64 `json:"padding_bottom,omitempty"`
	PaddingLeft       float64 `json:"padding_left,omitempty"`
	Width             string  `json:"width,omitempty"`
	MinWidth          string  `json:"min_width,omitempty"`
	MaxWidth          string  `json:"max_width,omitempty"`
	BackgroundColor   string  `json:"background_color,omitempty"`
	BorderWidth       float64 `json:"border_width,omitempty"`
	BorderColor       string  `json:"border_color,omitempty"`
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

func (a textVerticalAlign) String() string {
	switch a {
	case textVerticalAlignBaseline:
		return "baseline"
	case textVerticalAlignSub:
		return "sub"
	case textVerticalAlignSuper:
		return "super"
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
	defaults := defaultPDFStyles()
	resolver := &pdfStyleResolver{styles: clonePDFStyles(defaults), defaults: defaults, tracer: tracer}
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
	resolver.applyPDFStyleAdjustments()
	return resolver
}

func defaultPDFStyles() map[string]pdfBlockResolvedStyle {
	styles := map[string]pdfBlockResolvedStyle{
		pdfStyleHTML: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStyleBody: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStylePage: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStyleParagraph: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfParagraphSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleBodyTitleHeader:    headingPDFStyle(1),
		pdfStyleChapterTitleHeader: headingPDFStyle(1),
		pdfStyleSectionTitleHeader: headingPDFStyle(2),
		pdfStyleSubtitle:           subtitlePDFStyle(textAlignCenter, true, false, pdfSubtitleSpaceBefore, pdfSubtitleSpaceAfter, true),
		pdfStyleAnnotationSubtitle: subtitlePDFStyle(textAlignCenter, true, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
		pdfStylePoemSubtitle:       subtitlePDFStyle(textAlignCenter, false, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
		pdfStyleStanzaSubtitle:     subtitlePDFStyle(textAlignCenter, false, false, pdfBaseFontSize*0.25, pdfBaseFontSize*0.25, false),
		pdfStyleEpigraphSubtitle:   subtitlePDFStyle(textAlignRight, false, true, pdfBaseFontSize*0.3, pdfBaseFontSize*0.3, false),
		pdfStyleCiteSubtitle:       subtitlePDFStyle(textAlignLeft, false, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
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
			KeepTogether: true,
		},
		pdfStyleTOCItem: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTOCSpaceAfter,
			Orphans:    pdfSingleKeepLine,
			Widows:     pdfSingleKeepLine,
		},
		pdfStyleCode: {
			Paragraph: paragraphStyle{FontFamily: "monospace", FontSize: pdfBaseFontSize * 0.70, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, PreserveSpace: true, Hyphenation: paragraphHyphenationNone},
			Orphans:   pdfSingleKeepLine,
			Widows:    pdfSingleKeepLine,
		},
		pdfStyleTOCTitle:        headingPDFStyle(1),
		pdfStyleAnnotationTitle: headingPDFStyle(1),
		pdfStyleEmptyLine: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
		},
	}
	styles[pdfStyleBodyTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleBodyTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	titleAfterImageStyle := styles[pdfStyleParagraph]
	titleAfterImageStyle.SpaceBefore = pdfTitleAfterImageSpaceBefore
	styles[pdfStyleTitleAfterImage] = titleAfterImageStyle

	linkStyle := styles[pdfStyleParagraph]
	linkStyle.Paragraph.Underline = true
	styles[pdfStyleLinkExternal] = linkStyle
	styles[pdfStyleLinkInternal] = linkStyle
	styles[pdfStyleLinkFootnote] = linkStyle
	styles[pdfStyleLinkTOC] = linkStyle
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

func (r *pdfStyleResolver) applyPDFStyleAdjustments() {
	if r == nil {
		return
	}
	for _, name := range []string{
		pdfStylePoemTitle + "-first",
		pdfStylePoemTitle + "-next",
		pdfStyleStanzaTitle + "-first",
		pdfStyleStanzaTitle + "-next",
		pdfStyleFootnoteTitle + "-first",
		pdfStyleFootnoteTitle + "-next",
	} {
		style, ok := r.styles[name]
		if !ok {
			continue
		}
		style.SpaceBefore = r.defaultStyle(pdfStyleParagraph).SpaceBefore
		style.HasSpaceBefore = false
		style.SpaceAfter = r.defaultStyle(pdfStyleParagraph).SpaceAfter
		style.HasSpaceAfter = false
		r.styles[name] = style
	}
	if style, ok := r.styles[pdfStyleCode]; ok {
		style.Paragraph.Align = r.namedStyle(pdfStyleParagraph).Paragraph.Align
		r.styles[pdfStyleCode] = style
	}
	if style, ok := r.styles[pdfStyleFootnoteTitle]; ok {
		style.Paragraph.Align = r.namedStyle(pdfStyleParagraph).Paragraph.Align
		r.styles[pdfStyleFootnoteTitle] = style
	}
}

func subtitlePDFStyle(align textAlign, bold bool, italic bool, spaceBefore float64, spaceAfter float64, keepWithNext bool) pdfBlockResolvedStyle {
	style := pdfBlockResolvedStyle{
		Paragraph:    paragraphStyle{FontFamily: "serif", Bold: bold, Italic: italic, FontSize: pdfSubtitleFontSize, LineHeight: pdfSubtitleLineHeight, Align: align, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore:  spaceBefore,
		SpaceAfter:   spaceAfter,
		KeepTogether: true,
	}
	if keepWithNext {
		style.KeepWithNextLines = pdfSingleKeepLine
	}
	return style
}

func titleHeaderFirstVariantPDFStyle(base pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	base.SpaceBefore = pdfTitleFirstSpaceBefore
	base.HasSpaceBefore = true
	base.SpaceAfter = 0
	base.HasSpaceAfter = true
	return base
}

func titleHeaderNextVariantPDFStyle(base pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	base.SpaceBefore = 0
	base.HasSpaceBefore = true
	base.SpaceAfter = 0
	base.HasSpaceAfter = true
	return base
}

func (r *pdfStyleResolver) styleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfStyleNameForBlock(block)
	defaultStyle := r.defaultStyle(name)
	tagStyle := r.tagStyleForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(defaultStyle)
	style = mergePDFStyleOverrides(style, r.namedStyle(name), defaultStyle)
	style = r.applyContextInheritedBlockDefaults(style, tagStyle, block)
	classFallback := r.namedStyle(pdfStyleParagraph)
	for _, class := range strings.Fields(block.StyleClasses) {
		classStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		if contextStyleClassShouldSkipInheritedAndHorizontalMargins(block, class) {
			style = mergePDFContextClassStyleOverrides(style, classStyle, classFallback)
			continue
		}
		style = mergePDFStyleOverrides(style, classStyle, classFallback)
	}
	for _, selectorStyleName := range pdfElementClassStyleNames(block) {
		selectorStyle, ok := r.styles[selectorStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverrides(style, selectorStyle, classFallback)
	}
	for _, descStyleName := range r.contextDescendantStyleNames(block) {
		descStyle, ok := r.styles[descStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverrides(style, descStyle, classFallback)
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
	base.Paragraph = mergePDFLineHeightOverride(base.Paragraph, override.Paragraph, fallback.Paragraph)
	if override.Paragraph.LetterSpacing != fallback.Paragraph.LetterSpacing {
		base.Paragraph.LetterSpacing = override.Paragraph.LetterSpacing
	}
	if override.Paragraph.FirstLineIndent != fallback.Paragraph.FirstLineIndent {
		base.Paragraph.FirstLineIndent = override.Paragraph.FirstLineIndent
	}
	if override.Paragraph.Align != fallback.Paragraph.Align {
		base.Paragraph.Align = override.Paragraph.Align
	}
	if override.Paragraph.VerticalAlign != fallback.Paragraph.VerticalAlign {
		base.Paragraph.VerticalAlign = override.Paragraph.VerticalAlign
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
	if override.Paragraph.PreserveSpace != fallback.Paragraph.PreserveSpace {
		base.Paragraph.PreserveSpace = override.Paragraph.PreserveSpace
	}
	if override.Paragraph.Hyphenation != fallback.Paragraph.Hyphenation {
		base.Paragraph.Hyphenation = override.Paragraph.Hyphenation
	}
	if override.HasSpaceBefore || override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
	}
	if override.HasSpaceAfter || override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
	}
	if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
	}
	if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
	}
	if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
	}
	if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
	}
	if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
	}
	if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
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

func clonePDFStyles(src map[string]pdfBlockResolvedStyle) map[string]pdfBlockResolvedStyle {
	cloned := make(map[string]pdfBlockResolvedStyle, len(src))
	for name, style := range src {
		cloned[name] = style
	}
	return cloned
}

func (r *pdfStyleResolver) namedStyle(name string) pdfBlockResolvedStyle {
	if r != nil {
		if style, ok := r.styles[name]; ok {
			return style
		}
	}
	if r != nil {
		if style, ok := r.styles[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) defaultStyle(name string) pdfBlockResolvedStyle {
	if r != nil && r.defaults != nil {
		if style, ok := r.defaults[name]; ok {
			return style
		}
		if style, ok := r.defaults[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) rootParagraphStyle() paragraphStyle {
	base := r.defaultStyle(pdfStyleHTML).Paragraph
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleHTML).Paragraph, r.defaultStyle(pdfStyleHTML).Paragraph)
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleBody).Paragraph, rootDefault)
	return base
}

func (r *pdfStyleResolver) tagStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfTagStyleNameForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(r.defaultStyle(name))
	return mergePDFStyleOverrides(style, r.namedStyle(name), r.defaultStyle(name))
}

func (r *pdfStyleResolver) contextInheritedBlockStyle(tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	style := tagStyle
	var (
		left     float64
		right    float64
		hasLeft  bool
		hasRight bool
	)
	for _, class := range strings.Fields(block.ContextClasses) {
		contextStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		fallback := r.defaultStyle(class)
		style.Paragraph = mergePDFInheritedParagraphStyle(style.Paragraph, contextStyle.Paragraph, fallback.Paragraph)
		if contextStyle.MarginLeft != fallback.MarginLeft {
			left += contextStyle.MarginLeft
			hasLeft = true
		}
		if contextStyle.MarginRight != fallback.MarginRight {
			right += contextStyle.MarginRight
			hasRight = true
		}
	}
	if hasLeft {
		style.MarginLeft = left
	}
	if hasRight {
		style.MarginRight = right
	}
	return style
}

func (r *pdfStyleResolver) applyContextInheritedBlockDefaults(style, tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	contextStyle := r.contextInheritedBlockStyle(tagStyle, block)
	if style.Paragraph.FontFamily == tagStyle.Paragraph.FontFamily {
		style.Paragraph.FontFamily = contextStyle.Paragraph.FontFamily
	}
	if style.Paragraph.Bold == tagStyle.Paragraph.Bold {
		style.Paragraph.Bold = contextStyle.Paragraph.Bold
	}
	if style.Paragraph.Italic == tagStyle.Paragraph.Italic {
		style.Paragraph.Italic = contextStyle.Paragraph.Italic
	}
	if style.Paragraph.FontSize == tagStyle.Paragraph.FontSize {
		style.Paragraph.FontSize = contextStyle.Paragraph.FontSize
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == tagStyle.Paragraph.LineHeight {
		style.Paragraph.LineHeight = contextStyle.Paragraph.LineHeight
		style.Paragraph.LineHeightExplicit = contextStyle.Paragraph.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == tagStyle.Paragraph.LetterSpacing {
		style.Paragraph.LetterSpacing = contextStyle.Paragraph.LetterSpacing
	}
	if style.Paragraph.FirstLineIndent == tagStyle.Paragraph.FirstLineIndent {
		style.Paragraph.FirstLineIndent = contextStyle.Paragraph.FirstLineIndent
	}
	if style.Paragraph.Align == tagStyle.Paragraph.Align {
		style.Paragraph.Align = contextStyle.Paragraph.Align
	}
	if style.Paragraph.Color == tagStyle.Paragraph.Color {
		style.Paragraph.Color = contextStyle.Paragraph.Color
	}
	if style.Paragraph.PreserveSpace == tagStyle.Paragraph.PreserveSpace {
		style.Paragraph.PreserveSpace = contextStyle.Paragraph.PreserveSpace
	}
	if style.Paragraph.Hyphenation == tagStyle.Paragraph.Hyphenation {
		style.Paragraph.Hyphenation = contextStyle.Paragraph.Hyphenation
	}
	if style.MarginLeft == tagStyle.MarginLeft {
		style.MarginLeft = contextStyle.MarginLeft
	} else if contextStyle.MarginLeft != tagStyle.MarginLeft {
		style.MarginLeft += contextStyle.MarginLeft
	}
	if style.MarginRight == tagStyle.MarginRight {
		style.MarginRight = contextStyle.MarginRight
	} else if contextStyle.MarginRight != tagStyle.MarginRight {
		style.MarginRight += contextStyle.MarginRight
	}
	return style
}

func contextStyleClassShouldSkipInheritedAndHorizontalMargins(block pdfTextBlock, class string) bool {
	if class == "" {
		return false
	}
	found := false
	for _, contextClass := range strings.Fields(block.ContextClasses) {
		if contextClass == class {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	switch class {
	case pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleSectionTitle, pdfStyleAnnotation, pdfStyleFootnote, pdfStylePoem, pdfStyleStanza, pdfStyleEpigraph, pdfStyleCite:
		return true
	default:
		return false
	}
}

func mergePDFContextClassStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if override.HasSpaceBefore || override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
	}
	if override.HasSpaceAfter || override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
	}
	if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
	}
	if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
	}
	if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
	}
	if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
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

func (r *pdfStyleResolver) applyRootInheritedParagraphDefaults(style pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	root := r.rootParagraphStyle()
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	if style.Paragraph.FontFamily == rootDefault.FontFamily {
		style.Paragraph.FontFamily = root.FontFamily
	}
	if style.Paragraph.Bold == rootDefault.Bold {
		style.Paragraph.Bold = root.Bold
	}
	if style.Paragraph.Italic == rootDefault.Italic {
		style.Paragraph.Italic = root.Italic
	}
	if style.Paragraph.FontSize == rootDefault.FontSize {
		style.Paragraph.FontSize = root.FontSize
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == rootDefault.LineHeight {
		style.Paragraph.LineHeight = root.LineHeight
		style.Paragraph.LineHeightExplicit = root.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == rootDefault.LetterSpacing {
		style.Paragraph.LetterSpacing = root.LetterSpacing
	}
	if style.Paragraph.FirstLineIndent == rootDefault.FirstLineIndent {
		style.Paragraph.FirstLineIndent = root.FirstLineIndent
	}
	if style.Paragraph.Align == rootDefault.Align {
		style.Paragraph.Align = root.Align
	}
	if style.Paragraph.Color == rootDefault.Color {
		style.Paragraph.Color = root.Color
	}
	if style.Paragraph.PreserveSpace == rootDefault.PreserveSpace {
		style.Paragraph.PreserveSpace = root.PreserveSpace
	}
	if style.Paragraph.Hyphenation == rootDefault.Hyphenation {
		style.Paragraph.Hyphenation = root.Hyphenation
	}
	return style
}

func mergePDFInheritedParagraphStyle(base, override, fallback paragraphStyle) paragraphStyle {
	if override.FontFamily != fallback.FontFamily {
		base.FontFamily = override.FontFamily
	}
	if override.Bold != fallback.Bold {
		base.Bold = override.Bold
	}
	if override.Italic != fallback.Italic {
		base.Italic = override.Italic
	}
	if override.FontSize != fallback.FontSize {
		base.FontSize = override.FontSize
	}
	base = mergePDFLineHeightOverride(base, override, fallback)
	if override.LetterSpacing != fallback.LetterSpacing {
		base.LetterSpacing = override.LetterSpacing
	}
	if override.FirstLineIndent != fallback.FirstLineIndent {
		base.FirstLineIndent = override.FirstLineIndent
	}
	if override.Align != fallback.Align {
		base.Align = override.Align
	}
	if override.Color != fallback.Color {
		base.Color = override.Color
	}
	if override.PreserveSpace != fallback.PreserveSpace {
		base.PreserveSpace = override.PreserveSpace
	}
	if override.Hyphenation != fallback.Hyphenation {
		base.Hyphenation = override.Hyphenation
	}
	return base
}

func (r *pdfStyleResolver) pageStyle() pdfBlockResolvedStyle {
	page := r.defaultStyle(pdfStylePage)
	page = mergePDFStyleOverrides(page, r.namedStyle(pdfStylePage), r.defaultStyle(pdfStylePage))
	rootLeft, rootRight := r.rootHorizontalMargins()
	page.MarginLeft += rootLeft
	page.MarginRight += rootRight
	for _, name := range []string{pdfStyleHTML, pdfStyleBody} {
		root := r.namedStyle(name)
		fallback := r.defaultStyle(name)
		if root.HasSpaceBefore || root.SpaceBefore != fallback.SpaceBefore {
			page.SpaceBefore += root.SpaceBefore
		}
		if root.HasSpaceAfter || root.SpaceAfter != fallback.SpaceAfter {
			page.SpaceAfter += root.SpaceAfter
		}
	}
	return page
}

func (r *pdfStyleResolver) rootHorizontalMargins() (float64, float64) {
	var left float64
	var right float64
	for _, name := range []string{pdfStyleHTML, pdfStyleBody} {
		root := r.namedStyle(name)
		fallback := r.defaultStyle(name)
		if root.MarginLeft != fallback.MarginLeft {
			left += root.MarginLeft
		}
		if root.MarginRight != fallback.MarginRight {
			right += root.MarginRight
		}
	}
	return left, right
}

func (r *pdfStyleResolver) contextDescendantStyleNames(block pdfTextBlock) []string {
	ancestors := []string{pdfStyleHTML, pdfStyleBody}
	ancestors = append(ancestors, strings.Fields(block.ContextClasses)...)
	ancestors = slices.Compact(ancestors)
	candidates := pdfSelectorCandidatesForBlock(block)
	var names []string
	for _, ancestor := range ancestors {
		for _, candidate := range candidates {
			name := ancestor + "--" + candidate
			if _, ok := r.styles[name]; ok {
				names = append(names, name)
			}
		}
	}
	return slices.Compact(names)
}

func pdfSelectorCandidatesForBlock(block pdfTextBlock) []string {
	var candidates []string
	if tag := pdfElementTagForBlock(block); tag != "" {
		candidates = append(candidates, tag)
		for _, class := range strings.Fields(block.StyleClasses) {
			candidates = append(candidates, class)
			candidates = append(candidates, tag+"."+class)
		}
		return slices.Compact(candidates)
	}
	candidates = append(candidates, strings.Fields(block.StyleClasses)...)
	return slices.Compact(candidates)
}

func pdfElementClassStyleNames(block pdfTextBlock) []string {
	if tag := pdfElementTagForBlock(block); tag != "" {
		names := make([]string, 0, len(strings.Fields(block.StyleClasses)))
		for _, class := range strings.Fields(block.StyleClasses) {
			names = append(names, tag+"."+class)
		}
		return slices.Compact(names)
	}
	return nil
}

func pdfStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	return newPDFStyleResolver(nil, nil).styleForBlock(block)
}

func pdfTagStyleNameForBlock(block pdfTextBlock) string {
	switch pdfElementTagForBlock(block) {
	case "p":
		return pdfStyleParagraph
	case "h1":
		return pdfStyleChapterTitleHeader
	case "h2", "h3", "h4", "h5", "h6":
		return pdfStyleSectionTitleHeader
	case "img":
		return pdfStyleImage
	case "table":
		return pdfStyleTable
	case "code":
		return pdfStyleCode
	default:
		return pdfStyleNameForBlock(block)
	}
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

func pdfElementTagForBlock(block pdfTextBlock) string {
	switch block.Kind {
	case pdfBlockParagraph, pdfBlockSubtitle, pdfBlockPoem, pdfBlockTextAuthor, pdfBlockTOCEntry:
		return "p"
	case pdfBlockHeading:
		level := min(max(block.Depth, 1), 6)
		return "h" + strconv.Itoa(level)
	case pdfBlockImage:
		if block.StyleName != "" && block.StyleName != pdfStyleImage {
			return "p"
		}
		return "img"
	default:
		return ""
	}
}

func pdfHeadingStyleName(depth int) string {
	if depth <= 1 {
		return pdfStyleChapterTitleHeader
	}
	return pdfStyleSectionTitleHeader
}

func blockContentWidth(contentWidth float64, style pdfBlockResolvedStyle) float64 {
	available := max(contentWidth-style.MarginLeft-style.MarginRight-style.PaddingLeft-style.PaddingRight, pdfMinBlockWidth)
	width := available
	if style.HasWidth {
		width = style.Width.resolve(available)
	}
	if style.HasMinWidth {
		width = max(width, style.MinWidth.resolve(available))
	}
	if style.HasMaxWidth {
		width = min(width, style.MaxWidth.resolve(available))
	}
	return min(max(width, pdfMinBlockWidth), available)
}

func blockBoxWidth(contentWidth float64, style pdfBlockResolvedStyle) float64 {
	return blockContentWidth(contentWidth, style) + style.PaddingLeft + style.PaddingRight
}

func (l pdfBlockLength) resolve(available float64) float64 {
	if l.Percent {
		return available * l.Value / 100
	}
	return l.Value
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
	if sel.Ancestor != nil {
		return pdfDescendantSelectorStyleNames(sel)
	}
	if sel.Element != "" && sel.Class != "" {
		if strings.EqualFold(sel.Element, "img") {
			return []string{strings.ToLower(sel.Element) + "." + sel.Class}
		}
		return []string{sel.Class}
	}
	if sel.Class != "" {
		return []string{pdfClassSelectorStyleName(sel.Class)}
	}
	switch strings.ToLower(sel.Element) {
	case "html":
		return []string{pdfStyleHTML}
	case "body":
		return []string{pdfStyleBody}
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
	case "code":
		return []string{pdfStyleCode}
	default:
		return nil
	}
}

func pdfClassSelectorStyleName(class string) string {
	if pdfSelectorClassCollidesWithElement(class) {
		return "." + class
	}
	return class
}

func pdfSelectorClassCollidesWithElement(class string) bool {
	switch strings.ToLower(class) {
	case "html", "body", "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code", "a", "span", "div":
		return true
	default:
		return false
	}
}

func pdfDescendantSelectorStyleNames(sel css.Selector) []string {
	if sel.Ancestor == nil {
		return nil
	}
	ancestorNames := pdfSelectorStyleNames(*sel.Ancestor)
	if len(ancestorNames) == 0 {
		return nil
	}
	rightNames := pdfDescendantSelectorTargets(sel)
	if len(rightNames) == 0 {
		return nil
	}
	mapped := make([]string, 0, len(ancestorNames)*len(rightNames))
	for _, ancestor := range ancestorNames {
		for _, rightName := range rightNames {
			mapped = append(mapped, ancestor+"--"+rightName)
		}
	}
	return slices.Compact(mapped)
}

func pdfDescendantSelectorTargets(sel css.Selector) []string {
	if sel.Element != "" && sel.Class != "" {
		switch strings.ToLower(sel.Element) {
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code":
			return []string{strings.ToLower(sel.Element) + "." + sel.Class}
		}
		return nil
	}
	var targets []string
	if sel.Element != "" {
		switch strings.ToLower(sel.Element) {
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code":
			targets = append(targets, strings.ToLower(sel.Element))
		}
	}
	if sel.Class != "" {
		targets = append(targets, sel.Class)
	}
	return slices.Compact(targets)
}

func mergePDFLineHeightOverride(base, override, fallback paragraphStyle) paragraphStyle {
	if override.LineHeightExplicit {
		base.LineHeight = override.LineHeight
		base.LineHeightExplicit = true
		return base
	}
	if !base.LineHeightExplicit && override.LineHeight != fallback.LineHeight {
		base.LineHeight = override.LineHeight
	}
	return base
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
			ratio := points / style.Paragraph.FontSize
			style.Paragraph.FontSize = points
			if !style.Paragraph.LineHeightExplicit {
				style.Paragraph.LineHeight *= ratio
			}
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

func pdfBlockLengthString(length pdfBlockLength, ok bool) string {
	if !ok {
		return ""
	}
	if length.Percent {
		return strconv.FormatFloat(length.Value, 'f', -1, 64) + "%"
	}
	return strconv.FormatFloat(length.Value, 'f', -1, 64) + "pt"
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

func pdfDebugBackgroundColor(style pdfBlockResolvedStyle) string {
	if !style.HasBackground {
		return ""
	}
	return style.BackgroundColor.String()
}

func pdfDebugBorderColor(style pdfBlockResolvedStyle) string {
	if !style.HasBorder || style.BorderWidth <= 0 {
		return ""
	}
	return style.BorderColor.String()
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
			LetterSpacing:     style.Paragraph.LetterSpacing,
			FirstLineIndent:   style.Paragraph.FirstLineIndent,
			TextAlign:         style.Paragraph.Align.String(),
			VerticalAlign:     style.Paragraph.VerticalAlign.String(),
			Color:             style.Paragraph.Color.String(),
			Underline:         style.Paragraph.Underline,
			Strikethrough:     style.Paragraph.Strikethrough,
			PreserveSpace:     style.Paragraph.PreserveSpace,
			Hyphenation:       pdfHyphenationString(style.Paragraph.Hyphenation),
			SpaceBefore:       style.SpaceBefore,
			SpaceAfter:        style.SpaceAfter,
			MarginLeft:        style.MarginLeft,
			MarginRight:       style.MarginRight,
			PaddingTop:        style.PaddingTop,
			PaddingRight:      style.PaddingRight,
			PaddingBottom:     style.PaddingBottom,
			PaddingLeft:       style.PaddingLeft,
			Width:             pdfBlockLengthString(style.Width, style.HasWidth),
			MinWidth:          pdfBlockLengthString(style.MinWidth, style.HasMinWidth),
			MaxWidth:          pdfBlockLengthString(style.MaxWidth, style.HasMaxWidth),
			BackgroundColor:   pdfDebugBackgroundColor(style),
			BorderWidth:       style.BorderWidth,
			BorderColor:       pdfDebugBorderColor(style),
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
