package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFInlineRunFootnoteLinkDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	base := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph

	footnote := inlineRunParagraphStyle(resolver, base, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if !footnote.Underline {
		t.Fatalf("footnote link underline = false, want default link underline")
	}
	assertInlineFloat(t, footnote.FontSize, base.FontSize*0.80, "footnote link font size")
	if footnote.VerticalAlign != textVerticalAlignSuper {
		t.Fatalf("footnote link vertical-align = %v, want super", footnote.VerticalAlign)
	}
	if footnote.LineHeight != base.LineHeight {
		t.Fatalf("footnote link line-height = %v, want inherited base line-height %v", footnote.LineHeight, base.LineHeight)
	}
}

func TestPDFInlineRunFootnoteLinkContextAwareSizing(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	h1Block := pdfTextBlock{Kind: pdfBlockHeading, Depth: 1}
	h1 := resolver.styleForBlock(h1Block).Paragraph
	h2Block := pdfTextBlock{Kind: pdfBlockHeading, Depth: 2}
	h2 := resolver.styleForBlock(h2Block).Paragraph
	footnoteBlock := pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote}
	footnoteBody := resolver.styleForBlock(footnoteBlock).Paragraph

	ordinarySup := inlineRunParagraphStyle(resolver, paragraph, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, Superscript: true})
	assertInlineFloat(t, ordinarySup.FontSize, paragraph.FontSize*0.75, "ordinary sup footnote font size")

	h1Direct := inlineRunParagraphStyle(
		resolver,
		h1,
		pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, ContextClasses: inlineRunContextClassesForBlock(h1Block)},
	)
	assertInlineFloat(t, h1Direct.FontSize, h1.FontSize*0.90, "h1 direct footnote font size")

	h1Sup := inlineRunParagraphStyle(
		resolver,
		h1,
		pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, ContextClasses: inlineRunContextClassesForBlock(h1Block), Superscript: true},
	)
	assertInlineFloat(t, h1Sup.FontSize, h1.FontSize*0.70, "h1 sup footnote font size")

	h2Direct := inlineRunParagraphStyle(
		resolver,
		h2,
		pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, ContextClasses: inlineRunContextClassesForBlock(h2Block)},
	)
	assertInlineFloat(t, h2Direct.FontSize, h2.FontSize*0.90, "h2 direct footnote font size")

	codeRun := pdfInlineRun{StyleClasses: joinStyleClasses(pdfStyleCode, pdfStyleLinkFootnote), Code: true}
	code := inlineRunParagraphStyle(resolver, paragraph, codeRun)
	if code.FontFamily != "monospace" {
		t.Fatalf("code footnote font family = %q, want monospace", code.FontFamily)
	}
	assertInlineFloat(t, code.FontSize, inlineFootnoteContextParagraphStyle(resolver, paragraph, codeRun).FontSize*0.80, "code footnote font size")

	footnoteBodyLink := inlineRunParagraphStyle(
		resolver,
		footnoteBody,
		pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, ContextClasses: inlineRunContextClassesForBlock(footnoteBlock)},
	)
	assertInlineFloat(t, footnoteBodyLink.FontSize, footnoteBody.FontSize*0.80, "footnote body link font size")
}

func TestPDFInlineRunFootnoteSupFragmentDoesNotDoubleShrink(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	base := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	fragment, err := inlineRunFragment(
		pdfDocumentSpec{},
		newPDFFontRegistry(nil, nil),
		resolver,
		base,
		pdfInlineRun{StyleClasses: pdfStyleLinkFootnote, Superscript: true},
		"1",
		100,
	)
	if err != nil {
		t.Fatalf("inlineRunFragment() error = %v", err)
	}
	assertInlineFloat(t, fragment.FontSize, base.FontSize*0.75, "sup footnote fragment font size")
	if fragment.BaselineShift <= 0 {
		t.Fatalf("sup footnote baseline shift = %v, want positive", fragment.BaselineShift)
	}
}

func TestPDFInlineRunFootnoteLinkCSSOverrides(t *testing.T) {
	baselineResolver := newPDFStyleResolverWithDefaultCSS(t, `
		.link-footnote {
			vertical-align: baseline;
			font-size: 100%;
			text-decoration: none;
		}
	`)
	baseline := baselineResolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	baselineLink := inlineRunParagraphStyle(baselineResolver, baseline, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if baselineLink.VerticalAlign != textVerticalAlignBaseline || baselineLink.Underline || baselineLink.FontSize != baseline.FontSize {
		t.Fatalf(
			"baseline override style = underline:%t valign:%v font:%v base:%v",
			baselineLink.Underline,
			baselineLink.VerticalAlign,
			baselineLink.FontSize,
			baseline.FontSize,
		)
	}

	customSizeResolver := newPDFStyleResolverWithDefaultCSS(t, `
		.link-footnote {
			font-size: 60%;
			vertical-align: super;
		}
	`)
	customSizeBase := customSizeResolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	customSizeLink := inlineRunParagraphStyle(customSizeResolver, customSizeBase, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if customSizeLink.VerticalAlign != textVerticalAlignSuper {
		t.Fatalf("custom-size footnote vertical-align = %v, want super", customSizeLink.VerticalAlign)
	}
	assertInlineFloat(t, customSizeLink.FontSize, customSizeBase.FontSize*0.60, "custom-size footnote font size")

	decorationResolver := newPDFStyleResolverWithDefaultCSS(t, `
		.link-footnote {
			color: #ff0000;
			text-decoration: none;
		}
	`)
	decorationBase := decorationResolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	decorationLink := inlineRunParagraphStyle(decorationResolver, decorationBase, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	assertInlineFloat(t, decorationLink.FontSize, decorationBase.FontSize*0.80, "decoration-only footnote font size")
	if decorationLink.Color.String() != "#ff0000" || decorationLink.Underline {
		t.Fatalf("decoration-only footnote = color:%s underline:%t, want red without underline", decorationLink.Color, decorationLink.Underline)
	}
}

func TestPDFInlineRunFootnoteLinkClearsItalicWithDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	poem := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, StyleClasses: pdfStylePoem, ContextClasses: pdfStylePoem}).Paragraph
	if !poem.Italic {
		t.Fatalf("poem italic = false, want default.css italic context")
	}

	footnote := inlineRunParagraphStyle(resolver, poem, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if footnote.Italic {
		t.Fatalf("footnote link italic = true, want default.css font-style: normal to clear inherited italic")
	}
	if !footnote.Underline || footnote.VerticalAlign != textVerticalAlignSuper {
		t.Fatalf("footnote link style = underline:%t valign:%v, want default.css underline/superscript", footnote.Underline, footnote.VerticalAlign)
	}
	if footnote.LineHeight != poem.LineHeight {
		t.Fatalf("footnote link line-height = %v, want inherited poem line-height %v", footnote.LineHeight, poem.LineHeight)
	}
}

func TestPDFInlineRunReplacementStylesheetDoesNotKeepFootnoteLinkDefaults(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `.poem { font-style: italic; }`)
	poem := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, StyleClasses: pdfStylePoem, ContextClasses: pdfStylePoem}).Paragraph
	if !poem.Italic {
		t.Fatalf("poem italic = false, want replacement stylesheet italic context")
	}

	footnote := inlineRunParagraphStyle(resolver, poem, pdfInlineRun{StyleClasses: pdfStyleLinkFootnote})
	if !footnote.Italic || footnote.Underline || footnote.VerticalAlign != textVerticalAlignBaseline || footnote.FontSize != poem.FontSize {
		t.Fatalf(
			"link-footnote defaults survived replacement stylesheet: italic:%t underline:%t valign:%v font:%v poem font:%v",
			footnote.Italic,
			footnote.Underline,
			footnote.VerticalAlign,
			footnote.FontSize,
			poem.FontSize,
		)
	}
}

func TestPDFInlineRunBacklinkDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	base := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph

	backlink := inlineRunParagraphStyle(resolver, base, pdfInlineRun{StyleClasses: pdfStyleLinkBacklink})
	if !backlink.Underline || !backlink.Bold || backlink.Color.String() != "#808080" {
		t.Fatalf(
			"backlink style = underline:%t bold:%t color:%s, want default.css underline bold gray",
			backlink.Underline,
			backlink.Bold,
			backlink.Color,
		)
	}

	footnoteBase := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: pdfStyleFootnote}).Paragraph
	footnoteBacklink := inlineRunParagraphStyle(resolver, footnoteBase, pdfInlineRun{
		StyleClasses:   pdfStyleLinkBacklink,
		ContextClasses: joinStyleClasses(pdfStyleFootnote, "p"),
	})
	if footnoteBacklink.FontSize != footnoteBase.FontSize {
		t.Fatalf("footnote backlink font size = %v, want inherited footnote size %v", footnoteBacklink.FontSize, footnoteBase.FontSize)
	}
}

func assertInlineFloat(t *testing.T, got float64, want float64, name string) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestPDFInlineRunAppliesContextDescendantSelectors(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.footnote .accent { color: #ff0000; font-weight: bold; }
			p code { color: #336699; text-decoration: underline; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	base := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph

	accent := inlineRunParagraphStyle(resolver, base, pdfInlineRun{StyleClasses: "accent", ContextClasses: joinStyleClasses(pdfStyleFootnote, "p")})
	if accent.Color.String() != "#ff0000" {
		t.Fatalf("accent color = %s, want #ff0000 from .footnote .accent", accent.Color)
	}
	if !accent.Bold {
		t.Fatalf("accent bold = false, want true from .footnote .accent")
	}

	code := inlineRunParagraphStyle(resolver, base, pdfInlineRun{StyleClasses: pdfStyleCode, ContextClasses: "p", Code: true})
	if code.Color.String() != "#336699" {
		t.Fatalf("code color = %s, want #336699 from p code", code.Color)
	}
	if !code.Underline {
		t.Fatalf("code underline = false, want true from p code")
	}
}
