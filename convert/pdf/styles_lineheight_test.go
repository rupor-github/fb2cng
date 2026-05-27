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

func TestPDFStyleResolverHeadingLineHeightUsesKP3AdjustedLH(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, styleName := range []string{
		pdfStyleBodyTitleHeader,
		pdfStyleChapterTitleHeader,
		pdfStyleTOCTitle,
	} {
		heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, StyleName: styleName})
		if math.Abs(heading.Paragraph.LineHeight-pdfAdjustedLineHeight) > 0.001 {
			t.Fatalf("%s line height = %v, want KP3 adjusted 1lh %v", styleName, heading.Paragraph.LineHeight, pdfAdjustedLineHeight)
		}
		if !heading.Paragraph.LineHeightExplicit {
			t.Fatalf("%s line height should be explicit like KFX title text styles", styleName)
		}
	}
}

func TestPDFStyleResolverSectionTitleHeaderLineHeightUsesKP3SpecialLH(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	sectionTitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 2})
	if math.Abs(sectionTitle.Paragraph.LineHeight-pdfSectionTitleHeaderLineHeight) > 0.001 {
		t.Fatalf("section-title-header line height = %v, want KP3 special 1lh %v", sectionTitle.Paragraph.LineHeight, pdfSectionTitleHeaderLineHeight)
	}
	if !sectionTitle.Paragraph.LineHeightExplicit {
		t.Fatalf("section-title-header line height should be explicit like KFX title text styles")
	}
}

func TestPDFStyleResolverSubtitleLineHeightInheritsDefaultCSSRootRhythm(t *testing.T) {
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
		if math.Abs(subtitle.Paragraph.LineHeight-pdfDefaultCSSRootLineHeight) > 0.001 {
			t.Fatalf(
				"%s line height = %v, want default.css root line-height %v",
				styleName,
				subtitle.Paragraph.LineHeight,
				pdfDefaultCSSRootLineHeight,
			)
		}
	}
}

func TestPDFStyleResolverVerseTextAuthorAndDateLineHeightInheritDefaultCSSRootRhythm(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)
	for _, block := range []pdfTextBlock{
		{Kind: pdfBlockPoem},
		{Kind: pdfBlockTextAuthor},
		{Kind: pdfBlockParagraph, StyleClasses: pdfStyleDate},
	} {
		style := resolver.styleForBlock(block)
		if math.Abs(style.Paragraph.LineHeight-pdfDefaultCSSRootLineHeight) > 0.001 {
			t.Fatalf("%s line height = %v, want default.css root line-height %v", block.Kind, style.Paragraph.LineHeight, pdfDefaultCSSRootLineHeight)
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

func TestPDFStyleResolverDefaultCSSRootRhythmKeepsPDFBaseText(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `body { font-size: 80%; line-height: 150%; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if math.Abs(paragraph.Paragraph.FontSize-pdfBaseFontSize) > 0.001 {
		t.Fatalf("paragraph font size = %v, want PDF base %v for default.css reader rhythm", paragraph.Paragraph.FontSize, pdfBaseFontSize)
	}
	if math.Abs(paragraph.Paragraph.LineHeight-pdfBaseLineHeight) > 0.001 {
		t.Fatalf(
			"paragraph line height = %v, want PDF base line height %v for default.css reader rhythm",
			paragraph.Paragraph.LineHeight,
			pdfBaseLineHeight,
		)
	}
	if paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("default.css reader rhythm should not mark PDF base line height explicit")
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

func TestPDFStyleResolverRelativeFontSizeAndSpacingUseInheritedFont(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			body { font-size: 80%; line-height: 170%; }
			h1 { font-size: 140%; }
			.chapter-title { margin: 2em 0 1em 0; }
			.section-subtitle { margin: 1em 0; }
			p { font-size: 120%; }
			.note { margin-left: 2em; padding-left: 0.5em; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	rootFont := pdfBaseFontSize * 0.8

	heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	wantHeadingFont := rootFont * 1.4
	if math.Abs(heading.Paragraph.FontSize-wantHeadingFont) > 0.001 {
		t.Fatalf("heading font size = %v, want inherited 140%% %v", heading.Paragraph.FontSize, wantHeadingFont)
	}
	if math.Abs(heading.Paragraph.LineHeight-pdfAdjustedLineHeight) > 0.001 {
		t.Fatalf("heading line height = %v, want adjusted KP3 1lh %v", heading.Paragraph.LineHeight, pdfAdjustedLineHeight)
	}

	chapterHeading := resolver.styleForBlock(
		pdfTextBlock{Kind: pdfBlockHeading, Depth: 1, StyleClasses: pdfStyleChapterTitle, ContextClasses: pdfStyleChapterTitle},
	)
	if math.Abs(chapterHeading.SpaceBefore-rootFont*2) > 0.001 || math.Abs(chapterHeading.SpaceAfter-rootFont) > 0.001 {
		t.Fatalf("chapter title wrapper margins = %v/%v, want %v/%v", chapterHeading.SpaceBefore, chapterHeading.SpaceAfter, rootFont*2, rootFont)
	}

	subtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle, StyleName: pdfStyleSubtitle})
	if math.Abs(subtitle.Paragraph.FontSize-rootFont) > 0.001 || math.Abs(subtitle.SpaceBefore-rootFont) > 0.001 ||
		math.Abs(subtitle.SpaceAfter-rootFont) > 0.001 {
		t.Fatalf("subtitle font/margins = %v %v/%v, want root %v", subtitle.Paragraph.FontSize, subtitle.SpaceBefore, subtitle.SpaceAfter, rootFont)
	}
	if math.Abs(subtitle.Paragraph.LineHeight-rootFont*1.7) > 0.001 {
		t.Fatalf("subtitle line height = %v, want inherited body 170%% %v", subtitle.Paragraph.LineHeight, rootFont*1.7)
	}

	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "note"})
	wantParagraphFont := rootFont * 1.2
	if math.Abs(paragraph.Paragraph.FontSize-wantParagraphFont) > 0.001 {
		t.Fatalf("paragraph font size = %v, want inherited 120%% %v", paragraph.Paragraph.FontSize, wantParagraphFont)
	}
	if math.Abs(paragraph.MarginLeft-wantParagraphFont*2) > 0.001 || math.Abs(paragraph.PaddingLeft-wantParagraphFont*0.5) > 0.001 {
		t.Fatalf(
			"paragraph relative spacing = margin %v padding %v, want %v/%v",
			paragraph.MarginLeft,
			paragraph.PaddingLeft,
			wantParagraphFont*2,
			wantParagraphFont*0.5,
		)
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
