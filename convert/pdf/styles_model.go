package pdf

const (
	pdfPointsPerInch     = 72.0
	pdfCSSPixelsPerIn    = 96.0
	pdfKP3ContentWidthPx = 512.0
	pdfMinBlockWidth     = 12.0

	pdfBaseFontSize                       = 10.5
	pdfNormalLineHeightFactor             = 1.2
	pdfAdjustedLineHeightLH               = 100.0 / 99.0
	pdfAdjustedLineHeightFactor           = pdfNormalLineHeightFactor * pdfAdjustedLineHeightLH
	pdfSectionTitleHeaderLineHeightLH     = 0.982323
	pdfSectionTitleHeaderLineHeightFactor = pdfNormalLineHeightFactor * pdfSectionTitleHeaderLineHeightLH
	pdfBaseLineHeight                     = pdfBaseFontSize * pdfNormalLineHeightFactor
	pdfBodyIndent                         = 14.0
	pdfParagraphSpaceAfter                = pdfBaseFontSize * 0.3
	pdfHeadingBaseFontSize                = 16.0
	pdfHeadingMinFontSize                 = 11.0
	pdfHeadingLineHeightFactor            = pdfAdjustedLineHeightFactor
	pdfHeadingSpaceBefore                 = 10.0
	pdfHeadingSpaceAfter                  = 8.0
	pdfTitleFirstSpaceBefore              = pdfBaseLineHeight * 1.7
	pdfTitleAfterImageSpaceBefore         = pdfBaseLineHeight * 1.5
	pdfSubtitleFontSize                   = 11.0
	pdfSubtitleLineHeight                 = pdfSubtitleFontSize * pdfNormalLineHeightFactor
	pdfSubtitleSpaceBefore                = 6.0
	pdfSubtitleSpaceAfter                 = 5.0
	pdfCodeFontSize                       = pdfBaseFontSize * 0.70
	pdfCodeLineHeight                     = pdfCodeFontSize * pdfNormalLineHeightFactor
	pdfVerseLineHeight                    = pdfBaseFontSize * pdfNormalLineHeightFactor
	pdfVerseSpaceAfter                    = 2.0
	pdfTextAuthorFontSize                 = 10.0
	pdfTextAuthorLineHeight               = pdfTextAuthorFontSize * pdfNormalLineHeightFactor
	pdfTextAuthorSpaceAfter               = 4.0
	pdfTOCIndentPerDepth                  = 12.0
	pdfTOCSpaceAfter                      = 1.5
	pdfDefaultKeepLines                   = 2
	pdfSingleKeepLine                     = 1

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
