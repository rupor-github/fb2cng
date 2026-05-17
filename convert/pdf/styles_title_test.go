package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFStyleResolverTitleVariantMarginsMatchFlattenedWrapperDefaults(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	first := resolver.styleForBlock(pdfTextBlock{
		Kind:         pdfBlockHeading,
		StyleName:    pdfStyleChapterTitleHeader,
		StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-first"),
	})
	if first.SpaceBefore != pdfBaseFontSize*2 || first.SpaceAfter != pdfBaseFontSize {
		t.Fatalf("title-first margins = %v/%v, want default.css wrapper 2em/1em", first.SpaceBefore, first.SpaceAfter)
	}

	next := resolver.styleForBlock(pdfTextBlock{
		Kind:         pdfBlockHeading,
		StyleName:    pdfStyleChapterTitleHeader,
		StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-next"),
	})
	if next.SpaceBefore != pdfBaseFontSize*2 || next.SpaceAfter != pdfBaseFontSize {
		t.Fatalf("title-next margins = %v/%v, want default.css wrapper 2em/1em", next.SpaceBefore, next.SpaceAfter)
	}
}

func TestPDFStyleResolverParagraphTitleZeroMarginVariantKeepsBaseVerticalMargins(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.poem-title { margin: 10pt 0 5pt 0; }
			.poem-title-first { margin: 0; text-align: center; text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	title := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStylePoemTitle + " " + pdfStylePoemTitle + "-first"})
	if title.SpaceBefore != 10 || title.SpaceAfter != 5 {
		t.Fatalf("poem title margins = %v/%v, want base 10/5 preserved", title.SpaceBefore, title.SpaceAfter)
	}
	if title.Paragraph.Align != textAlignCenter {
		t.Fatalf("poem title align = %v, want center from first variant", title.Paragraph.Align)
	}
	if title.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("poem title indent = %v, want 0 from first variant", title.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverFootnoteTitleVariantKeepsBaseVerticalMargins(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.footnote-title { margin: 12pt 0 6pt 0; text-align: left; }
			.footnote-title-first { margin: 2pt 0; text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	title := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnoteTitle + " " + pdfStyleFootnoteTitle + "-first"})
	if title.SpaceBefore != 12 || title.SpaceAfter != 6 {
		t.Fatalf("footnote title margins = %v/%v, want base 12/6 preserved", title.SpaceBefore, title.SpaceAfter)
	}
	if title.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("footnote title indent = %v, want 0 from first variant", title.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverFootnoteTitleInheritsParagraphAlignment(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { text-align: right; }
			.footnote-title { text-align: left; font-weight: bold; }
			.footnote-title-first { text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	title := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnoteTitle + " " + pdfStyleFootnoteTitle + "-first", ContextClasses: pdfStyleFootnoteTitle})
	if title.Paragraph.Align != textAlignRight {
		t.Fatalf("footnote title align = %v, want right from paragraph inheritance", title.Paragraph.Align)
	}
	if !title.Paragraph.Bold {
		t.Fatalf("footnote title bold = false, want true from footnote-title")
	}
	if title.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("footnote title indent = %v, want 0 from first variant", title.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverAppliesFootnoteTitleContextDescendantSelectors(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.footnote-title p { text-indent: 0.5em; letter-spacing: 0.1em; }
			.footnote-title-first { text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	title := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnoteTitle + " " + pdfStyleFootnoteTitle + "-first", ContextClasses: pdfStyleFootnoteTitle})
	if title.Paragraph.FirstLineIndent != pdfBaseFontSize*0.5 {
		t.Fatalf("footnote title indent = %v, want %v from .footnote-title p", title.Paragraph.FirstLineIndent, pdfBaseFontSize*0.5)
	}
	if title.Paragraph.LetterSpacing != pdfBaseFontSize*0.1 {
		t.Fatalf("footnote title letter-spacing = %v, want %v from .footnote-title p", title.Paragraph.LetterSpacing, pdfBaseFontSize*0.1)
	}
}

func TestPDFStyleResolverTitleAfterImageKeepsHeadingTextAlignment(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	style := resolver.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockHeading,
		StyleName:      pdfStyleChapterTitleHeader,
		StyleClasses:   joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-next", pdfStyleTitleAfterImage),
		ContextClasses: pdfStyleChapterTitle,
	})

	if style.Paragraph.Align != textAlignCenter {
		t.Fatalf("title-after-image alignment = %v, want heading center alignment", style.Paragraph.Align)
	}
	if style.Paragraph.FontSize != resolver.styles[pdfStyleChapterTitleHeader].Paragraph.FontSize {
		t.Fatalf("title-after-image font size = %v, want heading font size", style.Paragraph.FontSize)
	}
	if style.SpaceBefore != pdfTitleAfterImageSpaceBefore || style.SpaceAfter != 0 {
		t.Fatalf("title-after-image margins = %v/%v, want %v/0", style.SpaceBefore, style.SpaceAfter, pdfTitleAfterImageSpaceBefore)
	}
}

func TestPDFStyleResolverHeadingDefaultsMatchCSSAndKFX(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	h1 := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	if h1.Paragraph.FontSize != pdfBaseFontSize*1.4 {
		t.Fatalf("h1 font size = %v, want default.css 140%%", h1.Paragraph.FontSize)
	}
	wantH1Margin := h1.Paragraph.FontSize * pdfHeadingH1MarginFactor
	if math.Abs(h1.SpaceBefore-wantH1Margin) > 0.001 || math.Abs(h1.SpaceAfter-wantH1Margin) > 0.001 {
		t.Fatalf("h1 margins = %v/%v, want KFX/CSS heading margins %v/%v", h1.SpaceBefore, h1.SpaceAfter, wantH1Margin, wantH1Margin)
	}

	h2 := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 2})
	if h2.Paragraph.FontSize != pdfBaseFontSize*1.2 {
		t.Fatalf("h2 font size = %v, want default.css 120%%", h2.Paragraph.FontSize)
	}
	wantH2Margin := h2.Paragraph.FontSize * pdfHeadingNestedMarginFactor
	if math.Abs(h2.SpaceBefore-wantH2Margin) > 0.001 || math.Abs(h2.SpaceAfter-wantH2Margin) > 0.001 {
		t.Fatalf("h2 margins = %v/%v, want KFX/CSS heading margins %v/%v", h2.SpaceBefore, h2.SpaceAfter, wantH2Margin, wantH2Margin)
	}
}

func TestPDFStyleResolverMapsHeadingAndTOCStyles(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			h1 { font-size: 150%; }
			.toc-item { text-align: right; margin-bottom: 2pt; }
			.section-subtitle { page-break-after: avoid; }
		`,
	}}}
	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	if heading.Paragraph.FontSize != 15.75 {
		t.Fatalf("heading font size = %v, want 15.75", heading.Paragraph.FontSize)
	}

	tocEntry := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockTOCEntry, Depth: 3})
	if tocEntry.Paragraph.Align != textAlignRight {
		t.Fatalf("toc align = %v, want right", tocEntry.Paragraph.Align)
	}
	if tocEntry.Paragraph.FirstLineIndent != pdfTOCNestedListIndent*2 {
		t.Fatalf("toc indent = %v, want %v", tocEntry.Paragraph.FirstLineIndent, pdfTOCNestedListIndent*2)
	}
	if tocEntry.SpaceAfter != 2 {
		t.Fatalf("toc margin-bottom = %v, want 2", tocEntry.SpaceAfter)
	}

	subtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle})
	if subtitle.KeepWithNextLines != 1 {
		t.Fatalf("subtitle keep-with-next = %d, want 1", subtitle.KeepWithNextLines)
	}
}

func TestPDFStyleResolverSectionSubtitleMarginsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	sectionSubtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle})
	if sectionSubtitle.SpaceBefore != pdfBaseFontSize || sectionSubtitle.SpaceAfter != pdfBaseFontSize {
		t.Fatalf("section subtitle margins = %v/%v, want default.css 1em/1em", sectionSubtitle.SpaceBefore, sectionSubtitle.SpaceAfter)
	}
}

func TestPDFStyleResolverUsesContextSpecificSubtitleDefaults(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	citeSubtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle, StyleName: pdfStyleCiteSubtitle, StyleClasses: pdfStyleCiteSubtitle})
	if citeSubtitle.Paragraph.Bold {
		t.Fatalf("cite-subtitle bold = true, want false to match KFX/default.css semantics")
	}
	if citeSubtitle.Paragraph.Align != textAlignLeft {
		t.Fatalf("cite-subtitle align = %v, want left", citeSubtitle.Paragraph.Align)
	}
	if citeSubtitle.KeepWithNextLines != 0 {
		t.Fatalf("cite-subtitle keep-with-next = %d, want 0", citeSubtitle.KeepWithNextLines)
	}

	sectionSubtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle})
	if !sectionSubtitle.Paragraph.Bold {
		t.Fatalf("section subtitle bold = false, want true")
	}
	if sectionSubtitle.Paragraph.Align != textAlignCenter {
		t.Fatalf("section subtitle align = %v, want center", sectionSubtitle.Paragraph.Align)
	}
	if sectionSubtitle.Paragraph.FontSize != pdfBaseFontSize {
		t.Fatalf("section subtitle font size = %v, want base font size %v", sectionSubtitle.Paragraph.FontSize, pdfBaseFontSize)
	}
}
