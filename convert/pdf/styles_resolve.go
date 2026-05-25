package pdf

import "strings"

func (r *pdfStyleResolver) styleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfStyleNameForBlock(block)
	defaultStyle := r.defaultStyle(name)
	inheritedFontSize := r.inheritedFontSizeForBlock(block)
	tagStyle := r.tagStyleForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(defaultStyle)
	style = mergePDFStyleOverridesWithFont(style, r.namedStyle(name), defaultStyle, inheritedFontSize)
	style = r.applyContextInheritedBlockDefaults(style, tagStyle, block)
	classFallback := r.namedStyle(pdfStyleParagraph)
	for _, class := range strings.Fields(block.StyleClasses) {
		classStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		if shouldSkipContextInheritedMargins(block, class) {
			style = mergePDFContextStyleOverrides(style, classStyle, classFallback, inheritedFontSize)
			continue
		}
		if pdfContainerStyleClass(class) {
			style = mergePDFContainerStyleOverrides(style, classStyle, classFallback, inheritedFontSize)
			continue
		}
		style = mergePDFStyleOverridesWithFont(style, classStyle, classFallback, inheritedFontSize)
	}
	for _, selectorStyleName := range pdfElementClassStyleNames(block) {
		selectorStyle, ok := r.styles[selectorStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverridesWithFont(style, selectorStyle, classFallback, inheritedFontSize)
	}
	for _, descStyleName := range r.contextDescendantStyleNames(block) {
		descStyle, ok := r.styles[descStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverridesWithFont(style, descStyle, classFallback, inheritedFontSize)
	}
	if block.Kind == pdfBlockTOCEntry {
		style.Paragraph.FirstLineIndent = max(float64(block.Depth-1)*pdfTOCNestedListIndent, 0)
	}
	return injectPDFImplicitLineHeight(style, defaultStyle)
}

func injectPDFImplicitLineHeight(style, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if style.Paragraph.LineHeightExplicit || fallback.Paragraph.FontSize <= 0 || fallback.Paragraph.LineHeight <= 0 || style.Paragraph.FontSize <= 0 {
		return style
	}
	if style.Paragraph.LineHeight != fallback.Paragraph.LineHeight || style.Paragraph.FontSize == fallback.Paragraph.FontSize {
		return style
	}
	style.Paragraph.LineHeight = fallback.Paragraph.LineHeight * style.Paragraph.FontSize / fallback.Paragraph.FontSize
	return style
}

func mergePDFStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	return mergePDFStyleOverridesWithFont(base, override, fallback, base.Paragraph.FontSize)
}

func mergePDFStyleOverridesWithFont(base, override, fallback pdfBlockResolvedStyle, inheritedFontSize float64) pdfBlockResolvedStyle {
	relativeLengthFontSize := pdfRelativeLengthFontSize(base, override, fallback, inheritedFontSize)
	if override.Paragraph.FontFamily != fallback.Paragraph.FontFamily {
		base.Paragraph.FontFamily = override.Paragraph.FontFamily
	}
	if override.Paragraph.HasBold || override.Paragraph.Bold != fallback.Paragraph.Bold {
		base.Paragraph.Bold = override.Paragraph.Bold
		base.Paragraph.HasBold = override.Paragraph.HasBold
	}
	if override.Paragraph.HasItalic || override.Paragraph.Italic != fallback.Paragraph.Italic {
		base.Paragraph.Italic = override.Paragraph.Italic
		base.Paragraph.HasItalic = override.Paragraph.HasItalic
	}
	if override.Paragraph.FontSizeSpec.Set {
		base.Paragraph.FontSize = pdfResolveCSSFontSizeSpec(override.Paragraph.FontSizeSpec, inheritedFontSize)
		base.Paragraph.FontSizeSpec = override.Paragraph.FontSizeSpec
	} else if override.Paragraph.FontSize != fallback.Paragraph.FontSize {
		base.Paragraph.FontSize = override.Paragraph.FontSize
		base.Paragraph.FontSizeSpec = pdfCSSLengthSpec{}
	}
	lineHeightOverride := override.Paragraph
	if override.Paragraph.FontSizeSpec.Set && !override.Paragraph.LineHeightExplicit {
		if override.Paragraph.LineHeight != fallback.Paragraph.LineHeight && override.Paragraph.FontSize > 0 {
			base.Paragraph.LineHeight = override.Paragraph.LineHeight * base.Paragraph.FontSize / override.Paragraph.FontSize
			lineHeightOverride.LineHeight = fallback.Paragraph.LineHeight
		} else {
			lineHeightOverride.LineHeight = fallback.Paragraph.LineHeight
		}
	}
	base.Paragraph = mergePDFLineHeightOverride(base.Paragraph, lineHeightOverride, fallback.Paragraph)
	if override.Paragraph.LineHeightSpec.Set {
		base.Paragraph.LineHeight = pdfResolveCSSLineHeightSpec(override.Paragraph.LineHeightSpec, base.Paragraph.FontSize)
		base.Paragraph.LineHeightSpec = override.Paragraph.LineHeightSpec
		base.Paragraph.LineHeightExplicit = true
	}
	if override.Paragraph.LetterSpacingSpec.Set {
		base.Paragraph.LetterSpacing = pdfResolveCSSLengthSpec(override.Paragraph.LetterSpacingSpec, base.Paragraph.FontSize)
		base.Paragraph.LetterSpacingSpec = override.Paragraph.LetterSpacingSpec
	} else if override.Paragraph.LetterSpacing != fallback.Paragraph.LetterSpacing {
		base.Paragraph.LetterSpacing = override.Paragraph.LetterSpacing
		base.Paragraph.LetterSpacingSpec = pdfCSSLengthSpec{}
	}
	if override.Paragraph.FirstLineIndentSpec.Set {
		base.Paragraph.FirstLineIndent = pdfResolveCSSLengthSpec(override.Paragraph.FirstLineIndentSpec, base.Paragraph.FontSize)
		base.Paragraph.FirstLineIndentSpec = override.Paragraph.FirstLineIndentSpec
		base.Paragraph.HasFirstLineIndent = override.Paragraph.HasFirstLineIndent
	} else if override.Paragraph.HasFirstLineIndent || override.Paragraph.FirstLineIndent != fallback.Paragraph.FirstLineIndent {
		base.Paragraph.FirstLineIndent = override.Paragraph.FirstLineIndent
		base.Paragraph.FirstLineIndentSpec = pdfCSSLengthSpec{}
		base.Paragraph.HasFirstLineIndent = override.Paragraph.HasFirstLineIndent
	}
	if override.Paragraph.HasAlign || override.Paragraph.Align != fallback.Paragraph.Align {
		base.Paragraph.Align = override.Paragraph.Align
		base.Paragraph.HasAlign = override.Paragraph.HasAlign
	}
	if override.Paragraph.HasVerticalAlign || override.Paragraph.VerticalAlign != fallback.Paragraph.VerticalAlign {
		base.Paragraph.VerticalAlign = override.Paragraph.VerticalAlign
		base.Paragraph.HasVerticalAlign = override.Paragraph.HasVerticalAlign
	}
	if override.Paragraph.Color != fallback.Paragraph.Color {
		base.Paragraph.Color = override.Paragraph.Color
	}
	if override.Paragraph.HasUnderline || override.Paragraph.Underline != fallback.Paragraph.Underline {
		base.Paragraph.Underline = override.Paragraph.Underline
		base.Paragraph.HasUnderline = override.Paragraph.HasUnderline
	}
	if override.Paragraph.HasStrikethrough || override.Paragraph.Strikethrough != fallback.Paragraph.Strikethrough {
		base.Paragraph.Strikethrough = override.Paragraph.Strikethrough
		base.Paragraph.HasStrikethrough = override.Paragraph.HasStrikethrough
	}
	if override.Paragraph.HasPreserveSpace || override.Paragraph.PreserveSpace != fallback.Paragraph.PreserveSpace {
		base.Paragraph.PreserveSpace = override.Paragraph.PreserveSpace
		base.Paragraph.HasPreserveSpace = override.Paragraph.HasPreserveSpace
	}
	if override.Paragraph.HasNoWrap || override.Paragraph.NoWrap != fallback.Paragraph.NoWrap {
		base.Paragraph.NoWrap = override.Paragraph.NoWrap
		base.Paragraph.HasNoWrap = override.Paragraph.HasNoWrap
	}
	if override.Paragraph.HasHyphenation || override.Paragraph.Hyphenation != fallback.Paragraph.Hyphenation {
		base.Paragraph.Hyphenation = override.Paragraph.Hyphenation
		base.Paragraph.HasHyphenation = override.Paragraph.HasHyphenation
	}
	if override.SpaceBeforeSpec.Set {
		base.SpaceBefore = pdfResolveCSSLengthSpec(override.SpaceBeforeSpec, relativeLengthFontSize)
		base.SpaceBeforeSpec = override.SpaceBeforeSpec
	} else if override.HasSpaceBefore || override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
		base.SpaceBeforeSpec = pdfCSSLengthSpec{}
	}
	if override.SpaceAfterSpec.Set {
		base.SpaceAfter = pdfResolveCSSLengthSpec(override.SpaceAfterSpec, relativeLengthFontSize)
		base.SpaceAfterSpec = override.SpaceAfterSpec
	} else if override.HasSpaceAfter || override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
		base.SpaceAfterSpec = pdfCSSLengthSpec{}
	}
	if override.MarginLeftSpec.Set {
		base.MarginLeft = pdfResolveCSSLengthSpec(override.MarginLeftSpec, relativeLengthFontSize)
		base.MarginLeftSpec = override.MarginLeftSpec
	} else if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
		base.MarginLeftSpec = pdfCSSLengthSpec{}
	}
	if override.MarginRightSpec.Set {
		base.MarginRight = pdfResolveCSSLengthSpec(override.MarginRightSpec, relativeLengthFontSize)
		base.MarginRightSpec = override.MarginRightSpec
	} else if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
		base.MarginRightSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingTopSpec.Set {
		base.PaddingTop = pdfResolveCSSLengthSpec(override.PaddingTopSpec, relativeLengthFontSize)
		base.PaddingTopSpec = override.PaddingTopSpec
	} else if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
		base.PaddingTopSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingRightSpec.Set {
		base.PaddingRight = pdfResolveCSSLengthSpec(override.PaddingRightSpec, relativeLengthFontSize)
		base.PaddingRightSpec = override.PaddingRightSpec
	} else if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
		base.PaddingRightSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingBottomSpec.Set {
		base.PaddingBottom = pdfResolveCSSLengthSpec(override.PaddingBottomSpec, relativeLengthFontSize)
		base.PaddingBottomSpec = override.PaddingBottomSpec
	} else if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
		base.PaddingBottomSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingLeftSpec.Set {
		base.PaddingLeft = pdfResolveCSSLengthSpec(override.PaddingLeftSpec, relativeLengthFontSize)
		base.PaddingLeftSpec = override.PaddingLeftSpec
	} else if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
		base.PaddingLeftSpec = pdfCSSLengthSpec{}
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
	}
	if override.HasKeepTogether || override.KeepTogether != fallback.KeepTogether {
		base.KeepTogether = override.KeepTogether
		base.HasKeepTogether = override.HasKeepTogether
	}
	if override.KeepWithNextLines != fallback.KeepWithNextLines {
		base.KeepWithNextLines = override.KeepWithNextLines
	}
	if override.HasPageBreakBefore || override.PageBreakBefore != fallback.PageBreakBefore || override.PageBreakBeforeMode != fallback.PageBreakBeforeMode {
		base.PageBreakBefore = override.PageBreakBefore
		base.PageBreakBeforeMode = override.PageBreakBeforeMode
		base.HasPageBreakBefore = override.HasPageBreakBefore
	}
	if override.HasPageBreakAfter || override.PageBreakAfter != fallback.PageBreakAfter || override.PageBreakAfterMode != fallback.PageBreakAfterMode {
		base.PageBreakAfter = override.PageBreakAfter
		base.PageBreakAfterMode = override.PageBreakAfterMode
		base.HasPageBreakAfter = override.HasPageBreakAfter
	}
	if override.HasHidden || override.Hidden != fallback.Hidden {
		base.Hidden = override.Hidden
		base.HasHidden = override.HasHidden
	}
	if override.Orphans != fallback.Orphans {
		base.Orphans = override.Orphans
	}
	if override.Widows != fallback.Widows {
		base.Widows = override.Widows
	}
	return base
}

func (r *pdfStyleResolver) namedStyle(name string) pdfBlockResolvedStyle {
	if r != nil {
		if style, ok := r.styles[name]; ok {
			return style
		}
	}
	if r != nil {
		if style, ok := r.styles[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) defaultStyle(name string) pdfBlockResolvedStyle {
	if r != nil && r.defaults != nil {
		if style, ok := r.defaults[name]; ok {
			return style
		}
		if style, ok := r.defaults[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) rootParagraphStyle() paragraphStyle {
	base := r.defaultStyle(pdfStyleHTML).Paragraph
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleHTML).Paragraph, r.defaultStyle(pdfStyleHTML).Paragraph)
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleBody).Paragraph, rootDefault)
	return normalizePDFDefaultCSSRootRhythm(base)
}

func normalizePDFDefaultCSSRootRhythm(style paragraphStyle) paragraphStyle {
	// convert/default.css uses body { font-size: 80%; line-height: 150%; } as a
	// Kindle-oriented reader setting. KP3/KFX keeps ordinary body text at 1rem/1lh
	// in that case, so native PDF should keep its intrinsic KP3-like base rhythm
	// instead of shrinking fixed-layout text to 6.72pt/10.08pt. Other explicit
	// root/body sizes remain honored, and users can still target PDF with
	// @media fbc-pdf or element-level rules.
	if !pdfCSSSpecPercent(style.FontSizeSpec, 80) || !pdfCSSSpecPercent(style.LineHeightSpec, 150) {
		return style
	}
	style.FontSize = pdfBaseFontSize
	style.FontSizeSpec = pdfCSSLengthSpec{}
	style.LineHeight = pdfBaseLineHeight
	style.LineHeightSpec = pdfCSSLengthSpec{}
	style.LineHeightExplicit = false
	return style
}

func pdfCSSSpecPercent(spec pdfCSSLengthSpec, value float64) bool {
	if !spec.Set || spec.Unit != "%" {
		return false
	}
	diff := spec.Value - value
	return diff > -0.001 && diff < 0.001
}

func (r *pdfStyleResolver) tagStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfTagStyleNameForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(r.defaultStyle(name))
	return mergePDFStyleOverridesWithFont(style, r.namedStyle(name), r.defaultStyle(name), r.inheritedFontSizeForBlock(block))
}

func (r *pdfStyleResolver) inheritedFontSizeForBlock(block pdfTextBlock) float64 {
	fontSize := r.rootParagraphStyle().FontSize
	fallback := r.defaultStyle(pdfStyleParagraph).Paragraph
	for _, class := range strings.Fields(block.ContextClasses) {
		if class == pdfStyleNameForBlock(block) {
			continue
		}
		contextStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		if contextStyle.Paragraph.FontSizeSpec.Set {
			fontSize = pdfResolveCSSFontSizeSpec(contextStyle.Paragraph.FontSizeSpec, fontSize)
			continue
		}
		if contextStyle.Paragraph.FontSize != fallback.FontSize {
			fontSize = contextStyle.Paragraph.FontSize
		}
	}
	if fontSize <= 0 {
		return pdfBaseFontSize
	}
	return fontSize
}

func (r *pdfStyleResolver) contextInheritedBlockStyle(tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	style := tagStyle
	inheritedFontSize := r.inheritedFontSizeForBlock(block)
	var (
		left     float64
		right    float64
		hasLeft  bool
		hasRight bool
	)
	for _, class := range strings.Fields(block.ContextClasses) {
		if class == pdfStyleNameForBlock(block) {
			continue
		}
		contextStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		fallback := r.namedStyle(pdfStyleParagraph)
		style.Paragraph = mergePDFInheritedParagraphStyle(style.Paragraph, contextStyle.Paragraph, fallback.Paragraph)
		marginFallback := fallback
		relativeLengthFontSize := pdfContextRelativeLengthFontSize(contextStyle, marginFallback, inheritedFontSize)
		if contextStyle.MarginLeftSpec.Set {
			left += pdfResolveCSSLengthSpec(contextStyle.MarginLeftSpec, relativeLengthFontSize)
			hasLeft = true
		} else if contextStyle.MarginLeft != marginFallback.MarginLeft {
			left += contextStyle.MarginLeft
			hasLeft = true
		}
		if contextStyle.MarginRightSpec.Set {
			right += pdfResolveCSSLengthSpec(contextStyle.MarginRightSpec, relativeLengthFontSize)
			hasRight = true
		} else if contextStyle.MarginRight != marginFallback.MarginRight {
			right += contextStyle.MarginRight
			hasRight = true
		}
	}
	if hasLeft {
		style.MarginLeft = left
	}
	if hasRight {
		style.MarginRight = right
	}
	return style
}

func (r *pdfStyleResolver) applyContextInheritedBlockDefaults(style, tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	contextStyle := r.contextInheritedBlockStyle(tagStyle, block)
	if style.Paragraph.FontFamily == tagStyle.Paragraph.FontFamily {
		style.Paragraph.FontFamily = contextStyle.Paragraph.FontFamily
	}
	if style.Paragraph.Bold == tagStyle.Paragraph.Bold {
		style.Paragraph.Bold = contextStyle.Paragraph.Bold
		style.Paragraph.HasBold = contextStyle.Paragraph.HasBold
	}
	if style.Paragraph.Italic == tagStyle.Paragraph.Italic {
		style.Paragraph.Italic = contextStyle.Paragraph.Italic
		style.Paragraph.HasItalic = contextStyle.Paragraph.HasItalic
	}
	if style.Paragraph.FontSize == tagStyle.Paragraph.FontSize {
		style.Paragraph.FontSize = contextStyle.Paragraph.FontSize
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == tagStyle.Paragraph.LineHeight {
		style.Paragraph.LineHeight = contextStyle.Paragraph.LineHeight
		style.Paragraph.LineHeightExplicit = contextStyle.Paragraph.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == tagStyle.Paragraph.LetterSpacing {
		style.Paragraph.LetterSpacing = contextStyle.Paragraph.LetterSpacing
	}
	if style.Paragraph.FirstLineIndent == tagStyle.Paragraph.FirstLineIndent {
		style.Paragraph.FirstLineIndent = contextStyle.Paragraph.FirstLineIndent
		style.Paragraph.HasFirstLineIndent = contextStyle.Paragraph.HasFirstLineIndent
	}
	if style.Paragraph.Align == tagStyle.Paragraph.Align {
		style.Paragraph.Align = contextStyle.Paragraph.Align
		style.Paragraph.HasAlign = contextStyle.Paragraph.HasAlign
	}
	if style.Paragraph.Color == tagStyle.Paragraph.Color {
		style.Paragraph.Color = contextStyle.Paragraph.Color
	}
	if style.Paragraph.PreserveSpace == tagStyle.Paragraph.PreserveSpace {
		style.Paragraph.PreserveSpace = contextStyle.Paragraph.PreserveSpace
		style.Paragraph.HasPreserveSpace = contextStyle.Paragraph.HasPreserveSpace
	}
	if style.Paragraph.NoWrap == tagStyle.Paragraph.NoWrap {
		style.Paragraph.NoWrap = contextStyle.Paragraph.NoWrap
		style.Paragraph.HasNoWrap = contextStyle.Paragraph.HasNoWrap
	}
	if style.Paragraph.Hyphenation == tagStyle.Paragraph.Hyphenation {
		style.Paragraph.Hyphenation = contextStyle.Paragraph.Hyphenation
		style.Paragraph.HasHyphenation = contextStyle.Paragraph.HasHyphenation
	}
	if style.MarginLeft == tagStyle.MarginLeft {
		style.MarginLeft = contextStyle.MarginLeft
	} else if contextStyle.MarginLeft != tagStyle.MarginLeft {
		style.MarginLeft += contextStyle.MarginLeft
	}
	if style.MarginRight == tagStyle.MarginRight {
		style.MarginRight = contextStyle.MarginRight
	} else if contextStyle.MarginRight != tagStyle.MarginRight {
		style.MarginRight += contextStyle.MarginRight
	}
	return style
}

func shouldSkipContextInheritedMargins(block pdfTextBlock, class string) bool {
	if class == "" || !pdfContainerStyleClass(class) {
		return false
	}
	for _, contextClass := range strings.Fields(block.ContextClasses) {
		if contextClass == class {
			return true
		}
	}
	return false
}

func pdfContainerStyleClass(class string) bool {
	switch class {
	case pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleSectionTitle, pdfStyleAnnotation, pdfStyleFootnote, pdfStylePoem, pdfStyleStanza, pdfStyleEpigraph, pdfStyleCite:
		return true
	default:
		return false
	}
}

func mergePDFContainerStyleOverrides(base, override, fallback pdfBlockResolvedStyle, inheritedFontSize float64) pdfBlockResolvedStyle {
	base = mergePDFContextStyleOverrides(base, override, fallback, inheritedFontSize)
	relativeLengthFontSize := pdfContextRelativeLengthFontSize(override, fallback, inheritedFontSize)
	if override.MarginLeftSpec.Set {
		base.MarginLeft = pdfResolveCSSLengthSpec(override.MarginLeftSpec, relativeLengthFontSize)
		base.MarginLeftSpec = override.MarginLeftSpec
	} else if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
		base.MarginLeftSpec = pdfCSSLengthSpec{}
	}
	if override.MarginRightSpec.Set {
		base.MarginRight = pdfResolveCSSLengthSpec(override.MarginRightSpec, relativeLengthFontSize)
		base.MarginRightSpec = override.MarginRightSpec
	} else if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
		base.MarginRightSpec = pdfCSSLengthSpec{}
	}
	return base
}

func mergePDFContextStyleOverrides(base, override, fallback pdfBlockResolvedStyle, inheritedFontSize float64) pdfBlockResolvedStyle {
	relativeLengthFontSize := pdfContextRelativeLengthFontSize(override, fallback, inheritedFontSize)
	if override.SpaceBeforeSpec.Set {
		spaceBefore := pdfResolveCSSLengthSpec(override.SpaceBeforeSpec, relativeLengthFontSize)
		if spaceBefore != 0 {
			base.SpaceBefore = spaceBefore
			base.SpaceBeforeSpec = override.SpaceBeforeSpec
		}
	} else if (override.HasSpaceBefore && override.SpaceBefore != 0) || (!override.HasSpaceBefore && override.SpaceBefore != fallback.SpaceBefore) {
		base.SpaceBefore = override.SpaceBefore
		base.SpaceBeforeSpec = pdfCSSLengthSpec{}
	}
	if override.SpaceAfterSpec.Set {
		spaceAfter := pdfResolveCSSLengthSpec(override.SpaceAfterSpec, relativeLengthFontSize)
		if spaceAfter != 0 {
			base.SpaceAfter = spaceAfter
			base.SpaceAfterSpec = override.SpaceAfterSpec
		}
	} else if (override.HasSpaceAfter && override.SpaceAfter != 0) || (!override.HasSpaceAfter && override.SpaceAfter != fallback.SpaceAfter) {
		base.SpaceAfter = override.SpaceAfter
		base.SpaceAfterSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingTopSpec.Set {
		base.PaddingTop = pdfResolveCSSLengthSpec(override.PaddingTopSpec, relativeLengthFontSize)
		base.PaddingTopSpec = override.PaddingTopSpec
	} else if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
		base.PaddingTopSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingRightSpec.Set {
		base.PaddingRight = pdfResolveCSSLengthSpec(override.PaddingRightSpec, relativeLengthFontSize)
		base.PaddingRightSpec = override.PaddingRightSpec
	} else if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
		base.PaddingRightSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingBottomSpec.Set {
		base.PaddingBottom = pdfResolveCSSLengthSpec(override.PaddingBottomSpec, relativeLengthFontSize)
		base.PaddingBottomSpec = override.PaddingBottomSpec
	} else if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
		base.PaddingBottomSpec = pdfCSSLengthSpec{}
	}
	if override.PaddingLeftSpec.Set {
		base.PaddingLeft = pdfResolveCSSLengthSpec(override.PaddingLeftSpec, relativeLengthFontSize)
		base.PaddingLeftSpec = override.PaddingLeftSpec
	} else if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
		base.PaddingLeftSpec = pdfCSSLengthSpec{}
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
	}
	if override.HasKeepTogether || override.KeepTogether != fallback.KeepTogether {
		base.KeepTogether = override.KeepTogether
		base.HasKeepTogether = override.HasKeepTogether
	}
	if override.KeepWithNextLines != fallback.KeepWithNextLines {
		base.KeepWithNextLines = override.KeepWithNextLines
	}
	if override.HasPageBreakBefore || override.PageBreakBefore != fallback.PageBreakBefore || override.PageBreakBeforeMode != fallback.PageBreakBeforeMode {
		base.PageBreakBefore = override.PageBreakBefore
		base.PageBreakBeforeMode = override.PageBreakBeforeMode
		base.HasPageBreakBefore = override.HasPageBreakBefore
	}
	if override.HasPageBreakAfter || override.PageBreakAfter != fallback.PageBreakAfter || override.PageBreakAfterMode != fallback.PageBreakAfterMode {
		base.PageBreakAfter = override.PageBreakAfter
		base.PageBreakAfterMode = override.PageBreakAfterMode
		base.HasPageBreakAfter = override.HasPageBreakAfter
	}
	if override.HasHidden || override.Hidden != fallback.Hidden {
		base.Hidden = override.Hidden
		base.HasHidden = override.HasHidden
	}
	if override.Orphans != fallback.Orphans {
		base.Orphans = override.Orphans
	}
	if override.Widows != fallback.Widows {
		base.Widows = override.Widows
	}
	return base
}

func pdfRelativeLengthFontSize(base, override, fallback pdfBlockResolvedStyle, inheritedFontSize float64) float64 {
	if override.Paragraph.FontSizeSpec.Set {
		return pdfResolveCSSFontSizeSpec(override.Paragraph.FontSizeSpec, inheritedFontSize)
	}
	if override.Paragraph.FontSize != fallback.Paragraph.FontSize {
		return override.Paragraph.FontSize
	}
	if base.Paragraph.FontSize > 0 {
		return base.Paragraph.FontSize
	}
	if inheritedFontSize > 0 {
		return inheritedFontSize
	}
	return pdfBaseFontSize
}

func pdfContextRelativeLengthFontSize(override, fallback pdfBlockResolvedStyle, inheritedFontSize float64) float64 {
	if override.Paragraph.FontSizeSpec.Set {
		return pdfResolveCSSFontSizeSpec(override.Paragraph.FontSizeSpec, inheritedFontSize)
	}
	if override.Paragraph.FontSize != fallback.Paragraph.FontSize && override.Paragraph.FontSize != pdfBaseFontSize {
		return override.Paragraph.FontSize
	}
	if inheritedFontSize > 0 {
		return inheritedFontSize
	}
	return pdfBaseFontSize
}

func pdfResolveCSSFontSizeSpec(spec pdfCSSLengthSpec, inheritedFontSize float64) float64 {
	if inheritedFontSize <= 0 {
		inheritedFontSize = pdfBaseFontSize
	}
	return pdfResolveCSSLengthSpec(spec, inheritedFontSize)
}

func pdfResolveCSSLineHeightSpec(spec pdfCSSLengthSpec, fontSize float64) float64 {
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	if spec.Keyword == "normal" {
		return fontSize * pdfNormalLineHeightFactor
	}
	if spec.Unit == "number" {
		return fontSize * spec.Value
	}
	return pdfResolveCSSLengthSpec(spec, fontSize)
}

func pdfResolveCSSLengthSpec(spec pdfCSSLengthSpec, fontSize float64) float64 {
	if !spec.Set {
		return 0
	}
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	switch spec.Unit {
	case "em":
		return spec.Value * fontSize
	case "rem":
		return spec.Value * pdfBaseFontSize
	case "%":
		return spec.Value * fontSize / 100
	default:
		return spec.Value
	}
}

func (r *pdfStyleResolver) applyRootInheritedParagraphDefaults(style pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	root := r.rootParagraphStyle()
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	if style.Paragraph.FontFamily == rootDefault.FontFamily {
		style.Paragraph.FontFamily = root.FontFamily
	}
	if !style.Paragraph.HasBold && style.Paragraph.Bold == rootDefault.Bold {
		style.Paragraph.Bold = root.Bold
		style.Paragraph.HasBold = root.HasBold
	}
	if !style.Paragraph.HasItalic && style.Paragraph.Italic == rootDefault.Italic {
		style.Paragraph.Italic = root.Italic
		style.Paragraph.HasItalic = root.HasItalic
	}
	if style.Paragraph.FontSize == rootDefault.FontSize {
		style.Paragraph.FontSize = root.FontSize
		style.Paragraph.FontSizeSpec = root.FontSizeSpec
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == rootDefault.LineHeight {
		style.Paragraph.LineHeight = root.LineHeight
		style.Paragraph.LineHeightSpec = root.LineHeightSpec
		style.Paragraph.LineHeightExplicit = root.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == rootDefault.LetterSpacing {
		style.Paragraph.LetterSpacing = root.LetterSpacing
		style.Paragraph.LetterSpacingSpec = root.LetterSpacingSpec
	}
	if !style.Paragraph.HasFirstLineIndent && style.Paragraph.FirstLineIndent == rootDefault.FirstLineIndent {
		style.Paragraph.FirstLineIndent = root.FirstLineIndent
		style.Paragraph.FirstLineIndentSpec = root.FirstLineIndentSpec
		style.Paragraph.HasFirstLineIndent = root.HasFirstLineIndent
	}
	if !style.Paragraph.HasAlign && style.Paragraph.Align == rootDefault.Align {
		style.Paragraph.Align = root.Align
		style.Paragraph.HasAlign = root.HasAlign
	}
	if style.Paragraph.Color == rootDefault.Color {
		style.Paragraph.Color = root.Color
	}
	if !style.Paragraph.HasPreserveSpace && style.Paragraph.PreserveSpace == rootDefault.PreserveSpace {
		style.Paragraph.PreserveSpace = root.PreserveSpace
		style.Paragraph.HasPreserveSpace = root.HasPreserveSpace
	}
	if !style.Paragraph.HasNoWrap && style.Paragraph.NoWrap == rootDefault.NoWrap {
		style.Paragraph.NoWrap = root.NoWrap
		style.Paragraph.HasNoWrap = root.HasNoWrap
	}
	if !style.Paragraph.HasHyphenation && style.Paragraph.Hyphenation == rootDefault.Hyphenation {
		style.Paragraph.Hyphenation = root.Hyphenation
		style.Paragraph.HasHyphenation = root.HasHyphenation
	}
	return style
}

func mergePDFInheritedParagraphStyle(base, override, fallback paragraphStyle) paragraphStyle {
	if override.FontFamily != fallback.FontFamily {
		base.FontFamily = override.FontFamily
	}
	if override.HasBold {
		base.Bold = override.Bold
		base.HasBold = true
	}
	if override.HasItalic {
		base.Italic = override.Italic
		base.HasItalic = true
	}
	if override.FontSizeSpec.Set {
		base.FontSize = pdfResolveCSSFontSizeSpec(override.FontSizeSpec, base.FontSize)
		base.FontSizeSpec = override.FontSizeSpec
	} else if override.FontSize != fallback.FontSize {
		base.FontSize = override.FontSize
		base.FontSizeSpec = pdfCSSLengthSpec{}
	}
	base = mergePDFLineHeightOverride(base, override, fallback)
	if override.LineHeightSpec.Set {
		base.LineHeight = pdfResolveCSSLineHeightSpec(override.LineHeightSpec, base.FontSize)
		base.LineHeightSpec = override.LineHeightSpec
		base.LineHeightExplicit = true
	}
	if override.LetterSpacingSpec.Set {
		base.LetterSpacing = pdfResolveCSSLengthSpec(override.LetterSpacingSpec, base.FontSize)
		base.LetterSpacingSpec = override.LetterSpacingSpec
	} else if override.LetterSpacing != fallback.LetterSpacing {
		base.LetterSpacing = override.LetterSpacing
		base.LetterSpacingSpec = pdfCSSLengthSpec{}
	}
	if override.FirstLineIndentSpec.Set {
		base.FirstLineIndent = pdfResolveCSSLengthSpec(override.FirstLineIndentSpec, base.FontSize)
		base.FirstLineIndentSpec = override.FirstLineIndentSpec
		base.HasFirstLineIndent = true
	} else if override.HasFirstLineIndent {
		base.FirstLineIndent = override.FirstLineIndent
		base.FirstLineIndentSpec = pdfCSSLengthSpec{}
		base.HasFirstLineIndent = true
	}
	if override.HasAlign {
		base.Align = override.Align
		base.HasAlign = true
	}
	if override.Color != fallback.Color {
		base.Color = override.Color
	}
	if override.HasPreserveSpace {
		base.PreserveSpace = override.PreserveSpace
		base.HasPreserveSpace = true
	}
	if override.HasNoWrap {
		base.NoWrap = override.NoWrap
		base.HasNoWrap = true
	}
	if override.HasHyphenation {
		base.Hyphenation = override.Hyphenation
		base.HasHyphenation = true
	}
	return base
}

func mergePDFLineHeightOverride(base, override, fallback paragraphStyle) paragraphStyle {
	if override.LineHeightExplicit {
		base.LineHeight = override.LineHeight
		base.LineHeightSpec = override.LineHeightSpec
		base.LineHeightExplicit = true
		return base
	}
	if !base.LineHeightExplicit && override.LineHeight != fallback.LineHeight {
		base.LineHeight = override.LineHeight
		base.LineHeightSpec = override.LineHeightSpec
	}
	return base
}
