package pdf

func defaultPDFStyles() map[string]pdfBlockResolvedStyle {
	styles := map[string]pdfBlockResolvedStyle{
		pdfStyleHTML:      basePDFStyle(textAlignLeft),
		pdfStyleBody:      basePDFStyle(textAlignLeft),
		pdfStylePage:      basePDFStyle(textAlignLeft),
		pdfStyleParagraph: intrinsicParagraphPDFStyle(),
		// Real FB2 section headings are emitted with these internal style names.
		// They model KP3/HTML heading fallbacks; FB2/default.css title-header
		// class rules are supplied by the active stylesheet when present.
		pdfStyleBodyTitleHeader:    intrinsicHeadingPDFStyle(1),
		pdfStyleChapterTitleHeader: intrinsicHeadingPDFStyle(1),
		pdfStyleSectionTitleHeader: intrinsicSectionTitleHeaderPDFStyle(),
		pdfStyleImage: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
			KeepTogether: true,
		},
		// FB2 inline/full-code handling preserves raw code whitespace as content
		// behavior; default.css may still supply the smaller font size.
		pdfStyleCode: {
			Paragraph: paragraphStyle{FontFamily: "monospace", FontSize: pdfBaseFontSize, LineHeight: pdfCodeLineHeight, LineHeightExplicit: true, Align: textAlignLeft, PreserveSpace: true, Hyphenation: paragraphHyphenationNone},
			Orphans:   pdfSingleKeepLine,
			Widows:    pdfSingleKeepLine,
		},
		// Flattened PDF tables need native keep-together/zero-indent safety even
		// when stylesheet table decoration is absent.
		pdfStyleTable: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore:  pdfBaseFontSize,
			SpaceAfter:   pdfBaseFontSize,
			KeepTogether: true,
		},
		pdfStyleTableCell: {
			Paragraph:     paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			PaddingTop:    pdfTableCellPadding,
			PaddingRight:  pdfTableCellPadding,
			PaddingBottom: pdfTableCellPadding,
			PaddingLeft:   pdfTableCellPadding,
			BorderWidth:   pdfTableCellBorderWidth,
			BorderColor:   pdfColor{R: 0, G: 0, B: 0},
			HasBorder:     true,
		},
		pdfStyleTableHeaderCell: {
			Paragraph:     paragraphStyle{FontFamily: "serif", Bold: true, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
			PaddingTop:    pdfTableCellPadding,
			PaddingRight:  pdfTableCellPadding,
			PaddingBottom: pdfTableCellPadding,
			PaddingLeft:   pdfTableCellPadding,
			BorderWidth:   pdfTableCellBorderWidth,
			BorderColor:   pdfColor{R: 0, G: 0, B: 0},
			HasBorder:     true,
		},
	}

	titleAfterImageStyle := styles[pdfStyleParagraph]
	titleAfterImageStyle.SpaceBefore = pdfTitleAfterImageSpaceBefore
	titleAfterImageStyle.HasSpaceBefore = true
	titleAfterImageStyle.SpaceAfter = 0
	titleAfterImageStyle.HasSpaceAfter = true
	styles[pdfStyleTitleAfterImage] = titleAfterImageStyle

	return styles
}

func basePDFStyle(align textAlign) pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: align, Hyphenation: paragraphHyphenationAuto},
	}
}

func intrinsicParagraphPDFStyle() pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore: pdfBaseFontSize,
		SpaceAfter:  pdfBaseFontSize,
		Orphans:     pdfDefaultKeepLines,
		Widows:      pdfDefaultKeepLines,
	}
}

func intrinsicHeadingPDFStyle(depth int) pdfBlockResolvedStyle {
	fontSize := pdfIntrinsicHeadingFontSize(depth)
	return headingPDFStyleWithLineHeight(fontSize, pdfAdjustedLineHeight, pdfHeadingMarginFactor(depth))
}

func intrinsicSectionTitleHeaderPDFStyle() pdfBlockResolvedStyle {
	fontSize := pdfIntrinsicHeadingFontSize(2)
	return headingPDFStyleWithLineHeight(fontSize, pdfSectionTitleHeaderLineHeight, pdfHeadingMarginFactor(2))
}

func headingPDFStyle(depth int) pdfBlockResolvedStyle {
	lineHeight := pdfAdjustedLineHeight
	if depth > 1 {
		lineHeight = pdfSectionTitleHeaderLineHeight
	}
	return headingPDFStyleWithLineHeight(pdfHeadingFontSize(depth), lineHeight, pdfHeadingMarginFactor(depth))
}

func pdfIntrinsicHeadingFontSize(depth int) float64 {
	switch {
	case depth <= 2:
		return pdfBaseFontSize * 1.5
	case depth == 3:
		return pdfBaseFontSize * 1.17
	case depth == 4:
		return pdfBaseFontSize
	case depth == 5:
		return pdfBaseFontSize * 0.83
	default:
		return pdfBaseFontSize * 0.67
	}
}

func pdfHeadingFontSize(depth int) float64 {
	if depth <= 1 {
		return pdfHeadingH1FontSize
	}
	return pdfHeadingNestedFontSize
}

func headingPDFStyleWithLineHeight(fontSize float64, lineHeight float64, marginFactor float64) pdfBlockResolvedStyle {
	space := fontSize * marginFactor
	return pdfBlockResolvedStyle{
		Paragraph:         paragraphStyle{FontFamily: "serif", Bold: true, FontSize: fontSize, LineHeight: lineHeight, LineHeightExplicit: true, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore:       space,
		SpaceAfter:        space,
		KeepTogether:      true,
		KeepWithNextLines: pdfDefaultKeepLines,
	}
}

func pdfHeadingMarginFactor(depth int) float64 {
	if depth <= 1 {
		return pdfHeadingH1MarginFactor
	}
	return pdfHeadingNestedMarginFactor
}

func (r *pdfStyleResolver) applyPDFStyleAdjustments() {
	if r == nil {
		return
	}
	r.applyPDFVariantInheritance()
	r.applyPDFHeadingMarginAdjustments()
	for _, name := range []string{
		pdfStylePoemTitle + "-first",
		pdfStylePoemTitle + "-next",
		pdfStyleStanzaTitle + "-first",
		pdfStyleStanzaTitle + "-next",
		pdfStyleFootnoteTitle + "-first",
		pdfStyleFootnoteTitle + "-next",
	} {
		style, ok := r.styles[name]
		if !ok {
			continue
		}
		paragraph := r.namedStyle(pdfStyleParagraph)
		style.SpaceBefore = paragraph.SpaceBefore
		style.SpaceBeforeSpec = pdfCSSLengthSpec{}
		style.HasSpaceBefore = false
		style.SpaceAfter = paragraph.SpaceAfter
		style.SpaceAfterSpec = pdfCSSLengthSpec{}
		style.HasSpaceAfter = false
		r.styles[name] = style
	}
	if style, ok := r.styles[pdfStyleCode]; ok {
		style.Paragraph.Align = r.namedStyle(pdfStyleParagraph).Paragraph.Align
		style.Paragraph.HasAlign = false
		r.styles[pdfStyleCode] = style
	}
	if style, ok := r.styles[pdfStyleTitleAfterImage]; ok {
		style.Paragraph = pdfParagraphClassFallbackStyle(r.namedStyle(pdfStyleParagraph).Paragraph)
		r.styles[pdfStyleTitleAfterImage] = style
	}
	paragraphAlign := r.namedStyle(pdfStyleParagraph).Paragraph.Align
	for _, name := range []string{
		pdfStyleFootnoteTitle,
		pdfStyleFootnoteTitle + "-first",
		pdfStyleFootnoteTitle + "-next",
	} {
		style, ok := r.styles[name]
		if !ok {
			continue
		}
		style.Paragraph.Align = paragraphAlign
		style.Paragraph.HasAlign = false
		r.styles[name] = style
	}
}

func pdfParagraphClassFallbackStyle(paragraph paragraphStyle) paragraphStyle {
	paragraph.HasBold = false
	paragraph.HasItalic = false
	paragraph.LineHeightExplicit = false
	paragraph.HasFirstLineIndent = false
	paragraph.HasAlign = false
	paragraph.HasVerticalAlign = false
	paragraph.HasUnderline = false
	paragraph.HasStrikethrough = false
	paragraph.HasPreserveSpace = false
	paragraph.HasNoWrap = false
	paragraph.HasHyphenation = false
	paragraph.Hyphenator = nil
	return paragraph
}

func (r *pdfStyleResolver) applyPDFVariantInheritance() {
	for name, style := range r.styles {
		baseName, ok := pdfVariantBaseStyleName(name)
		if !ok {
			continue
		}
		base, ok := r.styles[baseName]
		if !ok {
			continue
		}
		fallback := r.namedStyle(pdfStyleParagraph)
		style.Paragraph = mergePDFInheritedParagraphStyle(base.Paragraph, style.Paragraph, fallback.Paragraph)
		r.styles[name] = style
	}
}

func pdfVariantBaseStyleName(name string) (string, bool) {
	for _, suffix := range []string{"-first", "-next", "-break"} {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return name[:len(name)-len(suffix)], true
		}
	}
	return "", false
}

func (r *pdfStyleResolver) applyPDFHeadingMarginAdjustments() {
	for _, tt := range []struct {
		name  string
		depth int
	}{
		{name: pdfStyleBodyTitleHeader, depth: 1},
		{name: pdfStyleChapterTitleHeader, depth: 1},
		{name: pdfStyleTOCTitle, depth: 1},
		{name: pdfStyleSectionTitleHeader, depth: 2},
	} {
		style, ok := r.styles[tt.name]
		if !ok || style.Paragraph.FontSize <= 0 {
			continue
		}
		if !style.Paragraph.LineHeightExplicit {
			style.Paragraph.LineHeight = pdfAdjustedLineHeight
			if tt.name == pdfStyleSectionTitleHeader {
				style.Paragraph.LineHeight = pdfSectionTitleHeaderLineHeight
			}
			style.Paragraph.LineHeightExplicit = true
		}
		if !style.HasSpaceBefore && !style.HasSpaceAfter {
			space := style.Paragraph.FontSize * pdfHeadingMarginFactor(tt.depth)
			style.SpaceBefore = space
			style.SpaceAfter = space
		}
		r.styles[tt.name] = style
	}
}

func clonePDFStyles(src map[string]pdfBlockResolvedStyle) map[string]pdfBlockResolvedStyle {
	cloned := make(map[string]pdfBlockResolvedStyle, len(src))
	for name, style := range src {
		cloned[name] = style
	}
	return cloned
}
