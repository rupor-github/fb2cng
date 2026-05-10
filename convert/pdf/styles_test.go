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

func TestPDFCollapsedBlockStylesTreatsPageBreakAndEmptyLineAsBarriers(t *testing.T) {
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles["before"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceAfter: 4}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 6}

	for _, barrier := range []pdfBlockKind{pdfBlockPageBreak, pdfBlockEmptyLine} {
		styles := resolver.collapsedBlockStyles([]pdfTextBlock{
			{Kind: pdfBlockParagraph, StyleName: "before", Text: "before"},
			{Kind: barrier},
			{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
		})
		if styles[0].SpaceAfter != 4 || styles[2].SpaceBefore != 6 {
			t.Fatalf("barrier %s collapsed margins unexpectedly: before mb=%v after mt=%v", barrier, styles[0].SpaceAfter, styles[2].SpaceBefore)
		}
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
