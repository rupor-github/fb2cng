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
		pdfStyleAnnotationSubtitle: subtitlePDFStyle(textAlignCenter, true, false, pdfAnnotationSubtitleSpace, pdfAnnotationSubtitleSpace, false),
		pdfStylePoemSubtitle:       subtitlePDFStyle(textAlignCenter, false, false, pdfPoemSubtitleSpace, pdfPoemSubtitleSpace, false),
		pdfStyleStanzaSubtitle:     subtitlePDFStyle(textAlignCenter, false, false, pdfStanzaSubtitleSpace, pdfStanzaSubtitleSpace, false),
		pdfStyleEpigraphSubtitle:   subtitlePDFStyle(textAlignRight, false, true, pdfEpigraphSubtitleSpace, pdfEpigraphSubtitleSpace, false),
		pdfStyleCiteSubtitle:       subtitlePDFStyle(textAlignLeft, false, false, pdfCiteSubtitleSpace, pdfCiteSubtitleSpace, false),
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
		pdfStyleFootnote: {
			Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
		},
		pdfStyleCite: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfBaseFontSize,
			SpaceAfter:  pdfBaseFontSize,
			MarginLeft:  pdfBaseFontSize * 2,
			MarginRight: pdfBaseFontSize * 2,
		},
		pdfStylePoem: {
			Paragraph:  paragraphStyle{FontFamily: "serif", Italic: true, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			MarginLeft: pdfPoemMarginLeft,
		},
		pdfStyleStanza: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfStanzaSpace,
			SpaceAfter:  pdfStanzaSpace,
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
		pdfStyleDate: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignRight, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfDateSpace,
			SpaceAfter:  pdfDateSpace,
			Orphans:     pdfSingleKeepLine,
			Widows:      pdfSingleKeepLine,
		},
		pdfStyleImage: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
			KeepTogether: true,
		},
		pdfStyleVignette:           vignettePDFStyle(pdfVignetteSpace, pdfVignetteSpace),
		pdfStyleVignetteBookTop:    vignettePDFStyle(pdfVignetteTitleTopSpaceBefore, pdfVignetteTitleTopSpaceAfter),
		pdfStyleVignetteBookBottom: vignettePDFStyle(pdfVignetteTitleBottomSpaceBefore, pdfVignetteTitleBottomSpaceAfter),
		pdfStyleVignetteChapterTop: vignettePDFStyle(pdfVignetteTitleTopSpaceBefore, pdfVignetteTitleTopSpaceAfter),
		pdfStyleVignetteChapterBot: vignettePDFStyle(pdfVignetteTitleBottomSpaceBefore, pdfVignetteTitleBottomSpaceAfter),
		pdfStyleVignetteChapterEnd: vignettePDFStyle(pdfVignetteChapterEndSpace, pdfVignetteChapterEndSpace),
		pdfStyleVignetteSectionTop: vignettePDFStyle(pdfVignetteSectionTitleTopSpaceBefore, pdfVignetteSectionTitleTopSpaceAfter),
		pdfStyleVignetteSectionBot: vignettePDFStyle(pdfVignetteSectionTitleBottomBefore, pdfVignetteSectionTitleBottomAfter),
		pdfStyleVignetteSectionEnd: vignettePDFStyle(pdfVignetteSectionEndSpace, pdfVignetteSectionEndSpace),
		pdfStyleTOCItem: {
			Paragraph:  paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignLeft, Hyphenation: paragraphHyphenationAuto},
			SpaceAfter: pdfTOCSpaceAfter,
			Orphans:    pdfSingleKeepLine,
			Widows:     pdfSingleKeepLine,
		},
		pdfStyleCode: {
			Paragraph: paragraphStyle{FontFamily: "monospace", FontSize: pdfCodeFontSize, LineHeight: pdfCodeLineHeight, LineHeightExplicit: true, Align: textAlignLeft, PreserveSpace: true, Hyphenation: paragraphHyphenationNone},
			Orphans:   pdfSingleKeepLine,
			Widows:    pdfSingleKeepLine,
		},
		pdfStyleTable: {
			Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore:  pdfBaseFontSize,
			SpaceAfter:   pdfBaseFontSize,
			KeepTogether: true,
		},
		pdfStylePoemTitle:       paragraphTitlePDFStyle(pdfPoemTitleSpace, pdfPoemTitleSpace, false),
		pdfStyleStanzaTitle:     paragraphTitlePDFStyle(pdfStanzaTitleSpace, pdfStanzaTitleSpace, false),
		pdfStyleFootnoteTitle:   paragraphTitlePDFStyle(pdfFootnoteTitleSpaceBefore, pdfFootnoteTitleSpaceAfter, true),
		pdfStyleTOCTitle:        headingPDFStyle(1),
		pdfStyleAnnotationTitle: annotationTitlePDFStyle(),
		pdfStyleEmptyLine: {
			Paragraph:   paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Hyphenation: paragraphHyphenationAuto},
			SpaceBefore: pdfBaseFontSize,
			SpaceAfter:  pdfBaseFontSize,
		},
	}
	styles[pdfStyleBodyTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleBodyTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleBodyTitleHeader])
	styles[pdfStyleBodyTitleHeader+"-emptyline"] = titleHeaderEmptyLinePDFStyle(true)
	styles[pdfStyleChapterTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleChapterTitleHeader])
	styles[pdfStyleChapterTitleHeader+"-emptyline"] = titleHeaderEmptyLinePDFStyle(false)
	styles[pdfStyleSectionTitleHeader+"-first"] = titleHeaderFirstVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-next"] = titleHeaderNextVariantPDFStyle(styles[pdfStyleSectionTitleHeader])
	styles[pdfStyleSectionTitleHeader+"-emptyline"] = titleHeaderEmptyLinePDFStyle(false)
	styles[pdfStyleTOCTitle+"-emptyline"] = titleHeaderEmptyLinePDFStyle(false)
	styles[pdfStylePoemTitle+"-first"] = paragraphTitleVariantPDFStyle(true)
	styles[pdfStylePoemTitle+"-next"] = paragraphTitleVariantPDFStyle(true)
	styles[pdfStyleStanzaTitle+"-first"] = paragraphTitleVariantPDFStyle(true)
	styles[pdfStyleStanzaTitle+"-next"] = paragraphTitleVariantPDFStyle(true)
	styles[pdfStyleFootnoteTitle+"-first"] = paragraphTitleVariantPDFStyle(false)
	styles[pdfStyleFootnoteTitle+"-next"] = paragraphTitleVariantPDFStyle(false)
	titleAfterImageStyle := styles[pdfStyleParagraph]
	titleAfterImageStyle.SpaceBefore = pdfTitleAfterImageSpaceBefore
	styles[pdfStyleTitleAfterImage] = titleAfterImageStyle

	linkStyle := styles[pdfStyleParagraph]
	linkStyle.Paragraph.Underline = true
	styles[pdfStyleLinkExternal] = linkStyle
	styles[pdfStyleLinkInternal] = linkStyle
	footnoteLinkStyle := linkStyle
	footnoteLinkStyle.Paragraph.FontSize = pdfFootnoteLinkFontSize / pdfInlineScriptScale
	footnoteLinkStyle.Paragraph.LineHeight = pdfFootnoteLinkLineHeight / pdfInlineScriptScale
	footnoteLinkStyle.Paragraph.VerticalAlign = textVerticalAlignSuper
	styles[pdfStyleLinkFootnote] = footnoteLinkStyle
	styles[pdfStyleLinkTOC] = linkStyle
	backlinkStyle := linkStyle
	backlinkStyle.Paragraph.Bold = true
	backlinkStyle.Paragraph.Color = pdfColor{R: 0.5, G: 0.5, B: 0.5}
	styles[pdfStyleLinkBacklink] = backlinkStyle
	return styles
}

func headingPDFStyle(depth int) pdfBlockResolvedStyle {
	return headingPDFStyleWithLineHeightFactor(pdfHeadingFontSize(depth), pdfHeadingLineHeightFactor, pdfHeadingMarginFactor(depth))
}

func sectionTitleHeaderPDFStyle(depth int) pdfBlockResolvedStyle {
	return headingPDFStyleWithLineHeightFactor(pdfHeadingFontSize(depth), pdfSectionTitleHeaderLineHeightFactor, pdfHeadingMarginFactor(depth))
}

func pdfHeadingFontSize(depth int) float64 {
	if depth <= 1 {
		return pdfHeadingH1FontSize
	}
	return pdfHeadingNestedFontSize
}

func headingPDFStyleWithLineHeightFactor(fontSize float64, lineHeightFactor float64, marginFactor float64) pdfBlockResolvedStyle {
	space := fontSize * marginFactor
	return pdfBlockResolvedStyle{
		Paragraph:         paragraphStyle{FontFamily: "serif", Bold: true, FontSize: fontSize, LineHeight: fontSize * lineHeightFactor, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
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
		r.styles[name] = style
	}
}

func vignettePDFStyle(spaceBefore float64, spaceAfter float64) pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:    paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore:  spaceBefore,
		SpaceAfter:   spaceAfter,
		KeepTogether: true,
	}
}

func titleHeaderEmptyLinePDFStyle(bold bool) pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:   paragraphStyle{FontFamily: "serif", Bold: bold, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore: pdfTitleEmptyLineSpace,
		SpaceAfter:  pdfTitleEmptyLineSpace,
	}
}

func paragraphTitlePDFStyle(spaceBefore float64, spaceAfter float64, bold bool) pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:   paragraphStyle{FontFamily: "serif", Bold: bold, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore: spaceBefore,
		SpaceAfter:  spaceAfter,
	}
}

func annotationTitlePDFStyle() pdfBlockResolvedStyle {
	return pdfBlockResolvedStyle{
		Paragraph:   paragraphStyle{FontFamily: "serif", Bold: true, FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: textAlignCenter, Hyphenation: paragraphHyphenationAuto},
		SpaceBefore: pdfBaseFontSize,
		SpaceAfter:  pdfBaseFontSize,
	}
}

func paragraphTitleVariantPDFStyle(center bool) pdfBlockResolvedStyle {
	align := textAlignJustify
	if center {
		align = textAlignCenter
	}
	return pdfBlockResolvedStyle{
		Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: 0, HasFirstLineIndent: true, Align: align, Hyphenation: paragraphHyphenationAuto},
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
