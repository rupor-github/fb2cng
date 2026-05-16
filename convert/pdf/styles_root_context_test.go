package pdf

import (
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

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

func TestPDFStyleResolverDefaultIndentsOverrideRootInheritedIndent(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `body { text-indent: 2em; }`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.FirstLineIndent != pdfBodyIndent {
		t.Fatalf("paragraph indent = %v, want default paragraph indent %v", paragraph.Paragraph.FirstLineIndent, pdfBodyIndent)
	}
	for _, block := range []pdfTextBlock{
		{Kind: pdfBlockHeading, Depth: 1},
		{Kind: pdfBlockSubtitle},
		{Kind: pdfBlockPoem},
		{Kind: pdfBlockTextAuthor},
		{Kind: pdfBlockImage},
	} {
		style := resolver.styleForBlock(block)
		if style.Paragraph.FirstLineIndent != 0 {
			t.Fatalf("%s indent = %v, want explicit default zero indent", block.Kind, style.Paragraph.FirstLineIndent)
		}
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

func TestPDFStyleResolverDoesNotLetClassOnlyHTMLTagNamesOverrideElementDefaults(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			.p { text-align: right; }
			p.warning { text-indent: 0; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.Align != textAlignJustify {
		t.Fatalf("paragraph align = %v, want default justify; .p must not override p", paragraph.Paragraph.Align)
	}
	warning := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "warning"})
	if warning.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("warning indent = %v, want 0 from element-qualified p.warning", warning.Paragraph.FirstLineIndent)
	}
}

func TestPDFStyleResolverMapsNonImageElementClassSelectorsLikeKFX(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p.warning { text-indent: 0; text-align: right; }
			div.annotation p { font-style: italic; }
		`,
	}}}

	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))
	warning := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: "warning"})
	if warning.Paragraph.FirstLineIndent != 0 {
		t.Fatalf("warning indent = %v, want 0 from p.warning mapped as class", warning.Paragraph.FirstLineIndent)
	}
	if warning.Paragraph.Align != textAlignRight {
		t.Fatalf("warning align = %v, want right from p.warning mapped as class", warning.Paragraph.Align)
	}

	annotation := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, ContextClasses: pdfStyleAnnotation})
	if !annotation.Paragraph.Italic {
		t.Fatalf("annotation italic = false, want true from div.annotation p mapped as .annotation p")
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
