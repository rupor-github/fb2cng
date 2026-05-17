package pdf

import (
	"testing"
)

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

func TestPDFCollapsedBlockStylesKeepsSectionH2PageBreakOnlyAtWrapperStart(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t)

	styles := resolver.collapsedBlockStyles([]pdfTextBlock{
		{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("image-vignette", "vignette", pdfStyleVignetteSectionTop, pdfStyleSectionTitle, pdfStyleSectionTitleH2), ImageID: "top"},
		{Kind: pdfBlockHeading, StyleName: pdfStyleSectionTitleHeader, StyleClasses: joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2, pdfStyleSectionTitleHeader+"-first"), Text: "Section"},
		{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("image-vignette", "vignette", pdfStyleVignetteSectionBot, pdfStyleSectionTitle, pdfStyleSectionTitleH2), ImageID: "bottom"},
	})

	if !styles[0].PageBreakBefore {
		t.Fatalf("first section-title-h2 child page-break-before = false, want wrapper page break")
	}
	if styles[1].PageBreakBefore || styles[2].PageBreakBefore {
		t.Fatalf("inner section-title-h2 page breaks = %t/%t, want false/false", styles[1].PageBreakBefore, styles[2].PageBreakBefore)
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
