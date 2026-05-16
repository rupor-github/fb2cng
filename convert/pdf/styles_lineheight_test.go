package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFStyleResolverBaseLineHeightUsesKP3NormalRatio(t *testing.T) {
	resolver := newPDFStyleResolver(nil, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	wantLineHeight := pdfBaseFontSize * pdfNormalLineHeightFactor
	if math.Abs(paragraph.Paragraph.LineHeight-wantLineHeight) > 0.001 {
		t.Fatalf("paragraph line height = %v, want KP3 normal ratio %v", paragraph.Paragraph.LineHeight, wantLineHeight)
	}
}

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
	if paragraph.Paragraph.LineHeight != 13.4 {
		t.Fatalf("paragraph line height = %v, want explicit paragraph 13.4", paragraph.Paragraph.LineHeight)
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

func TestPDFStyleResolverCodeLineHeightPreservesKP3BaseRhythm(t *testing.T) {
	resolver := newPDFStyleResolver(nil, zaptest.NewLogger(t))
	code := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleCode})
	if math.Abs(code.Paragraph.LineHeight-pdfBaseLineHeight) > 0.001 {
		t.Fatalf("code line height = %v, want KP3 1lh base rhythm %v", code.Paragraph.LineHeight, pdfBaseLineHeight)
	}
}

func TestPDFStyleResolverHeadingLineHeightUsesKP3AdjustedRatio(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, styleName := range []string{
		pdfStyleBodyTitleHeader,
		pdfStyleChapterTitleHeader,
		pdfStyleTOCTitle,
	} {
		heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, StyleName: styleName})
		wantLineHeight := heading.Paragraph.FontSize * pdfAdjustedLineHeightFactor
		if math.Abs(heading.Paragraph.LineHeight-wantLineHeight) > 0.001 {
			t.Fatalf("%s line height = %v, want KP3 adjusted ratio %v", styleName, heading.Paragraph.LineHeight, wantLineHeight)
		}
	}
}

func TestPDFStyleResolverSectionTitleHeaderLineHeightUsesKP3SpecialRatio(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	sectionTitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 2})
	wantLineHeight := sectionTitle.Paragraph.FontSize * pdfSectionTitleHeaderLineHeightFactor
	if math.Abs(sectionTitle.Paragraph.LineHeight-wantLineHeight) > 0.001 {
		t.Fatalf("section-title-header line height = %v, want KP3 special ratio %v", sectionTitle.Paragraph.LineHeight, wantLineHeight)
	}
}

func TestPDFStyleResolverSubtitleLineHeightUsesKP3NormalRatio(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, styleName := range []string{
		pdfStyleSubtitle,
		pdfStyleAnnotationSubtitle,
		pdfStylePoemSubtitle,
		pdfStyleStanzaSubtitle,
		pdfStyleEpigraphSubtitle,
		pdfStyleCiteSubtitle,
	} {
		subtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle, StyleName: styleName})
		wantLineHeight := subtitle.Paragraph.FontSize * pdfNormalLineHeightFactor
		if math.Abs(subtitle.Paragraph.LineHeight-wantLineHeight) > 0.001 {
			t.Fatalf("%s line height = %v, want KP3 normal ratio %v", styleName, subtitle.Paragraph.LineHeight, wantLineHeight)
		}
	}
}

func TestPDFStyleResolverVerseTextAuthorAndDateLineHeightUseKP3NormalRatio(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, block := range []pdfTextBlock{
		{Kind: pdfBlockPoem},
		{Kind: pdfBlockTextAuthor},
		{Kind: pdfBlockParagraph, StyleClasses: pdfStyleDate},
	} {
		style := resolver.styleForBlock(block)
		wantLineHeight := style.Paragraph.FontSize * pdfNormalLineHeightFactor
		if math.Abs(style.Paragraph.LineHeight-wantLineHeight) > 0.001 {
			t.Fatalf("%s line height = %v, want KP3 normal ratio %v", block.Kind, style.Paragraph.LineHeight, wantLineHeight)
		}
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

func TestPDFStyleResolverSupportsNormalLineHeight(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `body { line-height: 200%; } p { line-height: normal; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	wantLineHeight := pdfBaseFontSize * pdfNormalLineHeightFactor
	if math.Abs(paragraph.Paragraph.LineHeight-wantLineHeight) > 0.001 {
		t.Fatalf("paragraph line height = %v, want normal %v", paragraph.Paragraph.LineHeight, wantLineHeight)
	}
	if !paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("normal line height should override inherited line height")
	}
}

func TestPDFStyleResolverNormalLineHeightUsesResolvedFontSize(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.big { font-size: 200%; line-height: normal; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "big"})
	wantLineHeight := pdfBaseFontSize * 2 * pdfNormalLineHeightFactor
	if math.Abs(paragraph.Paragraph.LineHeight-wantLineHeight) > 0.001 {
		t.Fatalf("paragraph line height = %v, want normal at class font size %v", paragraph.Paragraph.LineHeight, wantLineHeight)
	}
	if !paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("normal line height should be explicit")
	}
}
