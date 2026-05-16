package pdf

func defaultPDFStyles() map[string]pdfBlockResolvedStyle {
	styles := map[string]pdfBlockResolvedStyle{
		pdfStyleHTML: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStyleBody: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStylePage: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStyleParagraph: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, HasFirstLineIndent: true, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfParagraphSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleBodyTitleHeader:    headingPDFStyle(1),
		pdfStyleChapterTitleHeader: headingPDFStyle(1),
		pdfStyleSectionTitleHeader: sectionTitleHeaderPDFStyle(2),
		pdfStyleSubtitle:           subtitlePDFStyle(textAlignCenter, true, false, pdfSubtitleSpaceBefore, pdfSubtitleSpaceAfter, true),
		pdfStyleAnnotationSubtitle: subtitlePDFStyle(textAlignCenter, true, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
		pdfStylePoemSubtitle:       subtitlePDFStyle(textAlignCenter, false, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
		pdfStyleStanzaSubtitle:     subtitlePDFStyle(textAlignCenter, false, false, pdfBaseFontSize*0.25, pdfBaseFontSize*0.25, false),
		pdfStyleEpigraphSubtitle:   subtitlePDFStyle(textAlignRight, false, true, pdfBaseFontSize*0.3, pdfBaseFontSize*0.3, false),
		pdfStyleCiteSubtitle:       subtitlePDFStyle(textAlignLeft, false, false, pdfBaseFontSize*0.5, pdfBaseFontSize*0.5, false),
		pdfStyleAnnotation: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfAnnotationSpaceBefore,
			SpaceAfter:  pdfAnnotationSpaceAfter,
			MarginLeft:  pdfAnnotationHorizontalMargin,
			MarginRight: pdfAnnotationHorizontalMargin,
		},
		pdfStyleEpigraph: {
			Paragraph:   paragraphStyle{FontFamily: "serif", Italic: true, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignRight, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfEpigraphSpaceBefore,
			SpaceAfter:  pdfEpigraphSpaceAfter,
			MarginLeft:  pdfEpigraphMarginLeft,
		},
		pdfStyleCite: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfBaseFontSize,
			SpaceAfter:  pdfBaseFontSize,
			MarginLeft:  pdfBaseFontSize * 2,
			MarginRight: pdfBaseFontSize * 2,
		},
		pdfStyleVerse: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfVerseLineHeight, HasFirstLineIndent: true, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfVerseSpaceBefore,
			SpaceAfter:  pdfVerseSpaceAfter,
			MarginLeft:  pdfVerseMarginLeft,
			Orphans:     pdfDefaultKeepLines,
			Widows:      pdfDefaultKeepLines,
		},
		pdfStyleTextAuthor: {
			Paragraph:  paragraphStyle{FontFamily: "serif", Bold: true, Italic: true, FontSize: pdfTextAuthorFontSize, LineHeight: pdfTextAuthorLineHeight, HasFirstLineIndent: true, Align: textAlignRight, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTextAuthorSpaceAfter,
			Orphans:    pdfDefaultKeepLines,
			Widows:     pdfDefaultKeepLines,
		},
		pdfStyleImage: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
			KeepTogether: true,
		},
		pdfStyleTOCItem: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTOCSpaceAfter,
			Orphans:    pdfSingleKeepLine,
			Widows:     pdfSingleKeepLine,
		},
		pdfStyleCode: {
			Paragraph: paragraphStyle{FontFamily: "monospace", FontSize: pdfCodeFontSize, LineHeight: pdfCodeLineHeight, Align: textAlignLeft, PreserveSpace: true, Hyphenation: paragraphHyphenationNone},
			Orphans:   pdfSingleKeepLine,
			Widows:    pdfSingleKeepLine,
		},
		pdfStyleTOCTitle:        headingPDFStyle(1),
		pdfStyleAnnotationTitle: headingPDFStyle(1),
		pdfStyleEmptyLine: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
		},
	}
	styles[pdfStyleBodyTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleBodyTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	titleAfterImageStyle := styles[pdfStyleParagraph]
	titleAfterImageStyle.SpaceBefore = pdfTitleAfterImageSpaceBefore
	styles[pdfStyleTitleAfterImage] = titleAfterImageStyle

	linkStyle := styles[pdfStyleParagraph]
	linkStyle.Paragraph.Underline = true
	styles[pdfStyleLinkExternal] = linkStyle
	styles[pdfStyleLinkInternal] = linkStyle
	styles[pdfStyleLinkFootnote] = linkStyle
	styles[pdfStyleLinkTOC] = linkStyle
	return styles
}

func headingPDFStyle(depth int) pdfBlockResolvedStyle {
	return headingPDFStyleWithLineHeightFactor(pdfHeadingFontSize(depth), pdfHeadingLineHeightFactor)
}

func sectionTitleHeaderPDFStyle(depth int) pdfBlockResolvedStyle {
	return headingPDFStyleWithLineHeightFactor(pdfHeadingFontSize(depth), pdfSectionTitleHeaderLineHeightFactor)
}

func pdfHeadingFontSize(depth int) float64 {
	if depth <= 1 {
		return pdfHeadingH1FontSize
	}
	return pdfHeadingNestedFontSize
}

func headingPDFStyleWithLineHeightFactor(fontSize float64, lineHeightFactor float64) pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:         paragraphStyle{FontFamily: "serif", Bold: true, FontSize: fontSize, LineHeight: fontSize * lineHeightFactor, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore:       pdfHeadingSpaceBefore,
		SpaceAfter:        pdfHeadingSpaceAfter,
		KeepTogether:      true,
		KeepWithNextLines: pdfDefaultKeepLines,
	}
}

func (r *pdfStyleResolver) applyPDFStyleAdjustments() {
	if r == nil {
		return
	}
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
		style.SpaceBefore = r.defaultStyle(pdfStyleParagraph).SpaceBefore
		style.HasSpaceBefore = false
		style.SpaceAfter = r.defaultStyle(pdfStyleParagraph).SpaceAfter
		style.HasSpaceAfter = false
		r.styles[name] = style
	}
	if style, ok := r.styles[pdfStyleCode]; ok {
		style.Paragraph.Align = r.namedStyle(pdfStyleParagraph).Paragraph.Align
		r.styles[pdfStyleCode] = style
	}
	if style, ok := r.styles[pdfStyleFootnoteTitle]; ok {
		style.Paragraph.Align = r.namedStyle(pdfStyleParagraph).Paragraph.Align
		r.styles[pdfStyleFootnoteTitle] = style
	}
}

func subtitlePDFStyle(align textAlign, bold bool, italic bool, spaceBefore float64, spaceAfter float64, keepWithNext bool) pdfBlockResolvedStyle {
	style := pdfBlockResolvedStyle{
		Paragraph:    paragraphStyle{FontFamily: "serif", Bold: bold, Italic: italic, FontSize: pdfSubtitleFontSize, LineHeight: pdfSubtitleLineHeight, HasFirstLineIndent: true, Align: align, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore:  spaceBefore,
		SpaceAfter:   spaceAfter,
		KeepTogether: true,
	}
	if keepWithNext {
		style.KeepWithNextLines = pdfSingleKeepLine
	}
	return style
}

func titleHeaderFirstVariantPDFStyle(base pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	base.SpaceBefore = pdfTitleFirstSpaceBefore
	base.HasSpaceBefore = true
	base.SpaceAfter = 0
	base.HasSpaceAfter = true
	return base
}

func titleHeaderNextVariantPDFStyle(base pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	base.SpaceBefore = 0
	base.HasSpaceBefore = true
	base.SpaceAfter = 0
	base.HasSpaceAfter = true
	return base
}

func clonePDFStyles(src map[string]pdfBlockResolvedStyle) map[string]pdfBlockResolvedStyle {
	cloned := make(map[string]pdfBlockResolvedStyle, len(src))
	for name, style := range src {
		cloned[name] = style
	}
	return cloned
}
