package pdf

import (
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

func TestPDFStyleResolverAppliesStylesheet(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			p {
				font-size: 12pt;
				line-height: 1.5;
				text-align: center;
				text-indent: 2em;
				margin: 1em 2em 0.5em 3em;
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
	if paragraph.Paragraph.FontSize != 12 {
		t.Fatalf("paragraph font size = %v, want 12", paragraph.Paragraph.FontSize)
	}
	if paragraph.Paragraph.LineHeight != 18 {
		t.Fatalf("paragraph line height = %v, want 18", paragraph.Paragraph.LineHeight)
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
	if paragraph.Orphans != 3 || paragraph.Widows != 4 {
		t.Fatalf("paragraph orphans/widows = %d/%d, want 3/4", paragraph.Orphans, paragraph.Widows)
	}

	verse := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockPoem})
	if verse.MarginLeft != 21 {
		t.Fatalf("verse margin-left = %v, want 21", verse.MarginLeft)
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
