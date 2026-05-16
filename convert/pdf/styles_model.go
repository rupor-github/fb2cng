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
	pdfBodyIndent                         = pdfBaseFontSize
	pdfParagraphSpaceAfter                = 0.0
	pdfHeadingH1FontSize                  = pdfBaseFontSize * 1.4
	pdfHeadingNestedFontSize              = pdfBaseFontSize * 1.2
	pdfHeadingLineHeightFactor            = pdfAdjustedLineHeightFactor
	pdfHeadingH1MarginFactor              = 0.67
	pdfHeadingNestedMarginFactor          = 0.83
	pdfTitleFirstSpaceBefore              = pdfBaseFontSize * 2.0
	pdfTitleEmptyLineSpace                = pdfBaseFontSize * 0.8
	pdfTitleAfterImageSpaceBefore         = pdfBaseLineHeight * 1.5
	pdfSubtitleFontSize                   = pdfBaseFontSize
	pdfSubtitleLineHeight                 = pdfSubtitleFontSize * pdfNormalLineHeightFactor
	pdfSubtitleSpaceBefore                = pdfBaseFontSize
	pdfSubtitleSpaceAfter                 = pdfBaseFontSize
	pdfAnnotationSubtitleSpace            = pdfBaseFontSize * 0.5
	pdfPoemSubtitleSpace                  = pdfBaseFontSize * 0.5
	pdfStanzaSubtitleSpace                = pdfBaseFontSize * 0.25
	pdfEpigraphSubtitleSpace              = pdfBaseFontSize * 0.3
	pdfCiteSubtitleSpace                  = pdfBaseFontSize * 0.5
	pdfCodeFontSize                       = pdfBaseFontSize * 0.70
	pdfCodeLineHeight                     = pdfBaseLineHeight
	pdfFootnoteLinkFontSize               = pdfBaseFontSize * 0.80
	pdfFootnoteLinkLineHeight             = pdfFootnoteLinkFontSize * pdfNormalLineHeightFactor
	pdfVerseLineHeight                    = pdfBaseFontSize * pdfNormalLineHeightFactor
	pdfVerseSpaceBefore                   = pdfBaseFontSize * 0.25
	pdfVerseSpaceAfter                    = pdfBaseFontSize * 0.25
	pdfVerseMarginLeft                    = pdfBaseFontSize * 2.0
	pdfTextAuthorFontSize                 = pdfBaseFontSize
	pdfTextAuthorLineHeight               = pdfTextAuthorFontSize * pdfNormalLineHeightFactor
	pdfTextAuthorSpaceAfter               = 0.0
	pdfDateSpace                          = pdfBaseFontSize * 0.5
	pdfAnnotationSpaceBefore              = pdfBaseFontSize * 2.0
	pdfAnnotationSpaceAfter               = pdfBaseFontSize
	pdfAnnotationHorizontalMargin         = pdfBaseFontSize
	pdfEpigraphSpaceBefore                = pdfBaseFontSize * 0.4
	pdfEpigraphSpaceAfter                 = pdfBaseFontSize * 0.2
	pdfEpigraphMarginLeft                 = pdfBaseFontSize * 4.0
	pdfPoemMarginLeft                     = pdfBaseFontSize * 3.0
	pdfStanzaSpace                        = pdfBaseFontSize * 0.5
	pdfPoemTitleSpace                     = pdfBaseFontSize
	pdfStanzaTitleSpace                   = pdfBaseFontSize * 0.5
	pdfFootnoteTitleSpaceBefore           = pdfBaseFontSize
	pdfFootnoteTitleSpaceAfter            = pdfBaseFontSize * 0.5
	pdfVignetteSpace                      = pdfBaseFontSize * 0.5
	pdfVignetteTitleTopSpaceBefore        = pdfBaseFontSize
	pdfVignetteTitleTopSpaceAfter         = pdfBaseFontSize * 0.5
	pdfVignetteTitleBottomSpaceBefore     = pdfBaseFontSize * 0.5
	pdfVignetteTitleBottomSpaceAfter      = pdfBaseFontSize
	pdfVignetteChapterEndSpace            = pdfBaseFontSize * 1.5
	pdfVignetteSectionTitleTopSpaceBefore = pdfBaseFontSize * 0.8
	pdfVignetteSectionTitleTopSpaceAfter  = pdfBaseFontSize * 0.4
	pdfVignetteSectionTitleBottomBefore   = pdfBaseFontSize * 0.4
	pdfVignetteSectionTitleBottomAfter    = pdfBaseFontSize * 0.8
	pdfVignetteSectionEndSpace            = pdfBaseFontSize
	pdfTOCNestedListIndent                = pdfBaseFontSize
	pdfTOCSpaceAfter                      = 0.0
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
	pdfStyleDate               = "date"
	pdfStyleImage              = "image"
	pdfStyleImageVignette      = "image-vignette"
	pdfStyleVignette           = "vignette"
	pdfStyleVignetteBookTop    = "vignette-book-title-top"
	pdfStyleVignetteBookBottom = "vignette-book-title-bottom"
	pdfStyleVignetteChapterTop = "vignette-chapter-title-top"
	pdfStyleVignetteChapterBot = "vignette-chapter-title-bottom"
	pdfStyleVignetteChapterEnd = "vignette-chapter-end"
	pdfStyleVignetteSectionTop = "vignette-section-title-top"
	pdfStyleVignetteSectionBot = "vignette-section-title-bottom"
	pdfStyleVignetteSectionEnd = "vignette-section-end"
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
	pdfStyleLinkBacklink       = "link-backlink"
	pdfStyleTitleAfterImage    = "title-after-image"
	pdfStyleHTML               = "__html__"
	pdfStyleBody               = "__body__"
	pdfStylePage               = "__page__"
)

type pdfBlockResolvedStyle struct {
	Paragraph          paragraphStyle
	SpaceBefore        float64
	HasSpaceBefore     bool
	SpaceAfter         float64
	HasSpaceAfter      bool
	MarginLeft         float64
	MarginRight        float64
	PaddingTop         float64
	PaddingRight       float64
	PaddingBottom      float64
	PaddingLeft        float64
	Width              pdfBlockLength
	HasWidth           bool
	MinWidth           pdfBlockLength
	HasMinWidth        bool
	MaxWidth           pdfBlockLength
	HasMaxWidth        bool
	BackgroundColor    pdfColor
	HasBackground      bool
	BorderWidth        float64
	BorderColor        pdfColor
	HasBorder          bool
	KeepTogether       bool
	HasKeepTogether    bool
	KeepWithNextLines  int
	PageBreakBefore    bool
	HasPageBreakBefore bool
	PageBreakAfter     bool
	HasPageBreakAfter  bool
	Hidden             bool
	HasHidden          bool
	Orphans            int
	Widows             int
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
