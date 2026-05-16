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
	if math.Abs(footnote.FontSize-pdfFootnoteLinkFontSize) > 0.001 {
		t.Fatalf("footnote link font size = %v, want default.css 80%% %v", footnote.FontSize, pdfFootnoteLinkFontSize)
	}
	if footnote.VerticalAlign != textVerticalAlignSuper {
		t.Fatalf("footnote link vertical-align = %v, want super", footnote.VerticalAlign)
	}
}

func TestPDFInlineRunBacklinkDefaultsMatchDefaultCSS(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	base := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph

	backlink := inlineRunParagraphStyle(resolver, base, pdfInlineRun{StyleClasses: pdfStyleLinkBacklink})
	if !backlink.Underline || !backlink.Bold || backlink.Color.String() != "#808080" {
		t.Fatalf("backlink style = underline:%t bold:%t color:%s, want default.css underline bold gray", backlink.Underline, backlink.Bold, backlink.Color)
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
