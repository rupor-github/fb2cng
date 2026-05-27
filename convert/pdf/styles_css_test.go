package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFStyleResolverAppliesCodeStylesheet(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `code { white-space: pre-wrap; font-family: monospace; font-size: 70%; text-align: left; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	style := resolver.styles[pdfStyleCode]
	if !style.Paragraph.PreserveSpace || style.Paragraph.FontFamily != "monospace" {
		t.Fatalf("code style = %#v, want pre-wrap monospace", style.Paragraph)
	}
	if math.Abs(style.Paragraph.FontSize-pdfBaseFontSize*0.7) > 0.001 {
		t.Fatalf("code font size = %v, want 70%% base font size", style.Paragraph.FontSize)
	}
}

func TestPDFStyleResolverParagraphMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.SpaceBefore != 0 || paragraph.SpaceAfter != 0 {
		t.Fatalf("paragraph margins = %v/%v, want default.css no margins", paragraph.SpaceBefore, paragraph.SpaceAfter)
	}
}

func TestPDFStyleResolverTOCItemDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	tocEntry := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockTOCEntry, Depth: 2})
	if tocEntry.Paragraph.Align != textAlignLeft {
		t.Fatalf("toc-item align = %v, want default.css left", tocEntry.Paragraph.Align)
	}
	if tocEntry.SpaceBefore != 0 || tocEntry.SpaceAfter != 0 {
		t.Fatalf("toc-item margins = %v/%v, want default.css no margins", tocEntry.SpaceBefore, tocEntry.SpaceAfter)
	}
	if tocEntry.Paragraph.FirstLineIndent != pdfTOCNestedListIndent {
		t.Fatalf("toc-item indent = %v, want native nested-list indent %v", tocEntry.Paragraph.FirstLineIndent, pdfTOCNestedListIndent)
	}
}

func TestPDFStyleResolverImageDefaultsMatchDefaultCSS(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `body { text-indent: 2em; text-align: right; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	image := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage})
	if image.Paragraph.Align != textAlignCenter {
		t.Fatalf("image align = %v, want center", image.Paragraph.Align)
	}
	if image.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("image indent = %v, want 0", image.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverAnnotationMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `p { text-indent: 0; }`)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleAnnotation, ContextClasses: pdfStyleAnnotation})
	if paragraph.SpaceBefore != pdfDefaultCSSRootFontSize*2 || paragraph.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("annotation vertical margins = %v/%v, want default.css 2em/1em", paragraph.SpaceBefore, paragraph.SpaceAfter)
	}
	if paragraph.MarginLeft != pdfDefaultCSSRootFontSize || paragraph.MarginRight != pdfDefaultCSSRootFontSize {
		t.Fatalf("annotation horizontal margins = %v/%v, want default.css 1em/1em", paragraph.MarginLeft, paragraph.MarginRight)
	}
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("annotation paragraph indent = %v, want paragraph CSS indent preserved", paragraph.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverEpigraphDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `p { text-indent: 0; }`)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleEpigraph, ContextClasses: pdfStyleEpigraph})
	if math.Abs(paragraph.SpaceBefore-pdfDefaultCSSRootFontSize*0.4) > 0.001 || math.Abs(paragraph.SpaceAfter-pdfDefaultCSSRootFontSize*0.2) > 0.001 {
		t.Fatalf("epigraph vertical margins = %v/%v, want default.css 0.4em/0.2em", paragraph.SpaceBefore, paragraph.SpaceAfter)
	}
	if paragraph.MarginLeft != pdfDefaultCSSRootFontSize*4 || paragraph.MarginRight != 0 {
		t.Fatalf("epigraph horizontal margins = %v/%v, want default.css 4em/0", paragraph.MarginLeft, paragraph.MarginRight)
	}
	if paragraph.Paragraph.Align != textAlignRight || !paragraph.Paragraph.Italic {
		t.Fatalf("epigraph text style = align:%v italic:%t, want right italic", paragraph.Paragraph.Align, paragraph.Paragraph.Italic)
	}
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("epigraph paragraph indent = %v, want paragraph CSS indent preserved", paragraph.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverCiteMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleCite, ContextClasses: pdfStyleCite})
	if paragraph.SpaceBefore != pdfDefaultCSSRootFontSize || paragraph.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("cite paragraph vertical margins = %v/%v, want default.css 1em/1em", paragraph.SpaceBefore, paragraph.SpaceAfter)
	}
	if paragraph.MarginLeft != pdfDefaultCSSRootFontSize*2 || paragraph.MarginRight != pdfDefaultCSSRootFontSize*2 {
		t.Fatalf("cite paragraph horizontal margins = %v/%v, want default.css 2em/2em", paragraph.MarginLeft, paragraph.MarginRight)
	}
}

func TestPDFStyleResolverTableDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	table := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockTable, StyleName: pdfStyleTable})
	if table.SpaceBefore != pdfDefaultCSSRootFontSize || table.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("table margins = %v/%v, want default.css 1em/1em", table.SpaceBefore, table.SpaceAfter)
	}
	if !table.KeepTogether {
		t.Fatalf("table keep-together = false, want default.css page-break-inside avoid")
	}
	if table.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("table indent = %v, want 0", table.Paragraph.FirstLineIndent)
	}
	td := resolver.styleForTableCell(fb2.TableRow{}, fb2.TableCell{}, "")
	if td.PaddingLeft != pdfDefaultCSSRootFontSize*0.5 || td.BorderWidth == 0 || !td.HasBorder {
		t.Fatalf("td style padding/border = %v/%v/%t, want default.css cell padding and border", td.PaddingLeft, td.BorderWidth, td.HasBorder)
	}
	th := resolver.styleForTableCell(fb2.TableRow{}, fb2.TableCell{Header: true}, "")
	if !th.HasBackground || th.PaddingLeft != pdfDefaultCSSRootFontSize*0.5 || th.BorderWidth == 0 || !th.Paragraph.Bold ||
		th.Paragraph.Align != textAlignCenter {
		t.Fatalf(
			"th style bg/padding/border/bold/align = %t/%v/%v/%t/%v",
			th.HasBackground,
			th.PaddingLeft,
			th.BorderWidth,
			th.Paragraph.Bold,
			th.Paragraph.Align,
		)
	}
}

func TestPDFStyleResolverFootnoteDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote})
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("footnote paragraph indent = %v, want default.css 0", paragraph.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverVignetteDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	generic := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage, StyleClasses: joinStyleClasses(pdfStyleImageVignette, pdfStyleVignette)})
	if generic.SpaceBefore != pdfDefaultCSSRootFontSize*0.5 || generic.SpaceAfter != pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("vignette margins = %v/%v, want default.css 0.5em/0.5em", generic.SpaceBefore, generic.SpaceAfter)
	}
	if generic.Paragraph.Align != textAlignCenter || generic.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("vignette text style = align:%v indent:%v, want center/0", generic.Paragraph.Align, generic.Paragraph.FirstLineIndent)
	}

	chapterTop := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockImage, StyleClasses: joinStyleClasses(pdfStyleImageVignette, pdfStyleVignette, pdfStyleVignetteChapterTop)},
	)
	if chapterTop.SpaceBefore != pdfDefaultCSSRootFontSize || chapterTop.SpaceAfter != pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("chapter top vignette margins = %v/%v, want default.css 1em/0.5em", chapterTop.SpaceBefore, chapterTop.SpaceAfter)
	}

	sectionBottom := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockImage, StyleClasses: joinStyleClasses(pdfStyleImageVignette, pdfStyleVignette, pdfStyleVignetteSectionBot)},
	)
	if math.Abs(sectionBottom.SpaceBefore-pdfDefaultCSSRootFontSize*0.4) > 0.001 ||
		math.Abs(sectionBottom.SpaceAfter-pdfDefaultCSSRootFontSize*0.8) > 0.001 {
		t.Fatalf("section bottom vignette margins = %v/%v, want default.css 0.4em/0.8em", sectionBottom.SpaceBefore, sectionBottom.SpaceAfter)
	}

	chapterEnd := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockImage, StyleClasses: joinStyleClasses(pdfStyleImageVignette, pdfStyleVignette, pdfStyleVignetteChapterEnd)},
	)
	if math.Abs(chapterEnd.SpaceBefore-pdfDefaultCSSRootFontSize*1.5) > 0.001 ||
		math.Abs(chapterEnd.SpaceAfter-pdfDefaultCSSRootFontSize*1.5) > 0.001 {
		t.Fatalf("chapter end vignette margins = %v/%v, want default.css 1.5em/1.5em", chapterEnd.SpaceBefore, chapterEnd.SpaceAfter)
	}
}

func TestPDFStyleResolverPoemDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, StyleClasses: pdfStylePoem, ContextClasses: pdfStylePoem})
	if math.Abs(verse.MarginLeft-pdfDefaultCSSRootFontSize*5) > 0.001 {
		t.Fatalf("poem verse margin-left = %v, want default.css poem 3em + verse 2em", verse.MarginLeft)
	}
	if !verse.Paragraph.Italic {
		t.Fatalf("poem verse italic = false, want inherited from default.css .poem")
	}
	if verse.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("poem verse indent = %v, want default.css poem/verse zero indent", verse.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverTitleWrapperControlsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	chapter := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		Depth:          1,
		StyleName:      pdfStyleChapterTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-first"),
		ContextClasses: pdfStyleChapterTitle,
	})
	if !chapter.PageBreakBefore || !chapter.KeepTogether || chapter.KeepWithNextLines != pdfSingleKeepLine {
		t.Fatalf(
			"chapter title controls = page-before:%t keep:%t keep-next:%d, want default.css title wrapper controls",
			chapter.PageBreakBefore,
			chapter.KeepTogether,
			chapter.KeepWithNextLines,
		)
	}
	if chapter.SpaceBefore != pdfDefaultCSSRootFontSize*2 || chapter.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("chapter title margins = %v/%v, want default.css 2em/1em", chapter.SpaceBefore, chapter.SpaceAfter)
	}

	sectionH2 := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		Depth:          2,
		StyleName:      pdfStyleSectionTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2, pdfStyleSectionTitleHeader+"-first"),
		ContextClasses: joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2),
	})
	if !sectionH2.PageBreakBefore || !sectionH2.KeepTogether || sectionH2.KeepWithNextLines != pdfSingleKeepLine {
		t.Fatalf(
			"section h2 controls = page-before:%t keep:%t keep-next:%d, want default.css section-title-h2/title wrapper controls",
			sectionH2.PageBreakBefore,
			sectionH2.KeepTogether,
			sectionH2.KeepWithNextLines,
		)
	}

	sectionH3 := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		Depth:          3,
		StyleName:      pdfStyleSectionTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleHeader+"-first"),
		ContextClasses: pdfStyleSectionTitle,
	})
	if sectionH3.PageBreakBefore || !sectionH3.KeepTogether || sectionH3.KeepWithNextLines != pdfSingleKeepLine {
		t.Fatalf(
			"section h3 controls = page-before:%t keep:%t keep-next:%d, want default.css section-title without h2 page break",
			sectionH3.PageBreakBefore,
			sectionH3.KeepTogether,
			sectionH3.KeepWithNextLines,
		)
	}
}

func TestPDFStyleResolverTitleHeaderBreakDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, tt := range []struct {
		class string
		bold  bool
	}{
		{class: pdfStyleBodyTitleHeader + "-break", bold: true},
		{class: pdfStyleChapterTitleHeader + "-break", bold: true},
		{class: pdfStyleSectionTitleHeader + "-break", bold: true},
		{class: pdfStyleTOCTitle + "-break", bold: true},
	} {
		br := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: tt.class})
		if br.Paragraph.Align != textAlignCenter || br.Paragraph.Bold != tt.bold || br.SpaceBefore != 0 || br.SpaceAfter != 0 {
			t.Fatalf(
				"%s style = align:%v bold:%t margins:%v/%v, want default.css block break semantics",
				tt.class,
				br.Paragraph.Align,
				br.Paragraph.Bold,
				br.SpaceBefore,
				br.SpaceAfter,
			)
		}
	}
}

func TestPDFStyleResolverEmptyLineDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	emptyLine := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine})
	if emptyLine.SpaceBefore != pdfDefaultCSSRootFontSize || emptyLine.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("emptyline margins = %v/%v, want default.css 1em/1em", emptyLine.SpaceBefore, emptyLine.SpaceAfter)
	}
	if margin := pdfEmptyLineMargin(emptyLine); math.Abs(margin-pdfDefaultCSSRootLineHeight*0.5) > 0.001 {
		t.Fatalf("emptyline collapsed margin = %v, want KP3 0.5lh", margin)
	}
}

func TestPDFStyleResolverTitleHeaderEmptyLineDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, class := range []string{
		pdfStyleBodyTitleHeader + "-emptyline",
		pdfStyleChapterTitleHeader + "-emptyline",
		pdfStyleSectionTitleHeader + "-emptyline",
		pdfStyleTOCTitle + "-emptyline",
	} {
		emptyLine := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: class})
		if math.Abs(emptyLine.SpaceBefore-pdfDefaultCSSRootFontSize*0.8) > 0.001 ||
			math.Abs(emptyLine.SpaceAfter-pdfDefaultCSSRootFontSize*0.8) > 0.001 {
			t.Fatalf("%s margins = %v/%v, want default.css 0.8em/0.8em", class, emptyLine.SpaceBefore, emptyLine.SpaceAfter)
		}
		if margin := pdfEmptyLineMargin(emptyLine); math.Abs(margin-pdfDefaultCSSRootLineHeight*0.4) > 0.001 {
			t.Fatalf("%s collapsed empty-line margin = %v, want KP3 0.4lh", class, margin)
		}
	}
}

func TestPDFStyleResolverStanzaWrapperMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockTextAuthor, Text: "Князь Вяземский", ContextClasses: pdfStyleEpigraph},
		{
			Kind:           pdfBlockPoem,
			Text:           "«Мой дядя...",
			StyleClasses:   joinStyleClasses(pdfStylePoem, pdfStyleStanza),
			ContextClasses: joinStyleClasses(pdfStylePoem, pdfStyleStanza),
		},
		{
			Kind:           pdfBlockPoem,
			Text:           "Когда не в шутку занемог,",
			StyleClasses:   joinStyleClasses(pdfStylePoem, pdfStyleStanza),
			ContextClasses: joinStyleClasses(pdfStylePoem, pdfStyleStanza),
		},
	}, nil)
	if styles[1].SpaceBefore != pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("first stanza line margin-top = %v, want default.css stanza top %v", styles[1].SpaceBefore, pdfDefaultCSSRootFontSize*0.5)
	}
	if styles[2].SpaceBefore != pdfDefaultCSSRootFontSize*0.25 {
		t.Fatalf(
			"next stanza line margin-top = %v, want verse margin %v after wrapper edge adjustment",
			styles[2].SpaceBefore,
			pdfDefaultCSSRootFontSize*0.25,
		)
	}
}

func TestPDFStyleResolverAnnotationTitleDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	annotationTitle := resolver.styleForBlock(
		pdfTextBlock{
			Kind:           pdfBlockHeading,
			StyleName:      pdfStyleAnnotationTitle,
			StyleClasses:   pdfStyleAnnotationTitle + "-first",
			ContextClasses: pdfStyleAnnotationTitle,
		},
	)
	if annotationTitle.SpaceBefore != pdfDefaultCSSRootFontSize || annotationTitle.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("annotation title margins = %v/%v, want default.css 1em/1em", annotationTitle.SpaceBefore, annotationTitle.SpaceAfter)
	}
	if math.Abs(annotationTitle.Paragraph.FontSize-pdfDefaultCSSRootFontSize) > 0.001 ||
		math.Abs(annotationTitle.Paragraph.LineHeight-pdfDefaultCSSRootLineHeight) > 0.001 {
		t.Fatalf(
			"annotation title font/line = %v/%v, want default.css root font and line-height",
			annotationTitle.Paragraph.FontSize,
			annotationTitle.Paragraph.LineHeight,
		)
	}
	if !annotationTitle.Paragraph.Bold || annotationTitle.Paragraph.Align != textAlignCenter || annotationTitle.Paragraph.FirstLineIndent != 0 {
		t.Fatalf(
			"annotation title style = bold:%t align:%v indent:%v, want bold centered zero-indent",
			annotationTitle.Paragraph.Bold,
			annotationTitle.Paragraph.Align,
			annotationTitle.Paragraph.FirstLineIndent,
		)
	}
}

func TestPDFStyleResolverParagraphTitleDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	poemTitle := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStylePoemTitle + " " + pdfStylePoemTitle + "-first", ContextClasses: pdfStylePoem},
	)
	if poemTitle.SpaceBefore != pdfDefaultCSSRootFontSize || poemTitle.SpaceAfter != pdfDefaultCSSRootFontSize {
		t.Fatalf("poem title margins = %v/%v, want default.css 1em/1em", poemTitle.SpaceBefore, poemTitle.SpaceAfter)
	}
	if poemTitle.Paragraph.Align != textAlignCenter || poemTitle.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("poem title align/indent = %v/%v, want centered zero-indent variant", poemTitle.Paragraph.Align, poemTitle.Paragraph.FirstLineIndent)
	}
	if !poemTitle.Paragraph.Italic {
		t.Fatalf("poem title italic = false, want inherited from poem context")
	}

	stanzaTitle := resolver.styleForBlock(
		pdfTextBlock{
			Kind:           pdfBlockParagraph,
			StyleClasses:   pdfStyleStanzaTitle + " " + pdfStyleStanzaTitle + "-first",
			ContextClasses: joinStyleClasses(pdfStylePoem, pdfStyleStanza),
		},
	)
	if stanzaTitle.SpaceBefore != pdfDefaultCSSRootFontSize*0.5 || stanzaTitle.SpaceAfter != pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("stanza title margins = %v/%v, want default.css 0.5em/0.5em", stanzaTitle.SpaceBefore, stanzaTitle.SpaceAfter)
	}
	if stanzaTitle.Paragraph.Align != textAlignCenter || stanzaTitle.Paragraph.FirstLineIndent != 0 {
		t.Fatalf(
			"stanza title align/indent = %v/%v, want centered zero-indent variant",
			stanzaTitle.Paragraph.Align,
			stanzaTitle.Paragraph.FirstLineIndent,
		)
	}
}

func TestPDFStyleResolverFootnoteDefaultsAreCompactInPDF(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `p { text-align: right; }`)
	footnoteBody := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote})
	if math.Abs(footnoteBody.Paragraph.FontSize-pdfDefaultCSSRootFontSize*0.7) > 0.001 {
		t.Fatalf("footnote body font size = %v, want 70%% of PDF root %v", footnoteBody.Paragraph.FontSize, pdfDefaultCSSRootFontSize*0.7)
	}
	if footnoteBody.Paragraph.LineHeight >= pdfDefaultCSSRootLineHeight {
		t.Fatalf("footnote body line height = %v, want denser than main %v", footnoteBody.Paragraph.LineHeight, pdfDefaultCSSRootLineHeight)
	}
	footnoteEpigraph := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleEpigraph, ContextClasses: joinStyleClasses(pdfStyleFootnote, pdfStyleEpigraph)},
	)
	if math.Abs(footnoteEpigraph.Paragraph.FontSize-pdfDefaultCSSRootFontSize*0.7) > 0.001 {
		t.Fatalf("footnote epigraph font size = %v, want 70%% of PDF root %v", footnoteEpigraph.Paragraph.FontSize, pdfDefaultCSSRootFontSize*0.7)
	}
	if footnoteEpigraph.Paragraph.LineHeight >= pdfDefaultCSSRootLineHeight {
		t.Fatalf("footnote epigraph line height = %v, want denser than main %v", footnoteEpigraph.Paragraph.LineHeight, pdfDefaultCSSRootLineHeight)
	}

	footnoteTitle := resolver.styleForBlock(
		pdfTextBlock{
			Kind:           pdfBlockParagraph,
			StyleClasses:   pdfStyleFootnoteTitle + " " + pdfStyleFootnoteTitle + "-first",
			ContextClasses: pdfStyleFootnoteTitle,
		},
	)
	if math.Abs(footnoteTitle.Paragraph.FontSize-pdfDefaultCSSRootFontSize*0.7) > 0.001 {
		t.Fatalf("footnote title font size = %v, want 70%% of PDF root %v", footnoteTitle.Paragraph.FontSize, pdfDefaultCSSRootFontSize*0.7)
	}
	if footnoteTitle.SpaceBefore >= pdfDefaultCSSRootFontSize || footnoteTitle.SpaceAfter >= pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("footnote title margins = %v/%v, want denser than default 1em/0.5em", footnoteTitle.SpaceBefore, footnoteTitle.SpaceAfter)
	}
	if !footnoteTitle.Paragraph.Bold {
		t.Fatalf("footnote title bold = false, want default.css bold")
	}
	if footnoteTitle.Paragraph.Align != textAlignRight {
		t.Fatalf("footnote title align = %v, want inherited paragraph alignment", footnoteTitle.Paragraph.Align)
	}
	if footnoteTitle.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("footnote title indent = %v, want default.css zero-indent variant", footnoteTitle.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverVerseMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem})
	if verse.SpaceBefore != pdfDefaultCSSRootFontSize*0.25 || verse.SpaceAfter != pdfDefaultCSSRootFontSize*0.25 {
		t.Fatalf("verse vertical margins = %v/%v, want default.css 0.25em/0.25em", verse.SpaceBefore, verse.SpaceAfter)
	}
	if verse.MarginLeft != pdfDefaultCSSRootFontSize*2 {
		t.Fatalf("verse margin-left = %v, want default.css 2em", verse.MarginLeft)
	}
	if verse.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("verse indent = %v, want default.css 0", verse.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverContextSubtitleDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	tests := []struct {
		name      string
		styleName string
		space     float64
		align     textAlign
		bold      bool
		italic    bool
		keepWith  int
	}{
		{name: "annotation", styleName: pdfStyleAnnotationSubtitle, space: pdfDefaultCSSRootFontSize * 0.5, align: textAlignCenter, bold: true},
		{name: "poem", styleName: pdfStylePoemSubtitle, space: pdfDefaultCSSRootFontSize * 0.5, align: textAlignCenter},
		{name: "stanza", styleName: pdfStyleStanzaSubtitle, space: pdfDefaultCSSRootFontSize * 0.25, align: textAlignCenter},
		{name: "epigraph", styleName: pdfStyleEpigraphSubtitle, space: pdfDefaultCSSRootFontSize * 0.3, align: textAlignRight, italic: true},
		{name: "cite", styleName: pdfStyleCiteSubtitle, space: pdfDefaultCSSRootFontSize * 0.5, align: textAlignLeft},
	}
	for _, tt := range tests {
		subtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle, StyleName: tt.styleName, StyleClasses: tt.styleName})
		if subtitle.SpaceBefore != tt.space || subtitle.SpaceAfter != tt.space {
			t.Fatalf("%s subtitle margins = %v/%v, want default.css %v/%v", tt.name, subtitle.SpaceBefore, subtitle.SpaceAfter, tt.space, tt.space)
		}
		if subtitle.Paragraph.Align != tt.align || subtitle.Paragraph.Bold != tt.bold || subtitle.Paragraph.Italic != tt.italic ||
			subtitle.Paragraph.FirstLineIndent != 0 {
			t.Fatalf(
				"%s subtitle style = align:%v bold:%t italic:%t indent:%v, want align:%v bold:%t italic:%t indent:0",
				tt.name,
				subtitle.Paragraph.Align,
				subtitle.Paragraph.Bold,
				subtitle.Paragraph.Italic,
				subtitle.Paragraph.FirstLineIndent,
				tt.align,
				tt.bold,
				tt.italic,
			)
		}
		if subtitle.KeepWithNextLines != tt.keepWith {
			t.Fatalf("%s subtitle keep-with-next = %d, want %d", tt.name, subtitle.KeepWithNextLines, tt.keepWith)
		}
	}
}

func TestPDFStyleResolverDateDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `body { text-indent: 2em; text-align: left; }`)
	date := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleDate, ContextClasses: pdfStylePoem})
	if date.Paragraph.Align != textAlignRight {
		t.Fatalf("date align = %v, want default.css right", date.Paragraph.Align)
	}
	if date.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("date indent = %v, want default.css zero indent", date.Paragraph.FirstLineIndent)
	}
	if date.SpaceBefore != pdfDefaultCSSRootFontSize*0.5 || date.SpaceAfter != pdfDefaultCSSRootFontSize*0.5 {
		t.Fatalf("date margins = %v/%v, want default.css 0.5em/0.5em", date.SpaceBefore, date.SpaceAfter)
	}
}

func TestPDFStyleResolverTextAuthorDefaultsMatchKFXDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `body { text-indent: 2em; }`)
	textAuthor := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockTextAuthor})
	if textAuthor.Paragraph.Align != textAlignRight {
		t.Fatalf("text-author align = %v, want right", textAuthor.Paragraph.Align)
	}
	if !textAuthor.Paragraph.Bold || !textAuthor.Paragraph.Italic {
		t.Fatalf("text-author weight/style = bold:%t italic:%t, want both true", textAuthor.Paragraph.Bold, textAuthor.Paragraph.Italic)
	}
	if textAuthor.Paragraph.FontSize != pdfDefaultCSSRootFontSize {
		t.Fatalf("text-author font size = %v, want base font size %v", textAuthor.Paragraph.FontSize, pdfDefaultCSSRootFontSize)
	}
	if textAuthor.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("text-author indent = %v, want 0", textAuthor.Paragraph.FirstLineIndent)
	}
	if textAuthor.SpaceBefore != 0 || textAuthor.SpaceAfter != 0 {
		t.Fatalf("text-author margins = %v/%v, want default.css no margins", textAuthor.SpaceBefore, textAuthor.SpaceAfter)
	}
}

func TestPDFStyleResolverCodeBlockInheritsParagraphAlignment(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { text-align: right; }
			code { white-space: pre-wrap; font-family: monospace; font-size: 70%; text-align: left; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleCode})
	if paragraph.Paragraph.Align != textAlignRight {
		t.Fatalf("code block align = %v, want right from paragraph inheritance", paragraph.Paragraph.Align)
	}
	if paragraph.Paragraph.FontFamily != "monospace" {
		t.Fatalf("code block font family = %q, want monospace", paragraph.Paragraph.FontFamily)
	}
	if !paragraph.Paragraph.PreserveSpace {
		t.Fatalf("code block preserve-space = false, want true")
	}
}

func TestPDFStyleResolverAppliesStylesheet(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p {
				font-family: "Noto Sans", sans-serif;
				font-weight: bold;
				font-style: italic;
				color: #336699;
				text-decoration: underline line-through;
				vertical-align: super;
				font-size: 12pt;
				line-height: 1.5;
				letter-spacing: 0.2em;
				text-align: center;
				text-indent: 2em;
				margin: 1em 2em 0.5em 3em;
				padding: 0.25em 0.5em 0.75em 1em;
				width: 80%;
				min-width: 30pt;
				max-width: 72pt;
				background-color: #eee;
				border: 2px solid red;
				hyphens: manual;
				orphans: 3;
				widows: 4;
			}
			@media amzn-et {
				.verse { margin-left: 2em; text-indent: 0; }
			}
			@media fbc-pdf {
				.pdf-only { margin-right: 4em; }
			}
			@media fbc-pdf and amzn-kf8 and amzn-et {
				.pdf-combo { text-align: right; }
			}
			@media not fbc-pdf {
				.not-pdf { margin-left: 5em; }
			}
			@media not amzn-et {
				p { text-align: right; }
			}
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.FontFamily != "Noto Sans" {
		t.Fatalf("paragraph font family = %q, want Noto Sans", paragraph.Paragraph.FontFamily)
	}
	if !paragraph.Paragraph.Bold || !paragraph.Paragraph.Italic {
		t.Fatalf("paragraph font weight/style = bold:%t italic:%t, want both true", paragraph.Paragraph.Bold, paragraph.Paragraph.Italic)
	}
	if paragraph.Paragraph.Color.String() != "#336699" {
		t.Fatalf("paragraph color = %s, want #336699", paragraph.Paragraph.Color)
	}
	if !paragraph.Paragraph.Underline || !paragraph.Paragraph.Strikethrough {
		t.Fatalf(
			"paragraph decorations = underline:%t strikethrough:%t, want both true",
			paragraph.Paragraph.Underline,
			paragraph.Paragraph.Strikethrough,
		)
	}
	if paragraph.Paragraph.VerticalAlign != textVerticalAlignSuper {
		t.Fatalf("paragraph vertical align = %s, want super", paragraph.Paragraph.VerticalAlign)
	}
	if paragraph.Paragraph.FontSize != 12 {
		t.Fatalf("paragraph font size = %v, want 12", paragraph.Paragraph.FontSize)
	}
	if paragraph.Paragraph.LineHeight != 18 {
		t.Fatalf("paragraph line height = %v, want 18", paragraph.Paragraph.LineHeight)
	}
	if paragraph.Paragraph.LetterSpacing < 2.399 || paragraph.Paragraph.LetterSpacing > 2.401 {
		t.Fatalf("paragraph letter spacing = %v, want 2.4", paragraph.Paragraph.LetterSpacing)
	}
	if paragraph.Paragraph.Align != textAlignCenter {
		t.Fatalf("paragraph align = %v, want center", paragraph.Paragraph.Align)
	}
	if paragraph.Paragraph.FirstLineIndent != 24 {
		t.Fatalf("paragraph first-line indent = %v, want 24", paragraph.Paragraph.FirstLineIndent)
	}
	if paragraph.SpaceBefore != 12 || paragraph.SpaceAfter != 6 {
		t.Fatalf("paragraph vertical margins = %v/%v, want 12/6", paragraph.SpaceBefore, paragraph.SpaceAfter)
	}
	if paragraph.MarginLeft != 36 || paragraph.MarginRight != 24 {
		t.Fatalf("paragraph horizontal margins = %v/%v, want 36/24", paragraph.MarginLeft, paragraph.MarginRight)
	}
	if paragraph.PaddingTop != 3 || paragraph.PaddingRight != 6 || paragraph.PaddingBottom != 9 || paragraph.PaddingLeft != 12 {
		t.Fatalf(
			"paragraph padding = %v/%v/%v/%v, want 3/6/9/12",
			paragraph.PaddingTop,
			paragraph.PaddingRight,
			paragraph.PaddingBottom,
			paragraph.PaddingLeft,
		)
	}
	if !paragraph.HasWidth || !paragraph.Width.Percent || paragraph.Width.Value != 80 || !paragraph.HasMinWidth || paragraph.MinWidth.Value != 30 ||
		!paragraph.HasMaxWidth ||
		paragraph.MaxWidth.Value != 72 {
		t.Fatalf("paragraph width constraints = %#v/%#v/%#v", paragraph.Width, paragraph.MinWidth, paragraph.MaxWidth)
	}
	if width := blockContentWidth(220, paragraph); width != 72 {
		t.Fatalf("paragraph constrained content width = %v, want 72", width)
	}
	if !paragraph.HasBackground || paragraph.BackgroundColor.String() != "#eeeeee" {
		t.Fatalf("paragraph background = %t %s, want #eeeeee", paragraph.HasBackground, paragraph.BackgroundColor)
	}
	if !paragraph.HasBorder || paragraph.BorderWidth != 1.5 || paragraph.BorderColor.String() != "#ff0000" {
		t.Fatalf("paragraph border = %t %v %s, want 1.5pt red", paragraph.HasBorder, paragraph.BorderWidth, paragraph.BorderColor)
	}
	if paragraph.Paragraph.Hyphenation != paragraphHyphenationManual {
		t.Fatalf("paragraph hyphenation = %s, want manual", pdfHyphenationString(paragraph.Paragraph.Hyphenation))
	}
	if paragraph.Orphans != 3 || paragraph.Widows != 4 {
		t.Fatalf("paragraph orphans/widows = %d/%d, want 3/4", paragraph.Orphans, paragraph.Widows)
	}

	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem})
	if verse.MarginLeft != 24 {
		t.Fatalf("verse margin-left = %v, want 24", verse.MarginLeft)
	}

	pdfOnly := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "pdf-only"})
	if pdfOnly.MarginRight != 48 {
		t.Fatalf("fbc-pdf media margin-right = %v, want 48", pdfOnly.MarginRight)
	}
	pdfCombo := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "pdf-combo"})
	if pdfCombo.Paragraph.Align != textAlignRight {
		t.Fatalf("fbc-pdf/amzn media align = %v, want right", pdfCombo.Paragraph.Align)
	}
	notPDF := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "not-pdf"})
	if notPDF.MarginLeft == 60 {
		t.Fatalf("not fbc-pdf media rule applied in PDF context")
	}
}

func TestPDFStyleResolverAppliesParagraphStyleClasses(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { text-indent: 2em; text-align: justify; }
			p.has-dropcap { text-indent: 0; }
			.warning { text-align: right; margin-left: 1em; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	style := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "has-dropcap warning"})
	if style.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("class-adjusted first-line indent = %v, want 0", style.Paragraph.FirstLineIndent)
	}
	if style.Paragraph.Align != textAlignRight {
		t.Fatalf("class-adjusted text align = %v, want right", style.Paragraph.Align)
	}
	if style.MarginLeft != pdfBaseFontSize {
		t.Fatalf("class-adjusted margin-left = %v, want %v", style.MarginLeft, pdfBaseFontSize)
	}
}

func TestPDFStyleResolverPageBreakDisplayAndAbsoluteUnits(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.forced { page-break-before: always; break-after: page; display: none; }
			.avoid { page-break-before: avoid; page-break-after: avoid; }
			.metric { margin-left: 25.4mm; margin-right: 2.54cm; margin-top: 1in; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	forced := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "forced"})
	if !forced.PageBreakBefore || forced.PageBreakBeforeMode != pdfPageBreakAlways || !forced.PageBreakAfter ||
		forced.PageBreakAfterMode != pdfPageBreakAlways ||
		!forced.Hidden {
		t.Fatalf(
			"forced style page/visibility flags = before:%t/%v after:%t/%v hidden:%t, want forced breaks and hidden",
			forced.PageBreakBefore,
			forced.PageBreakBeforeMode,
			forced.PageBreakAfter,
			forced.PageBreakAfterMode,
			forced.Hidden,
		)
	}

	avoid := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "avoid"})
	if avoid.PageBreakBefore || avoid.PageBreakBeforeMode != pdfPageBreakAvoid || avoid.PageBreakAfter ||
		avoid.PageBreakAfterMode != pdfPageBreakAvoid ||
		avoid.KeepWithNextLines != pdfSingleKeepLine {
		t.Fatalf(
			"avoid style page-breaks = before:%t/%v after:%t/%v keep:%d, want avoid/avoid keep-next",
			avoid.PageBreakBefore,
			avoid.PageBreakBeforeMode,
			avoid.PageBreakAfter,
			avoid.PageBreakAfterMode,
			avoid.KeepWithNextLines,
		)
	}

	metric := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "metric"})
	if metric.MarginLeft != 72 || metric.MarginRight != 72 || metric.SpaceBefore != 72 {
		t.Fatalf("metric margins = left:%v right:%v top:%v, want all 72pt", metric.MarginLeft, metric.MarginRight, metric.SpaceBefore)
	}
}

func TestPDFStyleResolverReplacementStylesheetDoesNotKeepDefaultCSSClasses(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `p { text-indent: 0; text-align: left; }`)

	annotation := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleAnnotation, ContextClasses: pdfStyleAnnotation})
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if annotation.SpaceBefore != paragraph.SpaceBefore || annotation.SpaceAfter != paragraph.SpaceAfter || annotation.MarginLeft != 0 ||
		annotation.MarginRight != 0 {
		t.Fatalf(
			"annotation style survived replacement stylesheet: margins=%v/%v horizontal=%v/%v paragraph=%v/%v",
			annotation.SpaceBefore,
			annotation.SpaceAfter,
			annotation.MarginLeft,
			annotation.MarginRight,
			paragraph.SpaceBefore,
			paragraph.SpaceAfter,
		)
	}

	chapterTitle := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		Depth:          1,
		StyleName:      pdfStyleChapterTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-first"),
		ContextClasses: pdfStyleChapterTitle,
	})
	if chapterTitle.PageBreakBefore {
		t.Fatalf("chapter-title page break survived replacement stylesheet")
	}

	sectionH2 := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		Depth:          2,
		StyleName:      pdfStyleSectionTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2, pdfStyleSectionTitleHeader+"-first"),
		ContextClasses: joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2),
	})
	if sectionH2.PageBreakBefore {
		t.Fatalf("section-title-h2 page break survived replacement stylesheet")
	}

	footnote := inlineRunParagraphStyle(resolver, paragraph.Paragraph, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if footnote.Underline || footnote.VerticalAlign != textVerticalAlignBaseline || footnote.FontSize != paragraph.Paragraph.FontSize {
		t.Fatalf(
			"link-footnote defaults survived replacement stylesheet: underline:%t valign:%v font:%v paragraph font:%v",
			footnote.Underline,
			footnote.VerticalAlign,
			footnote.FontSize,
			paragraph.Paragraph.FontSize,
		)
	}
}

func TestPDFStyleResolverDefaultCSSPageBreakAvoidControls(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, class := range []string{pdfStyleTextAuthor, pdfStyleDate, pdfStyleVignetteChapterEnd, pdfStyleVignetteSectionEnd} {
		style := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: class})
		if style.PageBreakBefore || style.PageBreakBeforeMode != pdfPageBreakAvoid {
			t.Fatalf("%s page-break-before = %t/%v, want avoid", class, style.PageBreakBefore, style.PageBreakBeforeMode)
		}
	}
}

func TestPDFStyleResolverExplicitCSSResetsOverrideEarlierRules(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `
		.section-title-h2 { page-break-before: auto; }
		.section-subtitle { page-break-after: auto; }
		.hidden { display: none; }
		.hidden { display: block; }
		.italic { font-style: italic; }
		.italic { font-style: normal; }
		.bold { font-weight: bold; }
		.bold { font-weight: normal; }
		.link-external { text-decoration: none; }
	`)

	sectionH2 := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, StyleClasses: pdfStyleSectionTitleH2})
	if sectionH2.PageBreakBefore || sectionH2.PageBreakBeforeMode != pdfPageBreakAuto {
		t.Fatalf("section-title-h2 page-break-before survived explicit auto reset: %t/%v", sectionH2.PageBreakBefore, sectionH2.PageBreakBeforeMode)
	}

	sectionSubtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleSubtitle})
	if sectionSubtitle.PageBreakAfter || sectionSubtitle.PageBreakAfterMode != pdfPageBreakAuto || sectionSubtitle.KeepWithNextLines != 0 {
		t.Fatalf(
			"section-subtitle page-break-after avoid survived explicit auto reset: %t/%v keep:%d",
			sectionSubtitle.PageBreakAfter,
			sectionSubtitle.PageBreakAfterMode,
			sectionSubtitle.KeepWithNextLines,
		)
	}

	hidden := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "hidden"})
	if hidden.Hidden {
		t.Fatalf("display:none survived explicit display:block reset")
	}

	italic := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "italic"})
	if italic.Paragraph.Italic {
		t.Fatalf("font-style:italic survived explicit normal reset")
	}

	bold := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "bold"})
	if bold.Paragraph.Bold {
		t.Fatalf("font-weight:bold survived explicit normal reset")
	}

	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	link := inlineRunParagraphStyle(resolver, paragraph.Paragraph, pdfInlineRun{StyleClasses: pdfStyleLinkExternal})
	if link.Underline {
		t.Fatalf("link underline survived explicit text-decoration:none reset")
	}
}
