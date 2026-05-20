package pdf

const (
	pdfPointsPerInch     = 72.0
	pdfCSSPixelsPerIn    = 96.0
	pdfKP3PixelsPerEm    = 16.0
	pdfKP3ContentWidthPx = 512.0
	pdfMinBlockWidth     = 12.0

	// Native PDF defaults are chosen to match Kindle Previewer/KP3's default
	// fixed-screen visual rhythm rather than a print-style 12pt baseline. In KFX,
	// ordinary body text is emitted as 1rem/1lh and rendered by KP3 at roughly an
	// 8.4pt text size with about 14.28pt between baselines on the configured
	// 303.36x403.2pt test page. Keep these as the intrinsic PDF 1rem/1lh values;
	// stylesheet rules may still override them explicitly, while the default.css
	// Kindle reader hint body { font-size: 80%; line-height: 150%; } is normalized
	// in styles_resolve.go so it does not shrink native fixed-layout PDF text.
	pdfBaseFontSize                      = 8.4
	pdfNormalLineHeightFactor            = 1.7
	pdfAdjustedLineHeightLH              = 100.0 / 99.0
	pdfSectionTitleHeaderLineHeightLH    = 0.982323
	pdfBaseLineHeight                    = pdfBaseFontSize * pdfNormalLineHeightFactor
	pdfAdjustedLineHeight                = pdfBaseLineHeight * pdfAdjustedLineHeightLH
	pdfSectionTitleHeaderLineHeight      = pdfBaseLineHeight * pdfSectionTitleHeaderLineHeightLH
	pdfBodyIndent                        = pdfBaseFontSize
	pdfParagraphSpaceAfter               = 0.0
	pdfHeadingH1FontSize                 = pdfBaseFontSize * 1.4
	pdfHeadingNestedFontSize             = pdfBaseFontSize * 1.2
	pdfHeadingH1MarginFactor             = 0.67
	pdfHeadingNestedMarginFactor         = 0.83
	pdfTitleFirstSpaceBefore             = pdfBaseFontSize * 2.0
	pdfTitleEmptyLineSpace               = pdfBaseFontSize * 0.8
	pdfTitleAfterImageSpaceBefore        = pdfBaseLineHeight * 0.5999994
	pdfTitleVignetteMarginTop            = pdfBaseLineHeight * 0.697917
	pdfTitleFollowingSubtitleSpaceBefore = pdfBaseLineHeight * 0.833333
	pdfFullBlockImageMarginLH            = 2.6
	// KP3 sometimes keeps block images on a page even when their bottom edge
	// slightly crosses the nominal content bottom. Allow a small image-only
	// pagination slack so near-fitting image blocks match KP3 without changing
	// global page margins or text pagination.
	pdfBlockImageBottomFitOverflow        = pdfBaseLineHeight * 0.30
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
	pdfTableCellPadding                   = pdfBaseFontSize * 0.5
	pdfTableCellBorderWidth               = 0.45

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
	pdfStyleTableCell          = "td"
	pdfStyleTableHeaderCell    = "th"
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

type pdfPageBreakMode int

const (
	pdfPageBreakAuto pdfPageBreakMode = iota
	pdfPageBreakAlways
	pdfPageBreakAvoid
)

type pdfCSSLengthSpec struct {
	Value   float64
	Unit    string
	Keyword string
	Set     bool
}

type pdfBlockResolvedStyle struct {
	Paragraph           paragraphStyle
	SpaceBefore         float64
	SpaceBeforeSpec     pdfCSSLengthSpec
	HasSpaceBefore      bool
	SpaceAfter          float64
	SpaceAfterSpec      pdfCSSLengthSpec
	HasSpaceAfter       bool
	MarginLeft          float64
	MarginLeftSpec      pdfCSSLengthSpec
	MarginRight         float64
	MarginRightSpec     pdfCSSLengthSpec
	PaddingTop          float64
	PaddingTopSpec      pdfCSSLengthSpec
	PaddingRight        float64
	PaddingRightSpec    pdfCSSLengthSpec
	PaddingBottom       float64
	PaddingBottomSpec   pdfCSSLengthSpec
	PaddingLeft         float64
	PaddingLeftSpec     pdfCSSLengthSpec
	Width               pdfBlockLength
	HasWidth            bool
	MinWidth            pdfBlockLength
	HasMinWidth         bool
	MaxWidth            pdfBlockLength
	HasMaxWidth         bool
	BackgroundColor     pdfColor
	HasBackground       bool
	BorderWidth         float64
	BorderColor         pdfColor
	HasBorder           bool
	KeepTogether        bool
	HasKeepTogether     bool
	KeepWithNextLines   int
	PageBreakBefore     bool
	PageBreakBeforeMode pdfPageBreakMode
	HasPageBreakBefore  bool
	PageBreakAfter      bool
	PageBreakAfterMode  pdfPageBreakMode
	HasPageBreakAfter   bool
	Hidden              bool
	HasHidden           bool
	Orphans             int
	Widows              int
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
	NoWrap            bool    `json:"no_wrap,omitempty"`
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
