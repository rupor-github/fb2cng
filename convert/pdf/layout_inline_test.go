package pdf

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestLayoutPDFPagesAppliesInlineStyles(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, used, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "plain bold strike sub sup code",
			Runs: []pdfInlineRun{
				{Text: "plain "},
				{Text: "bold", Bold: true, Italic: true},
				{Text: " "},
				{Text: "strike", Strikethrough: true},
				{Text: " "},
				{Text: "sub", Subscript: true},
				{Text: " "},
				{Text: "sup", Superscript: true},
				{Text: " "},
				{Text: "code", Code: true},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one styled body line", pages)
	}
	line := pages[1].Lines[0]
	if got := pdfPageLineText(line); got != "plain bold strike sub sup code" {
		t.Fatalf("line text = %q", got)
	}
	var sawBoldItalic, sawStrike, sawSub, sawSup, sawCode bool
	for _, fragment := range line.Fragments {
		switch shapedRunes(fragment.Text) {
		case "bold":
			sawBoldItalic = fragment.FontKey.Bold && fragment.FontKey.Italic
		case "strike":
			sawStrike = fragment.Strikethrough
		case "sub":
			sawSub = fragment.BaselineShift < 0 && fragment.FontSize < line.FontSize
		case "sup":
			sawSup = fragment.BaselineShift > 0 && fragment.FontSize < line.FontSize
		case "code":
			sawCode = fragment.FontKey.Family == "monospace" && fragment.FontSize < line.FontSize
		}
	}
	if !sawBoldItalic || !sawStrike || !sawSub || !sawSup || !sawCode {
		t.Fatalf("inline fragments = %#v", line.Fragments)
	}
	for _, key := range []pdfFontKey{{Family: "serif"}, {Family: "serif", Bold: true, Italic: true}, {Family: "monospace"}} {
		if len(used[key]) == 0 {
			t.Fatalf("used glyphs for font key %#v missing in %#v", key, used)
		}
	}
}

func TestLayoutPDFPagesPreservesCodeBlockWhitespace(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  260,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     newPDFStyleResolverWithDefaultCSS(t),
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         "alpha beta",
			StyleClasses: pdfStyleCode,
			Runs: []pdfInlineRun{{
				Text:         "\n  alpha\n    beta\n  ",
				StyleClasses: pdfStyleCode,
				Code:         true,
			}},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 4 {
		t.Fatalf("layoutPDFPages() pages = %#v, want leading blank, two code lines, and trailing blank", pages)
	}
	if got := pdfPageLineText(pages[1].Lines[0]); got != "" {
		t.Fatalf("leading code line = %q, want preserved blank line", got)
	}
	if got := pdfPageLineText(pages[1].Lines[1]); got != "  alpha" {
		t.Fatalf("first code line = %q, want preserved indentation", got)
	}
	if got := pdfPageLineText(pages[1].Lines[2]); got != "    beta" {
		t.Fatalf("second code line = %q, want preserved indentation", got)
	}
	if got := pdfPageLineText(pages[1].Lines[3]); got != "" {
		t.Fatalf("trailing code line = %q, want preserved blank line", got)
	}
	for _, line := range pages[1].Lines[1:3] {
		if line.X != 24 || line.Fragments[0].FontKey.Family != "monospace" || line.Fragments[0].FontSize >= pdfBaseFontSize {
			t.Fatalf("code line = %#v, want left-aligned smaller monospace", line)
		}
	}
}

func TestLayoutPDFPagesKeepsBaseRhythmAfterCodeBlock(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  320,
		PageHeight: 240,
		Title:      "Title",
		Author:     "Author",
		Styles:     newPDFStyleResolverWithDefaultCSS(t),
		Blocks: []pdfTextBlock{
			{
				Kind:         pdfBlockParagraph,
				Text:         "alpha beta",
				StyleClasses: pdfStyleCode,
				Runs: []pdfInlineRun{{
					Text:         "alpha\nbeta",
					StyleClasses: pdfStyleCode,
					Code:         true,
				}},
			},
			{Kind: pdfBlockParagraph, Text: "normal paragraph"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 3 {
		t.Fatalf("layoutPDFPages() pages = %#v, want two code lines plus following paragraph", pages)
	}
	gap := pages[1].Lines[1].Y - pages[1].Lines[2].Y
	wantGap := pdfBaseLineHeight
	if math.Abs(gap-wantGap) > 0.001 {
		t.Fatalf("gap after code block = %v, want KP3 base rhythm %v", gap, wantGap)
	}
}

func TestLayoutPDFPagesRendersInlineImages(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 120
	img.Dim.Height = 60

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      520,
		PageHeight:     180,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Images:         fb2.BookImages{"inline": img},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "before after",
			Runs: []pdfInlineRun{
				{Text: "before "},
				{ImageID: "inline", StyleClasses: pdfStyleLinkInternal, LinkHref: "#target"},
				{Text: " after"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one inline image on body page", pages)
	}
	image := pages[1].Images[0]
	if image.ImageID != "inline" || image.Width <= 0 || image.Height <= 0 {
		t.Fatalf("inline image = %#v, want placed image", image)
	}
	if len(pages[1].Lines) != 1 {
		t.Fatalf("lines = %#v, want one body line", pages[1].Lines)
	}
	line := pages[1].Lines[0]
	var imageFragment *pdfPageLineFragment
	for i := range line.Fragments {
		fragment := &line.Fragments[i]
		if fragment.ImageID == "inline" {
			imageFragment = fragment
			break
		}
	}
	if imageFragment == nil {
		t.Fatalf("line fragments = %#v, want inline image fragment", line.Fragments)
	}
	if !imageFragment.Underline {
		t.Fatalf("inline image fragment = %#v, want link underline decoration", *imageFragment)
	}
	if math.Abs(image.Height-pdfBaseLineHeight) > 0.001 || math.Abs(imageFragment.ImageHeight-pdfBaseLineHeight) > 0.001 {
		t.Fatalf("inline image height = %v / fragment %v, want current line height %v", image.Height, imageFragment.ImageHeight, pdfBaseLineHeight)
	}
	if math.Abs(image.Y-(line.Y+imageFragment.BaselineShift)) > 0.001 || image.Y >= line.Y || image.Y+image.Height <= line.Y {
		t.Fatalf("inline image Y = %v, line Y = %v, fragment baseline shift = %v", image.Y, line.Y, imageFragment.BaselineShift)
	}
	if len(pages[1].Annotations) != 1 {
		t.Fatalf("annotations = %#v, want inline image link annotation", pages[1].Annotations)
	}
	annotation := pages[1].Annotations[0]
	if annotation.Href != "#target" || math.Abs(annotation.Rect.X1-image.X) > 0.001 || math.Abs(annotation.Rect.Y1-image.Y) > 0.001 || math.Abs(annotation.Rect.X2-(image.X+image.Width)) > 0.001 || math.Abs(annotation.Rect.Y2-(image.Y+image.Height)) > 0.001 {
		t.Fatalf("annotation = %#v, image = %#v, want image rectangle link", annotation, image)
	}
}

func TestLayoutPDFPagesAppliesInlineNamedStyleClasses(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	accent := resolver.styles[pdfStyleParagraph]
	accent.Paragraph.FontFamily = "sans-serif"
	accent.Paragraph.Bold = true
	accent.Paragraph.Italic = true
	accent.Paragraph.Color = pdfColor{R: 1}
	accent.Paragraph.Underline = true
	accent.Paragraph.VerticalAlign = textVerticalAlignSuper
	resolver.styles["accent"] = accent

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "plain styled",
			Runs: []pdfInlineRun{
				{Text: "plain "},
				{Text: "styled", StyleClasses: "accent"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one styled body line", pages)
	}
	var styled *pdfPageLineFragment
	for i := range pages[1].Lines[0].Fragments {
		fragment := &pages[1].Lines[0].Fragments[i]
		if shapedRunes(fragment.Text) == "styled" {
			styled = fragment
			break
		}
	}
	if styled == nil {
		t.Fatalf("styled fragment missing: %#v", pages[1].Lines[0].Fragments)
	}
	if styled.FontKey.Family != "sans-serif" || !styled.FontKey.Bold || !styled.FontKey.Italic || !styled.Underline || styled.Color.String() != "#ff0000" || styled.BaselineShift <= 0 || styled.FontSize >= pages[1].Lines[0].FontSize {
		t.Fatalf("styled fragment = %#v, want accent class styling", *styled)
	}
}

func TestLayoutPDFPagesAppliesInlineContextDescendantSelectors(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.footnote .accent { color: #ff0000; font-weight: bold; }`,
	}}}
	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:           pdfBlockParagraph,
			Text:           "plain styled",
			ContextClasses: pdfStyleFootnote,
			Runs: []pdfInlineRun{
				{Text: "plain "},
				{Text: "styled", StyleClasses: "accent"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one styled body line", pages)
	}
	var styled *pdfPageLineFragment
	for i := range pages[1].Lines[0].Fragments {
		fragment := &pages[1].Lines[0].Fragments[i]
		if shapedRunes(fragment.Text) == "styled" {
			styled = fragment
			break
		}
	}
	if styled == nil {
		t.Fatalf("styled fragment missing: %#v", pages[1].Lines[0].Fragments)
	}
	if styled.Color.String() != "#ff0000" || !styled.FontKey.Bold {
		t.Fatalf("styled fragment = %#v, want footnote descendant styling", *styled)
	}
}

func TestLayoutPDFPagesAppliesInlineDescendantSelectorsFromBlockStyleName(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `.toc-item .link-toc { color: #ff0000; font-weight: bold; }`,
	}}}
	resolver := newPDFStyleResolver(book, zaptest.NewLogger(t))

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:      pdfBlockTOCEntry,
			Text:      "Chapter",
			StyleName: pdfStyleTOCItem,
			Runs:      []pdfInlineRun{{Text: "Chapter", StyleClasses: pdfStyleLinkTOC}},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 || len(pages[1].Lines[0].Fragments) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one TOC entry fragment", pages)
	}
	fragment := pages[1].Lines[0].Fragments[0]
	if fragment.Color.String() != "#ff0000" || !fragment.FontKey.Bold {
		t.Fatalf("TOC link fragment = %#v, want descendant styling from block style name", fragment)
	}
}
