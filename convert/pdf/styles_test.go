package pdf

import (
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
	if style.Paragraph.FontSize < 7.34 || style.Paragraph.FontSize > 7.36 {
		t.Fatalf("code font size = %v, want 70%% base font size", style.Paragraph.FontSize)
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
		t.Fatalf("paragraph decorations = underline:%t strikethrough:%t, want both true", paragraph.Paragraph.Underline, paragraph.Paragraph.Strikethrough)
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
		t.Fatalf("paragraph padding = %v/%v/%v/%v, want 3/6/9/12", paragraph.PaddingTop, paragraph.PaddingRight, paragraph.PaddingBottom, paragraph.PaddingLeft)
	}
	if !paragraph.HasWidth || !paragraph.Width.Percent || paragraph.Width.Value != 80 || !paragraph.HasMinWidth || paragraph.MinWidth.Value != 30 || !paragraph.HasMaxWidth || paragraph.MaxWidth.Value != 72 {
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
	if verse.MarginLeft != 21 {
		t.Fatalf("verse margin-left = %v, want 21", verse.MarginLeft)
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

func TestPDFStyleResolverAppliesRootPageMargins(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -10pt 0 -8pt; } body { margin: 1pt 2pt 3pt 4pt; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	page := resolver.pageStyle()
	if page.MarginLeft != -4 || page.MarginRight != -8 || page.SpaceBefore != 1 || page.SpaceAfter != 3 {
		t.Fatalf("page margins = top %v right %v bottom %v left %v, want 1/-8/3/-4", page.SpaceBefore, page.MarginRight, page.SpaceAfter, page.MarginLeft)
	}
}

func TestPDFStyleResolverAppliesBodyTypographyAsRootInheritance(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			body { font-family: "Noto Sans", sans-serif; line-height: 1.5; color: #336699; }
			p { margin: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.FontFamily != "Noto Sans" {
		t.Fatalf("paragraph font family = %q, want body-inherited Noto Sans", paragraph.Paragraph.FontFamily)
	}
	if paragraph.Paragraph.LineHeight != pdfBaseFontSize*1.5 {
		t.Fatalf("paragraph line height = %v, want body-inherited %v", paragraph.Paragraph.LineHeight, pdfBaseFontSize*1.5)
	}
	if paragraph.Paragraph.Color.String() != "#336699" {
		t.Fatalf("paragraph color = %s, want body-inherited #336699", paragraph.Paragraph.Color)
	}

	heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	if heading.Paragraph.FontFamily != "Noto Sans" {
		t.Fatalf("heading font family = %q, want body-inherited Noto Sans", heading.Paragraph.FontFamily)
	}
	if heading.Paragraph.LineHeight != resolver.defaultStyle(pdfStyleChapterTitleHeader).Paragraph.LineHeight {
		t.Fatalf("heading line height = %v, want explicit heading line-height preserved", heading.Paragraph.LineHeight)
	}
}

func TestPDFStyleResolverAppliesRootDescendantSelectors(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			body p { text-align: center; line-height: 110%; }
			html p { text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.Align != textAlignCenter {
		t.Fatalf("paragraph align = %v, want center from body p", paragraph.Paragraph.Align)
	}
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("paragraph indent = %v, want 0 from html p", paragraph.Paragraph.FirstLineIndent)
	}
	if paragraph.Paragraph.LineHeight != pdfBaseFontSize*1.1 {
		t.Fatalf("paragraph line height = %v, want %v from body p", paragraph.Paragraph.LineHeight, pdfBaseFontSize*1.1)
	}
}

func TestPDFStyleResolverAppliesRootDescendantSelectorsToHeadingDepthAndImages(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			body h1 { margin-left: 13pt; }
			html h3 { margin-right: 7pt; }
			body img { margin-left: 11pt; }
			body img.image-vignette { margin-right: 5pt; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	h1 := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	if h1.MarginLeft != 13 {
		t.Fatalf("h1 margin-left = %v, want 13 from body h1", h1.MarginLeft)
	}
	if h1.MarginRight != 0 {
		t.Fatalf("h1 margin-right = %v, want 0 (no html h3 match)", h1.MarginRight)
	}

	h3 := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 3})
	if h3.MarginRight != 7 {
		t.Fatalf("h3 margin-right = %v, want 7 from html h3", h3.MarginRight)
	}
	if h3.MarginLeft != 0 {
		t.Fatalf("h3 margin-left = %v, want 0 (no body h1 match)", h3.MarginLeft)
	}

	image := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage})
	if image.MarginLeft != 11 {
		t.Fatalf("image margin-left = %v, want 11 from body img", image.MarginLeft)
	}

	vignette := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage, StyleClasses: "image-vignette vignette"})
	if vignette.MarginLeft != 11 {
		t.Fatalf("vignette margin-left = %v, want 11 from body img", vignette.MarginLeft)
	}
	if vignette.MarginRight != 5 {
		t.Fatalf("vignette margin-right = %v, want 5 from body img.image-vignette", vignette.MarginRight)
	}
}

func TestPDFStyleResolverAppliesContainerDescendantSelectors(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.epigraph p { text-align: right; text-indent: 0; margin-left: 11pt; }
			.annotation img { margin-left: 9pt; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	epigraphParagraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: pdfStyleEpigraph})
	if epigraphParagraph.Paragraph.Align != textAlignRight {
		t.Fatalf("epigraph paragraph align = %v, want right from .epigraph p", epigraphParagraph.Paragraph.Align)
	}
	if epigraphParagraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("epigraph paragraph indent = %v, want 0 from .epigraph p", epigraphParagraph.Paragraph.FirstLineIndent)
	}
	if epigraphParagraph.MarginLeft != 11 {
		t.Fatalf("epigraph paragraph margin-left = %v, want 11 from .epigraph p", epigraphParagraph.MarginLeft)
	}

	annotationImage := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage, StyleClasses: "image-block", ContextClasses: pdfStyleAnnotation})
	if annotationImage.MarginLeft != 9 {
		t.Fatalf("annotation image margin-left = %v, want 9 from .annotation img", annotationImage.MarginLeft)
	}
}

func TestPDFStyleResolverAppliesNestedContainerDescendantSelectorsInOrder(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.annotation p { text-align: center; text-indent: 0; }
			.cite p { text-align: left; margin-left: 9pt; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: joinStyleClasses(pdfStyleAnnotation, pdfStyleCite)})
	if paragraph.Paragraph.Align != textAlignLeft {
		t.Fatalf("nested paragraph align = %v, want inner cite left", paragraph.Paragraph.Align)
	}
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("nested paragraph indent = %v, want outer annotation indent reset", paragraph.Paragraph.FirstLineIndent)
	}
	if paragraph.MarginLeft != 9 {
		t.Fatalf("nested paragraph margin-left = %v, want 9 from inner cite", paragraph.MarginLeft)
	}
}

func TestPDFStyleResolverAppliesContainerInheritedPropertiesBeforeTagDefaults(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { text-indent: 1em; text-align: justify; }
			.footnote { text-indent: 0; text-align: center; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: pdfStyleFootnote})
	if paragraph.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("footnote paragraph indent = %v, want 0 from container inheritance", paragraph.Paragraph.FirstLineIndent)
	}
	if paragraph.Paragraph.Align != textAlignCenter {
		t.Fatalf("footnote paragraph align = %v, want center from container inheritance", paragraph.Paragraph.Align)
	}
}

func TestPDFStyleResolverAppliesContainerInheritedMarginsAndKeepsExplicitTextAuthorStyle(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.cite { margin-left: 9pt; margin-right: 7pt; font-style: italic; text-align: left; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	textAuthor := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockTextAuthor, ContextClasses: pdfStyleCite})
	if textAuthor.MarginLeft != 9 || textAuthor.MarginRight != 7 {
		t.Fatalf("cite text-author margins = %v/%v, want 9/7 from container inheritance", textAuthor.MarginLeft, textAuthor.MarginRight)
	}
	if !textAuthor.Paragraph.Italic {
		t.Fatalf("cite text-author italic = false, want true from container inheritance")
	}
	if textAuthor.Paragraph.Align != textAlignRight {
		t.Fatalf("cite text-author align = %v, want explicit text-author right preserved", textAuthor.Paragraph.Align)
	}
}

func TestPDFStyleResolverDescendantSelectorOverridesContainerInheritedProperty(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { text-indent: 1em; }
			.footnote { text-indent: 0; }
			.footnote p { text-indent: 0.5em; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: pdfStyleFootnote})
	if paragraph.Paragraph.FirstLineIndent != pdfBaseFontSize*0.5 {
		t.Fatalf("footnote paragraph indent = %v, want %v from descendant selector", paragraph.Paragraph.FirstLineIndent, pdfBaseFontSize*0.5)
	}
}

func TestPDFStyleResolverAccumulatesInnermostContainerMarginWithExplicitVerseMargin(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p { margin-left: -8pt; }
			.poem { margin-left: 3em; font-style: italic; }
			.verse { margin-left: 2em; text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, ContextClasses: pdfStylePoem, StyleClasses: pdfStylePoem})
	if verse.MarginLeft != pdfBaseFontSize*5 {
		t.Fatalf("poem verse margin-left = %v, want accumulated %v", verse.MarginLeft, pdfBaseFontSize*5)
	}
	if !verse.Paragraph.Italic {
		t.Fatalf("poem verse italic = false, want inherited true")
	}
}

func TestPDFStyleResolverDoesNotDoubleCountDuplicateContextClassMargins(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.cite { margin-left: 2em; margin-right: 1em; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleCite, ContextClasses: pdfStyleCite})
	if paragraph.MarginLeft != pdfBaseFontSize*2 || paragraph.MarginRight != pdfBaseFontSize {
		t.Fatalf("cite paragraph margins = %v/%v, want 2em/1em without double count", paragraph.MarginLeft, paragraph.MarginRight)
	}
}

func TestPDFStyleResolverAccumulatesNestedContainerMarginsAcrossContextChain(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.annotation { margin-left: 1em; margin-right: 0.5em; }
			.cite { margin-left: 2em; margin-right: 0.25em; font-style: italic; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleCite, ContextClasses: joinStyleClasses(pdfStyleAnnotation, pdfStyleCite)})
	if paragraph.MarginLeft != pdfBaseFontSize*3 {
		t.Fatalf("nested cite margin-left = %v, want accumulated %v", paragraph.MarginLeft, pdfBaseFontSize*3)
	}
	if paragraph.MarginRight != pdfBaseFontSize*0.75 {
		t.Fatalf("nested cite margin-right = %v, want accumulated %v", paragraph.MarginRight, pdfBaseFontSize*0.75)
	}
	if !paragraph.Paragraph.Italic {
		t.Fatalf("nested cite italic = false, want inherited true")
	}
}

func TestPDFStyleResolverAppliesNestedStanzaContextInheritance(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.poem { margin-left: 3em; font-style: italic; }
			.stanza { margin-left: 1em; font-family: "Noto Sans", sans-serif; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem, StyleClasses: pdfStylePoem, ContextClasses: joinStyleClasses(pdfStylePoem, pdfStyleStanza)})
	if verse.MarginLeft != pdfBaseFontSize*4 {
		t.Fatalf("stanza verse margin-left = %v, want accumulated %v", verse.MarginLeft, pdfBaseFontSize*4)
	}
	if verse.Paragraph.FontFamily != "Noto Sans" {
		t.Fatalf("stanza verse font family = %q, want Noto Sans from stanza context", verse.Paragraph.FontFamily)
	}
	if !verse.Paragraph.Italic {
		t.Fatalf("stanza verse italic = false, want inherited true from poem context")
	}
}

func TestPDFStyleResolverAppliesElementQualifiedImageSelectors(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			img.image-block { margin-left: 9pt; }
			img.image-vignette { margin-right: 4pt; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	image := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage, StyleClasses: "image-block"})
	if image.MarginLeft != 9 {
		t.Fatalf("image-block margin-left = %v, want 9", image.MarginLeft)
	}
	if image.MarginRight != 0 {
		t.Fatalf("image-block margin-right = %v, want 0", image.MarginRight)
	}

	vignette := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockImage, StyleClasses: "image-vignette vignette"})
	if vignette.MarginRight != 4 {
		t.Fatalf("image-vignette margin-right = %v, want 4", vignette.MarginRight)
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
	if paragraph.Paragraph.LineHeight != pdfBaseLineHeight*2 {
		t.Fatalf("paragraph line height = %v, want implicit scaled %v", paragraph.Paragraph.LineHeight, pdfBaseLineHeight*2)
	}
	if paragraph.Paragraph.LineHeightExplicit {
		t.Fatalf("implicit scaled line height should not be marked explicit")
	}
}

func TestPDFStyleResolverTitleNextVariantClearsHeadingMargins(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}

	style := resolver.styleForBlock(pdfTextBlock{
		Kind:         pdfBlockHeading,
		StyleName:    pdfStyleChapterTitleHeader,
		StyleClasses: pdfStyleChapterTitleHeader + "-next",
	})

	if style.SpaceBefore != 0 || style.SpaceAfter != 0 {
		t.Fatalf("title-next margins = %v/%v, want 0/0", style.SpaceBefore, style.SpaceAfter)
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
	title := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnoteTitle + " " + pdfStyleFootnoteTitle + "-first"})
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

func TestPDFStyleResolverTitleAfterImageKeepsHeadingTextAlignment(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}

	style := resolver.styleForBlock(pdfTextBlock{
		Kind:         pdfBlockHeading,
		StyleName:    pdfStyleChapterTitleHeader,
		StyleClasses: joinStyleClasses(pdfStyleChapterTitleHeader+"-next", pdfStyleTitleAfterImage),
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

func TestPDFCollapsedBlockStylesApplyContainerVerticalMarginsOnlyAtEdges(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 1}
	resolver.styles[pdfStyleAnnotation] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 20, SpaceAfter: 10, MarginLeft: 5, MarginRight: 7}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, Text: "one", StyleClasses: pdfStyleAnnotation},
		{Kind: pdfBlockParagraph, Text: "two", StyleClasses: pdfStyleAnnotation},
		{Kind: pdfBlockParagraph, Text: "outside"},
	})
	if styles[0].SpaceBefore != 20 || styles[0].SpaceAfter != 0 {
		t.Fatalf("first annotation margins = %v/%v, want 20/0 after collapse", styles[0].SpaceBefore, styles[0].SpaceAfter)
	}
	if styles[1].SpaceBefore != 1 || styles[1].SpaceAfter != 0 {
		t.Fatalf("last annotation margins = %v/%v, want collapsed base paragraph gap/top and zero after collapse", styles[1].SpaceBefore, styles[1].SpaceAfter)
	}
	if styles[2].SpaceBefore != 10 {
		t.Fatalf("following block margin-top = %v, want collapsed annotation bottom", styles[2].SpaceBefore)
	}
	if styles[0].MarginLeft != 5 || styles[1].MarginRight != 7 {
		t.Fatalf("container horizontal margins were not preserved: %#v %#v", styles[0], styles[1])
	}
}

func TestPDFCollapsedBlockStylesKeepContainerThroughEmptyLine(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}}
	resolver.styles[pdfStyleEmptyLine] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 10, SpaceAfter: 10}
	resolver.styles[pdfStyleAnnotation] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 20, SpaceAfter: 10, MarginLeft: 5}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, Text: "one", StyleClasses: pdfStyleAnnotation},
		{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: pdfStyleAnnotation},
		{Kind: pdfBlockParagraph, Text: "two", StyleClasses: pdfStyleAnnotation},
	})
	if !styles[1].Hidden {
		t.Fatalf("empty line hidden = false, want hidden")
	}
	if styles[0].SpaceBefore != 20 || styles[0].SpaceAfter != 0 || styles[2].SpaceBefore != 6 || styles[2].SpaceAfter != 10 {
		t.Fatalf("container empty-line margins = first %v/%v second %v/%v, want 20/0 6/10", styles[0].SpaceBefore, styles[0].SpaceAfter, styles[2].SpaceBefore, styles[2].SpaceAfter)
	}
}

func TestPDFCollapsedBlockStylesCollapseAdjacentMargins(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 6}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
		{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
	})
	if styles[0].SpaceAfter != 0 {
		t.Fatalf("previous margin-bottom = %v, want 0", styles[0].SpaceAfter)
	}
	if styles[1].SpaceBefore != 6 {
		t.Fatalf("current margin-top = %v, want collapsed max 6", styles[1].SpaceBefore)
	}
}

func TestPDFCollapsedBlockStylesHandlesNegativeMargins(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["positive"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 6}
	resolver.styles["negative"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: -2}
	resolver.styles["more-negative"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: -3}
	resolver.styles["least-negative"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: -5}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "positive", Text: "positive"},
		{Kind: pdfBlockParagraph, StyleName: "negative", Text: "negative"},
		{Kind: pdfBlockParagraph, StyleName: "more-negative", Text: "more-negative"},
		{Kind: pdfBlockParagraph, StyleName: "least-negative", Text: "least-negative"},
	})
	if styles[1].SpaceBefore != 4 {
		t.Fatalf("mixed-sign collapsed margin = %v, want 4", styles[1].SpaceBefore)
	}
	if styles[3].SpaceBefore != -5 {
		t.Fatalf("negative collapsed margin = %v, want -5", styles[3].SpaceBefore)
	}
}

func TestPDFCollapsedBlockStylesTreatsPageBreakAsBarrier(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 6}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
		{Kind: pdfBlockPageBreak},
		{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
	})
	if styles[0].SpaceAfter != 4 || styles[2].SpaceBefore != 6 {
		t.Fatalf("page break collapsed margins unexpectedly: before mb=%v after mt=%v", styles[0].SpaceAfter, styles[2].SpaceBefore)
	}
}

func TestPDFCollapsedBlockStylesAppliesEmptyLineMarginToNextText(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["empty"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 10}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
		{Kind: pdfBlockEmptyLine, StyleName: "empty"},
		{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
	})
	if !styles[1].Hidden {
		t.Fatalf("empty line style hidden = false, want skipped layout block")
	}
	if styles[0].SpaceAfter != 0 || styles[2].SpaceBefore != 6 {
		t.Fatalf("empty line margins: before mb=%v after mt=%v, want 0/6", styles[0].SpaceAfter, styles[2].SpaceBefore)
	}
}

func TestPDFCollapsedBlockStylesAppliesEmptyLineBeforeImageToPreviousBlock(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["empty"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 10}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
		{Kind: pdfBlockEmptyLine, StyleName: "empty"},
		{Kind: pdfBlockImage, ImageID: "image"},
	})
	if styles[0].SpaceAfter != 0 || styles[2].SpaceBefore != 6 {
		t.Fatalf("empty line before image collapsed margins: before mb=%v image mt=%v, want 0/6", styles[0].SpaceAfter, styles[2].SpaceBefore)
	}
}

func TestPDFCollapsedBlockStylesSkipsHiddenBlocks(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["hidden"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 100, SpaceAfter: 100, Hidden: true}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 6}

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
		{Kind: pdfBlockParagraph, StyleName: "hidden", Text: "hidden"},
		{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
	})
	if styles[0].SpaceAfter != 0 || styles[2].SpaceBefore != 6 {
		t.Fatalf("hidden block margins affected collapse: before mb=%v after mt=%v", styles[0].SpaceAfter, styles[2].SpaceBefore)
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
	if tocEntry.Paragraph.FirstLineIndent != 24 {
		t.Fatalf("toc indent = %v, want 24", tocEntry.Paragraph.FirstLineIndent)
	}
	if tocEntry.SpaceAfter != 2 {
		t.Fatalf("toc margin-bottom = %v, want 2", tocEntry.SpaceAfter)
	}

	subtitle := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockSubtitle})
	if subtitle.KeepWithNextLines != 1 {
		t.Fatalf("subtitle keep-with-next = %d, want 1", subtitle.KeepWithNextLines)
	}
}

func TestPDFStyleResolverUsesContextSpecificSubtitleDefaults(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}

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
}

func TestPDFStyleResolverPageBreakDisplayAndAbsoluteUnits(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.forced { page-break-before: always; break-after: page; display: none; }
			.metric { margin-left: 25.4mm; margin-right: 2.54cm; margin-top: 1in; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	forced := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "forced"})
	if !forced.PageBreakBefore || !forced.PageBreakAfter || !forced.Hidden {
		t.Fatalf("forced style page/visibility flags = before:%t after:%t hidden:%t, want all true", forced.PageBreakBefore, forced.PageBreakAfter, forced.Hidden)
	}

	metric := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "metric"})
	if metric.MarginLeft != 72 || metric.MarginRight != 72 || metric.SpaceBefore != 72 {
		t.Fatalf("metric margins = left:%v right:%v top:%v, want all 72pt", metric.MarginLeft, metric.MarginRight, metric.SpaceBefore)
	}
}
