package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFStyleResolverPreservesExplicitParagraphLineHeightAgainstRootInheritance(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			body { line-height: 200%; }
			p { line-height: 13.4pt; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.LineHeight != pdfBaseLineHeight {
		t.Fatalf("paragraph line height = %v, want explicit paragraph %v", paragraph.Paragraph.LineHeight, pdfBaseLineHeight)
	}
	if !paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("paragraph line height should stay marked explicit")
	}
}

func TestPDFStyleResolverClassFontSizeKeepsExplicitBaseLineHeight(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { line-height: 18pt; }
			.big { font-size: 200%; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "big"})
	if paragraph.Paragraph.FontSize != pdfBaseFontSize*2 {
		t.Fatalf("paragraph font size = %v, want %v", paragraph.Paragraph.FontSize, pdfBaseFontSize*2)
	}
	if paragraph.Paragraph.LineHeight != 18 {
		t.Fatalf("paragraph line height = %v, want explicit 18pt", paragraph.Paragraph.LineHeight)
	}
}

func TestPDFStyleResolverClassFontSizeStillAdjustsImplicitLineHeight(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.big { font-size: 200%; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "big"})
	if paragraph.Paragraph.FontSize != pdfBaseFontSize*2 {
		t.Fatalf("paragraph font size = %v, want %v", paragraph.Paragraph.FontSize, pdfBaseFontSize*2)
	}
	if math.Abs(paragraph.Paragraph.LineHeight-pdfBaseLineHeight*2) > 0.001 {
		t.Fatalf("paragraph line height = %v, want implicit scaled %v", paragraph.Paragraph.LineHeight, pdfBaseLineHeight*2)
	}
	if paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("implicit scaled line height should not be marked explicit")
	}
}

func TestPDFStyleResolverImplicitLineHeightUsesSemanticBlockDefault(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.big { font-size: 200%; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, StyleClasses: "big"})
	if verse.Paragraph.FontSize != pdfBaseFontSize*2 {
		t.Fatalf("verse font size = %v, want %v", verse.Paragraph.FontSize, pdfBaseFontSize*2)
	}
	wantLineHeight := pdfVerseLineHeight * 2
	if math.Abs(verse.Paragraph.LineHeight-wantLineHeight) > 0.001 {
		t.Fatalf("verse line height = %v, want implicit semantic default scaled %v", verse.Paragraph.LineHeight, wantLineHeight)
	}
	if verse.Paragraph.LineHeightExplicit {
		t.Fatalf("implicit semantic line height should not be marked explicit")
	}
}

func TestPDFStyleResolverImplicitLineHeightInjectedAfterRootFontSize(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `body { font-size: 200%; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.FontSize != pdfBaseFontSize*2 {
		t.Fatalf("paragraph font size = %v, want root inherited %v", paragraph.Paragraph.FontSize, pdfBaseFontSize*2)
	}
	if math.Abs(paragraph.Paragraph.LineHeight-pdfBaseLineHeight*2) > 0.001 {
		t.Fatalf("paragraph line height = %v, want injected %v", paragraph.Paragraph.LineHeight, pdfBaseLineHeight*2)
	}
	if paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("injected line height should not be marked explicit")
	}
}
